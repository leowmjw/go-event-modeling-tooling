package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
)

// starter delivers rf 21 Pricing.CartPriced by starting the parent workflow.
// It does not wait: Sales then waits at the CheckoutScreen gate (cmd/checkout).
func main() {
	workflowID := flag.String("workflow-id", "checkout-c-100", "parent Temporal workflow ID")
	emptyCart := flag.Bool("empty-cart", false, "price an empty cart (rejection scenario)")
	flag.Parse()

	cart := orderfulfillment.CartPriced{
		CartID: "c-100", CustomerID: "cust-7", Total: 125,
		Items: []orderfulfillment.LineItem{
			{SKU: "sku-1", Qty: 1, UnitPrice: 75},
			{SKU: "sku-2", Qty: 1, UnitPrice: 50},
		},
	}
	if *emptyCart {
		cart.Items, cart.Total = []orderfulfillment.LineItem{}, 0
	}

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID: *workflowID, TaskQueue: orderfulfillment.TaskQueue,
	}, orderfulfillment.OrderFulfillmentWorkflowName, orderfulfillment.OrderFulfillmentInput{CartPriced: cart})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("started %s (run %s); Sales waits at %s%s for the checkout signal\n",
		run.GetID(), run.GetRunID(), run.GetID(), orderfulfillment.SalesChildSuffix)
}
