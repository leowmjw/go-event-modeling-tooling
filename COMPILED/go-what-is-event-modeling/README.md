# Hotel Booking — Go Temporal

Standalone implementation of `what-is-event-modeling`, pipeline v1.0.0.

- Task queue: `hotel-booking-v1`
- Workflow: `HotelBookingWorkflowV1`
- IR: [`../what-is-event-modeling/main/pipeline.yaml`](../what-is-event-modeling/main/pipeline.yaml)
- Provenance: [`../what-is-event-modeling/main/provenance.json`](../what-is-event-modeling/main/provenance.json)

## Node coverage

| IR node | Go symbol | Shape |
|---|---|---|
| `decide_search_rooms` | `DecideSearchRooms` | decision |
| `project_room_list` | `ProjectRoomList` | projection/query |
| `gate_book_room` | `book_room` | signal |
| `decide_book_room` | `DecideBookRoom` | decision |
| `project_booking_confirmation` | `ProjectBookingConfirmation` | projection/query |
| `gate_check_in` | `check_in` | signal |
| `decide_check_in` | `DecideCheckIn` | decision |
| `project_room_status` | `ProjectRoomStatus` | projection/query |
| `gate_check_out` | `check_out` | signal |
| `decide_check_out` | `DecideCheckOut` | decision |
| `billing_processor` | `BillingProcessor` | deterministic translation |
| `decide_take_payment` | `DecideTakePayment` | decision |
| `project_invoice` | `ProjectInvoice` | projection/result |

No external activities are required; IDs, dates, and amounts enter through typed commands or prior events.

## Demo

```sh
mise run doctor
mise run check
mise run demo
mise run start
mise run room-list
mise run book
mise run checkin
mise run checkout
mise run demo:stop
```

The tests exercise the complete success path and every modeled rejection, including exact invoice projection. Python-emitter semantics differ only in that this Go application exposes each modeled read model through a typed query.
