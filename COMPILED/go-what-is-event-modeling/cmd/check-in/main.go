package main

import (
	"context"
	"example.com/go-what-is-event-modeling/hotel"
	"go.temporal.io/sdk/client"
	"log"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	err = c.SignalWorkflow(context.Background(), "hotel-booking-b-1", "", hotel.CheckInSignal, hotel.CheckInCommand{BookingID: "b-1"})
	if err != nil {
		log.Fatal(err)
	}
}
