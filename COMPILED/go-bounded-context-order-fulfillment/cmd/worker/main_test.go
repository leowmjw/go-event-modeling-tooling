package main

import (
	"testing"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
)

func TestEnvironmentAdaptersRejectUnknownValues(t *testing.T) {
	if _, err := paymentGatewayFromEnv("maybe"); err == nil {
		t.Fatal("expected invalid payment outcome to fail")
	}
	if _, err := inventoryFromEnv("maybe"); err == nil {
		t.Fatal("expected invalid inventory outcome to fail")
	}
}

func TestDeferredInventoryAllocatesAfterReplenishmentRetry(t *testing.T) {
	service, err := inventoryFromEnv("deferred")
	if err != nil {
		t.Fatal(err)
	}
	command := orderfulfillment.AllocateShipmentCommand{OrderID: "ord-100", WarehouseID: "wh-1"}
	first, err := service.Allocate(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Allocate(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	if first.Available || !second.Available {
		t.Fatalf("unexpected inventory sequence: first=%#v second=%#v", first, second)
	}
}
