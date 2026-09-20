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
	err = c.SignalWorkflow(context.Background(), "hotel-booking-b-1", "", hotel.BookRoomSignal, hotel.BookRoomCommand{BookingID: "b-1", RoomID: "r-42", GuestID: "g-7", CheckIn: "2024-06-01", CheckOut: "2024-06-05"})
	if err != nil {
		log.Fatal(err)
	}
}
