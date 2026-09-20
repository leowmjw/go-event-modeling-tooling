package hotel

import (
	"fmt"

	"go.temporal.io/sdk/workflow"
)

func Workflow(ctx workflow.Context, input SearchRoomsCommand) (WorkflowResult, error) {
	search, err := DecideSearchRooms(input)
	if err != nil {
		return WorkflowResult{}, err
	}
	result := WorkflowResult{Search: search, RoomList: ProjectRoomList(search)}
	if err := workflow.SetQueryHandler(ctx, RoomListQuery, func() (RoomList, error) { return result.RoomList, nil }); err != nil {
		return WorkflowResult{}, err
	}

	var bookCommand BookRoomCommand
	workflow.GetSignalChannel(ctx, BookRoomSignal).Receive(ctx, &bookCommand)
	result.Booking, err = DecideBookRoom(bookCommand, search)
	if err != nil {
		return WorkflowResult{}, err
	}
	if result.Booking.Booked == nil {
		return result, nil
	}
	result.Confirmation, err = ProjectBookingConfirmation(result.Booking)
	if err != nil {
		return WorkflowResult{}, err
	}
	if err := workflow.SetQueryHandler(ctx, BookingQuery, func() (BookingConfirmation, error) { return result.Confirmation, nil }); err != nil {
		return WorkflowResult{}, err
	}

	var checkInCommand CheckInCommand
	workflow.GetSignalChannel(ctx, CheckInSignal).Receive(ctx, &checkInCommand)
	result.CheckIn, err = DecideCheckIn(checkInCommand, *result.Booking.Booked)
	if err != nil {
		return WorkflowResult{}, err
	}
	result.RoomStatus = ProjectRoomStatus(result.CheckIn)
	if err := workflow.SetQueryHandler(ctx, RoomStatusQuery, func() (RoomStatus, error) { return result.RoomStatus, nil }); err != nil {
		return WorkflowResult{}, err
	}

	var checkOutCommand CheckOutCommand
	workflow.GetSignalChannel(ctx, CheckOutSignal).Receive(ctx, &checkOutCommand)
	result.CheckOut, err = DecideCheckOut(checkOutCommand, result.CheckIn)
	if err != nil {
		return WorkflowResult{}, err
	}
	if result.CheckOut.CheckedOut == nil {
		return result, nil
	}
	paymentCommand := BillingProcessor(*result.CheckOut.CheckedOut)
	result.Payment, err = DecideTakePayment(paymentCommand)
	if err != nil {
		return WorkflowResult{}, err
	}
	if result.Payment.Taken == nil {
		return result, nil
	}
	result.Invoice, err = ProjectInvoice(result.Payment)
	if err != nil {
		return WorkflowResult{}, err
	}
	if result.Invoice.BookingID == "" {
		return WorkflowResult{}, fmt.Errorf("invoice is mandatory")
	}
	return result, nil
}
