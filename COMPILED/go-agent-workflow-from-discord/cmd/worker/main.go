package main

import (
	"context"
	"example.com/go-agent-workflow-from-discord/agentworkflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"log"
)

type repository struct{}

func (repository) FindInvoices(context.Context, agentworkflow.InvoiceSearchRequested) (agentworkflow.RepositoryResult, error) {
	return agentworkflow.RepositoryResult{ConversationID: "conv-1001", Count: 2, Customers: []string{"X", "Y"}}, nil
}
func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	w := worker.New(c, agentworkflow.TaskQueue, worker.Options{})
	agentworkflow.Register(w, &agentworkflow.Activities{Dependencies: repository{}})
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
