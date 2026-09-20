"""Sales context — extracted implementations.

Source: testdata/fixtures/bounded-context-order-fulfillment.evml, frames
01-05 (checkout slice) and 16-20 (billing failure recovery slice, owned by
Sales per the 'Sales: billing failure recovery' banner).

The context has three boundary inputs: the PlaceOrderCommand UI entry,
rf 21 Pricing.CartPriced, and rf 16 Billing.PaymentDeclined.
"""

from __future__ import annotations


def decide_place_order(command: dict, prior_events: list | None = None) -> dict:
    """Decide the result of `cmd PlaceOrder` (tf 03).

    Input:  PlaceOrderCommand {cartId, customerId, total}
    Output: OrderPlaced {orderId, customerId, total}
            or OrderPlacementRejected {cartId, reason}

    GWT cases (tf 03):
      - "place order from a priced cart": total 125.00 -> OrderPlaced
      - "reject order when cart has no priced items": total 0.00 -> rejected

    orderId is assigned by the order service; derived deterministically from
    cartId (ord-<cartId suffix>) so the pure function stays replay-safe.
    """
    total = float(command.get("total", 0) or 0)
    if total <= 0:
        return {
            "event": "OrderPlacementRejected",
            "cartId": command["cartId"],
            "reason": "empty_cart",
        }
    return {
        "event": "OrderPlaced",
        "orderId": "ord-" + str(command["cartId"]).split("-")[-1],
        "customerId": command["customerId"],
        "total": total,
    }


def project_cart_summary(event: dict) -> dict:
    """Fold `rf evt Pricing.CartPriced` (rf 21) into `rmo CartSummary` (tf 02).

    Expected shape is the CartSummary02 data block:
      {cartId, customerId, items: [{sku, qty, unitPrice}], total}
    """
    return {
        "cartId": event["cartId"],
        "customerId": event["customerId"],
        "items": event["items"],
        "total": float(event["total"]),
    }


def project_order_status(event: dict) -> dict:
    """Fold `evt OrderPlaced` (tf 04) into `rmo OrderStatus` (tf 05).

    Expected shape is the OrderStatus05 data block:
      {orderId, customerId, status: "pending_payment"}
    """
    if event.get("event") == "OrderPlacementRejected":
        return {
            "orderId": None,
            "customerId": event.get("customerId"),
            "status": "rejected:" + event.get("reason", ""),
        }
    return {
        "orderId": event["orderId"],
        "customerId": event["customerId"],
        "status": "pending_payment",
    }


def translate_sales_recovery(event: dict) -> dict:
    """Map `rf evt Billing.PaymentDeclined` (rf 16) to CancelOrderCommand (tf 18).

    Input:  Billing.PaymentDeclined {orderId, reason}
    Output: CancelOrderCommand {orderId, reason: "payment_declined"}
    """
    return {"orderId": event["orderId"], "reason": "payment_declined"}


def decide_cancel_order(command: dict, prior_events: list | None = None) -> dict:
    """Decide the result of `cmd CancelOrder` (tf 18).

    Input:  CancelOrderCommand {orderId, reason}
            prior_events: events already emitted for this order
    Output: OrderCancelled {orderId, reason}
            or OrderCancellationIgnored {orderId, reason}

    GWT cases (tf 18):
      - "cancel order after payment declines" (given PaymentDeclined)
        -> OrderCancelled
      - "ignore cancellation when order is already cancelled"
        (given OrderCancelled) -> OrderCancellationIgnored
    """
    already_cancelled = any(
        e.get("event") == "OrderCancelled" or e.get("type") == "OrderCancelled"
        for e in (prior_events or [])
    )
    if already_cancelled:
        return {
            "event": "OrderCancellationIgnored",
            "orderId": command["orderId"],
            "reason": "already_cancelled",
        }
    return {
        "event": "OrderCancelled",
        "orderId": command["orderId"],
        "reason": command["reason"],
    }


def project_cancelled_order_status(event: dict) -> dict:
    """Fold `evt OrderCancelled` (tf 19) into `rmo CancelledOrderStatus` (tf 20).

    Expected shape: {orderId, status: "cancelled"}
    """
    return {"orderId": event["orderId"], "status": "cancelled"}
