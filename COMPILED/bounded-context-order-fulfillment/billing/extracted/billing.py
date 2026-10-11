"""Billing context — extracted implementations.

Source: testdata/fixtures/bounded-context-order-fulfillment.evml, frames 06-10, 22.
The authorize/decline outcome belongs to the payment gateway (note 08), so it is
an external_call whose result feeds the pure decision.
"""

from __future__ import annotations

from typing import Callable

# Deployment configuration, not pipeline data: the worker registers the real
# payment gateway client with configure_gateway() at startup.
_gateway: Callable[[dict, str], dict] | None = None


def configure_gateway(client: Callable[[dict, str], dict] | None) -> None:
    global _gateway
    _gateway = client


def translate_billing(event: dict) -> dict:
    """Map rf 06 Sales.OrderPlaced to AuthorizePaymentCommand (tf 08)."""
    return {"orderId": event["orderId"], "amount": float(event["total"]), "paymentMethodId": event["paymentMethodId"]}


def authorize_payment_with_gateway(command: dict) -> dict:
    """external_call: ask the payment gateway to authorize the charge.

    Calls the configured client as `client(command, idempotency_key)`; the key
    is the orderId (note 08), so a retry after a lost response cannot charge
    twice. Returns the validated GatewayAuthorization: approved with
    authorizationId, or declined with reason.
    """
    if _gateway is None:
        raise RuntimeError("payment gateway client is not configured")
    result = _gateway(command, command["orderId"])
    if result.get("approved") is True:
        if not result.get("authorizationId"):
            raise ValueError("approved gateway result has no authorizationId")
        return {"approved": True, "authorizationId": result["authorizationId"]}
    if result.get("approved") is False:
        if not result.get("reason"):
            raise ValueError("declined gateway result has no reason")
        return {"approved": False, "reason": result["reason"]}
    raise ValueError("gateway result has no approved flag")


def decide_authorize_payment(command: dict, gateway: dict) -> dict:
    """Decide cmd AuthorizePayment (tf 08) from the gateway result.

    gwt 08 "authorize payment when the gateway approves the charge" -> PaymentAuthorized
    gwt 08 "decline payment when the gateway declines the charge" -> PaymentDeclined
    """
    if gateway["approved"]:
        return {
            "event": "PaymentAuthorized",
            "authorized": {
                "orderId": command["orderId"],
                "authorizationId": gateway["authorizationId"],
                "amount": command["amount"],
            },
        }
    return {"event": "PaymentDeclined", "declined": {"orderId": command["orderId"], "reason": gateway["reason"]}}


def project_payment_status(event: dict) -> dict:
    """Fold PaymentAuthorized / PaymentDeclined into rmo PaymentStatus (tf 10, gwt 10)."""
    if event["event"] == "PaymentAuthorized":
        return {"orderId": event["authorized"]["orderId"], "status": "authorized"}
    declined = event["declined"]
    return {"orderId": declined["orderId"], "status": "declined", "reason": declined["reason"]}
