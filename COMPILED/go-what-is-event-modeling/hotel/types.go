package hotel

const (
	TaskQueue       = "hotel-booking-v1"
	WorkflowName    = "HotelBookingWorkflowV1"
	BookRoomSignal  = "book_room"
	CheckInSignal   = "check_in"
	CheckOutSignal  = "check_out"
	RoomListQuery   = "room_list"
	BookingQuery    = "booking_confirmation"
	RoomStatusQuery = "room_status"
)

type Room struct {
	ID    string  `json:"id"`
	Type  string  `json:"type"`
	Price float64 `json:"price"`
}
type SearchRoomsCommand struct {
	RoomType string `json:"roomType"`
	CheckIn  string `json:"checkIn"`
	CheckOut string `json:"checkOut"`
}
type RoomsSearched struct {
	RoomType  string `json:"roomType"`
	CheckIn   string `json:"checkIn"`
	CheckOut  string `json:"checkOut"`
	Rooms     []Room `json:"rooms"`
	Available int    `json:"available"`
}
type RoomList struct {
	Rooms []Room `json:"rooms"`
}
type BookRoomCommand struct {
	BookingID string `json:"bookingId"`
	RoomID    string `json:"roomId"`
	GuestID   string `json:"guestId"`
	CheckIn   string `json:"checkIn"`
	CheckOut  string `json:"checkOut"`
}
type RoomBooked struct {
	BookingID string `json:"bookingId"`
	RoomID    string `json:"roomId"`
	GuestID   string `json:"guestId"`
	CheckIn   string `json:"checkIn"`
	CheckOut  string `json:"checkOut"`
}
type RoomBookingRejected struct {
	BookingID string `json:"bookingId"`
	RoomID    string `json:"roomId"`
	Reason    string `json:"reason"`
}
type BookRoomResult struct {
	Event    string               `json:"event"`
	Booked   *RoomBooked          `json:"roomBooked,omitempty"`
	Rejected *RoomBookingRejected `json:"roomBookingRejected,omitempty"`
}
type BookingConfirmation struct {
	BookingID string `json:"bookingId"`
	RoomID    string `json:"roomId"`
	CheckIn   string `json:"checkIn"`
	CheckOut  string `json:"checkOut"`
}
type CheckInCommand struct {
	BookingID string `json:"bookingId"`
}
type GuestCheckedIn struct {
	BookingID string `json:"bookingId"`
	RoomID    string `json:"roomId"`
}
type GuestCheckInRejected struct {
	BookingID string `json:"bookingId"`
	Reason    string `json:"reason"`
}
type CheckInResult struct {
	Event     string                `json:"event"`
	CheckedIn *GuestCheckedIn       `json:"guestCheckedIn,omitempty"`
	Rejected  *GuestCheckInRejected `json:"guestCheckInRejected,omitempty"`
}
type RoomStatus struct {
	Status string `json:"status"`
}
type CheckOutCommand struct {
	BookingID string `json:"bookingId"`
}
type GuestCheckedOut struct {
	BookingID   string  `json:"bookingId"`
	Nights      int     `json:"nights"`
	NightlyRate float64 `json:"nightlyRate"`
	Amount      float64 `json:"amount"`
}
type GuestCheckOutRejected struct {
	BookingID string `json:"bookingId"`
	Reason    string `json:"reason"`
}
type CheckOutResult struct {
	Event      string                 `json:"event"`
	CheckedOut *GuestCheckedOut       `json:"guestCheckedOut,omitempty"`
	Rejected   *GuestCheckOutRejected `json:"guestCheckOutRejected,omitempty"`
}
type TakePaymentCommand struct {
	BookingID string  `json:"bookingId"`
	Nights    int     `json:"nights"`
	Amount    float64 `json:"amount"`
}
type PaymentTaken struct {
	BookingID string  `json:"bookingId"`
	Nights    int     `json:"nights"`
	Amount    float64 `json:"amount"`
}
type PaymentRejected struct {
	BookingID string `json:"bookingId"`
	Reason    string `json:"reason"`
}
type PaymentResult struct {
	Event    string           `json:"event"`
	Taken    *PaymentTaken    `json:"paymentTaken,omitempty"`
	Rejected *PaymentRejected `json:"paymentRejected,omitempty"`
}
type Invoice struct {
	BookingID string  `json:"bookingId"`
	Nights    int     `json:"nights"`
	Amount    float64 `json:"amount"`
}
type WorkflowResult struct {
	Search       RoomsSearched       `json:"search"`
	RoomList     RoomList            `json:"roomList"`
	Booking      BookRoomResult      `json:"booking"`
	Confirmation BookingConfirmation `json:"confirmation"`
	CheckIn      CheckInResult       `json:"checkIn"`
	RoomStatus   RoomStatus          `json:"roomStatus"`
	CheckOut     CheckOutResult      `json:"checkOut"`
	Payment      PaymentResult       `json:"payment"`
	Invoice      Invoice             `json:"invoice"`
}
