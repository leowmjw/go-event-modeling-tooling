from extracted.main import (
    billing_processor,
    decide_book_room,
    decide_check_in,
    decide_check_out,
    decide_search_rooms,
    decide_take_payment,
    project_booking_confirmation,
    project_invoice,
    project_room_list,
    project_room_status,
)


def test_hotel_success_path_and_data_blocks() -> None:
    search = decide_search_rooms({"roomType": "double", "checkIn": "2024-06-01", "checkOut": "2024-06-05"})
    assert project_room_list(search) == {"rooms": [{"id": "r-42", "type": "double", "price": 100}, {"id": "r-55", "type": "double", "price": 110}]}
    booking = decide_book_room({"bookingId": "b-1", "roomId": "r-42", "guestId": "g-7", "checkIn": "2024-06-01", "checkOut": "2024-06-05"}, search)
    assert project_booking_confirmation(booking) == {"bookingId": "b-1", "roomId": "r-42", "checkIn": "2024-06-01", "checkOut": "2024-06-05"}
    check_in = decide_check_in({"bookingId": "b-1"}, booking)
    assert check_in == {"event": "GuestCheckedIn", "bookingId": "b-1", "roomId": "r-42"}
    assert project_room_status(check_in) == {"status": "occupied"}
    check_out = decide_check_out({"bookingId": "b-1"}, check_in)
    assert check_out == {"event": "GuestCheckedOut", "bookingId": "b-1", "nights": 4, "nightlyRate": 100, "amount": 400}
    payment = decide_take_payment(billing_processor(check_out))
    assert payment == {"event": "PaymentTaken", "bookingId": "b-1", "nights": 4, "amount": 400}
    assert project_invoice(payment) == {"bookingId": "b-1", "nights": 4, "amount": 400}


def test_rejection_branches() -> None:
    search = {"rooms": []}
    assert decide_book_room({"bookingId": "b-2", "roomId": "r-42"}, search)["event"] == "RoomBookingRejected"
    assert decide_check_in({"bookingId": "b-1"}, {"roomId": "r-42"}, {"event": "GuestCheckedIn"})["event"] == "GuestCheckInRejected"
    assert decide_check_out({"bookingId": "b-1"}, {"event": "RoomBooked"})["event"] == "GuestCheckOutRejected"
    assert decide_take_payment({"bookingId": "b-1", "nights": 4, "amount": 0}) == {"event": "PaymentRejected", "bookingId": "b-1", "reason": "amount_must_be_positive"}
