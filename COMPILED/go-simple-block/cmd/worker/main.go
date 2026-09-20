package main

import (
	"log"

	"example.com/go-simple-block/simpleblock"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	w := worker.New(c, simpleblock.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(simpleblock.Workflow, workflow.RegisterOptions{Name: simpleblock.WorkflowName})
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
