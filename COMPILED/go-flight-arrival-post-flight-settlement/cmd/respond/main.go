package main

import (
	"context"
	"example.com/go-flight-arrival-post-flight-settlement/settlement"
	"go.temporal.io/sdk/client"
	"log"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	err = c.SignalWorkflow(context.Background(), "flight-HLT-421/marketing", "", settlement.RecoveryResponseSignal, settlement.RespondToRecoveryOfferCommand{OfferID: "offer-77", TravelerID: "visitor-a", Response: "accepted", RespondedAt: "2024-06-15T17:05:00Z"})
	if err != nil {
		log.Fatal(err)
	}
}
