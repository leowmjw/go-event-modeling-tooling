# go-event-modeling-tooling

Event Modeling tooling in idiomatic Go, built for **working sessions with
domain experts**, not just for drawing diagrams after the fact.

- `.evml` parser → AST, SVG renderer, zero non-stdlib dependencies (`evml` package).
- `evml` CLI: `svg` (with a stage lens), `lint` (open questions & completeness gaps), `diff` (semantic comparison of two versions).
- `cmd/evmlweb`: a local Datastar web app where a facilitator and domain experts
  walk the timeline, add steps without knowing the notation, park open
  questions, sketch proposals as **staging** / **future** next to the as-is
  process, compare drafts, and promote one to the baseline. Works with or
  without a local LLM.

## Quick start

```bash
mise run build                          # bin/evml
mise run webapp:nollm                   # http://localhost:8080 — no model needed
mise run webapp                         # same, with the Kronk-backed assistant tab

./bin/evml svg  testdata/fixtures/fintech-instant-payment-v2.evml -d out
./bin/evml svg  testdata/fixtures/fintech-instant-payment-v2.evml -d out --stage current   # as-is only
./bin/evml lint testdata/fixtures/fintech-instant-payment-v2.evml                          # hotspots + gaps
./bin/evml diff testdata/fixtures/fintech-instant-payment-v1.evml testdata/fixtures/fintech-instant-payment-v2.evml
```

---

## Demo script for domain experts (≈25 minutes)

Uses the instant account-to-account payment flow. Everything below is done in
the browser; no one needs to read the notation. Before you start:

```bash
mise run webapp:nollm        # open http://localhost:8080 on the room's screen
```

### Act 1 — The normal scenario, as it runs today (5 min)

1. **Flow ▸ `fintech-instant-payment-v1` ▸ Open.** A draft `v1` is created for
   today; the saved file is untouched until you promote.
2. **Lens ▸ As-is.** Say: *"Solid boxes are what happens today. Dashed or faded
   boxes are ideas — there are none on screen right now."*
3. **Walk left to right**, one chapter band at a time:
   - *Payment Initiation* — `SendMoneyScreen` (`@Customer`) → `InitiatePayment`
     → `PaymentInitiated` → `PaymentStatus`.
   - *Ledger: hold funds* — the event crosses into Ledger as an external fact,
     `LedgerTranslator` turns it into `HoldFunds` → `FundsHeld`; the rejection
     `FundsHoldRejected` is the insufficient-funds path.
   - *Clearing Gateway* — `SendCreditTransfer` (pacs.008) → scheme answers with
     one pacs.002 → `CompletePayment` or `FailPayment`.
   - *Ledger: settle or release* and *Notifications*.
4. **Click any box** (try `HoldFunds`). The Steps tab opens the inspector:
   type, who does it, which slice, how many scenarios cover it. Say: *"A command
   with zero scenarios is a step we haven't really understood yet."*
5. **Scenarios tab ▸ Coverage.** Show that every command has a happy path and at
   least one rejection. Point at the three for `InitiatePayment`: within limits,
   above scheme maximum, malformed IBAN.

Talking point: colours are the notation — orange screens/automations, blue
commands/read models, green facts. Time flows left to right, always.

### Act 2 — Adding a scenario the room just thought of (5 min)

Ask the room: *"Is there any rule we didn't cover?"* Someone will say "a payment
to your own account". Capture it live:

1. Click `InitiatePayment` (step 02) → **+ Scenario** (or Scenarios tab, pick
   step 02).
2. Title: `reject a payment to the debtor's own account`.
3. Given: `event AccountOpened { accountId: "acc-118203" }`
   When: `command InitiatePayment { debtorAccountId: "acc-118203", creditorIban: "DE…same" }`
   Then: `event PaymentInitiationRejected { reasonCode: "SAME_ACCOUNT" }`
4. **Add scenario.** It appears under the box in the diagram and the Coverage
   count goes up. The Compare tab now shows `+ 1 scenario(s)`.

Ask the question twice more. Rejections are facts too: if the room can name the
reason code, it belongs in the model.

### Act 3 — Proposing a change without breaking "today" (7 min)

Ops wants sanctions screening before the payment leaves. Model it as a
**proposal**, not as fact:

1. Click `FundsHeld` (step 08) → **+ Step after this**.
   Kind *Automation*, name `Sanctions screener`, stage **Staging** → Add.
   It renders amber-dashed with a `STAGING` badge.
2. Add two more the same way: a *Command* `Screen payment against sanctions
   lists`, then an *Event* `Payment cleared`. (For the hit path, add an
   *Event* `Payment held for review` — one decision, two possible facts.)
3. **Lens ▸ As-is.** The proposal disappears; the as-is flow is still complete.
   **Lens ▸ + Staging.** It comes back. Say: *"We can always show the regulator
   what we do today and the board what we intend."*
4. Someone asks *"who can release a hit, and how fast?"* Nobody knows.
   **Questions tab ▸ About step** `Screen payment…` ▸ type the question ▸
   **Park this question.** A red sticky and a red count appear on the step.
   The Compare ▸ Health check lists it; `evml lint --strict` will fail until it
   is resolved. Say: *"Parking beats arguing. It is visible until someone owns
   it."*
5. When Compliance answers later: **Record decision** on the question. It turns
   into a note on the model with the question and the decision.
6. **Slices tab** ▸ Kind *Slice*, name `Real-time sanctions screening`, first
   and last step of your new block, Status *InProgress*, Stage *Staging*. The
   band above the timeline now tracks delivery as well as intent.

### Act 4 — Versions and what changed (6 min)

1. **Compare tab.** Read it aloud as the meeting summary: steps added, the new
   scenario, the parked question, the slice. This is the diff between today's
   draft and the saved baseline.
2. **+ New draft.** Fork the session into `v2` to try a different shape (for
   example the screening *before* the hold). Switch between the tabs; each keeps
   its own diagram and session log.
3. **Promote to baseline** on the draft the room agrees with. Confirm. The
   file `testdata/fixtures/fintech-instant-payment-v1.evml` is overwritten and
   the session log records the change summary. Staging and future stay in the
   file on purpose — they are part of the shared picture.
4. **Export .evml / .svg** for the minutes.

Then show what two real sessions look like across time. Open flow
`fintech-instant-payment-v2` — the same process after the next two workshops:

- Confirmation of Payee shipped (**current**).
- The v1 question *"what if the scheme never answers?"* became the
  *Scheme timeout auto-reversal* slice, promoted **future → current**, with the
  decision recorded as a note on the watchdog.
- *Real-time sanctions screening* and the *Analyst review desk*
  (`@ComplianceAnalyst`) are **staging** with three open questions.
- *Recall / request for return* and *Fraud scoring and step-up* are **future**.

And in the terminal:

```bash
./bin/evml diff testdata/fixtures/fintech-instant-payment-v1.evml testdata/fixtures/fintech-instant-payment-v2.evml
# + 101 ui ConfirmationOfPayeeScreen …        added steps
# ~ 12 ClearingTranslator: sources changed     the proposal wired in beside the as-is edge
# » 21 SchemeTimeoutWatchdog: future → current  promotion
# ✓ 2 question(s) resolved                     hotspots turned into notes
./bin/evml lint testdata/fixtures/fintech-instant-payment-v2.evml --strict   # exit 1 while questions stay open
```

### If the demo goes off-script

- Something was added wrongly: click it → **Remove**. Notes, questions,
  scenarios and slice ranges attached to it are cleaned up.
- A step turned out to be "how it already works": click it → **current**.
- The room wants to go faster than the forms: **Source tab**, edit the text,
  **Apply**. A bad edit is rejected with a line number and nothing changes.
- No local model on the laptop: the Assistant tab is disabled; every other
  tab works.

Full facilitation guide: [WORKSHOP.md](WORKSHOP.md).

---

## The notation in one screen

```evml
eventmodeling
actor Customer
chapter "Top-up" 01-04
slice "Velocity check" 05-08 status InProgress stage staging

tf 01 ui  TopUpScreen @Customer
tf 02 cmd TopUpWallet { walletId: "w-1", amount: 50.00 }
tf 03 evt WalletToppedUp { walletId: "w-1", amount: 50.00 }
tf 04 rmo WalletBalance { balance: 150.00 }
tf 05 pcr VelocityMonitor ->> 03
...
tf 12 evt TopUpPointsGranted { points: 5 } #future

hotspot 06 { Is 5 top-ups per day the right threshold? }

gwt 02 "reject below minimum"
  given
    evt WalletOpened
  when
    cmd TopUpWallet { amount: 5.00 }
  then
    evt TopUpRejected { reason: "below_minimum" }
```

Full grammar: [EVENT_MODELING.md](EVENT_MODELING.md). How to think and what
to ask: [SKILL.md](SKILL.md). Running a session: [WORKSHOP.md](WORKSHOP.md).
Conventions for contributors and agents: [AGENTS.md](AGENTS.md).

## Worked FinTech examples (`testdata/fixtures/`)

| Fixture | What it shows |
|---|---|
| `fintech-instant-payment-v1.evml` → `-v2.evml` | Same flow across two workshop sessions: v2 adds Confirmation of Payee, a sanctions hold with analyst desk (staging), the 10 s scheme timeout that resolves a v1 hotspot, recall and fraud step-up (future). `evml diff` between them is the change log. |
| `fintech-card-dispute-chargeback.evml` | Reg E clocks as read models with deadline-guard processors, provisional credit, representment, arbitration, notice-before-reversal; small-value auto-adjudication in staging, pre-dispute alerts in future. |
| `fintech-loan-lifecycle.evml` | Origination → underwriting (three separate decisions, maker-checker desk) → disbursement with ACH return → servicing → arrears → collections → closure; hardship restructure in staging, top-up and open banking in future. |
| `fintech-payment-reconciliation.evml` | Settlement-file ingest, auto-matching, break classification, suspense, aging/escalation, maker-checker write-off; tolerance auto-clear in staging with an open question for Finance. |
| `staging-lens-hotspots.evml` | Minimal example of every workshop construct. |
