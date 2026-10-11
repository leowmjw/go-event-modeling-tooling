"""Fulfillment context — extracted implementations.

Source: testdata/fixtures/bounded-context-order-fulfillment.evml, frames 11-15,
23, 25, 26, 28, 29. Allocation outcome belongs to warehouse inventory (note 13),
so it is an external_call whose result feeds the pure decision.
"""

from __future__ import annotations

from typing import Callable

# Deployment configuration, not pipeline data: the worker registers the real
# warehouse inventory client with configure_warehouse() at startup.
_warehouse: Callable[[dict, str], dict] | None = None


def configure_warehouse(client: Callable[[dict, str], dict] | None) -> None:
    global _warehouse
    _warehouse = client


def _items(items: list) -> list:
    return [{"sku": item["sku"], "qty": item["qty"]} for item in items]


def project_paid_orders_to_ship(order_placed: dict, payment_authorized: dict | None = None) -> dict:
    """Fold rf 28 Sales.OrderPlaced + rf 11 Billing.PaymentAuthorized into
    rmo PaidOrdersToShip (tf 29, data PaidOrdersToShip29, gwt 29)."""
    paid = payment_authorized is not None and payment_authorized["orderId"] == order_placed["orderId"]
    return {
        "orderId": order_placed["orderId"],
        "items": _items(order_placed["items"]),
        "status": "ready_to_ship" if paid else "awaiting_payment",
    }


def translate_fulfillment(view: dict) -> dict:
    """pcr FulfillmentTranslator (tf 12): AllocateShipmentCommand for a ready order."""
    if view["status"] != "ready_to_ship":
        raise ValueError(f"order {view['orderId']} is not ready to ship")
    return {"orderId": view["orderId"], "items": _items(view["items"])}


def allocate_shipment_with_warehouse(command: dict, *, attempt: int = 1) -> dict:
    """external_call: ask warehouse inventory to allocate the items.

    Calls the configured client as `client(command, idempotency_key)`. The key
    is `<orderId>/allocation-<attempt>` (note 13): retries of one attempt reuse
    it, a replenishment-triggered re-issue (the runtime's feedback loop passes
    the next attempt number) gets a fresh one.
    """
    if _warehouse is None:
        raise RuntimeError("warehouse inventory client is not configured")
    result = _warehouse(command, f"{command['orderId']}/allocation-{attempt}")
    if result.get("allocated") is True:
        if not result.get("shipmentId") or not result.get("warehouseId"):
            raise ValueError("allocated warehouse result needs shipmentId and warehouseId")
        return {"allocated": True, "shipmentId": result["shipmentId"], "warehouseId": result["warehouseId"]}
    if result.get("allocated") is False:
        if not result.get("missingSkus"):
            raise ValueError("unavailable warehouse result needs missingSkus")
        return {"allocated": False, "missingSkus": list(result["missingSkus"])}
    raise ValueError("warehouse result has no allocated flag")


def decide_allocate_shipment(command: dict, allocation: dict) -> dict:
    """Decide cmd AllocateShipment (tf 13) from the warehouse result.

    gwt 13 "allocate shipment when inventory is available" -> ShipmentAllocated
    gwt 13 "defer shipment when inventory is not available" -> ShipmentAllocationDeferred
    """
    if allocation["allocated"]:
        return {
            "event": "ShipmentAllocated",
            "allocated": {
                "orderId": command["orderId"],
                "shipmentId": allocation["shipmentId"],
                "warehouseId": allocation["warehouseId"],
            },
        }
    return {
        "event": "ShipmentAllocationDeferred",
        "deferred": {
            "orderId": command["orderId"],
            "reason": "inventory_unavailable",
            "missingSkus": list(allocation["missingSkus"]),
        },
    }


def project_shipment_queue(event: dict) -> dict:
    """Fold the allocation result into rmo ShipmentQueue (tf 15, data ShipmentQueue15, gwt 15)."""
    if event["event"] == "ShipmentAllocated":
        allocated = event["allocated"]
        return {
            "shipmentId": allocated["shipmentId"],
            "orderId": allocated["orderId"],
            "warehouseId": allocated["warehouseId"],
            "status": "queued_for_picking",
        }
    deferred = event["deferred"]
    return {"orderId": deferred["orderId"], "status": "awaiting_inventory", "missingSkus": list(deferred["missingSkus"])}


def retry_allocation(replenished: dict, deferred: dict, view: dict) -> dict | None:
    """pcr AllocationRetrier (tf 26, note 26).

    gwt 26 "retry allocation when a missing sku is replenished" -> AllocateShipment
    gwt 26 "keep waiting when an unrelated sku is replenished" -> no command
    """
    if deferred["event"] != "ShipmentAllocationDeferred":
        return None
    if replenished["sku"] not in deferred["deferred"]["missingSkus"]:
        return None
    return {"orderId": view["orderId"], "items": _items(view["items"])}
