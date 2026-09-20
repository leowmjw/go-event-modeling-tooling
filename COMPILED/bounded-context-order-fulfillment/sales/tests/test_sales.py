"""Golden tests for the sales context — checkout slice (tf 03) and
billing failure recovery slice (tf 18), plus the CartSummary projection."""

from extracted.sales import (
    decide_cancel_order,
    decide_place_order,
    project_cancelled_order_status,
    project_cart_summary,
    project_order_status,
    translate_sales_recovery,
)

# --- tf 03 cmd PlaceOrder -------------------------------------------------


def test_place_order_from_a_priced_cart():
    given = [{"event": "CartPriced", "cartId": "c-100", "total": 125.00}]
    when = {"cartId": "c-100", "customerId": "cust-7", "total": 125.00}
    assert decide_place_order(when, given) == {
        "event": "OrderPlaced",
        "orderId": "ord-100",
        "customerId": "cust-7",
        "total": 125.00,
    }


def test_reject_order_when_cart_has_no_priced_items():
    given = [{"event": "CartPriced", "cartId": "c-100", "total": 0.00}]
    when = {"cartId": "c-100", "customerId": "cust-7", "total": 0.00}
    assert decide_place_order(when, given) == {
        "event": "OrderPlacementRejected",
        "cartId": "c-100",
        "reason": "empty_cart",
    }


def test_project_order_status_matches_data_block():
    event = {
        "event": "OrderPlaced",
        "orderId": "ord-100",
        "customerId": "cust-7",
        "total": 125.00,
    }
    # data OrderStatus05
    assert project_order_status(event) == {
        "orderId": "ord-100",
        "customerId": "cust-7",
        "status": "pending_payment",
    }


# --- rf 21 -> tf 02 rmo CartSummary ----------------------------------------


def test_project_cart_summary_matches_data_block():
    event = {
        "cartId": "c-100",
        "customerId": "cust-7",
        "items": [
            {"sku": "sku-1", "qty": 1, "unitPrice": 75.00},
            {"sku": "sku-2", "qty": 1, "unitPrice": 50.00},
        ],
        "total": 125.00,
    }
    # data CartSummary02
    assert project_cart_summary(event) == event


# --- tf 18 cmd CancelOrder (recovery slice) ---------------------------------

DECLINED = {"orderId": "ord-100", "reason": "insufficient_funds"}
CANCEL = {"orderId": "ord-100", "reason": "payment_declined"}


def test_translate_billing_payment_declined():
    assert translate_sales_recovery(DECLINED) == CANCEL


def test_cancel_order_after_payment_declines():
    given = [
        {"event": "Billing.PaymentDeclined", "orderId": "ord-100", "reason": "insufficient_funds"}
    ]
    assert decide_cancel_order(CANCEL, given) == {
        "event": "OrderCancelled",
        "orderId": "ord-100",
        "reason": "payment_declined",
    }


def test_ignore_cancellation_when_order_is_already_cancelled():
    given = [
        {"event": "OrderCancelled", "orderId": "ord-100", "reason": "payment_declined"}
    ]
    assert decide_cancel_order(CANCEL, given) == {
        "event": "OrderCancellationIgnored",
        "orderId": "ord-100",
        "reason": "already_cancelled",
    }


def test_project_cancelled_order_status_matches_frame_payload():
    event = {"event": "OrderCancelled", "orderId": "ord-100", "reason": "payment_declined"}
    # tf 20 rmo CancelledOrderStatus { orderId: "ord-100", status: "cancelled" }
    assert project_cancelled_order_status(event) == {
        "orderId": "ord-100",
        "status": "cancelled",
    }
