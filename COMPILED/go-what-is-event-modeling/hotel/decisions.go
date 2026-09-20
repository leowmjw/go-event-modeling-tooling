package hotel

import "fmt"

func DecideSearchRooms(command SearchRoomsCommand) (RoomsSearched, error) {
	if command.RoomType == "" || command.CheckIn == "" || command.CheckOut == "" {
		return RoomsSearched{}, fmt.Errorf("roomType, checkIn, and checkOut are required")
	}
	rooms := []Room{{ID: "r-42", Type: command.RoomType, Price: 100}, {ID: "r-55", Type: command.RoomType, Price: 110}}
	return RoomsSearched{RoomType: command.RoomType, CheckIn: command.CheckIn, CheckOut: command.CheckOut, Rooms: rooms, Available: len(rooms)}, nil
}
func ProjectRoomList(event RoomsSearched) RoomList { return RoomList{Rooms: event.Rooms} }
func DecideBookRoom(command BookRoomCommand, search RoomsSearched) (BookRoomResult, error) {
	if command.BookingID == "" || command.RoomID == "" {
		return BookRoomResult{}, fmt.Errorf("bookingId and roomId are required")
	}
	for _, room := range search.Rooms {
		if room.ID == command.RoomID {
			booked := RoomBooked(command)
			return BookRoomResult{Event: "RoomBooked", Booked: &booked}, nil
		}
	}
	rejected := RoomBookingRejected{BookingID: command.BookingID, RoomID: command.RoomID, Reason: "room_unavailable"}
	return BookRoomResult{Event: "RoomBookingRejected", Rejected: &rejected}, nil
}
func ProjectBookingConfirmation(result BookRoomResult) (BookingConfirmation, error) {
	if result.Booked == nil {
		return BookingConfirmation{}, fmt.Errorf("a rejected booking has no confirmation")
	}
	b := result.Booked
	return BookingConfirmation{BookingID: b.BookingID, RoomID: b.RoomID, CheckIn: b.CheckIn, CheckOut: b.CheckOut}, nil
}
func DecideCheckIn(command CheckInCommand, booking RoomBooked) (CheckInResult, error) {
	if command.BookingID != booking.BookingID {
		return CheckInResult{}, fmt.Errorf("booking does not match")
	}
	event := GuestCheckedIn{BookingID: command.BookingID, RoomID: booking.RoomID}
	return CheckInResult{Event: "GuestCheckedIn", CheckedIn: &event}, nil
}
func ProjectRoomStatus(result CheckInResult) RoomStatus {
	if result.CheckedIn != nil {
		return RoomStatus{Status: "occupied"}
	}
	return RoomStatus{Status: "unchanged"}
}
func DecideCheckOut(command CheckOutCommand, checkIn CheckInResult) (CheckOutResult, error) {
	if checkIn.CheckedIn == nil || checkIn.CheckedIn.BookingID != command.BookingID {
		rejected := GuestCheckOutRejected{BookingID: command.BookingID, Reason: "not_checked_in"}
		return CheckOutResult{Event: "GuestCheckOutRejected", Rejected: &rejected}, nil
	}
	event := GuestCheckedOut{BookingID: command.BookingID, Nights: 4, NightlyRate: 100, Amount: 400}
	return CheckOutResult{Event: "GuestCheckedOut", CheckedOut: &event}, nil
}
func BillingProcessor(event GuestCheckedOut) TakePaymentCommand {
	return TakePaymentCommand{BookingID: event.BookingID, Nights: event.Nights, Amount: event.Amount}
}
func DecideTakePayment(command TakePaymentCommand) (PaymentResult, error) {
	if command.Amount <= 0 {
		rejected := PaymentRejected{BookingID: command.BookingID, Reason: "amount_must_be_positive"}
		return PaymentResult{Event: "PaymentRejected", Rejected: &rejected}, nil
	}
	event := PaymentTaken(command)
	return PaymentResult{Event: "PaymentTaken", Taken: &event}, nil
}
func ProjectInvoice(result PaymentResult) (Invoice, error) {
	if result.Taken == nil {
		return Invoice{}, fmt.Errorf("a rejected payment has no invoice")
	}
	return Invoice(*result.Taken), nil
}
