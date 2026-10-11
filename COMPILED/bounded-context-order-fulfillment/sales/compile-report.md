# Compile report: sales

**Context banner:** `Sales bounded context` (+ sub-banners `Sales: billing failure recovery`, `Sales: payment confirmation`)
**Pipeline:** `bounded-context-order-fulfillment-sales` v1.0.0 — on `Pricing.CartPriced`, show `CartSummary`, wait for the customer's `PlaceOrder`, decide `OrderPlaced` / `OrderPlacementRejected`; then close the order from the billing outcome: `ConfirmOrder` on `Billing.PaymentAuthorized`, `CancelOrder` on `Billing.PaymentDeclined`. `OrderStatus` folds every order event.

## Node count by kind
- `pure_function`: 7
- `hitl_gate`: 1

## Phase 2 classification

| Frame | Node | Kind | Justification |
|---|---|---|---|
| rf 21 evt Pricing.CartPriced | — | pipeline input `cart_priced` | Boundary event from pricing; starts the workflow. |
| tf 02 rmo CartSummary | `project_cart_summary` | `pure_function` | Fold of CartPriced; `data CartSummary02` is the golden value. |
| tf 01 ui CheckoutScreen | `gate_place_order` | `hitl_gate` | Mid-flow `ui`: sourced from `rmo 02`, feeds `cmd 03`. Signal `place_order`; `note 01` names the channel. |
| tf 03 cmd PlaceOrder | `decide_place_order` | `pure_function` | Both gwts are payload comparisons; rejection branch → `mandatory: true`. |
| tf 04 evt OrderPlaced | — | output type | `data OrderPlaced04`; crosses to billing (`rf 06`) and fulfillment (`rf 28`). |
| tf 27 evt OrderPlacementRejected | — | output type | Rejection alternative of `decide_place_order`. |
| tf 05 rmo OrderStatus | `project_order_status` | `pure_function` | Fold over placed/confirmed/cancelled; values fixed by `data OrderStatus05` + 3 gwts. |
| rf 30 evt Billing.PaymentAuthorized | — | pipeline input `payment_authorized` | Delivered to the running workflow. |
| tf 31 pcr SalesConfirmationTranslator | `translate_sales_confirmation` | `pure_function` | Translator sourced from an `rf`. |
| tf 32 cmd ConfirmOrder | `decide_confirm_order` | `pure_function` | Idempotency branch from history → `mandatory: true`. |
| tf 33 evt OrderConfirmed / tf 34 evt OrderConfirmationIgnored | — | output types | Alternatives of `decide_confirm_order`. |
| rf 16 evt Billing.PaymentDeclined | — | pipeline input `payment_declined` | Delivered to the running workflow. |
| tf 17 pcr SalesRecoveryTranslator | `translate_sales_recovery` | `pure_function` | Translator sourced from an `rf`. |
| tf 18 cmd CancelOrder | `decide_cancel_order` | `pure_function` | Idempotency branch from history → `mandatory: true`. |
| tf 19 evt OrderCancelled / tf 24 evt OrderCancellationIgnored | — | output types | Alternatives of `decide_cancel_order`. |

## HITL gates
| Gate | Signal | Payload | Timeout |
|---|---|---|---|
| `gate_place_order` | `place_order` | `PlaceOrderCommand` from CheckoutScreen | none — see judgment calls |

## Crystallization log

| Node | `impl:` | Status | Tests |
|---|---|---|---|
| `project_cart_summary` | `extracted/sales.py` | working | `data CartSummary02` |
| `decide_place_order` | `extracted/sales.py` | working — items/total from cart; empty cart → reject; cart mismatch → invalid input | gwt 03 ×2 + mismatch |
| `project_order_status` | `extracted/sales.py` | working | gwt 05 ×3, `data OrderStatus05`, rejected → none |
| `translate_sales_confirmation` | `extracted/sales.py` | working | tf 32 payload |
| `decide_confirm_order` | `extracted/sales.py` | working | gwt 32 ×2 |
| `translate_sales_recovery` | `extracted/sales.py` | working | tf 18 payload |
| `decide_cancel_order` | `extracted/sales.py` | working | gwt 18 ×2 + never-placed |

`tests/test_sales.py`: 15 tests, all passing. Every `then` payload is asserted in full.

## Cross-context links
- Consumes `Pricing.CartPriced` (`rf 21`) — produced outside this model.
- Produces `OrderPlaced` (`decide_place_order`, exit node) → `rf 06` of **billing** and `rf 28` of **fulfillment**.
- Consumes `Billing.PaymentAuthorized` (`rf 30`) and `Billing.PaymentDeclined` (`rf 16`) from **billing** (`decide_authorize_payment`).

## Judgment calls
- **Input envelope.** Four boundary facts share one `SalesInput`: `cart_priced` (required, starts the run), `payment_authorized` / `payment_declined` (delivered later), `order_history`. Entry nodes bind `pipeline.input.<field>`, so no node receives a fact of the wrong type.
- **Order history.** `decide_confirm_order` / `decide_cancel_order` need prior events of the order; IR bindings cannot reference a node's own earlier outputs, so the history is an explicit optional input `order_history` that the runtime fills from durable state (never a caller). Absent means nothing was recorded.
- **Gate timeout.** `config.hitl.default_timeout` (7d) is not applied: the model has no abandonment event, so a timeout would end the context without a terminal fact. Add e.g. `evt CheckoutAbandoned` to the model if checkout should expire.
- **Rejection rule.** "no priced items" is implemented as `items == []` (the gwt label); a non-empty cart with total 0 would be placed.

## Open questions
None.

## Changes (recompile 2026-10-03) — 0.2.0 → 1.0.0 (major)
- Parser fix: `->>` after a payload had been silently dropped, so the prior intake was missing `tf 13 ->> 12 ->> 26`, `tf 22/23/24` sources. The `.evml` now follows the grammar's clause order.
- Model (with the expert's go-ahead): `orderId` and `paymentMethodId` now come from `cmd PlaceOrder` (CheckoutScreen, `note 03`); `OrderPlaced` carries `cartId`, `paymentMethodId`, `items`; added `tf 27 evt OrderPlacementRejected`; added the **payment confirmation** slice (`rf 30`–`tf 34`) so Sales closes the authorized branch; removed `tf 20 rmo CancelledOrderStatus` (merged into `OrderStatus`, one question); `gwt 05` State View scenarios fix the status values.
- **Removed node** `project_cancelled_order_status` (major). **Added** `gate_place_order`, `translate_sales_confirmation`, `decide_confirm_order`. `decide_place_order` now binds the gate output and `CartSummary` (total no longer trusted from the command); decide nodes bind `order_history`.
- In-flight runs of the old version must drain on the old worker.
