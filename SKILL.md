# SKILL.md — Generating Event Modeling DSL from Natural Language

This file teaches an AI agent how to turn a non-technical user's description of
a system into a valid `.evml` file for `go-event-modeling-tooling`.

Read `EVENT_MODELING.md` for the complete grammar reference.  This skill guide
focuses on **how to think**, **what to ask**, and **how to structure output**.

---

## Who uses this skill?

A non-technical stakeholder describes a business workflow in plain English.
The agent produces a syntactically correct `.evml` file that renders correctly
as an SVG event model.

---

## Step 1 — Understand the domain

Ask the user (or infer from context) the answers to these four questions.
**Do not generate any DSL until you have all four answers.**

| # | Question | Why it matters |
|---|---|---|
| 1 | **What is the system for?** | Names the top-level domain (e.g. "hotel booking", "online shop"). |
| 2 | **Who are the users / actors?** | Identifies UI screens (`ui`). |
| 3 | **What actions can they take?** | Each action becomes a command (`cmd`). |
| 4 | **What facts does the system record?** | Each recorded fact is an event (`evt`). |

Then ask **one follow-up round** to discover:

- What does each screen *show* to the user? → read models (`rmo`)
- Are there background jobs, bots, or integrations? → processors (`pcr`)
- Are there any external systems that trigger behaviour? → reset frames (`rf`)

---

## Step 2 — Map to entity types

| User phrase | Entity type | Keyword |
|---|---|---|
| "the user sees / views / searches" | Screen / wireframe | `ui` |
| "the user clicks / submits / requests / sends" | Command | `cmd` |
| "the system records / stores / emits" | Event | `evt` |
| "the system displays / shows a list / a dashboard" | Read model | `rmo` |
| "the system automatically / in the background / a job" | Processor | `pcr` |
| "an external system sends / triggers" | Reset frame | `rf` |

---

## Step 3 — Build the timeline

Event modeling always flows **left → right** in time.  Within one business
flow the canonical sequence is:

```
UI screen → Command → Event → Read model
```

A processor sits between an event it reacts to and the command it issues:

```
Event → Processor → Command → Event
```

Rules:
- Assign sequential **two-digit numeric IDs** starting at `01`.
- Use `tf` for every step that belongs to the main flow.
- Use `rf` only when a new external boundary begins (e.g. an event arriving
  from another system).
- Use `->>` to make explicit source connections when the auto-inference would
  be wrong (e.g. a processor that reads from multiple events).

---

## Step 4 — Name things consistently

Follow these naming conventions so the SVG is readable:

| Type | Convention | Example |
|---|---|---|
| UI screen | `<Noun>Screen` or `<Verb><Noun>Desk` | `BookRoomScreen`, `CheckInDesk` |
| Command | `<Verb><Noun>` | `BookRoom`, `AddToCart`, `CheckIn` |
| Event | `<Noun><PastTense>` | `RoomBooked`, `CartUpdated`, `GuestCheckedIn` |
| Read model | `<Noun>` or `<Noun>List` | `RoomList`, `BookingConfirmation`, `Invoice` |
| Processor | `<Noun>Processor` or `<FunctionName>Agent` | `BillingProcessor`, `Agent_A` |

Use `Namespace.Name` (dot-notation) only when two bounded contexts share the
same event name and disambiguation is needed (e.g. `Cart.ItemAdded` vs
`Warehouse.ItemAdded`).

---

## Step 5 — Write inline payloads

Every frame *may* carry a short payload showing the key fields involved.
Keep payloads concise — show only the fields that matter for understanding.

```evml
tf 02 cmd BookRoom { roomId: "r-42", guestId: "g-7" }
tf 03 evt RoomBooked { roomId: "r-42", checkIn: "2024-06-01", checkOut: "2024-06-05" }
```

For complex or reused payloads, extract to a `data` block and reference it:

```evml
tf 04 rmo RoomList [[RoomList04]]

data RoomList04 {
  rooms: [
    { id: "r-42", type: "double", price: 100 }
    { id: "r-55", type: "double", price: 110 }
  ]
}
```

---

## Step 6 — Write GWT scenarios

For each **command** or **event** the user wants to specify behaviour, write
one or more `gwt` blocks anchored to that frame's ID.

Structure:
- **given** — the system state before the action (past events, read models).
- **when** — the action being taken (the command).  Optional if testing a
  pure state transition.
- **then** — what the system records (the resulting event) or produces.

```evml
gwt 06 "book available room"
  given
    evt RoomsSearched { roomType: "double", available: 3 }
  when
    cmd BookRoom { roomId: "r-42", guestId: "g-7" }
  then
    evt RoomBooked { roomId: "r-42", guestId: "g-7" }

gwt 06 "room already booked"
  given
    evt RoomBooked { roomId: "r-42" }
  when
    cmd BookRoom { roomId: "r-42", guestId: "g-8" }
  then
    evt BookingRejected { roomId: "r-42", reason: "already booked" }
```

Write **at least two scenarios per command**: the happy path and at least one
edge case or rejection path.

---

## Step 7 — Validate before emitting

Before producing the final `.evml`, mentally check:

- [ ] File starts with `eventmodeling` on line 1.
- [ ] Every frame ID is **unique** (no duplicates).
- [ ] Every `->>` references an ID that is declared somewhere in the file.
- [ ] Every `[[ref]]` matches a `data <ref>` block name.
- [ ] Every `gwt <id>` references a declared frame ID.
- [ ] Every `gwt` has at least one `given` statement and one `then` statement.
- [ ] Brace depth is balanced in every payload.
- [ ] Names contain no spaces (use `_` or `PascalCase`).

---

## Full worked example

### User's description

> "We run an online shop.  Customers browse products and add them to a cart.
> When they checkout the system charges their card and sends a receipt.
> If payment fails the order is cancelled."

### Agent's reasoning

1. **Screens**: `BrowseProductsScreen`, `CartScreen`, `CheckoutScreen`
2. **Commands**: `AddToCart`, `Checkout`, `ChargeCard`
3. **Events**: `CartUpdated`, `OrderPlaced`, `PaymentTaken`, `PaymentFailed`, `OrderCancelled`
4. **Read models**: `ProductList`, `CartSummary`, `OrderReceipt`
5. **Processor**: `PaymentProcessor` (reacts to `OrderPlaced`, issues `ChargeCard`)
6. **GWT scenarios**: happy path checkout, payment failure path

### Generated `.evml`

```evml
eventmodeling

// ── Browse & Cart ─────────────────────────────────────
tf 01 ui BrowseProductsScreen
tf 02 rmo ProductList { items: [{ id: "p-1", name: "Widget", price: 9.99 }] }

tf 03 ui CartScreen
tf 04 cmd AddToCart { productId: "p-1", qty: 1 }
tf 05 evt CartUpdated { productId: "p-1", qty: 1 }
tf 06 rmo CartSummary [[CartSummary06]]

// ── Checkout ──────────────────────────────────────────
tf 07 ui CheckoutScreen
tf 08 cmd Checkout { cartId: "c-1", email: "alice@example.com" }
tf 09 evt OrderPlaced { orderId: "o-1", total: 9.99 }

// ── Payment ───────────────────────────────────────────
tf 10 pcr PaymentProcessor ->> 09
tf 11 cmd ChargeCard { orderId: "o-1", amount: 9.99 }
tf 12 evt PaymentTaken { orderId: "o-1", amount: 9.99 }
tf 13 rmo OrderReceipt [[OrderReceipt13]]

rf 14 evt PaymentFailed { orderId: "o-1", reason: "insufficient funds" }
tf 15 evt OrderCancelled { orderId: "o-1" }

// ── Data blocks ───────────────────────────────────────
data CartSummary06 {
  cartId: "c-1"
  items: [{ id: "p-1", name: "Widget", qty: 1, price: 9.99 }]
  total: 9.99
}

data OrderReceipt13 {
  orderId: "o-1"
  email: "alice@example.com"
  total: 9.99
  status: "paid"
}

// ── Scenarios ─────────────────────────────────────────
gwt 05 "add new item to empty cart"
  given
    evt CartUpdated { qty: 0 }
  when
    cmd AddToCart { productId: "p-1", qty: 1 }
  then
    evt CartUpdated { productId: "p-1", qty: 1 }

gwt 05 "increment existing item"
  given
    evt CartUpdated { productId: "p-1", qty: 1 }
  when
    cmd AddToCart { productId: "p-1", qty: 1 }
  then
    evt CartUpdated { productId: "p-1", qty: 2 }

gwt 09 "place order from cart"
  given
    evt CartUpdated { productId: "p-1", qty: 1 }
  when
    cmd Checkout { cartId: "c-1" }
  then
    evt OrderPlaced { orderId: "o-1", total: 9.99 }

gwt 12 "successful payment"
  given
    evt OrderPlaced { orderId: "o-1", total: 9.99 }
  when
    cmd ChargeCard { orderId: "o-1", amount: 9.99 }
  then
    evt PaymentTaken { orderId: "o-1", amount: 9.99 }

gwt 15 "payment failure cancels order"
  given
    evt PaymentFailed { orderId: "o-1" }
  then
    evt OrderCancelled { orderId: "o-1" }
```

---

## Quick-reference cheat sheet

```
eventmodeling                              ← required first line

tf <id> ui   <ScreenName>                 ← user sees a screen
tf <id> cmd  <VerbNoun>   { payload }     ← user takes an action
tf <id> evt  <NounVerbed> { payload }     ← system records a fact
tf <id> rmo  <NounName>   [[DataRef]]     ← system shows data
tf <id> pcr  <NounProcessor> ->> <srcId> ← background automation

rf <id> evt  Namespace.ExternalEvent      ← external boundary

data <RefName> {                          ← named data block
  field: value
}

note <id> { free text }                   ← annotation on a frame

gwt <id> "scenario label"
  given
    evt SomeEvent { field: value }
  when
    cmd SomeCommand { field: value }
  then
    evt ResultEvent { field: value }
```

---

## Common mistakes to avoid

| Mistake | Fix |
|---|---|
| Spaces inside an identifier | Use `PascalCase` or `snake_case` |
| Missing `eventmodeling` header | Always put it on line 1 |
| `gwt` with no `given` | Every scenario must have at least one `given` |
| `gwt` with no `then` | Every scenario must have at least one `then` |
| Referencing an undefined frame ID | Declare `tf`/`rf` before or after — order doesn't matter for references, but the ID must exist |
| Unbalanced braces in payload | Count `{` and `}` — they must match |
| Reusing the same frame ID | Each `tf`/`rf` must have a unique numeric ID |
| Putting a command after another command without an event | Insert an `evt` in between — commands react to events or UI actions |
