# go-event-modeling-tooling

Idiomatic Go port of [Event Modeling](https://eventmodeling.io/) tooling with:

- An `.evml` parser that builds an AST of Event Modeling constructs
- An SVG renderer that lays frames out left-to-right across context swimlanes
- A validator (`ValidateConnections`) enforcing which entity types may source which
- A `evml` CLI for rendering `.evml` files from the command line
- `cmd/evmlweb`, a local DataStar web app for iterating on models with a local LLM

## The `.evml` DSL (at a glance)

A file starts with `eventmodeling`, then declarations:

```
tf 01 ui DepositScreen                       # timeframe: UI (PascalCase identifier)
tf 02 cmd InitiateDeposit { ... }             # command (reacts to UI)
tf 03 evt DepositReceived { ... }           # event (reacts to command)
tf 04 rmo AccountBalance [[Balance04]]      # read model (reacts to event)
rf 11 evt Core.WithdrawalRequested { ... }  # reset frame: boundary into a context
tf 12 pcr FraudCheckProcessor ->> 11        # processor sourcing frame 11
```

Entity keywords: `ui` (UI/Automation), `cmd` (Command), `evt` (Event), `rmo`
(read model), `pcr` (processor), `scn`/`rn`/`screen`/`command`/`readmodel`/
`processor` (aliases), `rf` (reset frame — an external boundary event). Use
`->>` to pin a frame's source explicitly; otherwise the renderer infers the
nearest allowed predecessor. Data blocks (`data Foo { ... }`) are referenced
with `[[Foo]]`; `note`, `gwt` (Given/When/Then), and `//` comments are also
supported.

Run `go run ./cmd/evml svg model.evml -d out` to render a file, or
`go run ./cmd/evml svg --help`.

## Usage

### CLI

```bash
go run ./cmd/evml svg /path/to/model.evml -d out
```

Generates `out/<name>.svg`. Re-render every fixture with the mise task:

```bash
mise run svg          # only fixtures that changed
mise run svg --all    # every fixture
```

### Local web studio (`cmd/evmlweb`)

`cmd/evmlweb` is a self-contained local app (Go + [Datastar](https://data-star.dev)
+ [Kronk](https://www.kronkai.com) for local LLM inference). It uses zero deps
from the root module — run it independently:

```bash
cd cmd/evmlweb && go run .
# then open http://localhost:8080
```

The right rail is a three-tab surface for working with domain experts:

- **Discussion** (default) — chat with the local LLM to evolve the model.
- **Source** — edit the `.evml` directly and click "Validate & render" to update
  the diagram with live parse/validation feedback (no LLM round-trip needed).
- **Open questions** — a per-draft staging checklist. Add anything unclear as you
  go; tag an item as a **future goal** to track roadmap items separately. Items
  are persisted with the draft and **forked into new versions**, so staging state
  carries forward. The "+ New version" field optionally names a version
  (`v1: happy path`, `v2: with fraud`) so the tab timeline reads like a backlog.

Append `?debug=1` to any studio URL to log fetch bodies and DataStar events as
`[evmlweb:debug]` in the browser console.

## FinTech Enterprise examples

`testdata/fixtures/fintech-payments-v{N}.evml` are four progressive snapshots of
the same platform across bounded contexts. Open any of them from the studio's
**Flow** picker, or browse rendered thumbnails at **http://localhost:8080/examples**
and click "Open in studio" (deep-links via `/?flow=<slug>`). They also render
with the CLI (`evml svg`).

| Version | Bounded contexts | Adds |
|---|---|---|
| **v1-core** | Core Payments | Deposit/Withdrawal → Ledger; basic GWT (happy path + rejections). |
| **v2-fraud** | Core → Fraud | `WithdrawalRequested` crosses into `Fraud`; `RiskEvaluated` forks a held (flagged) branch and a cleared branch. |
| **v3-reconciliation** | Core → Fraud → Settlement | Daily `SettlementBatch`, bank `StatementReceived` reconciliation; a mismatch is fed back across the boundary for investigation. |
| **v4-async-fails** | Core → Fraud → Settlement → Compliance/Recovery | Async `Bank.PayoutFailed` → `ReverseAndReissuePayout` → `PayoutReissued` (the partial-failure / async-integration-failure pattern). |

Each version carries the previous model forward; its header comment lists the
future goals not yet modeled. A complete walk-through:

1. Open **v1-core** — confirm the happy path (deposit → balance, withdrawal → ledger) and the two rejection scenarios in the GWT blocks.
2. Open **v2-fraud** — notice the `Fraud` swimlane and the `RiskEvaluated { isFlagged }` fork: both `WithdrawalFlagged` (held) and `RiskAccepted`→`WithdrawalCompleted` (cleared) are modelled in one timeline.
3. Open **v3-reconciliation** — notice `Settlement` and `Bank` swimlanes; follow the `$75.00` variance from `ReconciliationMismatch` back to `InvestigateDiscrepancy`.
4. Open **v4-async-fails** — notice the `Compliance/Recovery` swimlane; the bank's `PayoutFailed` arrives *asynchronously* (`rf`, no internal source) and the recovery processor reverses and reissues over a different rail.
5. In the studio: open any version, switch to the **Open questions** tab, drop in a future-goal item, hit **+ New version**, and confirm the question carries into the new tab — that's the staging loop domain experts use to negotiate scope without touching code.
