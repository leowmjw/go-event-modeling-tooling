"""Fulfillment context — extracted implementations.

Source: testdata/fixtures/bounded-context-order-fulfillment.evml, frames 11-15.
rf 11 Billing.PaymentAuthorized is the pipeline input. The allocate/defer
branch depends on warehouse inventory, which the event does not carry, so
`inventory_available` is injected (see compile-report judgment calls).
"""

from __future__ import annotations


def translate_fulfillment(event: dict, warehouse_id: str = "wh-1") -> dict:
    """Map `rf evt Billing.PaymentAuthorized` (tf 11) to AllocateShipmentCommand (tf 13).

    Input:  Billing.PaymentAuthorized {orderId, amount}
    Output: AllocateShipmentCommand {orderId, warehouseId}

    warehouseId is not carried by the event; it is the fulfilment router's
    chosen warehouse (parameterised for the real lookup at runtime).
    """
    return {"orderId": event["orderId"], "warehouseId": warehouse_id}


def decide_allocate_shipment(
    command: dict,
    inventory_available: bool = True,
    shipment_id: str = "sh-55",
) -> dict:
    """Decide the result of `cmd AllocateShipment` (tf 13).

    Input:  AllocateShipmentCommand {orderId, warehouseId}
    Output: ShipmentAllocated {orderId, shipmentId, warehouseId}
            or ShipmentAllocationDeferred {orderId, reason}

    GWT cases (tf 13):
      - "allocate shipment after payment is authorized" -> ShipmentAllocated
      - "defer shipment when inventory is not available" -> deferred

    Inventory state is external to the command payload; `inventory_available`
    stands in for the stock check. `shipment_id` is assigned by the shipment
    service and injected so the function stays pure.
    """
    if not inventory_available:
        return {
            "event": "ShipmentAllocationDeferred",
            "orderId": command["orderId"],
            "reason": "inventory_unavailable",
        }
    return {
        "event": "ShipmentAllocated",
        "orderId": command["orderId"],
        "shipmentId": shipment_id,
        "warehouseId": command["warehouseId"],
    }


def retry_allocation(replenished: dict, deferred: dict | None = None) -> dict:
    """`pcr AllocationRetrier` (tf 26) — re-issue AllocateShipmentCommand.

    Sources: rf 25 Warehouse.InventoryReplenished and tf 23
    ShipmentAllocationDeferred (the deferred allocation carries the orderId).
    Output: AllocateShipmentCommand {orderId, warehouseId}
    """
    return {
        "orderId": (deferred or {})["orderId"],
        "warehouseId": replenished["warehouseId"],
    }


def project_shipment_queue(event: dict) -> dict:
    """Fold tf 14 evt into `rmo ShipmentQueue` (tf 15).

    Expected shape is the ShipmentQueue15 data block:
      {shipmentId, orderId, warehouseId, status: "queued_for_picking"}
    """
    if event.get("event") == "ShipmentAllocationDeferred":
        return {
            "shipmentId": None,
            "orderId": event["orderId"],
            "warehouseId": None,
            "status": "deferred:" + event.get("reason", ""),
        }
    return {
        "shipmentId": event["shipmentId"],
        "orderId": event["orderId"],
        "warehouseId": event["warehouseId"],
        "status": "queued_for_picking",
    }
