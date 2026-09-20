package main

import (
	"context"
	"flag"
	"log"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
)

func main() {
	workflowID := flag.String("workflow-id", "order-ord-100", "parent Temporal workflow ID")
	warehouseID := flag.String("warehouse-id", "wh-1", "replenished warehouse ID")
	sku := flag.String("sku", "sku-1", "replenished SKU")
	flag.Parse()

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	childWorkflowID := *workflowID + "/fulfillment"
	if err := c.SignalWorkflow(context.Background(), childWorkflowID, "", orderfulfillment.InventoryReplenishedSignal, orderfulfillment.InventoryReplenished{
		WarehouseID: *warehouseID,
		SKU:         *sku,
	}); err != nil {
		log.Fatal(err)
	}
}
