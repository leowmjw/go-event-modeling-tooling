# Go bounded-context order fulfillment

Standalone Go + Temporal implementation generated from `bounded-context-order-fulfillment.evml` and the validated rote IR in the adjacent `COMPILED/bounded-context-order-fulfillment/` directory. Runtime code has no dependency on the repository root, Python, or rote.

## Architecture

- `OrderFulfillmentWorkflowV1` — parent process workflow.
- `SalesWorkflowV1` — place/reject an order and project `CartSummary`/`OrderStatus`.
- `BillingWorkflowV1` — authorize payment and project `PaymentStatus`.
- `FulfillmentWorkflowV1` — allocate shipment, project/query `ShipmentQueue`, and wait durably for inventory replenishment after deferral.
- `SalesRecoveryWorkflowV1` — cancel an order idempotently from prior domain events and project `CancelledOrderStatus`.
- Task queue: `bounded-context-order-fulfillment-v1`.

The parent starts child workflows with stable IDs derived from the parent workflow ID. Cross-context facts are typed child inputs. Payment and inventory I/O are activities; workflows contain deterministic orchestration, decisions, and projections only.

## IR coverage

| Context | IR node | Go mapping | Runtime shape |
|---|---|---|---|
| sales | `decide_place_order` | `DecidePlaceOrder` | deterministic decision |
| sales | `project_cart_summary` | `ProjectCartSummary` | returned projection |
| sales | `project_order_status` | `ProjectOrderStatus` | returned projection |
| sales | `translate_sales_recovery` | `TranslateSalesRecovery` | deterministic translator |
| sales | `decide_cancel_order` | `DecideCancelOrder` | prior-event decision |
| sales | `project_cancelled_order_status` | `CancelledOrderStatusView` construction | returned projection |
| billing | `translate_billing` | `TranslateBilling` | deterministic translator |
| billing | `decide_authorize_payment` | `AuthorizePayment` + `DecidePayment` | activity boundary + decision |
| billing | `project_payment_status` | `PaymentStatusView` construction | returned projection |
| fulfillment | `translate_fulfillment` | `TranslateFulfillment` | deterministic translator |
| fulfillment | `decide_allocate_shipment` | `CheckInventory` + `DecideShipment` | activity boundary + decision |
| fulfillment | `project_shipment_queue` | `ShipmentQueueView` | returned and queryable projection |
| fulfillment | `retry_allocation` | `inventory_replenished` signal loop | durable feedback edge |

All 13 IR nodes are represented. No IR node, edge, exit projection, retry, or gwt is report-only.

## External activity contracts

`Activities` injects:

- `PaymentGateway.Authorize` → `PaymentGatewayResponse`
- `InventoryService.Allocate` → `InventoryResponse`

Payment uses a 10-second start-to-close timeout and inventory uses 30 seconds. Both use four total attempts (`retry.max: 3` plus the initial attempt), one-second initial backoff, exponential coefficient 2, and a one-minute cap.

The demo worker supports:

- `PAYMENT_OUTCOME=authorized|declined`
- `INVENTORY_OUTCOME=allocated|deferred`

`INVENTORY_OUTCOME=deferred` returns deferred on the first allocation and allocated after a matching replenishment signal. Invalid values fail worker startup rather than silently choosing behavior.

## Demo prerequisites

The demo assumes these commands are available:

```sh
mise --version
overmind --version
temporal --version
```

`mise` installs the latest stable Go release (currently 1.27) and Overmind from the local `.mise.toml`; Temporal CLI is expected at `$HOME/go/bin/temporal`. `mise run doctor` verifies Go, mise-managed Overmind, tmux, Temporal CLI, module integrity, and required demo files. Overmind runs the `Procfile`: a persistent local Temporal development server on `localhost:7233` with UI at `http://localhost:8233`, plus the worker. Local state is stored under ignored `.temporal/`.

## Demo: successful order

From this directory:

```sh
mise install
mise run doctor
mise run check
mise run demo
```

Keep that terminal running. In another terminal:

```sh
mise run start -- -workflow-id order-success
```

The result contains `OrderPlaced`, `PaymentAuthorized`, `ShipmentAllocated`, and all projected read models. Inspect parent and child histories in the Temporal UI.

Stop services when finished:

```sh
mise run demo:stop
```

## Demo: rejected empty cart

With the demo running:

```sh
mise run reject
```

The sales child returns `OrderPlacementRejected`; billing and fulfillment are never started.

## Demo: declined payment and cancellation

Start the stack with a declined gateway:

```sh
PAYMENT_OUTCOME=declined mise run demo
```

Then run:

```sh
mise run start -- -workflow-id order-declined
```

The result contains `PaymentDeclined`, `OrderCancelled`, and `CancelledOrderStatus`. Stop the stack before switching worker outcomes.

## Demo: deferred shipment and replenishment

Start a worker whose first inventory allocation defers:

```sh
INVENTORY_OUTCOME=deferred mise run demo
```

Start the order in a second terminal; it waits durably in fulfillment:

```sh
mise run start -- -workflow-id order-deferred
```

While it waits, query the child read model:

```sh
temporal workflow query \
  --workflow-id order-deferred/fulfillment \
  --type shipment_queue
```

Expected status: `deferred:inventory_unavailable`.

Send the modeled `Warehouse.InventoryReplenished` fact from a third terminal:

```sh
go run ./cmd/replenish \
  -workflow-id order-deferred \
  -warehouse-id wh-1 \
  -sku sku-1
```

The child retries allocation, projects `queued_for_picking`, completes, and releases the parent. The blocked starter then prints the final JSON result.

The convenience tasks target the default `order-ord-100` workflow:

```sh
mise run query
mise run replenish
```

## Development checks

```sh
mise run doctor
mise run test
mise run vet
mise run check
```

`mise run check` requires clean `gofmt`, clean `go mod tidy -diff`, and passing race tests and vet. Temporal testsuite tests need no server. External activities are replaced with anonymous functions registered through `RegisterActivityWithOptions`; closure counters assert exact calls without interface mocks.

Coverage includes all eight gwts, every read-model projection, invalid boundary values, parent authorized/rejected/declined paths, deferred shipment plus live query and replenishment, full parent feedback handling, and retry after a transient payment failure.

## Source artifacts

- Intake snapshot: `model.json` (sha256/source metadata included).
- Runtime-neutral IR, provenance, and compile reports remain authoritative in `../bounded-context-order-fulfillment/{sales,billing,fulfillment}/`; they are not duplicated here to avoid drift.
- Pipeline versions: sales `0.2.0`, billing `0.2.0`, fulfillment `0.2.0`.
- Go SDK: `go.temporal.io/sdk v1.48.0` (released 2026-08-18).

## Temporal parity and determinism

This implementation preserves public-SDK activity timeout/retry semantics, mandatory execution, typed exit results, read-model projections, deterministic workflow code, and external I/O isolation. The fulfillment feedback edge is represented by a durable Signal and Query on the stateful fulfillment child instead of an invalid cyclic DAG edge. The parent/child topology is intentional for the model's bounded contexts.

Workflow code uses only Temporal workflow APIs for orchestration. It performs no filesystem, network, database, wall-clock, random, native goroutine, or native-channel operations. No HITL, fan-out, LLM, MCP, or agent-loop nodes occur in this model.
