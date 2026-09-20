package hotel

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
)

func TestWorkflowSuccessPath(t *testing.T) {
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(BookRoomSignal, BookRoomCommand{BookingID: "b-1", RoomID: "r-42", GuestID: "g-7", CheckIn: "2024-06-01", CheckOut: "2024-06-05"})
	}, time.Second)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(CheckInSignal, CheckInCommand{BookingID: "b-1"}) }, 2*time.Second)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(CheckOutSignal, CheckOutCommand{BookingID: "b-1"}) }, 3*time.Second)
	env.ExecuteWorkflow(Workflow, SearchRoomsCommand{RoomType: "double", CheckIn: "2024-06-01", CheckOut: "2024-06-05"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result WorkflowResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Search.Available != 2 || result.Booking.Event != "RoomBooked" || result.CheckIn.Event != "GuestCheckedIn" || result.CheckOut.Event != "GuestCheckedOut" || result.Payment.Event != "PaymentTaken" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Invoice != (Invoice{BookingID: "b-1", Nights: 4, Amount: 400}) {
		t.Fatalf("invoice = %#v", result.Invoice)
	}
}

func TestDecisionRejections(t *testing.T) {
	booking, err := DecideBookRoom(BookRoomCommand{BookingID: "b-2", RoomID: "missing"}, RoomsSearched{Rooms: []Room{}})
	if err != nil || booking.Rejected == nil || booking.Rejected.Reason != "room_unavailable" {
		t.Fatalf("booking = %#v, err = %v", booking, err)
	}
	checkout, err := DecideCheckOut(CheckOutCommand{BookingID: "b-1"}, CheckInResult{Event: "GuestCheckInRejected"})
	if err != nil || checkout.Rejected == nil || checkout.Rejected.Reason != "not_checked_in" {
		t.Fatalf("checkout = %#v, err = %v", checkout, err)
	}
	payment, err := DecideTakePayment(TakePaymentCommand{BookingID: "b-1", Nights: 4, Amount: 0})
	if err != nil || payment.Rejected == nil || payment.Rejected.Reason != "amount_must_be_positive" {
		t.Fatalf("payment = %#v, err = %v", payment, err)
	}
}
