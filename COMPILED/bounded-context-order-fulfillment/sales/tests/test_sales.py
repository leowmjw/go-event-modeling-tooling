"""Golden tests for the sales context: every gwt on frames 03, 05, 18 and 32,
plus the CartSummary02 / OrderPlaced04 / OrderStatus05 data blocks."""

import pytest

from extracted.sales import (
    decide_cancel_order,
    decide_confirm_order,
    decide_place_order,
    project_cart_summary,
    project_order_status,
    translate_sales_confirmation,
    translate_sales_recovery,
)

ITEMS = [
    {"sku": "sku-1", "qty": 1, "unitPrice": 75.00},
    {"sku": "sku-2", "qty": 1, "unitPrice": 50.00},
]
CART_PRICED = {"cartId": "c-100", "customerId": "cust-7", "items": ITEMS, "total": 125.00}
PLACE_ORDER = {"orderId": "ord-100", "cartId": "c-100", "customerId": "cust-7", "paymentMethodId": "pm-9"}
# data OrderPlaced04
ORDER_PLACED = {
    "orderId": "ord-100",
    "cartId": "c-100",
    "customerId": "cust-7",
    "paymentMethodId": "pm-9",
    "items": ITEMS,
    "total": 125.00,
}
PLACED = {"event": "OrderPlaced", "placed": ORDER_PLACED}


def test_project_cart_summary_matches_data_block():
    # data CartSummary02
    assert project_cart_summary(CART_PRICED) == CART_PRICED


def test_place_order_from_a_priced_cart():
    cart = project_cart_summary(CART_PRICED)
    assert decide_place_order(PLACE_ORDER, cart) == PLACED


def test_reject_order_when_cart_has_no_priced_items():
    cart = project_cart_summary({"cartId": "c-100", "customerId": "cust-7", "items": [], "total": 0.00})
    assert decide_place_order(PLACE_ORDER, cart) == {
        "event": "OrderPlacementRejected",
        "rejected": {"orderId": "ord-100", "cartId": "c-100", "reason": "empty_cart"},
    }


def test_place_order_for_a_different_cart_is_invalid_input():
    cart = project_cart_summary(CART_PRICED)
    with pytest.raises(ValueError):
        decide_place_order({**PLACE_ORDER, "cartId": "c-999"}, cart)


def test_order_awaits_payment_once_placed():
    # gwt 05 + data OrderStatus05
    assert project_order_status(PLACED) == {"orderId": "ord-100", "customerId": "cust-7", "status": "pending_payment"}


def test_order_is_confirmed_after_payment_is_authorized():
    confirmed = {"event": "OrderConfirmed", "orderId": "ord-100", "authorizationId": "auth-1"}
    assert project_order_status(PLACED, confirmation=confirmed) == {
        "orderId": "ord-100",
        "customerId": "cust-7",
        "status": "confirmed",
    }


def test_order_is_cancelled_after_payment_declines():
    cancelled = {"event": "OrderCancelled", "orderId": "ord-100", "reason": "payment_declined"}
    assert project_order_status(PLACED, cancellation=cancelled) == {
        "orderId": "ord-100",
        "customerId": "cust-7",
        "status": "cancelled",
    }


def test_rejected_placement_has_no_order_status():
    rejected = {"event": "OrderPlacementRejected", "rejected": {"orderId": "ord-100", "cartId": "c-100", "reason": "empty_cart"}}
    assert project_order_status(rejected) is None


CONFIRM = {"orderId": "ord-100", "authorizationId": "auth-1"}


def test_translate_billing_payment_authorized():
    assert translate_sales_confirmation({"orderId": "ord-100", "authorizationId": "auth-1"}) == CONFIRM


def test_confirm_order_after_payment_is_authorized():
    history = [{"event": "OrderPlaced", "orderId": "ord-100"}]
    assert decide_confirm_order(CONFIRM, PLACED, history) == {
        "event": "OrderConfirmed",
        "orderId": "ord-100",
        "authorizationId": "auth-1",
    }


def test_ignore_confirmation_when_order_is_already_confirmed():
    history = [{"event": "OrderPlaced", "orderId": "ord-100"}, {"event": "OrderConfirmed", "orderId": "ord-100"}]
    assert decide_confirm_order(CONFIRM, PLACED, history) == {
        "event": "OrderConfirmationIgnored",
        "orderId": "ord-100",
        "reason": "already_confirmed",
    }


CANCEL = {"orderId": "ord-100", "reason": "payment_declined"}


def test_translate_billing_payment_declined():
    assert translate_sales_recovery({"orderId": "ord-100", "reason": "insufficient_funds"}) == CANCEL


def test_cancel_order_after_payment_declines():
    history = [{"event": "OrderPlaced", "orderId": "ord-100"}]
    assert decide_cancel_order(CANCEL, PLACED, history) == {
        "event": "OrderCancelled",
        "orderId": "ord-100",
        "reason": "payment_declined",
    }


def test_ignore_cancellation_when_order_is_already_cancelled():
    history = [{"event": "OrderPlaced", "orderId": "ord-100"}, {"event": "OrderCancelled", "orderId": "ord-100"}]
    assert decide_cancel_order(CANCEL, PLACED, history) == {
        "event": "OrderCancellationIgnored",
        "orderId": "ord-100",
        "reason": "already_cancelled",
    }


def test_closing_an_order_that_was_never_placed_is_invalid():
    with pytest.raises(ValueError):
        decide_cancel_order({**CANCEL, "orderId": "ord-404"}, PLACED, [])
