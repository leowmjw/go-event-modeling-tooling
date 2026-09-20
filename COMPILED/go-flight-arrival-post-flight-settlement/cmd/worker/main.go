package main

import (
	"example.com/go-flight-arrival-post-flight-settlement/settlement"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"log"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	w := worker.New(c, settlement.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(settlement.ParentWorkflow, workflow.RegisterOptions{Name: settlement.ParentWorkflowName})
	w.RegisterWorkflowWithOptions(settlement.OperationsWorkflow, workflow.RegisterOptions{Name: settlement.OperationsWorkflowName})
	w.RegisterWorkflowWithOptions(settlement.CompensationWorkflow, workflow.RegisterOptions{Name: settlement.CompensationWorkflowName})
	w.RegisterWorkflowWithOptions(settlement.FinanceWorkflow, workflow.RegisterOptions{Name: settlement.FinanceWorkflowName})
	w.RegisterWorkflowWithOptions(settlement.MarketingWorkflow, workflow.RegisterOptions{Name: settlement.MarketingWorkflowName})
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
