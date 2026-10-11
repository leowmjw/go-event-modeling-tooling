# Go bounded-context order fulfillment

Standalone Go + Temporal implementation generated from `bounded-context-order-fulfillment.evml` and the validated rote IR in the adjacent `COMPILED/bounded-context-order-fulfillment/` directory. Runtime code has no dependency on the repository root, Python, or rote.

## Architecture

- `OrderFulfillmentWorkflowV2` — parent; coordinates the cross-context facts (`rf 06`/`rf 28` from `OrderPlaced`, `rf 11`/`rf 30` from `PaymentAuthorized`, `rf 16` from `PaymentDeclined`).
- `SalesWorkflowV2` (`<id>/sales`) — projects `CartSummary`, waits at the CheckoutScreen gate for `PlaceOrder`, decides placement, then closes the order from the billing outcome (`ConfirmOrder` / `CancelOrder`) and projects `OrderStatus`.
- `BillingWorkflowV2` (`<id>/billing`) — authorizes payment through the gateway activity and projects `PaymentStatus`.
- `FulfillmentWorkflowV2` (`<id>/fulfillment`) — projects `PaidOrdersToShip`, allocates through the warehouse activity, projects `ShipmentQueue`, and on deferral waits durably for `InventoryReplenished` of a missing SKU.
- Task queue: `bounded-context-order-fulfillment-v2`. V2 is a new workflow type set because every context's input/output changed incompatibly with V1; drain V1 runs on a V1 worker.

Flow: the starter delivers `CartPriced` and returns. Sales waits for the `place_order` signal (sent by `cmd/checkout`), then reports the placement to the parent with `SignalExternalWorkflow`. On `OrderPlaced` the parent runs Billing, sends the outcome to Sales on `billing_outcome` with `SignalChildWorkflow`, and on authorization starts Fulfillment. A rejected placement ends the run inside Sales.

| Signal | Target | Payload | IR mapping |
|---|---|---|---|
| `place_order` | `<id>/sales` | `PlaceOrderCommand` | `gate_place_order` (hitl_gate, signal `place_order`) |
| `order_placement` | parent | `PlaceOrderResult` | cross-context link: `decide_place_order` exit |
| `billing_outcome` | `<id>/sales` | `BillingOutcome` | `pipeline.input.payment_authorized` / `payment_declined` |
| `inventory_replenished` | `<id>/fulfillment` | `InventoryReplenished` | `pipeline.input.inventory_replenished` + `feedback_to` loop |

| Query | Workflow | Returns |
|---|---|---|
| `cart_summary` | sales | `CartSummaryView` |
| `order_status` | sales | `*OrderStatusView` (null before placement / when rejected) |
| `payment_status` | billing | `*PaymentStatusView` |
| `paid_orders_to_ship` | fulfillment | `PaidOrdersToShipView` |
| `shipment_queue` | fulfillment | `*ShipmentQueueView` |

## IR coverage

| Context | IR node | Kind | Go mapping | Runtime shape |
|---|---|---|---|---|
| sales | `project_cart_summary` | pure_function | `ProjectCartSummary` | workflow projection + `cart_summary` query |
| sales | `gate_place_order` | hitl_gate | `place_order` signal channel in `SalesWorkflow` | durable wait, no timeout; invalid commands are logged and the gate keeps waiting |
| sales | `decide_place_order` | pure_function | `DecidePlaceOrder` | deterministic decision; result signalled to parent |
| sales | `translate_sales_confirmation` | pure_function | `TranslateSalesConfirmation` | deterministic translator |
| sales | `decide_confirm_order` | pure_function | `DecideConfirmOrder` | decision over workflow-recorded `History` |
| sales | `translate_sales_recovery` | pure_function | `TranslateSalesRecovery` | deterministic translator |
| sales | `decide_cancel_order` | pure_function | `DecideCancelOrder` | decision over workflow-recorded `History` |
| sales | `project_order_status` | pure_function | `ProjectOrderStatus` | workflow projection + `order_status` query |
| billing | `translate_billing` | pure_function | `TranslateBilling` | deterministic translator |
| billing | `authorize_payment_with_gateway` | external_call | `Activities.AuthorizePaymentWithGateway` | activity, 10s, 4 attempts, key `orderId` |
| billing | `decide_authorize_payment` | pure_function | `DecideAuthorizePayment` | deterministic decision |
| billing | `project_payment_status` | pure_function | `ProjectPaymentStatus` | projection + `payment_status` query |
| fulfillment | `project_paid_orders_to_ship` | pure_function | `ProjectPaidOrdersToShip` | projection + `paid_orders_to_ship` query |
| fulfillment | `translate_fulfillment` | pure_function | `TranslateFulfillment` | deterministic translator |
| fulfillment | `allocate_shipment_with_warehouse` | external_call | `Activities.AllocateShipmentWithWarehouse` | activity, 30s, 4 attempts, key `<orderId>/allocation-<n>` |
| fulfillment | `decide_allocate_shipment` | pure_function | `DecideAllocateShipment` | deterministic decision |
| fulfillment | `project_shipment_queue` | pure_function | `ProjectShipmentQueue` | projection + `shipment_queue` query |
| fulfillment | `retry_allocation` | pure_function | `RetryAllocation` in the `inventory_replenished` loop | durable Signal feedback edge (`feedback_to: allocate_shipment_with_warehouse`) |

All 18 IR nodes are mapped; none is report-only. Every IR edge is a data dependency in the workflow code; the gate edge uses the `place_order` signal. Mandatory decision nodes always run before their context completes. Exit nodes are the typed child results (`SalesResult.Placement`/`OrderStatus`, `BillingResult.Payment`/`PaymentStatus`, `FulfillmentResult.ShipmentQueue`).

Idempotency comes from workflow state, never from callers: Sales records every decided event in `History` and the confirm/cancel decisions read it, so a duplicate `billing_outcome` produces `OrderConfirmationIgnored` / `OrderCancellationIgnored`.

## External activity contracts

`Activities` injects:

- `PaymentGateway.Authorize(ctx, AuthorizePaymentCommand, idempotencyKey)` → `GatewayAuthorization` — approved with `authorizationId`, or declined with `reason` (note 08).
- `WarehouseInventory.Allocate(ctx, AllocateShipmentCommand, idempotencyKey)` → `WarehouseAllocation` — allocated with `shipmentId` and `warehouseId`, or unavailable with `missingSkus` (note 13).

An incomplete result fails the activity as non-retryable (`InvalidGatewayResult` / `InvalidWarehouseResult`); nothing is defaulted. Transport errors retry with one-second initial backoff, coefficient 2, and four total attempts (`retry.max: 3` plus the first). Activity retries reuse the same idempotency key; a replenishment-triggered re-issue uses the next attempt number.

The demo worker supports:

- `PAYMENT_OUTCOME=approved|declined` (declined reason `insufficient_funds`)
- `INVENTORY_OUTCOME=allocated|deferred` (deferred reports `sku-1` missing on each order's first attempt key)

Both demo adapters replay the stored answer for a repeated idempotency key. Invalid values fail worker startup.

## Demo prerequisites

`mise` installs the latest stable Go and Overmind from the local `.mise.toml`; the Temporal CLI is expected at `$HOME/go/bin/temporal`; Overmind needs `tmux`. `mise run doctor` verifies all of these plus module integrity and the demo files. Overmind runs the `Procfile`: a local Temporal dev server on `localhost:7233` (UI `http://localhost:8233`, state in ignored `.temporal/`) and the worker.

## Demo

```sh
mise install
mise run check
mise run demo            # keep running; Ctrl-C or `mise run demo:stop` in another terminal
```

In another terminal, step through one order (default workflow ID `checkout-c-100`; override with `WORKFLOW_ID=...`):

```sh
mise run start                 # CartPriced → Sales waits at the CheckoutScreen gate
mise run query:cart-summary    # CartSummary the customer sees
mise run checkout              # PlaceOrder {orderId: ord-100, paymentMethodId: pm-9}
mise run result                # final JSON: OrderPlaced, OrderConfirmed, PaymentAuthorized, ShipmentAllocated
mise run query:order-status    # {"status":"confirmed", ...}
```

Each scenario task uses a fresh workflow ID and prints the final JSON:

| Task | Worker started with | Outcome |
|---|---|---|
| `mise run scenario:happy` | defaults | placed → authorized → confirmed → allocated |
| `mise run scenario:reject` | defaults | `OrderPlacementRejected{empty_cart}`; billing/fulfillment never start |
| `mise run scenario:decline` | `PAYMENT_OUTCOME=declined mise run demo` | `PaymentDeclined` → `OrderCancelled`, status `cancelled`, no fulfillment |
| `mise run scenario:deferred` | `INVENTORY_OUTCOME=deferred mise run demo` | `ShipmentAllocationDeferred{missingSkus:[sku-1]}`; `sku-9` replenishment ignored; `sku-1` replenishment re-allocates (attempts 2) |

Stop the stack before switching worker outcomes. Other tasks: `mise run replenish` (`SKU=...`), `query:payment-status`, `query:paid-orders`, `query:shipment-queue`.

## Development checks

```sh
mise run check   # gofmt, go mod tidy -diff, go test -race ./..., go vet ./...
```

Tests need no server (`go.temporal.io/sdk/testsuite`); activities are anonymous functions registered with `RegisterActivityWithOptions` that record every request.

- `decisions_test.go` — all 21 gwts and all 5 `data` blocks, asserting complete payloads, plus invalid-input and incomplete-result cases.
- `workflows_test.go` — gate ignores an invalid command; duplicate authorized/declined outcomes are ignored from history; malformed outcome ignored; empty-cart rejection; billing approve/decline with exact request and idempotency key; gateway retry succeeding on attempt 4 and failing after 4 with the same key; incomplete gateway result not retried; fulfillment immediate allocation; deferral with unrelated then matching SKU and attempt-scoped keys; warehouse retry exhaustion with the same key; parent happy / decline / reject / deferred paths with child queries and signals by child workflow ID; demo adapter idempotency.
- `cmd/worker/main_test.go` — environment adapter selection.

## Source artifacts

- Intake snapshot: `model.json` (source sha256 `fb5f913d12b2c2e453ba86e8026db5a7b3edde561a8065063bc8ede02837ac44`).
- Runtime-neutral IR, provenance, and compile reports remain authoritative in `../bounded-context-order-fulfillment/{sales,billing,fulfillment}/`.
- Pipeline versions: sales `1.0.0`, billing `0.3.0`, fulfillment `1.0.0`.
- Go SDK: `go.temporal.io/sdk v1.48.0`.

## Temporal parity and determinism

Workflow code uses only Temporal workflow APIs (`Selector`, signal channels, child workflows, activities, query handlers). It performs no filesystem, network, wall-clock, random, native goroutine, or native-channel operations. The HITL gate has no timeout because the model defines no abandonment event. The fulfillment feedback edge is a durable Signal loop on the stateful child instead of a cyclic DAG edge. Each replenishment signal adds history; a long-lived deferral with heavy signal traffic would need Continue-As-New, which this model does not require.
