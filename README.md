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
./bin/evml svg testdata/fixtures/fintech-instant-payment-v2.evml -d out
./bin/evml svg testdata/fixtures/fintech-instant-payment-v2.evml -d out --stage current   # as-is only
./bin/evml lint testdata/fixtures/fintech-instant-payment-v2.evml            # hotspots + gaps
./bin/evml diff testdata/fixtures/fintech-instant-payment-v1.evml testdata/fixtures/fintech-instant-payment-v2.evml

mise run webapp:nollm                   # http://localhost:8080 — no model needed
mise run webapp                         # same, with the Kronk-backed assistant tab
```

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
