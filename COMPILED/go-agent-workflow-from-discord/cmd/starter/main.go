package main

import (
	"context"
	"example.com/go-agent-workflow-from-discord/agentworkflow"
	"fmt"
	"go.temporal.io/sdk/client"
	"log"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	in := agentworkflow.AskQuestion{ConversationID: "conv-1001", Question: "How many invoices were sent out yesterday?", RequestedAt: "2026-09-20T13:00:00Z", BusinessTimezone: "America/New_York"}
	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{ID: "agent-workflow-conv-1001", TaskQueue: agentworkflow.TaskQueue}, agentworkflow.ParentWorkflowName, in)
	if err != nil {
		log.Fatal(err)
	}
	var out agentworkflow.ParentResult
	if err := run.Get(context.Background(), &out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", out)
}
