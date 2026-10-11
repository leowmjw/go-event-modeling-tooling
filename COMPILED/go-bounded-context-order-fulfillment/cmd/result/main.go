package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"example.com/go-bounded-context-order-fulfillment/orderfulfillment"
	"go.temporal.io/sdk/client"
)

// result waits for the parent workflow and prints its result as JSON.
func main() {
	workflowID := flag.String("workflow-id", "checkout-c-100", "parent Temporal workflow ID")
	timeout := flag.Duration("timeout", 30*time.Second, "how long to wait for completion")
	flag.Parse()

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	var result orderfulfillment.OrderFulfillmentResult
	if err := c.GetWorkflow(ctx, *workflowID, "").Get(ctx, &result); err != nil {
		log.Fatal(err)
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
}
