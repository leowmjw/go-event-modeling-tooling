"""Golden tests for the fulfillment context: every gwt on frames 13, 15, 26
and 29, the PaidOrdersToShip29 / ShipmentQueue15 data blocks, and the
warehouse contract from note 13."""

import pytest

from extracted.fulfillment import (
    allocate_shipment_with_warehouse,
    configure_warehouse,
    decide_allocate_shipment,
    project_paid_orders_to_ship,
    project_shipment_queue,
    retry_allocation,
    translate_fulfillment,
)

ITEMS = [{"sku": "sku-1", "qty": 1}, {"sku": "sku-2", "qty": 1}]
ORDER_PLACED = {"orderId": "ord-100", "items": ITEMS}
PAYMENT_AUTHORIZED = {"orderId": "ord-100", "amount": 125.00}
READY = {"orderId": "ord-100", "items": ITEMS, "status": "ready_to_ship"}
COMMAND = {"orderId": "ord-100", "items": ITEMS}
ALLOCATED = {
    "event": "ShipmentAllocated",
    "allocated": {"orderId": "ord-100", "shipmentId": "sh-55", "warehouseId": "wh-1"},
}
DEFERRED = {
    "event": "ShipmentAllocationDeferred",
    "deferred": {"orderId": "ord-100", "reason": "inventory_unavailable", "missingSkus": ["sku-1"]},
}
AWAITING_INVENTORY = {"orderId": "ord-100", "status": "awaiting_inventory", "missingSkus": ["sku-1"]}


@pytest.fixture(autouse=True)
def _reset_warehouse():
    yield
    configure_warehouse(None)


def test_paid_order_is_ready_to_ship():
    # gwt 29 + data PaidOrdersToShip29
    assert project_paid_orders_to_ship(ORDER_PLACED, PAYMENT_AUTHORIZED) == READY


def test_unpaid_order_is_not_ready_to_ship():
    assert project_paid_orders_to_ship(ORDER_PLACED) == {**READY, "status": "awaiting_payment"}


def test_translator_issues_allocate_shipment_for_ready_order():
    # tf 13 payload
    assert translate_fulfillment(READY) == COMMAND


def test_translator_refuses_unpaid_order():
    with pytest.raises(ValueError):
        translate_fulfillment({**READY, "status": "awaiting_payment"})


def test_warehouse_idempotency_key_is_scoped_by_attempt():
    keys = []

    def warehouse(command, key):
        keys.append(key)
        return {"allocated": True, "shipmentId": "sh-55", "warehouseId": "wh-1"}

    configure_warehouse(warehouse)
    allocate_shipment_with_warehouse(COMMAND)
    allocate_shipment_with_warehouse(COMMAND, attempt=2)
    assert keys == ["ord-100/allocation-1", "ord-100/allocation-2"]


def test_unconfigured_warehouse_fails_loudly():
    with pytest.raises(RuntimeError):
        allocate_shipment_with_warehouse(COMMAND)


@pytest.mark.parametrize("result", [{"allocated": True, "shipmentId": "sh-55"}, {"allocated": False}, {}])
def test_incomplete_warehouse_result_is_rejected_not_defaulted(result):
    configure_warehouse(lambda command, key: result)
    with pytest.raises(ValueError):
        allocate_shipment_with_warehouse(COMMAND)


def test_allocate_shipment_when_inventory_is_available():
    allocation = {"allocated": True, "shipmentId": "sh-55", "warehouseId": "wh-1"}
    assert decide_allocate_shipment(COMMAND, allocation) == ALLOCATED


def test_defer_shipment_when_inventory_is_not_available():
    assert decide_allocate_shipment(COMMAND, {"allocated": False, "missingSkus": ["sku-1"]}) == DEFERRED


def test_allocated_shipment_is_queued_for_picking():
    # gwt 15 + data ShipmentQueue15
    assert project_shipment_queue(ALLOCATED) == {
        "shipmentId": "sh-55",
        "orderId": "ord-100",
        "warehouseId": "wh-1",
        "status": "queued_for_picking",
    }


def test_deferred_shipment_awaits_inventory():
    assert project_shipment_queue(DEFERRED) == AWAITING_INVENTORY


def test_retry_allocation_when_a_missing_sku_is_replenished():
    replenished = {"warehouseId": "wh-1", "sku": "sku-1"}
    assert retry_allocation(replenished, DEFERRED, READY) == COMMAND


def test_keep_waiting_when_an_unrelated_sku_is_replenished():
    replenished = {"warehouseId": "wh-1", "sku": "sku-9"}
    assert retry_allocation(replenished, DEFERRED, READY) is None
    assert project_shipment_queue(DEFERRED) == AWAITING_INVENTORY
