"""Billing context — extracted implementations.

Source: testdata/fixtures/bounded-context-order-fulfillment.evml, frames 06-10.
rf 06 Sales.OrderPlaced is the pipeline input; the translator maps it to
AuthorizePaymentCommand. The authorize/decline branch is the issuer's
decision, so `issuer_result` is injected (see compile-report judgment calls).
"""

from __future__ import annotations


def translate_billing(event: dict, payment_method_id: str = "pm-9") -> dict:
    """Map `rf evt Sales.OrderPlaced` (tf 06) to AuthorizePaymentCommand (tf 08).

    Input:  Sales.OrderPlaced {orderId, total}
    Output: AuthorizePaymentCommand {orderId, amount, paymentMethodId}

    paymentMethodId is not carried by the event; it is the customer's stored
    default instrument, resolved by the translator (parameterised so tests and
    the runtime can supply the real lookup).
    """
    return {
        "orderId": event["orderId"],
        "amount": float(event["total"]),
        "paymentMethodId": payment_method_id,
    }


def decide_authorize_payment(
    command: dict,
    issuer_result: str = "authorized",
    authorization_id: str = "auth-1",
) -> dict:
    """Decide the result of `cmd AuthorizePayment` (tf 08).

    Input:  AuthorizePaymentCommand {orderId, amount, paymentMethodId}
    Output: PaymentAuthorized {orderId, authorizationId, amount}
            or PaymentDeclined {orderId, reason}

    GWT cases (tf 08):
      - "authorize payment for a newly placed order" -> PaymentAuthorized
      - "decline payment when issuer rejects the charge" -> PaymentDeclined

    The two gwts share identical given/when payloads — the outcome is the
    card issuer's, not derivable from the command. `issuer_result`
    ("authorized" | "declined") stands in for the issuer response at IR
    level; emission may reclassify this node as an external_call.
    """
    if issuer_result == "declined":
        return {
            "event": "PaymentDeclined",
            "orderId": command["orderId"],
            "reason": "issuer_declined",
        }
    return {
        "event": "PaymentAuthorized",
        "orderId": command["orderId"],
        "authorizationId": authorization_id,
        "amount": float(command["amount"]),
    }


def project_payment_status(event: dict) -> dict:
    """Fold tf 09 evt into `rmo PaymentStatus` (tf 10).

    Expected shape: {orderId, status} — "authorized" on PaymentAuthorized,
    "declined" on PaymentDeclined.
    """
    status = "authorized" if event.get("event") == "PaymentAuthorized" else "declined"
    return {"orderId": event["orderId"], "status": status}
