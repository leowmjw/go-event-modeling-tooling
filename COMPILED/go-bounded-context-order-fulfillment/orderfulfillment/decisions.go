package orderfulfillment

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// Sales

// ProjectCartSummary is IR node sales.project_cart_summary.
func ProjectCartSummary(event CartPriced) (CartSummaryView, error) {
	if event.CartID == "" || event.CustomerID == "" {
		return CartSummaryView{}, errors.New("CartPriced requires cartId and customerId")
	}
	if math.IsNaN(event.Total) || math.IsInf(event.Total, 0) || event.Total < 0 {
		return CartSummaryView{}, fmt.Errorf("CartPriced total %v is invalid", event.Total)
	}
	return CartSummaryView{
		CartID:     event.CartID,
		CustomerID: event.CustomerID,
		Items:      slices.Clone(event.Items),
		Total:      event.Total,
	}, nil
}

// DecidePlaceOrder is IR node sales.decide_place_order: items and total come
// from the CartSummary the screen showed, never from the command.
func DecidePlaceOrder(command PlaceOrderCommand, cart CartSummaryView) (PlaceOrderResult, error) {
	if command.CartID != cart.CartID || command.CustomerID != cart.CustomerID {
		return PlaceOrderResult{}, errors.New("PlaceOrder does not match the priced cart")
	}
	if command.OrderID == "" || command.PaymentMethodID == "" {
		return PlaceOrderResult{}, errors.New("orderId and paymentMethodId are required")
	}
	if len(cart.Items) == 0 {
		return PlaceOrderResult{Event: "OrderPlacementRejected", Rejected: &OrderPlacementRejected{
			OrderID: command.OrderID, CartID: command.CartID, Reason: "empty_cart",
		}}, nil
	}
	return PlaceOrderResult{Event: "OrderPlaced", Placed: &OrderPlaced{
		OrderID:         command.OrderID,
		CartID:          cart.CartID,
		CustomerID:      cart.CustomerID,
		PaymentMethodID: command.PaymentMethodID,
		Items:           slices.Clone(cart.Items),
		Total:           cart.Total,
	}}, nil
}

// TranslateSalesConfirmation is IR node sales.translate_sales_confirmation.
func TranslateSalesConfirmation(event SalesPaymentAuthorized) (ConfirmOrderCommand, error) {
	if event.OrderID == "" || event.AuthorizationID == "" {
		return ConfirmOrderCommand{}, errors.New("PaymentAuthorized requires orderId and authorizationId")
	}
	return ConfirmOrderCommand{OrderID: event.OrderID, AuthorizationID: event.AuthorizationID}, nil
}

// DecideConfirmOrder is IR node sales.decide_confirm_order.
func DecideConfirmOrder(command ConfirmOrderCommand, placement PlaceOrderResult, history []RecordedEvent) (OrderConfirmationResult, error) {
	if err := requirePlaced(placement, command.OrderID); err != nil {
		return OrderConfirmationResult{}, err
	}
	if seen(history, "OrderConfirmed", command.OrderID) {
		return OrderConfirmationResult{Event: "OrderConfirmationIgnored", OrderID: command.OrderID, Reason: "already_confirmed"}, nil
	}
	return OrderConfirmationResult{Event: "OrderConfirmed", OrderID: command.OrderID, AuthorizationID: command.AuthorizationID}, nil
}

// TranslateSalesRecovery is IR node sales.translate_sales_recovery.
func TranslateSalesRecovery(event SalesPaymentDeclined) (CancelOrderCommand, error) {
	if event.OrderID == "" {
		return CancelOrderCommand{}, errors.New("PaymentDeclined requires orderId")
	}
	return CancelOrderCommand{OrderID: event.OrderID, Reason: "payment_declined"}, nil
}

// DecideCancelOrder is IR node sales.decide_cancel_order.
func DecideCancelOrder(command CancelOrderCommand, placement PlaceOrderResult, history []RecordedEvent) (OrderCancellationResult, error) {
	if err := requirePlaced(placement, command.OrderID); err != nil {
		return OrderCancellationResult{}, err
	}
	if seen(history, "OrderCancelled", command.OrderID) {
		return OrderCancellationResult{Event: "OrderCancellationIgnored", OrderID: command.OrderID, Reason: "already_cancelled"}, nil
	}
	return OrderCancellationResult{Event: "OrderCancelled", OrderID: command.OrderID, Reason: command.Reason}, nil
}

// ProjectOrderStatus is IR node sales.project_order_status. It returns nil
// for a rejected placement: there is no order to show.
func ProjectOrderStatus(placement PlaceOrderResult, confirmation *OrderConfirmationResult, cancellation *OrderCancellationResult) *OrderStatusView {
	if placement.Placed == nil {
		return nil
	}
	status := "pending_payment"
	if confirmation != nil && confirmation.Event == "OrderConfirmed" {
		status = "confirmed"
	}
	if cancellation != nil && cancellation.Event == "OrderCancelled" {
		status = "cancelled"
	}
	return &OrderStatusView{OrderID: placement.Placed.OrderID, CustomerID: placement.Placed.CustomerID, Status: status}
}

func requirePlaced(placement PlaceOrderResult, orderID string) error {
	if placement.Placed == nil || placement.Placed.OrderID != orderID {
		return fmt.Errorf("order %s was never placed", orderID)
	}
	return nil
}

func seen(history []RecordedEvent, event, orderID string) bool {
	return slices.Contains(history, RecordedEvent{Event: event, OrderID: orderID})
}

// Billing

// TranslateBilling is IR node billing.translate_billing.
func TranslateBilling(event SalesOrderPlaced) (AuthorizePaymentCommand, error) {
	if event.OrderID == "" || event.PaymentMethodID == "" {
		return AuthorizePaymentCommand{}, errors.New("OrderPlaced requires orderId and paymentMethodId")
	}
	return AuthorizePaymentCommand{OrderID: event.OrderID, Amount: event.Total, PaymentMethodID: event.PaymentMethodID}, nil
}

// ValidateGatewayAuthorization enforces the note 08 result contract.
func ValidateGatewayAuthorization(result GatewayAuthorization) error {
	if result.Approved && result.AuthorizationID == "" {
		return errors.New("approved gateway result has no authorizationId")
	}
	if !result.Approved && result.Reason == "" {
		return errors.New("declined gateway result has no reason")
	}
	return nil
}

// DecideAuthorizePayment is IR node billing.decide_authorize_payment.
func DecideAuthorizePayment(command AuthorizePaymentCommand, gateway GatewayAuthorization) (PaymentAuthorizationResult, error) {
	if err := ValidateGatewayAuthorization(gateway); err != nil {
		return PaymentAuthorizationResult{}, err
	}
	if gateway.Approved {
		return PaymentAuthorizationResult{Event: "PaymentAuthorized", Authorized: &PaymentAuthorized{
			OrderID: command.OrderID, AuthorizationID: gateway.AuthorizationID, Amount: command.Amount,
		}}, nil
	}
	return PaymentAuthorizationResult{Event: "PaymentDeclined", Declined: &PaymentDeclined{
		OrderID: command.OrderID, Reason: gateway.Reason,
	}}, nil
}

// ProjectPaymentStatus is IR node billing.project_payment_status.
func ProjectPaymentStatus(result PaymentAuthorizationResult) PaymentStatusView {
	if result.Authorized != nil {
		return PaymentStatusView{OrderID: result.Authorized.OrderID, Status: "authorized"}
	}
	return PaymentStatusView{OrderID: result.Declined.OrderID, Status: "declined", Reason: result.Declined.Reason}
}

// Fulfillment

// ProjectPaidOrdersToShip is IR node fulfillment.project_paid_orders_to_ship.
func ProjectPaidOrdersToShip(order FulfillmentOrderPlaced, payment *FulfillmentPaymentAuthorized) (PaidOrdersToShipView, error) {
	if order.OrderID == "" || len(order.Items) == 0 {
		return PaidOrdersToShipView{}, errors.New("OrderPlaced requires orderId and items")
	}
	status := "awaiting_payment"
	if payment != nil && payment.OrderID == order.OrderID {
		status = "ready_to_ship"
	}
	return PaidOrdersToShipView{OrderID: order.OrderID, Items: slices.Clone(order.Items), Status: status}, nil
}

// TranslateFulfillment is IR node fulfillment.translate_fulfillment.
func TranslateFulfillment(view PaidOrdersToShipView) (AllocateShipmentCommand, error) {
	if view.Status != "ready_to_ship" {
		return AllocateShipmentCommand{}, fmt.Errorf("order %s is not ready to ship", view.OrderID)
	}
	return AllocateShipmentCommand{OrderID: view.OrderID, Items: slices.Clone(view.Items)}, nil
}

// AllocationIdempotencyKey is the note 13 key: retries of one attempt reuse
// it, a replenishment-triggered re-issue gets a fresh one.
func AllocationIdempotencyKey(orderID string, attempt int) string {
	return fmt.Sprintf("%s/allocation-%d", orderID, attempt)
}

// ValidateWarehouseAllocation enforces the note 13 result contract.
func ValidateWarehouseAllocation(result WarehouseAllocation) error {
	if result.Allocated && (result.ShipmentID == "" || result.WarehouseID == "") {
		return errors.New("allocated warehouse result needs shipmentId and warehouseId")
	}
	if !result.Allocated && len(result.MissingSKUs) == 0 {
		return errors.New("unavailable warehouse result needs missingSkus")
	}
	return nil
}

// DecideAllocateShipment is IR node fulfillment.decide_allocate_shipment.
func DecideAllocateShipment(command AllocateShipmentCommand, allocation WarehouseAllocation) (ShipmentAllocationResult, error) {
	if err := ValidateWarehouseAllocation(allocation); err != nil {
		return ShipmentAllocationResult{}, err
	}
	if allocation.Allocated {
		return ShipmentAllocationResult{Event: "ShipmentAllocated", Allocated: &ShipmentAllocated{
			OrderID: command.OrderID, ShipmentID: allocation.ShipmentID, WarehouseID: allocation.WarehouseID,
		}}, nil
	}
	return ShipmentAllocationResult{Event: "ShipmentAllocationDeferred", Deferred: &ShipmentAllocationDeferred{
		OrderID: command.OrderID, Reason: "inventory_unavailable", MissingSKUs: slices.Clone(allocation.MissingSKUs),
	}}, nil
}

// ProjectShipmentQueue is IR node fulfillment.project_shipment_queue.
func ProjectShipmentQueue(result ShipmentAllocationResult) ShipmentQueueView {
	if result.Allocated != nil {
		a := result.Allocated
		return ShipmentQueueView{ShipmentID: a.ShipmentID, OrderID: a.OrderID, WarehouseID: a.WarehouseID, Status: "queued_for_picking"}
	}
	d := result.Deferred
	return ShipmentQueueView{OrderID: d.OrderID, Status: "awaiting_inventory", MissingSKUs: slices.Clone(d.MissingSKUs)}
}

// RetryAllocation is IR node fulfillment.retry_allocation; nil means keep waiting.
func RetryAllocation(replenished InventoryReplenished, last ShipmentAllocationResult, view PaidOrdersToShipView) *AllocateShipmentCommand {
	if last.Deferred == nil || !slices.Contains(last.Deferred.MissingSKUs, replenished.SKU) {
		return nil
	}
	return &AllocateShipmentCommand{OrderID: view.OrderID, Items: slices.Clone(view.Items)}
}
