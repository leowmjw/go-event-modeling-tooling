"""Golden tests for the fulfillment context — one per gwt on tf 13."""

from extracted.fulfillment import (
    decide_allocate_shipment,
    project_shipment_queue,
    retry_allocation,
    translate_fulfillment,
)

EVENT = {"orderId": "ord-100", "amount": 125.00}
COMMAND = {"orderId": "ord-100", "warehouseId": "wh-1"}


def test_translate_billing_payment_authorized():
    assert translate_fulfillment(EVENT) == COMMAND


def test_allocate_shipment_after_payment_is_authorized():
    given = [{"event": "Billing.PaymentAuthorized", "orderId": "ord-100", "amount": 125.00}]
    assert decide_allocate_shipment(
        COMMAND, inventory_available=True, shipment_id="sh-55"
    ) == {
        "event": "ShipmentAllocated",
        "orderId": "ord-100",
        "shipmentId": "sh-55",
        "warehouseId": "wh-1",
    }


def test_defer_shipment_when_inventory_is_not_available():
    assert decide_allocate_shipment(COMMAND, inventory_available=False) == {
        "event": "ShipmentAllocationDeferred",
        "orderId": "ord-100",
        "reason": "inventory_unavailable",
    }


def test_retry_allocation_reissues_command_for_deferred_order():
    replenished = {"warehouseId": "wh-1", "sku": "sku-1"}
    deferred = {"orderId": "ord-100", "reason": "inventory_unavailable"}
    # tf 26 pcr AllocationRetrier ->> 25 ->> 23
    assert retry_allocation(replenished, deferred) == COMMAND


def test_project_shipment_queue_matches_data_block():
    event = {
        "event": "ShipmentAllocated",
        "orderId": "ord-100",
        "shipmentId": "sh-55",
        "warehouseId": "wh-1",
    }
    # data ShipmentQueue15
    assert project_shipment_queue(event) == {
        "shipmentId": "sh-55",
        "orderId": "ord-100",
        "warehouseId": "wh-1",
        "status": "queued_for_picking",
    }
