package orderfulfillment

import (
	"reflect"
	"strings"
	"testing"
)

// Fixture values from testdata/fixtures/bounded-context-order-fulfillment.evml.

func cartPriced() CartPriced {
	return CartPriced{CartID: "c-100", CustomerID: "cust-7", Items: lineItems(), Total: 125}
}

func lineItems() []LineItem {
	return []LineItem{{SKU: "sku-1", Qty: 1, UnitPrice: 75}, {SKU: "sku-2", Qty: 1, UnitPrice: 50}}
}

func shipmentItems() []ShipmentItem {
	return []ShipmentItem{{SKU: "sku-1", Qty: 1}, {SKU: "sku-2", Qty: 1}}
}

func placeOrderCommand() PlaceOrderCommand {
	return PlaceOrderCommand{OrderID: "ord-100", CartID: "c-100", CustomerID: "cust-7", PaymentMethodID: "pm-9"}
}

// orderPlaced04 is data OrderPlaced04.
func orderPlaced04() *OrderPlaced {
	return &OrderPlaced{OrderID: "ord-100", CartID: "c-100", CustomerID: "cust-7", PaymentMethodID: "pm-9", Items: lineItems(), Total: 125}
}

func placedResult() PlaceOrderResult {
	return PlaceOrderResult{Event: "OrderPlaced", Placed: orderPlaced04()}
}

func authorizePaymentCommand() AuthorizePaymentCommand {
	return AuthorizePaymentCommand{OrderID: "ord-100", Amount: 125, PaymentMethodID: "pm-9"}
}

func allocateShipmentCommand() AllocateShipmentCommand {
	return AllocateShipmentCommand{OrderID: "ord-100", Items: shipmentItems()}
}

func deferredResult() ShipmentAllocationResult {
	return ShipmentAllocationResult{Event: "ShipmentAllocationDeferred", Deferred: &ShipmentAllocationDeferred{
		OrderID: "ord-100", Reason: "inventory_unavailable", MissingSKUs: []string{"sku-1"},
	}}
}

func readyToShip() PaidOrdersToShipView {
	return PaidOrdersToShipView{OrderID: "ord-100", Items: shipmentItems(), Status: "ready_to_ship"}
}

func assertEqual[T any](t *testing.T, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %#v\nwant %#v", got, want)
	}
}

func TestDataCartSummary02(t *testing.T) {
	got, err := ProjectCartSummary(cartPriced())
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, got, CartSummaryView{CartID: "c-100", CustomerID: "cust-7", Items: lineItems(), Total: 125})
}

func TestGWT03PlaceOrder(t *testing.T) {
	tests := []struct {
		name string
		cart CartPriced
		want PlaceOrderResult
	}{
		{
			name: "place order from a priced cart",
			cart: cartPriced(),
			want: PlaceOrderResult{Event: "OrderPlaced", Placed: orderPlaced04()},
		},
		{
			name: "reject order when cart has no priced items",
			cart: CartPriced{CartID: "c-100", CustomerID: "cust-7", Items: []LineItem{}, Total: 0},
			want: PlaceOrderResult{Event: "OrderPlacementRejected", Rejected: &OrderPlacementRejected{OrderID: "ord-100", CartID: "c-100", Reason: "empty_cart"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cart, err := ProjectCartSummary(tc.cart)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecidePlaceOrder(placeOrderCommand(), cart)
			if err != nil {
				t.Fatal(err)
			}
			assertEqual(t, got, tc.want)
		})
	}
}

func TestDecidePlaceOrderRejectsInvalidCommands(t *testing.T) {
	cart, err := ProjectCartSummary(cartPriced())
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*PlaceOrderCommand){
		"other cart":            func(c *PlaceOrderCommand) { c.CartID = "c-999" },
		"other customer":        func(c *PlaceOrderCommand) { c.CustomerID = "cust-9" },
		"missing orderId":       func(c *PlaceOrderCommand) { c.OrderID = "" },
		"missing paymentMethod": func(c *PlaceOrderCommand) { c.PaymentMethodID = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			command := placeOrderCommand()
			mutate(&command)
			if _, err := DecidePlaceOrder(command, cart); err == nil {
				t.Fatal("expected invalid input error")
			}
		})
	}
}

func TestGWT05OrderStatus(t *testing.T) {
	confirmed := &OrderConfirmationResult{Event: "OrderConfirmed", OrderID: "ord-100", AuthorizationID: "auth-1"}
	cancelled := &OrderCancellationResult{Event: "OrderCancelled", OrderID: "ord-100", Reason: "payment_declined"}
	tests := []struct {
		name         string
		confirmation *OrderConfirmationResult
		cancellation *OrderCancellationResult
		status       string
	}{
		{name: "order awaits payment once placed (data OrderStatus05)", status: "pending_payment"},
		{name: "order is confirmed after payment is authorized", confirmation: confirmed, status: "confirmed"},
		{name: "order is cancelled after payment declines", cancellation: cancelled, status: "cancelled"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ProjectOrderStatus(placedResult(), tc.confirmation, tc.cancellation)
			assertEqual(t, got, &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: tc.status})
		})
	}
	rejected := PlaceOrderResult{Event: "OrderPlacementRejected", Rejected: &OrderPlacementRejected{OrderID: "ord-100", CartID: "c-100", Reason: "empty_cart"}}
	if got := ProjectOrderStatus(rejected, nil, nil); got != nil {
		t.Fatalf("rejected placement has no OrderStatus, got %#v", got)
	}
}

func TestGWT32ConfirmOrder(t *testing.T) {
	command, err := TranslateSalesConfirmation(SalesPaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1"})
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, command, ConfirmOrderCommand{OrderID: "ord-100", AuthorizationID: "auth-1"})

	t.Run("confirm order after payment is authorized", func(t *testing.T) {
		got, err := DecideConfirmOrder(command, placedResult(), []RecordedEvent{{Event: "OrderPlaced", OrderID: "ord-100"}})
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, got, OrderConfirmationResult{Event: "OrderConfirmed", OrderID: "ord-100", AuthorizationID: "auth-1"})
	})
	t.Run("ignore confirmation when order is already confirmed", func(t *testing.T) {
		history := []RecordedEvent{{Event: "OrderPlaced", OrderID: "ord-100"}, {Event: "OrderConfirmed", OrderID: "ord-100"}}
		got, err := DecideConfirmOrder(command, placedResult(), history)
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, got, OrderConfirmationResult{Event: "OrderConfirmationIgnored", OrderID: "ord-100", Reason: "already_confirmed"})
	})
	t.Run("never placed", func(t *testing.T) {
		if _, err := DecideConfirmOrder(command, PlaceOrderResult{}, nil); err == nil {
			t.Fatal("expected error for an order that was never placed")
		}
	})
}

func TestGWT18CancelOrder(t *testing.T) {
	command, err := TranslateSalesRecovery(SalesPaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"})
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, command, CancelOrderCommand{OrderID: "ord-100", Reason: "payment_declined"})

	t.Run("cancel order after payment declines", func(t *testing.T) {
		got, err := DecideCancelOrder(command, placedResult(), []RecordedEvent{{Event: "OrderPlaced", OrderID: "ord-100"}})
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, got, OrderCancellationResult{Event: "OrderCancelled", OrderID: "ord-100", Reason: "payment_declined"})
	})
	t.Run("ignore cancellation when order is already cancelled", func(t *testing.T) {
		history := []RecordedEvent{{Event: "OrderPlaced", OrderID: "ord-100"}, {Event: "OrderCancelled", OrderID: "ord-100"}}
		got, err := DecideCancelOrder(command, placedResult(), history)
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, got, OrderCancellationResult{Event: "OrderCancellationIgnored", OrderID: "ord-100", Reason: "already_cancelled"})
	})
	t.Run("never placed", func(t *testing.T) {
		if _, err := DecideCancelOrder(command, PlaceOrderResult{}, nil); err == nil {
			t.Fatal("expected error for an order that was never placed")
		}
	})
}

func TestGWT08AuthorizePayment(t *testing.T) {
	command, err := TranslateBilling(SalesOrderPlaced{OrderID: "ord-100", PaymentMethodID: "pm-9", Total: 125})
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, command, authorizePaymentCommand())

	tests := []struct {
		name    string
		gateway GatewayAuthorization
		want    PaymentAuthorizationResult
	}{
		{
			name:    "authorize payment when the gateway approves the charge",
			gateway: GatewayAuthorization{Approved: true, AuthorizationID: "auth-1"},
			want:    PaymentAuthorizationResult{Event: "PaymentAuthorized", Authorized: &PaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1", Amount: 125}},
		},
		{
			name:    "decline payment when the gateway declines the charge",
			gateway: GatewayAuthorization{Reason: "insufficient_funds"},
			want:    PaymentAuthorizationResult{Event: "PaymentDeclined", Declined: &PaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecideAuthorizePayment(command, tc.gateway)
			if err != nil {
				t.Fatal(err)
			}
			assertEqual(t, got, tc.want)
		})
	}
	for _, bad := range []GatewayAuthorization{{Approved: true}, {}} {
		if _, err := DecideAuthorizePayment(command, bad); err == nil {
			t.Fatalf("expected incomplete gateway result %#v to fail", bad)
		}
	}
}

func TestGWT10PaymentStatus(t *testing.T) {
	authorized := ProjectPaymentStatus(PaymentAuthorizationResult{Event: "PaymentAuthorized", Authorized: &PaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1", Amount: 125}})
	assertEqual(t, authorized, PaymentStatusView{OrderID: "ord-100", Status: "authorized"})
	declined := ProjectPaymentStatus(PaymentAuthorizationResult{Event: "PaymentDeclined", Declined: &PaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}})
	assertEqual(t, declined, PaymentStatusView{OrderID: "ord-100", Status: "declined", Reason: "insufficient_funds"})
}

func TestGWT29PaidOrdersToShip(t *testing.T) {
	order := FulfillmentOrderPlaced{OrderID: "ord-100", Items: shipmentItems()}
	t.Run("paid order is ready to ship (data PaidOrdersToShip29)", func(t *testing.T) {
		got, err := ProjectPaidOrdersToShip(order, &FulfillmentPaymentAuthorized{OrderID: "ord-100", Amount: 125})
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, got, readyToShip())
	})
	t.Run("unpaid order is not ready to ship", func(t *testing.T) {
		got, err := ProjectPaidOrdersToShip(order, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(t, got, PaidOrdersToShipView{OrderID: "ord-100", Items: shipmentItems(), Status: "awaiting_payment"})
		if _, err := TranslateFulfillment(got); err == nil {
			t.Fatal("FulfillmentTranslator must refuse an unpaid order")
		}
	})
}

func TestGWT13AllocateShipment(t *testing.T) {
	command, err := TranslateFulfillment(readyToShip())
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, command, allocateShipmentCommand())

	tests := []struct {
		name       string
		allocation WarehouseAllocation
		want       ShipmentAllocationResult
	}{
		{
			name:       "allocate shipment when inventory is available",
			allocation: WarehouseAllocation{Allocated: true, ShipmentID: "sh-55", WarehouseID: "wh-1"},
			want:       ShipmentAllocationResult{Event: "ShipmentAllocated", Allocated: &ShipmentAllocated{OrderID: "ord-100", ShipmentID: "sh-55", WarehouseID: "wh-1"}},
		},
		{
			name:       "defer shipment when inventory is not available",
			allocation: WarehouseAllocation{MissingSKUs: []string{"sku-1"}},
			want:       deferredResult(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecideAllocateShipment(command, tc.allocation)
			if err != nil {
				t.Fatal(err)
			}
			assertEqual(t, got, tc.want)
		})
	}
	for _, bad := range []WarehouseAllocation{{Allocated: true, ShipmentID: "sh-55"}, {Allocated: true, WarehouseID: "wh-1"}, {}} {
		if _, err := DecideAllocateShipment(command, bad); err == nil {
			t.Fatalf("expected incomplete warehouse result %#v to fail", bad)
		}
	}
}

func TestGWT15ShipmentQueue(t *testing.T) {
	queued := ProjectShipmentQueue(ShipmentAllocationResult{Event: "ShipmentAllocated", Allocated: &ShipmentAllocated{OrderID: "ord-100", ShipmentID: "sh-55", WarehouseID: "wh-1"}})
	assertEqual(t, queued, ShipmentQueueView{ShipmentID: "sh-55", OrderID: "ord-100", WarehouseID: "wh-1", Status: "queued_for_picking"})
	waiting := ProjectShipmentQueue(deferredResult())
	assertEqual(t, waiting, ShipmentQueueView{OrderID: "ord-100", Status: "awaiting_inventory", MissingSKUs: []string{"sku-1"}})
}

func TestGWT26RetryAllocation(t *testing.T) {
	t.Run("retry allocation when a missing sku is replenished", func(t *testing.T) {
		got := RetryAllocation(InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-1"}, deferredResult(), readyToShip())
		want := allocateShipmentCommand()
		assertEqual(t, got, &want)
	})
	t.Run("keep waiting when an unrelated sku is replenished", func(t *testing.T) {
		if got := RetryAllocation(InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-9"}, deferredResult(), readyToShip()); got != nil {
			t.Fatalf("unexpected retry %#v", got)
		}
		assertEqual(t, ProjectShipmentQueue(deferredResult()), ShipmentQueueView{OrderID: "ord-100", Status: "awaiting_inventory", MissingSKUs: []string{"sku-1"}})
	})
}

func TestAllocationIdempotencyKey(t *testing.T) {
	if got := AllocationIdempotencyKey("ord-100", 2); got != "ord-100/allocation-2" {
		t.Fatalf("got %q", got)
	}
	if !strings.HasPrefix(AllocationIdempotencyKey("ord-100", 1), "ord-100/") {
		t.Fatal("key must be scoped to the order")
	}
}
