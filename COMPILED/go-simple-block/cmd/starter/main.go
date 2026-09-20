package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"example.com/go-simple-block/simpleblock"
	"go.temporal.io/sdk/client"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	input := simpleblock.WorkflowInput{Command: simpleblock.AddItemCommand{ItemID: "item-1", Description: "john", Image: "avatar_john", Price: 20.4}}
	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{ID: "simple-block-item-1", TaskQueue: simpleblock.TaskQueue}, simpleblock.WorkflowName, input)
	if err != nil {
		log.Fatal(err)
	}
	var result simpleblock.WorkflowResult
	if err := run.Get(context.Background(), &result); err != nil {
		log.Fatal(err)
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
}
