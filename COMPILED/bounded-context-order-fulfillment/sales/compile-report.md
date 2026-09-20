# Compile report: sales

**Context banner:** Sales bounded context (+ `Sales: billing failure recovery` sub-banner)
**Pipeline:** `bounded-context-order-fulfillment-sales` v0.2.0 — two slices: (1) on `PlaceOrderCommand`, decide the order → `OrderPlaced`/`OrderPlacementRejected`; (2) on `Billing.PaymentDeclined`, cancel the order → `OrderCancelled`/`OrderCancellationIgnored`. `Pricing.CartPriced` feeds the `CartSummary` read model.

## Node count by kind
- `pure_function`: 6

## Phase 2 classification

| Frame | Node | Kind | Justification |
|---|---|---|---|
| tf 01 ui CheckoutScreen | — | pipeline input | First frame of the context, feeds `cmd PlaceOrder`. |
| rf 21 evt Pricing.CartPriced | — | pipeline input | Boundary event from the pricing context. |
| tf 02 rmo CartSummary | `project_cart_summary` | `pure_function` | Projection folding CartPriced into the CartSummary02 view. |
| tf 03 cmd PlaceOrder | `decide_place_order` | `pure_function` | Command handler; gwt rejection branch → `mandatory: true`. |
| tf 04 evt OrderPlaced | — | output type | Event is the `decide_place_order` output. |
| tf 05 rmo OrderStatus | `project_order_status` | `pure_function` | Projection into OrderStatus05 view. |
| rf 16 evt Billing.PaymentDeclined | — | pipeline input | Boundary event from billing. |
| tf 17 pcr SalesRecoveryTranslator | `translate_sales_recovery` | `pure_function` | Translator pcr sourced from an `rf`. |
| tf 18 cmd CancelOrder | `decide_cancel_order` | `pure_function` | Command handler; idempotency branch → `mandatory: true`. |
| tf 19 evt OrderCancelled | — | output type | Event is the `decide_cancel_order` output. |
| tf 24 evt OrderCancellationIgnored | — | output type | Alternative `decide_cancel_order` output. |
| tf 20 rmo CancelledOrderStatus | `project_cancelled_order_status` | `pure_function` | Projection into the cancelled-status view. |

## HITL gates
None — the only `ui` is the context's entry frame.

## Crystallization log

| Node | `impl:` | Status | Tests |
|---|---|---|---|
| `decide_place_order` | `extracted/sales.py` | **working** — total ≤ 0 → reject; `orderId` = `ord-<cartId suffix>` | 2 gwts |
| `project_cart_summary` | `extracted/sales.py` | **working** — straight fold | `data CartSummary02` |
| `project_order_status` | `extracted/sales.py` | **working** — total over the result union | `data OrderStatus05` |
| `translate_sales_recovery` | `extracted/sales.py` | **working** | translator test vs tf 18 payload |
| `decide_cancel_order` | `extracted/sales.py` | **working** — prior `OrderCancelled` → `OrderCancellationIgnored` | 2 gwts |
| `project_cancelled_order_status` | `extracted/sales.py` | **working** | tf 20 payload |

`tests/test_sales.py`: 8 tests, all passing.

## Cross-context links
- Consumes `Pricing.CartPriced` (`rf 21`) — produced outside this model (pricing context).
- Consumes `Billing.PaymentDeclined` (`rf 16`) produced by **billing** (`decide_authorize_payment`).
- Produces `OrderPlaced` → `rf 06` input of **billing** (`decide_place_order` is a listed exit node).

## Judgment calls
- Three boundary inputs in one context: modelled as a union `SalesInput` (`PlaceOrderCommand` | `Pricing.CartPriced` | `Billing.PaymentDeclined`); the three entry nodes each bind `pipeline.input` and the runtime dispatches on event/command type.
- `orderId` derived `ord-<cartId suffix>` for replay-safety; real assignment belongs to the order service.
- `rf 21` payload enriched with `customerId`/`items` so the CartSummary fold is fully determined by `data CartSummary02`.
- Recovery slice merged here per the renamed banner (`Sales: billing failure recovery`); the former `sales-recovery` context directory was removed.

## Open questions
None — all prior questions resolved in the model (see Changes).

## Changes (recompile 2026-09-19)
- `.evml`: added `rf 21 evt Pricing.CartPriced` + `->> 21` on CartSummary (fixes the sourceless read model); `tf 01 ui ->> 02`; added `tf 24 evt OrderCancellationIgnored ->> 18`; renamed the recovery banner to `Sales: billing failure recovery`.
- Absorbed the sales-recovery context (frames 16-20,24): +3 nodes, tests merged.
- Implemented all functions for real; `tests/test_sales.py` now 8 tests.
- `edges:`/`input:` schema shapes fixed; `load_pipeline` + `rote emit` pass; `conftest.py` added.
