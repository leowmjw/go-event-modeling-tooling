package main

import (
	"example.com/go-what-is-event-modeling/hotel"
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
	w := worker.New(c, hotel.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(hotel.Workflow, workflow.RegisterOptions{Name: hotel.WorkflowName})
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
