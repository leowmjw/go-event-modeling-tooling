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

> **Rule — "no question, no read model":** only create an `rmo` frame when you
> can name the single question it answers (e.g. "what rooms are available?").
> If you can't state the question, it isn't a read model — either drop it or
> it's really a screen (`ui`) or a data payload on an existing frame. Never
> create an `rmo` "just in case" or as a generic passthrough for an event.

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

---

## Event Modeling Best Practices & Cheatsheet

> Synthesised from eventmodelers.ai, the official Event Modeling cheat sheet
> (eventmodeling.org), and community sources.  Use this section as a reference
> when deciding *what* to model and *how* to model it well.

---

### Principles

#### 1. Complete Picture
Model the *entire* flow left-to-right — from the first user intention all the
way to the last read model.  Do not leave gaps.  If you cannot draw a
continuous path from trigger to view, the system is not fully understood yet.

#### 2. Information Completeness
**A view can only be built from existing events.**  If a read model needs a
field that no event carries, the event model is incomplete.  Ask: *"Is this
data recorded somewhere on the event stream?"*  If the answer is no, add an
event or enrich an existing one.

#### 3. Ubiquitous Language
Every name — commands, events, read models, screens — must use the business
domain vocabulary that *all* stakeholders share.  Non-technical participants
must be able to read the model and understand it without translation.

- ✅ `OrderPlaced`, `GuestCheckedIn`, `PaymentRefunded`
- ❌ `UpdateOccurred`, `DataChanged`, `RecordSaved`

#### 4. Events Are Immutable Facts
Events describe *what happened*, not what to do.  They are written once and
never mutated.  The entire current state of the system can be rebuilt by
replaying the event stream from the beginning.

---

### The Four Patterns

| Pattern | Flow | Who initiates | Purpose |
|---|---|---|---|
| **Command** | UI → `cmd` → `evt` | Human user | Record a state change triggered by intent |
| **View** | `evt`(s) → `rmo` | System projection | Build a read model from past events |
| **Automation** | `evt` → `pcr` → `cmd` → `evt` | Automated processor | React to events without human intervention |
| **Translation** | External `evt` → `rf` → `pcr` → `cmd` → `evt` | External system boundary | Adapt an outside event into the internal model |

#### Command Pattern (State Change)
```
[Screen] → cmd VerbNoun → evt NounPastTensed
```
- Every state change starts with a human or system **trigger**.
- A command expresses **intent**; it may be rejected (no event emitted).
- An event is **always** the result of a successful command — it is the durable fact.

#### View Pattern (State Query)
```
evt NounPastTensed → rmo NounList / NounDetail
```
- Read models are *projections* — they are derived entirely from events.
- A read model never writes; it never triggers a command.
- If a screen needs data, trace back to the event that carries it.

#### Automation Pattern (Processor / Robot)

Canonical shape (per eventmodelers.ai): the processor watches a **read
model**, not the raw event —

```
evt NounPastTensed → rmo NounModel → pcr NounProcessor → cmd VerbNoun → evt NounPastTensed
```

This repo also supports a **shorthand** that skips the intermediate read
model when there's nothing worth projecting — the processor sources
straight off the event:

```
evt NounPastTensed → pcr NounProcessor → cmd VerbNoun → evt NounPastTensed
```

- The processor (`pcr`) replaces the human — it watches an event or a read
  model (e.g. an agent inspecting a tool-call result) and issues a command
  automatically.
- Use `->>` to wire the processor to its source frame(s) explicitly.
- A processor may consume **multiple** sources (`->> 05 ->> 06`), mixing `evt`
  and `rmo` frames as needed.
- Prefer the canonical (via-`rmo`) shape when the processor's decision
  depends on *derived/aggregated* state (e.g. "3 failed attempts"); use the
  shorthand when it reacts to a single raw fact.

#### Translation Pattern (External Boundary)

Canonical shape — the translator also watches a read model built from the
external event, not the raw external event itself:

```
rf ExternalNamespace.EventName → rmo NounModel → pcr TranslatorName → cmd InternalVerbNoun → evt InternalNounPastTensed
```

Shorthand (translator sources directly off the reset frame):

```
rf ExternalNamespace.EventName → pcr TranslatorName → cmd InternalVerbNoun → evt InternalNounPastTensed
```

- Use a **reset frame** (`rf`) to mark where an external event enters.
- A translator processor maps the external schema to the internal ubiquitous
  language — the rest of the model never sees the external format.
- Use `Namespace.EventName` dot-notation to signal the external boundary.

---

### Swimlane Organisation

```
┌─────────────────────────────────────┐
│  UI / Automation (ui, pcr)          │  ← salmon / orange
├─────────────────────────────────────┤
│  Command / Read Model (cmd, rmo)    │  ← blue
├─────────────────────────────────────┤
│  Events (evt)                       │  ← green
└─────────────────────────────────────┘
```

Rules:
- Time flows **left to right**.  Earlier steps are on the left.
- A frame belongs to the swimlane of its entity type — the renderer places it
  automatically.
- Processors (`pcr`) sit in the **UI/Automation** lane because they act like
  automated users.
- Keep swimlanes narrow — if a lane is very tall, you have too many frame types
  in one step; split the flow.

---

### Slices & Scenarios

A **slice** is a vertical cut through the timeline covering one coherent user
story, e.g. "Book a Room".  Slices:
- Have a clear start (a UI screen or external trigger) and end (a read model
  the user or system can observe).
- Map directly to development tasks: one slice = one sprint story.
- Can be modelled and implemented independently.

A **scenario** (`gwt`) is the specification for one slice variant:

| Section | Meaning | DSL keyword |
|---|---|---|
| **Given** | Past events that set up the starting state | `given` |
| **When** | The command being issued (the action) | `when` |
| **Then** | The event(s) that result | `then` |

Write **at minimum two scenarios per command**:
1. The **happy path** — the normal successful case.
2. At least one **edge/rejection path** — what happens when the precondition
   is not met (e.g. room already booked, insufficient funds, duplicate entry).

---

### Anti-Patterns to Spot

> From the eventmodelers.ai cheat sheet.  Check the model against these
> before finalising — each one is a smell that the slice boundaries or
> responsibilities are drawn wrong.

| Anti-pattern | Shape | What it means | Fix |
|---|---|---|---|
| **Left Chair** | One `cmd` → many unrelated `evt` | The command is doing too much, or the events belong to different decisions | Split the command, or move unrelated events to their own command (see corner-case pattern §3, "one decision = one command + event pair") |
| **Right Chair** | Many unrelated `evt` → one `rmo` | The read model is answering more than one question | Split into separate read models, one per question |
| **Bed** | One `ui` → many `cmd` | The screen is doing the work of several screens | Split the screen, or consolidate the commands if they're really one intent |
| **Shelf** | Some slices have many `gwt` scenarios, others have none | Uneven scenario coverage — usually means part of the model wasn't actually worked through | Revisit under-covered slices; ask "is there a rule we didn't cover?" three times |

See the **"no question, no read model"** rule in Step 1 and the "One read
model = one question" rule under Read Model Naming Rules — this
anti-pattern is what happens when that discipline is skipped.

---

### Event Naming Rules

| Rule | Example |
|---|---|
| Always **past tense** | `RoomBooked` not `BookRoom` |
| Business language, not technical | `GuestCheckedIn` not `StatusUpdated` |
| Noun + past-tense verb | `OrderPlaced`, `PaymentFailed`, `InvoiceSent` |
| Specific, not generic | `InventoryReserved` not `RecordChanged` |
| No abbreviations unless universal | `OrderId` not `OId` |

---

### Command Naming Rules

| Rule | Example |
|---|---|
| **Imperative verb + noun** | `BookRoom`, `AddToCart`, `CheckIn` |
| Expresses **intent**, not result | `SubmitOrder` not `OrderWasSubmitted` |
| Named from the *user's* perspective | `RequestRefund` not `ProcessRefund` |

---

### Read Model Naming Rules

| Rule | Example |
|---|---|
| **Noun** — what the user *sees* | `RoomList`, `BookingConfirmation`, `Invoice` |
| Use `List`, `Summary`, `Detail`, `Dashboard` suffixes | `CartSummary`, `OrderDetail` |
| Never a verb | `RoomList` not `ListRooms` |
| **Answers exactly one question** | `RoomList` answers "which rooms are available?" — it does not also answer "who booked what?" |

**One read model = one question.** Before naming an `rmo` frame, write down
the question it answers in plain language. If you need "and" to describe
it ("shows the cart *and* the user's order history"), split it into two
read models. This is the same discipline as the "Right Chair" anti-pattern
below — a read model folding events that answer *different* questions is a
sign the model needs to be split, not that the read model is doing its job
well.

---

### State Store vs State Transfer

| Concept | State Store (CRUD) | State Transfer (Event Sourcing) |
|---|---|---|
| What is saved | Current snapshot only | Append-only log of all events |
| Auditability | Lost on update | Full history preserved |
| Reconstructibility | Not possible | Any past state can be replayed |
| Event Modeling fit | ⚠ Partial (lose history) | ✅ Full fit |

Event Modeling naturally leads to **state transfer** — the event stream *is*
the source of truth.  Read models are ephemeral projections that can be rebuilt
at any time.

---

### Information Completeness Checklist

Before finalising the `.evml` file, verify:

- [ ] Every read model (`rmo`) can be built solely from events already in the model.
- [ ] Every read model (`rmo`) answers exactly one named question — no "no question, no read model" violations.
- [ ] Every command has at least one event that results from it.
- [ ] Every event name is past tense and uses domain language.
- [ ] Every processor has an explicit source event (`->>`) and produces a command.
- [ ] Every external boundary is a reset frame (`rf`) with a namespaced event.
- [ ] At least two `gwt` scenarios exist per command (happy + edge/rejection).
- [ ] No field appears in a view without a corresponding event carrying it.
- [ ] All names are understandable to a non-technical domain expert.

---

### Pattern Selection Guide

When a user describes a behaviour, pick the pattern:

| Description | Pattern |
|---|---|
| "User clicks / submits / fills a form" | **Command** — `ui` → `cmd` → `evt` |
| "The screen shows / displays / lists" | **View** — `evt` → `rmo` |
| "The system automatically sends / schedules / retries" | **Automation** — `evt` → `pcr` → `cmd` → `evt` |
| "An email arrives / a webhook fires / an API calls us" | **Translation** — `rf` + `pcr` → internal `cmd` → `evt` |
| "Users from another system" | **Translation** — use `Namespace.EventName` |

---

## Advanced corner-case patterns

The patterns below were distilled from modelling the *Flight Arrival &
Post-Flight Settlement* domain, where a single business process spans four
bounded contexts (Operations → Compensation → Finance → Marketing).  Apply
them whenever a flow crosses context boundaries or has non-trivial failure paths.

---

### 1. Per-context completeness — every branch must close in every context it touches

**Rule:** When a workflow branch (e.g. "flight is on time") bypasses a later
bounded context, that context still needs a terminal outcome recorded in its
own state history.  An open case with no terminal event is a data-model gap.

**Anti-pattern:** Operations emits `FlightClosedOnTime`; Compensation context
is never notified — its state history has no closure record.

**Fix:** Emit an `rf` boundary event from Operations into Compensation, then
use a `*ClosureProcessor` to issue a `Close*Case` command that records the
terminal `*CaseClosed` event.

```evml
rf 17 evt Operations.FlightClosedOnTime { flightId: "HLT-421" }
tf 18 pcr CompensationClosureProcessor ->> 17
tf 19 cmd CloseCompensationCase { flightId: "HLT-421", reason: "on_time_no_claim" }
tf 20 evt CompensationCaseClosed { flightId: "HLT-421", reason: "on_time_no_claim" }
```

---

### 2. Processor as decision guard — suppression belongs in `pcr`, not in a command

**Rule:** When a rule *prevents* a command from being issued (e.g. skip delay
evaluation because the flight is on time), the guard belongs in the **processor**
that decides whether to emit the command.  A command that accepts empty args and
self-rejects hides a decision in the wrong layer.

**Anti-pattern:**
```evml
// Wrong — command self-rejects when list is empty
tf 12 cmd CalculateCompensation { delayMinutes: 14, affectedPassengerIds: [] }
tf 13 evt CompensationSkipped { reason: "on_time" }
```

**Fix — processor emits a suppression event; the command is never issued:**
```evml
tf 09 pcr CompensationProcessor ->> 08
// When gate-door delta ≤ threshold the processor emits:
//   evt DelayEvaluationSuppressed { reason: "within_on_time_threshold" }
// and does NOT emit the EvaluateDelay command at all.
```

GWT for the guard lives on the *processor* frame ID, not on the command frame:
```evml
gwt 09 "CompensationProcessor suppresses evaluation for on-time arrivals"
  given
    evt Operations.GateOpened { gateDoorOpenTime: "...", scheduledArrival: "..." }
  when
    cmd EvaluateDelay { onTimeDefinition: "gate_door_open", delayMinutes: 14 }
  then
    evt DelayEvaluationSuppressed { reason: "within_on_time_threshold" }
```

---

### 3. State transition atomicity — one decision = one command + event pair

**Rule:** Each distinct decision step is a separate transition in State History
(`Previous State → New State → Transition Time`).  Never merge two decisions
into one command.

**Anti-pattern:** `CalculateCompensation` implicitly checks eligibility and
calculates an amount in one step — hiding the eligibility-determination transition.

**Fix:** Introduce `VerifyPassengerEligibility → PassengerEligibilityVerified`
*before* `CalculateCompensation`:

```evml
tf 12 cmd VerifyPassengerEligibility { flightId: "HLT-421", passengerIds: ["pax-1", "pax-2"] }
tf 13 evt PassengerEligibilityVerified { eligibleIds: ["pax-1"], ineligibleIds: ["pax-2"] }
tf 14 cmd CalculateCompensation { flightId: "HLT-421", eligiblePassengerIds: ["pax-1"] }
tf 15 evt CompensationCalculated { totalCompensationAmount: 170.00 }
```

This makes the ineligibility decision visible, auditable, and independently
testable via its own GWT scenarios.

---

### 4. Partial failure feedback — batches need partial-rejection `rf` loops

**Rule:** When a context processes a *batch* and only some items are rejected,
the rejected subset must be carried back across the context boundary via its own
named `rf`.  All-or-nothing `rf` events miss the partial case.

**Anti-pattern:** Finance emits either `DisbursementApproved` (all) or
`DisbursementRejected` (all) — no event for the mixed case.

**Fix:**
```evml
// Happy path — fully approved
tf 25 evt DisbursementApproved { totalAmount: 510.00 }

// Partial rejection fed back to Compensation
rf 35 evt Finance.PartialDisbursementRejected {
  rejectedPassengerIds: ["pax-3"], rejectedAmount: 170.00, reason: "no_billable_account"
}
tf 36 pcr CompensationRecoveryTranslator ->> 35
tf 37 cmd EscalateUnresolvedClaim { passengerIds: ["pax-3"], amount: 170.00 }
tf 38 evt ClaimEscalated { passengerIds: ["pax-3"], amount: 170.00 }
```

---

### 5. Actor-response terminal states — `sent` is not a terminal state

**Rule:** When the system sends something to a human actor (email, offer,
notification), the *response* is a distinct state that must be recorded.
`RecoveryOfferSent` is not a terminal state — `RecoveryOfferAccepted` and
`RecoveryOfferDeclined` are.

**Anti-pattern:** Model ends at `RecoveryOfferSent`.

**Fix:** Add a `ui` response screen, a `RespondTo*` command, and separate
outcome events for each response branch:

```evml
tf 47 ui RecoveryOfferResponseScreen
tf 48 cmd RespondToRecoveryOffer { offerId: "offer-77", travelerId: "visitor-a", response: "accepted" }
tf 49 evt RecoveryOfferAccepted { offerId: "offer-77", travelerId: "visitor-a" }
tf 50 rmo MarketingConversionReport { acceptedCount: 1, declinedCount: 0 }
```

GWT must cover both branches:
```evml
gwt 48 "traveler accepts recovery offer"
  given
    evt RecoveryOfferSent { offerId: "offer-77" }
  when
    cmd RespondToRecoveryOffer { offerId: "offer-77", response: "accepted" }
  then
    evt RecoveryOfferAccepted { offerId: "offer-77" }

gwt 48 "traveler declines recovery offer"
  given
    evt RecoveryOfferSent { offerId: "offer-77" }
  when
    cmd RespondToRecoveryOffer { offerId: "offer-77", response: "declined" }
  then
    evt RecoveryOfferDeclined { offerId: "offer-77" }
```

---

### 6. Async external-integration failure — model the full reversal lifecycle

**Rule:** Integrations with external payment rails (ACH, SWIFT, card networks)
can fail *asynchronously* after `PayoutIssued`.  State History must capture
every transition: `issued → failed → reversed → reissued`.

**Anti-pattern:** Model ends at `PayoutIssued` — no path for ACH returns.

**Fix:** Add a reset frame for the asynchronous failure event, then route
through a recovery processor:

```evml
rf 29 evt Finance.PayoutFailed { disbursementId: "disb-9901", failureCode: "R03" }
tf 30 pcr PayoutRecoveryProcessor ->> 29
tf 31 cmd ReverseAndReissuePayout { disbursementId: "disb-9901", newPaymentRails: "WIRE" }
tf 32 evt PayoutReversed { disbursementId: "disb-9901", failureCode: "R03" }
tf 33 evt PayoutReissued { disbursementId: "disb-9901", newPaymentRails: "WIRE" }
```

GWT rejection scenario — guard against reversing an already-reversed payout:
```evml
gwt 31 "reject reissuance when already reversed"
  given
    evt PayoutReversed { disbursementId: "disb-9901" }
  when
    cmd ReverseAndReissuePayout { disbursementId: "disb-9901" }
  then
    evt PayoutReversalRejected { reason: "already_reversed" }
```

---

### Updated Information Completeness Checklist (advanced)

Add these checks after the standard checklist in Step 7:

- [ ] **Per-context closure:** every bounded context a branch *passes through* has a terminal event — not just the main happy path.
- [ ] **No command self-rejection:** suppression decisions live in the processor, not in a command with empty/invalid arguments.
- [ ] **One transition per decision:** eligibility checks, validation steps, and calculations are separate command+event pairs.
- [ ] **Partial failure `rf`:** if a context processes a batch, a named `rf` handles partial rejection back to the source context.
- [ ] **Actor-response terminals:** every `*Sent` event has corresponding `*Accepted` / `*Declined` (or equivalent) outcome events and GWT scenarios.
- [ ] **Async integration failures:** every external-rails payout/send frame has a failure `rf` path covering reversal and reissuance.
