# Compile report: billing

**Context banner:** Billing bounded context
**Pipeline:** `bounded-context-order-fulfillment-billing` v0.2.0 — on `Sales.OrderPlaced`, authorize payment; emit `PaymentAuthorized` or `PaymentDeclined`.

## Node count by kind
- `pure_function`: 3

## Phase 2 classification

| Frame | Node | Kind | Justification |
|---|---|---|---|
| rf 06 evt Sales.OrderPlaced | — | pipeline input | Boundary event is the context input. |
| tf 07 pcr BillingTranslator | `translate_billing` | `pure_function` | Translator pcr sourced from an `rf`. |
| tf 08 cmd AuthorizePayment | `decide_authorize_payment` | `pure_function` | Command handler; gwt rejection branch → `mandatory: true`. `note 08` supplies timeout/retry and marks the external gateway. |
| tf 09 evt PaymentAuthorized | — | output type | Success output of `decide_authorize_payment`. |
| tf 22 evt PaymentDeclined | — | output type | Rejection output of `decide_authorize_payment` (frame added 2026-09-19). |
| tf 10 rmo PaymentStatus | `project_payment_status` | `pure_function` | Projection into the payment-status view. |

## HITL gates
None.

## Crystallization log

| Node | `impl:` | Status | Tests |
|---|---|---|---|
| `translate_billing` | `extracted/billing.py` | **working** | translator test vs tf 08 payload |
| `decide_authorize_payment` | `extracted/billing.py` | **working** — branch on injected `issuer_result` | 2 gwts |
| `project_payment_status` | `extracted/billing.py` | **working** | tf 10 payload |

`tests/test_billing.py`: 4 tests, all passing.

## Cross-context links
- Consumes `Sales.OrderPlaced` (`rf 06`) from **sales** (`decide_place_order`).
- Produces `PaymentAuthorized` → `rf 11` input of **fulfillment**.
- Produces `PaymentDeclined` → `rf 16` input of **sales** (recovery slice). `decide_authorize_payment` is a listed exit node.

## Judgment calls
- The two tf 08 gwts share identical given/when payloads — authorize vs decline is the card issuer's decision. Implemented with injected `issuer_result`/`authorization_id`; `note 08` (`external_system: payment_gateway`, `timeout: 10s`, `retry: 3× exponential`) is carried into the node config — emission candidate for `external_call`.
- `paymentMethodId` (`pm-9`) is not on `Sales.OrderPlaced`; translator resolves the stored instrument (parameterised default).

## Open questions
None blocking. Residual: `decide_authorize_payment` should likely become `external_call` at emission once the real gateway is named.

## Changes (recompile 2026-09-19)
- `.evml`: added `tf 22 evt PaymentDeclined ->> 08` (the decline event now has a frame) and `note 08` runtime config.
- Node gained `timeout`/`retry`/`constants.external_system` from the note; `decide_authorize_payment` added to `exit_nodes` (both outcomes cross rf boundaries).
- Working implementations + 4 golden tests; `load_pipeline` + `rote emit` pass.
