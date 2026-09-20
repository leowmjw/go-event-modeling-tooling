package main

import (
	"context"
	"example.com/go-what-is-event-modeling/hotel"
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
	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{ID: "hotel-booking-b-1", TaskQueue: hotel.TaskQueue}, hotel.WorkflowName, hotel.SearchRoomsCommand{RoomType: "double", CheckIn: "2024-06-01", CheckOut: "2024-06-05"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(run.GetID())
}
