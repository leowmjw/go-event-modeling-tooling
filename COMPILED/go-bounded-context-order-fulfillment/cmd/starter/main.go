package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
)

func main() {
	workflowID := flag.String("workflow-id", "order-ord-100", "Temporal workflow ID")
	total := flag.Float64("total", 125, "order total")
	flag.Parse()

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	input := orderfulfillment.OrderFulfillmentInput{
		Cart: orderfulfillment.CartPriced{
			CartID: "c-100", CustomerID: "cust-7", Total: *total,
			Items: []orderfulfillment.LineItem{
				{SKU: "sku-1", Quantity: 1, UnitPrice: 75},
				{SKU: "sku-2", Quantity: 1, UnitPrice: 50},
			},
		},
		Command:         orderfulfillment.PlaceOrderCommand{CartID: "c-100", CustomerID: "cust-7", Total: *total},
		PaymentMethodID: "pm-9",
		WarehouseID:     "wh-1",
	}
	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID: *workflowID, TaskQueue: orderfulfillment.TaskQueue,
	}, orderfulfillment.OrderFulfillmentWorkflowName, input)
	if err != nil {
		log.Fatal(err)
	}
	var result orderfulfillment.OrderFulfillmentResult
	if err := run.Get(context.Background(), &result); err != nil {
		log.Fatal(err)
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
}
