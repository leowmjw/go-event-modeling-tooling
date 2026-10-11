# Compile report: billing

**Context banner:** `Billing bounded context`
**Pipeline:** `bounded-context-order-fulfillment-billing` v0.3.0 — on `Sales.OrderPlaced`, authorize payment through the gateway; emit `PaymentAuthorized` or `PaymentDeclined`; project `PaymentStatus`.

## Node count by kind
- `pure_function`: 3
- `external_call`: 1

## Phase 2 classification

| Frame | Node | Kind | Justification |
|---|---|---|---|
| rf 06 evt Sales.OrderPlaced | — | pipeline input | Boundary event; carries `paymentMethodId` chosen at checkout. |
| tf 07 pcr BillingTranslator | `translate_billing` | `pure_function` | Translator sourced from an `rf`. |
| tf 08 cmd AuthorizePayment | `authorize_payment_with_gateway` | `external_call` | `note 08` names `payment_gateway`, its result contract, timeout, retry and idempotency key. No MCP server is named, so no `mcp:` binding. |
| tf 08 cmd AuthorizePayment | `decide_authorize_payment` | `pure_function` | Pure decision over command + gateway result; decline branch → `mandatory: true`. |
| tf 09 evt PaymentAuthorized / tf 22 evt PaymentDeclined | — | output types | Alternatives of `decide_authorize_payment`. |
| tf 10 rmo PaymentStatus | `project_payment_status` | `pure_function` | Values fixed by the frame payload + 2 gwts. |

## HITL gates
None.

## Crystallization log

| Node | `impl:` | Status | Tests |
|---|---|---|---|
| `translate_billing` | `extracted/billing.py` | working | tf 08 payload |
| `authorize_payment_with_gateway` | `extracted/billing.py` | working adapter — client registered via `configure_gateway`; validates the result | idempotency key, unconfigured, incomplete results ×3 |
| `decide_authorize_payment` | `extracted/billing.py` | working | gwt 08 ×2 |
| `project_payment_status` | `extracted/billing.py` | working | gwt 10 ×2 |

`tests/test_billing.py`: 10 tests, all passing.

## Cross-context links
- Consumes `Sales.OrderPlaced` (`rf 06`) from **sales** (`decide_place_order`).
- Produces `PaymentAuthorized` → `rf 11` of **fulfillment** and `rf 30` of **sales**; `PaymentDeclined` → `rf 16` of **sales**. `decide_authorize_payment` is the exit node.

## Judgment calls
- One `cmd` frame yields two nodes (external call + pure decision) because the outcome is the gateway's, not derivable from the command; both carry `source.section: tf 08 cmd AuthorizePayment`.
- The gateway client is deployment configuration (ir-schema rule 3), not a node parameter.
- An incomplete gateway result (approved without `authorizationId`, declined without `reason`) is an error, never defaulted.

## Open questions
None.

## Changes (recompile 2026-10-03) — 0.2.0 → 0.3.0 (minor)
- Model: `Sales.OrderPlaced` now carries `paymentMethodId` (no more `pm-9` default in the translator); `note 08` gains `idempotency_key` and `gateway_result`; `gwt 08` decline reason unified to `insufficient_funds` (matches `tf 22` / `rf 16`); `gwt 10` pins `PaymentStatus` for both outcomes.
- **Added** `authorize_payment_with_gateway` (external_call); timeout/retry moved onto it from `decide_authorize_payment`, which now binds the gateway output instead of an injected `issuer_result`. Node ids preserved.
