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
	inventory, err := inventoryFromEnv(os.Getenv("INVENTORY_OUTCOME"))
	if err != nil {
		log.Fatal(err)
	}
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	w := worker.New(c, orderfulfillment.TaskQueue, worker.Options{})
	if err := orderfulfillment.Register(w, &orderfulfillment.Activities{PaymentGateway: paymentGateway, Inventory: inventory}); err != nil {
		log.Fatal(err)
	}
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}

func paymentGatewayFromEnv(value string) (orderfulfillment.PaymentGateway, error) {
	switch value {
	case "", "authorized":
		return orderfulfillment.StaticPaymentGateway{Response: orderfulfillment.PaymentGatewayResponse{Authorized: true, AuthorizationID: "auth-1"}}, nil
	case "declined":
		return orderfulfillment.StaticPaymentGateway{Response: orderfulfillment.PaymentGatewayResponse{DeclineReason: "issuer_declined"}}, nil
	default:
		return nil, fmt.Errorf("PAYMENT_OUTCOME must be authorized or declined, got %q", value)
	}
}

func inventoryFromEnv(value string) (orderfulfillment.InventoryService, error) {
	allocated := orderfulfillment.InventoryResponse{Available: true, ShipmentID: "sh-55"}
	switch value {
	case "", "allocated":
		return orderfulfillment.StaticInventoryService{Response: allocated}, nil
	case "deferred":
		return &orderfulfillment.SequencedInventoryService{Responses: []orderfulfillment.InventoryResponse{{}, allocated}}, nil
	default:
		return nil, fmt.Errorf("INVENTORY_OUTCOME must be allocated or deferred, got %q", value)
	}
}
