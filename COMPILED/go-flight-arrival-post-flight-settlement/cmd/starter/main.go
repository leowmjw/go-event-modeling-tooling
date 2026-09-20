package main

import (
	"context"
	"example.com/go-flight-arrival-post-flight-settlement/settlement"
	"flag"
	"fmt"
	"go.temporal.io/sdk/client"
	"log"
)

func main() {
	onTime := flag.Bool("on-time", false, "run the on-time closure path")
	flag.Parse()
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	touch := "2024-06-15T14:32:00Z"
	if *onTime {
		touch = "2024-06-15T14:28:00Z"
	}
	input := settlement.ParentInput{Operations: settlement.OperationsInput{Flight: settlement.Flight{FlightID: "HLT-421", Origin: "JFK", Destination: "LAX", ScheduledArrival: "2024-06-15T14:30:00Z", CurrentStatus: "in_flight", AircraftID: "N42HL"}, WheelsDown: settlement.RecordWheelsDownCommand{FlightID: "HLT-421", WheelsTouchTime: touch, AircraftID: "N42HL"}, GateArrival: settlement.RecordGateArrivalCommand{FlightID: "HLT-421", GateDoorOpenTime: "2024-06-15T14:47:00Z", GateID: "B12"}, OnTime: *onTime}, PassengerIDs: []string{"pax-1", "pax-2", "pax-3"}, ApprovedAt: "2024-06-15T15:10:00Z", ScheduledDate: "2024-06-16", OfferID: "offer-77", Channel: "email", SentAt: "2024-06-15T16:00:00Z"}
	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{ID: "flight-HLT-421", TaskQueue: settlement.TaskQueue}, settlement.ParentWorkflowName, input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(run.GetID())
}
