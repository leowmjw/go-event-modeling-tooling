package main

import (
	"context"
	"flag"
	"log"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
)

// checkout is the CheckoutScreen: it sends cmd PlaceOrder to the Sales gate.
func main() {
	workflowID := flag.String("workflow-id", "checkout-c-100", "parent Temporal workflow ID")
	orderID := flag.String("order-id", "ord-100", "orderId assigned by CheckoutScreen")
	paymentMethodID := flag.String("payment-method", "pm-9", "payment method the customer picked")
	flag.Parse()

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	err = c.SignalWorkflow(context.Background(), *workflowID+orderfulfillment.SalesChildSuffix, "", orderfulfillment.PlaceOrderSignal, orderfulfillment.PlaceOrderCommand{
		OrderID: *orderID, CartID: "c-100", CustomerID: "cust-7", PaymentMethodID: *paymentMethodID,
	})
	if err != nil {
		log.Fatal(err)
	}
}
