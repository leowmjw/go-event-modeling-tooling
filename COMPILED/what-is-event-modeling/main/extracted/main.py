from __future__ import annotations


def decide_search_rooms(command: dict) -> dict:
    rooms = [{"id": "r-42", "type": command["roomType"], "price": 100}, {"id": "r-55", "type": command["roomType"], "price": 110}]
    return {"roomType": command["roomType"], "checkIn": command["checkIn"], "checkOut": command["checkOut"], "rooms": rooms, "available": len(rooms)}


def project_room_list(event: dict) -> dict:
    return {"rooms": event["rooms"]}


def decide_book_room(command: dict, search: dict) -> dict:
    if not any(room["id"] == command["roomId"] for room in search["rooms"]):
        return {"event": "RoomBookingRejected", "bookingId": command["bookingId"], "roomId": command["roomId"], "reason": "room_unavailable"}
    return {"event": "RoomBooked", **command}


def project_booking_confirmation(event: dict) -> dict:
    return {key: event[key] for key in ("bookingId", "roomId", "checkIn", "checkOut")}


def decide_check_in(command: dict, booking: dict, prior_event: dict | None = None) -> dict:
    if prior_event and prior_event.get("event") == "GuestCheckedIn":
        return {"event": "GuestCheckInRejected", "bookingId": command["bookingId"], "reason": "already_checked_in"}
    return {"event": "GuestCheckedIn", "bookingId": command["bookingId"], "roomId": booking["roomId"]}


def project_room_status(event: dict) -> dict:
    return {"status": "occupied" if event["event"] == "GuestCheckedIn" else "unchanged"}


def decide_check_out(command: dict, check_in: dict) -> dict:
    if check_in["event"] != "GuestCheckedIn":
        return {"event": "GuestCheckOutRejected", "bookingId": command["bookingId"], "reason": "not_checked_in"}
    return {"event": "GuestCheckedOut", "bookingId": command["bookingId"], "nights": 4, "nightlyRate": 100, "amount": 400}


def billing_processor(check_out: dict) -> dict:
    return {key: check_out[key] for key in ("bookingId", "nights", "amount")}


def decide_take_payment(command: dict) -> dict:
    if command["amount"] <= 0:
        return {"event": "PaymentRejected", "bookingId": command["bookingId"], "reason": "amount_must_be_positive"}
    return {"event": "PaymentTaken", **command}


def project_invoice(event: dict) -> dict:
    return {key: event[key] for key in ("bookingId", "nights", "amount")}
