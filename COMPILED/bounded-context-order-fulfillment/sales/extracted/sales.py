"""Sales context — extracted implementations.

Source: testdata/fixtures/bounded-context-order-fulfillment.evml — checkout
slice (frames 01-05, 21, 27), billing failure recovery (16-19, 24) and payment
confirmation (30-34), all owned by Sales via `Sales: …` banners.

`history` is the event log this context already recorded for the order
(`pipeline.input.order_history`), supplied from durable workflow state. None
means nothing has been recorded yet.
"""

from __future__ import annotations


def project_cart_summary(event: dict) -> dict:
    """Fold rf 21 Pricing.CartPriced into rmo CartSummary (data CartSummary02)."""
    return {
        "cartId": event["cartId"],
        "customerId": event["customerId"],
        "items": [dict(item) for item in event["items"]],
        "total": float(event["total"]),
    }


def decide_place_order(command: dict, cart: dict) -> dict:
    """Decide cmd PlaceOrder (tf 03) against the CartSummary the screen showed.

    gwt 03 "place order from a priced cart" -> OrderPlaced (items/total from cart)
    gwt 03 "reject order when cart has no priced items" -> OrderPlacementRejected
    """
    if command["cartId"] != cart["cartId"] or command["customerId"] != cart["customerId"]:
        raise ValueError("PlaceOrder does not match the priced cart")
    if not command["orderId"] or not command["paymentMethodId"]:
        raise ValueError("orderId and paymentMethodId are required")
    if not cart["items"]:
        return {
            "event": "OrderPlacementRejected",
            "rejected": {
                "orderId": command["orderId"],
                "cartId": command["cartId"],
                "reason": "empty_cart",
            },
        }
    return {
        "event": "OrderPlaced",
        "placed": {
            "orderId": command["orderId"],
            "cartId": cart["cartId"],
            "customerId": cart["customerId"],
            "paymentMethodId": command["paymentMethodId"],
            "items": [dict(item) for item in cart["items"]],
            "total": cart["total"],
        },
    }


def translate_sales_confirmation(event: dict) -> dict:
    """Map rf 30 Billing.PaymentAuthorized to ConfirmOrderCommand (tf 32)."""
    return {"orderId": event["orderId"], "authorizationId": event["authorizationId"]}


def _placed_order(placed: dict, order_id: str) -> dict:
    order = placed.get("placed") if placed.get("event") == "OrderPlaced" else None
    if order is None or order["orderId"] != order_id:
        raise ValueError(f"order {order_id} was never placed")
    return order


def _seen(history: list | None, event: str, order_id: str) -> bool:
    return any(e["event"] == event and e["orderId"] == order_id for e in history or [])


def decide_confirm_order(command: dict, placed: dict, history: list | None) -> dict:
    """Decide cmd ConfirmOrder (tf 32).

    gwt 32 "confirm order after payment is authorized" -> OrderConfirmed
    gwt 32 "ignore confirmation when order is already confirmed"
        -> OrderConfirmationIgnored{already_confirmed}
    """
    order_id = command["orderId"]
    _placed_order(placed, order_id)
    if _seen(history, "OrderConfirmed", order_id):
        return {"event": "OrderConfirmationIgnored", "orderId": order_id, "reason": "already_confirmed"}
    return {"event": "OrderConfirmed", "orderId": order_id, "authorizationId": command["authorizationId"]}


def translate_sales_recovery(event: dict) -> dict:
    """Map rf 16 Billing.PaymentDeclined to CancelOrderCommand (tf 18)."""
    return {"orderId": event["orderId"], "reason": "payment_declined"}


def decide_cancel_order(command: dict, placed: dict, history: list | None) -> dict:
    """Decide cmd CancelOrder (tf 18).

    gwt 18 "cancel order after payment declines" -> OrderCancelled
    gwt 18 "ignore cancellation when order is already cancelled"
        -> OrderCancellationIgnored{already_cancelled}
    """
    order_id = command["orderId"]
    _placed_order(placed, order_id)
    if _seen(history, "OrderCancelled", order_id):
        return {"event": "OrderCancellationIgnored", "orderId": order_id, "reason": "already_cancelled"}
    return {"event": "OrderCancelled", "orderId": order_id, "reason": command["reason"]}


def project_order_status(placed: dict, confirmation: dict | None = None, cancellation: dict | None = None) -> dict | None:
    """Fold order events into rmo OrderStatus (tf 05, data OrderStatus05, gwt 05).

    Returns None for a rejected placement: there is no order to show.
    """
    if placed.get("event") != "OrderPlaced":
        return None
    order = placed["placed"]
    status = "pending_payment"
    if confirmation and confirmation["event"] == "OrderConfirmed":
        status = "confirmed"
    if cancellation and cancellation["event"] == "OrderCancelled":
        status = "cancelled"
    return {"orderId": order["orderId"], "customerId": order["customerId"], "status": status}
