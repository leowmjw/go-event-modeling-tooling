package main

import (
	"context"
	"flag"
	"log"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
)

// replenish delivers rf 25 Warehouse.InventoryReplenished to a deferred Fulfillment child.
func main() {
	workflowID := flag.String("workflow-id", "checkout-c-100", "parent Temporal workflow ID")
	warehouseID := flag.String("warehouse-id", "wh-1", "replenished warehouse ID")
	sku := flag.String("sku", "sku-1", "replenished SKU")
	flag.Parse()

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	err = c.SignalWorkflow(context.Background(), *workflowID+orderfulfillment.FulfillmentChildSuffix, "", orderfulfillment.InventoryReplenishedSignal, orderfulfillment.InventoryReplenished{
		WarehouseID: *warehouseID,
		SKU:         *sku,
	})
	if err != nil {
		log.Fatal(err)
	}
}
