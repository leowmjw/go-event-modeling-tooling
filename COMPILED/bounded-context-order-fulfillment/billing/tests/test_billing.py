"""Golden tests for the billing context — one per gwt on tf 08."""

from extracted.billing import (
    decide_authorize_payment,
    project_payment_status,
    translate_billing,
)

EVENT = {"orderId": "ord-100", "total": 125.00}
COMMAND = {"orderId": "ord-100", "amount": 125.00, "paymentMethodId": "pm-9"}


def test_translate_sales_order_placed():
    assert translate_billing(EVENT) == COMMAND


def test_authorize_payment_for_a_newly_placed_order():
    given = [{"event": "Sales.OrderPlaced", "orderId": "ord-100", "total": 125.00}]
    assert decide_authorize_payment(COMMAND, issuer_result="authorized") == {
        "event": "PaymentAuthorized",
        "orderId": "ord-100",
        "authorizationId": "auth-1",
        "amount": 125.00,
    }


def test_decline_payment_when_issuer_rejects_the_charge():
    assert decide_authorize_payment(COMMAND, issuer_result="declined") == {
        "event": "PaymentDeclined",
        "orderId": "ord-100",
        "reason": "issuer_declined",
    }


def test_project_payment_status_matches_frame_payload():
    event = {
        "event": "PaymentAuthorized",
        "orderId": "ord-100",
        "authorizationId": "auth-1",
        "amount": 125.00,
    }
    # tf 10 rmo PaymentStatus { orderId: "ord-100", status: "authorized" }
    assert project_payment_status(event) == {
        "orderId": "ord-100",
        "status": "authorized",
    }
