package main

import (
	"fmt"
	"log"
	"os"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	paymentGateway, err := paymentGatewayFromEnv(os.Getenv("PAYMENT_OUTCOME"))
	if err != nil {
		log.Fatal(err)
	}
	warehouse, err := warehouseFromEnv(os.Getenv("INVENTORY_OUTCOME"))
	if err != nil {
		log.Fatal(err)
	}
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	w := worker.New(c, orderfulfillment.TaskQueue, worker.Options{})
	if err := orderfulfillment.Register(w, &orderfulfillment.Activities{PaymentGateway: paymentGateway, Warehouse: warehouse}); err != nil {
		log.Fatal(err)
	}
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}

func paymentGatewayFromEnv(value string) (orderfulfillment.PaymentGateway, error) {
	switch value {
	case "", "approved":
		return &orderfulfillment.DemoPaymentGateway{Result: orderfulfillment.GatewayAuthorization{Approved: true, AuthorizationID: "auth-1"}}, nil
	case "declined":
		return &orderfulfillment.DemoPaymentGateway{Result: orderfulfillment.GatewayAuthorization{Reason: "insufficient_funds"}}, nil
	default:
		return nil, fmt.Errorf("PAYMENT_OUTCOME must be approved or declined, got %q", value)
	}
}

func warehouseFromEnv(value string) (orderfulfillment.WarehouseInventory, error) {
	warehouse := &orderfulfillment.DemoWarehouse{ShipmentID: "sh-55", WarehouseID: "wh-1"}
	switch value {
	case "", "allocated":
		return warehouse, nil
	case "deferred":
		warehouse.UnavailableAttempts = 1
		warehouse.MissingSKUs = []string{"sku-1"}
		return warehouse, nil
	default:
		return nil, fmt.Errorf("INVENTORY_OUTCOME must be allocated or deferred, got %q", value)
	}
}
