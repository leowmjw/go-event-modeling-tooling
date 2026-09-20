package orderfulfillment

import (
	"fmt"
	"math"
	"strings"
)

func DecidePlaceOrder(command PlaceOrderCommand) (SalesResult, error) {
	if command.CartID == "" || command.CustomerID == "" {
		return SalesResult{}, fmt.Errorf("cartId and customerId are required")
	}
	if math.IsNaN(command.Total) || math.IsInf(command.Total, 0) {
		return SalesResult{}, fmt.Errorf("total must be finite")
	}
	if !strings.Contains(command.CartID, "-") || lastSegment(command.CartID) == "" {
		return SalesResult{}, fmt.Errorf("cartId must contain a non-empty suffix after '-'")
	}
	if command.Total <= 0 {
		return SalesResult{
			Event:    "OrderPlacementRejected",
			Rejected: &OrderPlacementRejected{CartID: command.CartID, Reason: "empty_cart"},
		}, nil
	}
	return SalesResult{
		Event: "OrderPlaced",
		Placed: &OrderPlaced{
			OrderID:    "ord-" + lastSegment(command.CartID),
			CustomerID: command.CustomerID,
			Total:      command.Total,
		},
	}, nil
}

func ProjectCartSummary(event CartPriced) (CartSummaryView, error) {
	if event.CartID == "" || event.CustomerID == "" {
		return CartSummaryView{}, fmt.Errorf("cartId and customerId are required")
	}
	return CartSummaryView{CartID: event.CartID, CustomerID: event.CustomerID, Items: event.Items, Total: event.Total}, nil
}

func ProjectOrderStatus(result SalesResult) OrderStatusView {
	if result.Placed != nil {
		return OrderStatusView{OrderID: result.Placed.OrderID, CustomerID: result.Placed.CustomerID, Status: "pending_payment"}
	}
	return OrderStatusView{Status: "rejected"}
}

func TranslateBilling(order OrderPlaced, paymentMethodID string) (AuthorizePaymentCommand, error) {
	if order.OrderID == "" || paymentMethodID == "" {
		return AuthorizePaymentCommand{}, fmt.Errorf("orderId and paymentMethodId are required")
	}
	return AuthorizePaymentCommand{OrderID: order.OrderID, Amount: order.Total, PaymentMethodID: paymentMethodID}, nil
}

func DecidePayment(command AuthorizePaymentCommand, response PaymentGatewayResponse) (PaymentResult, error) {
	if response.Authorized {
		if response.AuthorizationID == "" {
			return PaymentResult{}, fmt.Errorf("authorizationId is required for an authorized payment")
		}
		return PaymentResult{
			Event:      "PaymentAuthorized",
			Authorized: &PaymentAuthorized{OrderID: command.OrderID, AuthorizationID: response.AuthorizationID, Amount: command.Amount},
			Status:     PaymentStatusView{OrderID: command.OrderID, Status: "authorized"},
		}, nil
	}
	reason := response.DeclineReason
	if reason == "" {
		reason = "issuer_declined"
	}
	return PaymentResult{
		Event:    "PaymentDeclined",
		Declined: &PaymentDeclined{OrderID: command.OrderID, Reason: reason},
		Status:   PaymentStatusView{OrderID: command.OrderID, Status: "declined"},
	}, nil
}

func TranslateFulfillment(payment PaymentAuthorized, warehouseID string) (AllocateShipmentCommand, error) {
	if payment.OrderID == "" || warehouseID == "" {
		return AllocateShipmentCommand{}, fmt.Errorf("orderId and warehouseId are required")
	}
	return AllocateShipmentCommand{OrderID: payment.OrderID, WarehouseID: warehouseID}, nil
}

func DecideShipment(command AllocateShipmentCommand, response InventoryResponse) (ShipmentResult, error) {
	if response.Available {
		if response.ShipmentID == "" {
			return ShipmentResult{}, fmt.Errorf("shipmentId is required when inventory is available")
		}
		return ShipmentResult{
			Event:     "ShipmentAllocated",
			Allocated: &ShipmentAllocated{OrderID: command.OrderID, ShipmentID: response.ShipmentID, WarehouseID: command.WarehouseID},
			Queue: ShipmentQueueView{
				ShipmentID: response.ShipmentID, OrderID: command.OrderID, WarehouseID: command.WarehouseID, Status: "queued_for_picking",
			},
		}, nil
	}
	return ShipmentResult{
		Event:    "ShipmentAllocationDeferred",
		Deferred: &ShipmentAllocationDeferred{OrderID: command.OrderID, Reason: "inventory_unavailable"},
		Queue:    ShipmentQueueView{OrderID: command.OrderID, WarehouseID: command.WarehouseID, Status: "deferred:inventory_unavailable"},
	}, nil
}

func TranslateSalesRecovery(declined PaymentDeclined) (CancelOrderCommand, error) {
	if declined.OrderID == "" {
		return CancelOrderCommand{}, fmt.Errorf("orderId is required")
	}
	return CancelOrderCommand{OrderID: declined.OrderID, Reason: "payment_declined"}, nil
}

func DecideCancelOrder(command CancelOrderCommand, priorEvents []OrderEvent) (CancellationResult, error) {
	if command.OrderID == "" {
		return CancellationResult{}, fmt.Errorf("orderId is required")
	}
	for _, event := range priorEvents {
		if event.OrderID == command.OrderID && event.Type == "OrderCancelled" {
			return CancellationResult{
				Event:   "OrderCancellationIgnored",
				Ignored: &OrderCancellationIgnored{OrderID: command.OrderID, Reason: "already_cancelled"},
				Status:  CancelledOrderStatusView{OrderID: command.OrderID, Status: "cancelled"},
			}, nil
		}
	}
	return CancellationResult{
		Event:     "OrderCancelled",
		Cancelled: &OrderCancelled{OrderID: command.OrderID, Reason: command.Reason},
		Status:    CancelledOrderStatusView{OrderID: command.OrderID, Status: "cancelled"},
	}, nil
}

func lastSegment(value string) string {
	parts := strings.Split(value, "-")
	return parts[len(parts)-1]
}
