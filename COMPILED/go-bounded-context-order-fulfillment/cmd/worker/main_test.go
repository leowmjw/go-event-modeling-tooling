package main

import (
	"reflect"
	"testing"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
)

func TestEnvironmentAdaptersRejectUnknownValues(t *testing.T) {
	if _, err := paymentGatewayFromEnv("maybe"); err == nil {
		t.Fatal("expected invalid payment outcome to fail")
	}
	if _, err := warehouseFromEnv("maybe"); err == nil {
		t.Fatal("expected invalid inventory outcome to fail")
	}
}

func TestDeclinedGatewayGivesReason(t *testing.T) {
	gateway, err := paymentGatewayFromEnv("declined")
	if err != nil {
		t.Fatal(err)
	}
	got, err := gateway.Authorize(t.Context(), orderfulfillment.AuthorizePaymentCommand{OrderID: "ord-100", Amount: 125, PaymentMethodID: "pm-9"}, "ord-100")
	if err != nil {
		t.Fatal(err)
	}
	if want := (orderfulfillment.GatewayAuthorization{Reason: "insufficient_funds"}); got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestDeferredWarehouseAllocatesOnNextAttemptPerOrder(t *testing.T) {
	warehouse, err := warehouseFromEnv("deferred")
	if err != nil {
		t.Fatal(err)
	}
	items := []orderfulfillment.ShipmentItem{{SKU: "sku-1", Qty: 1}, {SKU: "sku-2", Qty: 1}}
	var got []orderfulfillment.WarehouseAllocation
	for _, call := range []struct{ order, key string }{
		{"ord-100", "ord-100/allocation-1"},
		{"ord-100", "ord-100/allocation-1"},
		{"ord-100", "ord-100/allocation-2"},
		{"ord-200", "ord-200/allocation-1"},
	} {
		result, err := warehouse.Allocate(t.Context(), orderfulfillment.AllocateShipmentCommand{OrderID: call.order, Items: items}, call.key)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, result)
	}
	deferred := orderfulfillment.WarehouseAllocation{MissingSKUs: []string{"sku-1"}}
	allocated := orderfulfillment.WarehouseAllocation{Allocated: true, ShipmentID: "sh-55", WarehouseID: "wh-1"}
	want := []orderfulfillment.WarehouseAllocation{deferred, deferred, allocated, deferred}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
