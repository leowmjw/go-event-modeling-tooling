"""Golden tests for the billing context: every gwt on frames 08 and 10, plus
the gateway contract from note 08."""

import pytest

from extracted.billing import (
    authorize_payment_with_gateway,
    configure_gateway,
    decide_authorize_payment,
    project_payment_status,
    translate_billing,
)


@pytest.fixture(autouse=True)
def _reset_gateway():
    yield
    configure_gateway(None)

SALES_ORDER_PLACED = {"orderId": "ord-100", "paymentMethodId": "pm-9", "total": 125.00}
COMMAND = {"orderId": "ord-100", "amount": 125.00, "paymentMethodId": "pm-9"}
AUTHORIZED = {
    "event": "PaymentAuthorized",
    "authorized": {"orderId": "ord-100", "authorizationId": "auth-1", "amount": 125.00},
}
DECLINED = {"event": "PaymentDeclined", "declined": {"orderId": "ord-100", "reason": "insufficient_funds"}}


def test_translate_sales_order_placed():
    # tf 08 payload
    assert translate_billing(SALES_ORDER_PLACED) == COMMAND


def test_gateway_receives_order_id_as_idempotency_key():
    calls = []

    def gateway(command, key):
        calls.append((command, key))
        return {"approved": True, "authorizationId": "auth-1"}

    configure_gateway(gateway)
    assert authorize_payment_with_gateway(COMMAND) == {"approved": True, "authorizationId": "auth-1"}
    assert calls == [(COMMAND, "ord-100")]


def test_unconfigured_gateway_fails_loudly():
    with pytest.raises(RuntimeError):
        authorize_payment_with_gateway(COMMAND)


@pytest.mark.parametrize(
    "result",
    [{"approved": True}, {"approved": False}, {}],
)
def test_incomplete_gateway_result_is_rejected_not_defaulted(result):
    configure_gateway(lambda command, key: result)
    with pytest.raises(ValueError):
        authorize_payment_with_gateway(COMMAND)


def test_authorize_payment_when_the_gateway_approves_the_charge():
    assert decide_authorize_payment(COMMAND, {"approved": True, "authorizationId": "auth-1"}) == AUTHORIZED


def test_decline_payment_when_the_gateway_declines_the_charge():
    assert decide_authorize_payment(COMMAND, {"approved": False, "reason": "insufficient_funds"}) == DECLINED


def test_payment_status_shows_an_authorized_payment():
    # gwt 10 + tf 10 payload
    assert project_payment_status(AUTHORIZED) == {"orderId": "ord-100", "status": "authorized"}


def test_payment_status_shows_a_declined_payment():
    assert project_payment_status(DECLINED) == {"orderId": "ord-100", "status": "declined", "reason": "insufficient_funds"}
