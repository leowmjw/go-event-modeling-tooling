# Compile report: fulfillment

**Context banner:** `Fulfillment bounded context`
**Pipeline:** `bounded-context-order-fulfillment-fulfillment` v1.0.0 — fold `Sales.OrderPlaced` + `Billing.PaymentAuthorized` into `PaidOrdersToShip`; allocate through warehouse inventory → `ShipmentAllocated` / `ShipmentAllocationDeferred`; on `Warehouse.InventoryReplenished` for a missing SKU, re-issue the allocation.

## Node count by kind
- `pure_function`: 5
- `external_call`: 1

## Phase 2 classification

| Frame | Node | Kind | Justification |
|---|---|---|---|
| rf 28 evt Sales.OrderPlaced | — | pipeline input `order_placed` | Order items enter through translation, not by reading Sales data. |
| rf 11 evt Billing.PaymentAuthorized | — | pipeline input `payment_authorized` | Boundary event. |
| tf 29 rmo PaidOrdersToShip | `project_paid_orders_to_ship` | `pure_function` | Canonical translation read model; `data PaidOrdersToShip29` + 2 gwts. |
| tf 12 pcr FulfillmentTranslator | `translate_fulfillment` | `pure_function` | Deterministic: ready order → command. |
| tf 13 cmd AllocateShipment | `allocate_shipment_with_warehouse` | `external_call` | `note 13` names `warehouse_inventory`, result contract, timeout, retry, idempotency key. |
| tf 13 cmd AllocateShipment | `decide_allocate_shipment` | `pure_function` | Pure decision over command + warehouse result; deferral branch → `mandatory: true`. |
| tf 14 evt ShipmentAllocated / tf 23 evt ShipmentAllocationDeferred | — | output types | Alternatives of `decide_allocate_shipment`. |
| tf 15 rmo ShipmentQueue | `project_shipment_queue` | `pure_function` | `data ShipmentQueue15` + 2 gwts. |
| rf 25 evt Warehouse.InventoryReplenished | — | pipeline input `inventory_replenished` | Delivered to the running workflow while deferred. |
| tf 26 pcr AllocationRetrier | `retry_allocation` | `pure_function` | Rule stated in `note 26` + 2 gwts. |

## HITL gates
None.

## Crystallization log

| Node | `impl:` | Status | Tests |
|---|---|---|---|
| `project_paid_orders_to_ship` | `extracted/fulfillment.py` | working | gwt 29 ×2, data block |
| `translate_fulfillment` | `extracted/fulfillment.py` | working | tf 13 payload, unpaid refused |
| `allocate_shipment_with_warehouse` | `extracted/fulfillment.py` | working adapter — client via `configure_warehouse` | attempt-scoped key, unconfigured, incomplete ×3 |
| `decide_allocate_shipment` | `extracted/fulfillment.py` | working | gwt 13 ×2 |
| `project_shipment_queue` | `extracted/fulfillment.py` | working | gwt 15 ×2, data block |
| `retry_allocation` | `extracted/fulfillment.py` | working | gwt 26 ×2 |

`tests/test_fulfillment.py`: 15 tests, all passing.

## Cross-context links
- Consumes `Sales.OrderPlaced` (`rf 28`) from **sales** and `Billing.PaymentAuthorized` (`rf 11`) from **billing**.
- Consumes `Warehouse.InventoryReplenished` (`rf 25`) — produced outside this model (warehouse system).

## Judgment calls
- **Retry feedback edge.** `tf 13 ->> 26` makes `retry_allocation → allocate_shipment_with_warehouse` a cycle. It is recorded as `constants.feedback_to` instead of an edge; runtimes deliver replenishment to the running workflow and loop.
- **Idempotency key per attempt.** A single `orderId` key would make the warehouse replay the cached deferral on re-issue, so the key is `<orderId>/allocation-<attempt>` (`note 13`). The attempt number is runtime state passed as a keyword argument (1 on a one-shot run).
- An incomplete warehouse result is an error, never defaulted. The warehouse, not Fulfillment, chooses `warehouseId`.

## Open questions
None.

## Changes (recompile 2026-10-03) — 0.2.0 → 1.0.0 (major)
- Parser fix restored `tf 13 ->> 12 ->> 26` and `tf 23 ->> 13` in the intake.
- Model: added `rf 28 Sales.OrderPlaced` + `tf 29 rmo PaidOrdersToShip` (order items now reach Fulfillment); `AllocateShipment` carries `items` instead of a defaulted `warehouseId`; `ShipmentAllocationDeferred` carries `missingSkus`; `AllocationRetrier` also reads `PaidOrdersToShip` and only reacts to missing SKUs (`note 26`); gwts 15/26/29 added.
- **Added** `project_paid_orders_to_ship`, `allocate_shipment_with_warehouse`. `translate_fulfillment` now binds the read model; `AllocateShipmentCommand` schema changed incompatibly (major). In-flight runs of the old version must drain on the old worker.
