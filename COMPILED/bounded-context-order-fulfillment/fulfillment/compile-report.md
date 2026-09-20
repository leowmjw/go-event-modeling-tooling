# Compile report: fulfillment

**Context banner:** Fulfillment bounded context
**Pipeline:** `bounded-context-order-fulfillment-fulfillment` v0.2.0 — on `Billing.PaymentAuthorized`, allocate shipment → `ShipmentAllocated`/`ShipmentAllocationDeferred`; on `Warehouse.InventoryReplenished`, re-issue allocation for deferred orders.

## Node count by kind
- `pure_function`: 4

## Phase 2 classification

| Frame | Node | Kind | Justification |
|---|---|---|---|
| rf 11 evt Billing.PaymentAuthorized | — | pipeline input | Boundary event. |
| tf 12 pcr FulfillmentTranslator | `translate_fulfillment` | `pure_function` | Translator pcr sourced from an `rf`. |
| tf 13 cmd AllocateShipment | `decide_allocate_shipment` | `pure_function` | Command handler; deferral branch → `mandatory: true`. `note 13` supplies timeout/retry + external system. |
| tf 14 evt ShipmentAllocated | — | output type | Success output. |
| tf 23 evt ShipmentAllocationDeferred | — | output type | Deferral output (frame added 2026-09-19); also feeds `retry_allocation`. |
| tf 15 rmo ShipmentQueue | `project_shipment_queue` | `pure_function` | Projection into ShipmentQueue15 view. |
| rf 25 evt Warehouse.InventoryReplenished | — | pipeline input | Second boundary event (added 2026-09-19). |
| tf 26 pcr AllocationRetrier | `retry_allocation` | `pure_function` | Deterministic rule pcr — re-issues the command when stock returns. |

## HITL gates
None.

## Crystallization log

| Node | `impl:` | Status | Tests |
|---|---|---|---|
| `translate_fulfillment` | `extracted/fulfillment.py` | **working** | translator test vs tf 13 payload |
| `decide_allocate_shipment` | `extracted/fulfillment.py` | **working** — branch on injected `inventory_available` | 2 gwts |
| `project_shipment_queue` | `extracted/fulfillment.py` | **working** — total over the result union | `data ShipmentQueue15` |
| `retry_allocation` | `extracted/fulfillment.py` | **working** — orderId from the deferred event, warehouseId from replenishment | 1 test |

`tests/test_fulfillment.py`: 5 tests, all passing.

## Cross-context links
- Consumes `Billing.PaymentAuthorized` (`rf 11`) from **billing** (`decide_authorize_payment`).
- Consumes `Warehouse.InventoryReplenished` (`rf 25`) — produced outside this model (warehouse context).

## Judgment calls
- Allocate vs defer depends on warehouse inventory, external to the command payload — injected `inventory_available`/`shipment_id`; `note 13` marks `external_system: warehouse_inventory` (emission candidate for `external_call`).
- **Retry loop breaks the DAG:** `AllocationRetrier` feeds `cmd AllocateShipment`, but an edge `retry_allocation → decide_allocate_shipment` would create a cycle. Modelled instead as an exit node emitting a fresh `AllocateShipmentCommand`; the runtime should re-dispatch it as a new command on the same workflow (or a signal), not as a graph edge.
- `warehouseId` not carried by `PaymentAuthorized`; translator picks the warehouse (parameterised default `wh-1`).

## Open questions
None — the deferred-allocation retry is now modelled (rf 25 + tf 26) and the missing evt frames were added.

## Changes (recompile 2026-09-19)
- `.evml`: added `tf 23 evt ShipmentAllocationDeferred ->> 13`, `rf 25 evt Warehouse.InventoryReplenished`, `tf 26 pcr AllocationRetrier ->> 25 ->> 23`, `note 13`, and explicit `->> 12 ->> 26` sources on `cmd AllocateShipment`.
- +1 node (`retry_allocation`); node config from `note 13`; input widened to a union.
- Working implementations + 5 tests; `load_pipeline` + `rote emit` pass.
