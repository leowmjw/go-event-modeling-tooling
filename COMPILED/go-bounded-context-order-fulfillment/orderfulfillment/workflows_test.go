package orderfulfillment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const parentID = "checkout-c-100"

type gatewayFunc func(GatewayAuthorizationRequest) (GatewayAuthorization, error)
type warehouseFunc func(WarehouseAllocationRequest) (WarehouseAllocation, error)

// recorder captures activity requests; testsuite may run activities on other goroutines.
type recorder struct {
	mu        sync.Mutex
	payments  []GatewayAuthorizationRequest
	warehouse []WarehouseAllocationRequest
}

func (r *recorder) paymentRequests() []GatewayAuthorizationRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]GatewayAuthorizationRequest(nil), r.payments...)
}

func (r *recorder) warehouseRequests() []WarehouseAllocationRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]WarehouseAllocationRequest(nil), r.warehouse...)
}

func newEnv(t *testing.T, gateway gatewayFunc, warehouse warehouseFunc) (*testsuite.TestWorkflowEnvironment, *recorder) {
	t.Helper()
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	rec := &recorder{}
	env.RegisterWorkflowWithOptions(OrderFulfillmentWorkflow, workflow.RegisterOptions{Name: OrderFulfillmentWorkflowName})
	env.RegisterWorkflowWithOptions(SalesWorkflow, workflow.RegisterOptions{Name: SalesWorkflowName})
	env.RegisterWorkflowWithOptions(BillingWorkflow, workflow.RegisterOptions{Name: BillingWorkflowName})
	env.RegisterWorkflowWithOptions(FulfillmentWorkflow, workflow.RegisterOptions{Name: FulfillmentWorkflowName})
	env.RegisterActivityWithOptions(func(_ context.Context, request GatewayAuthorizationRequest) (GatewayAuthorization, error) {
		rec.mu.Lock()
		rec.payments = append(rec.payments, request)
		rec.mu.Unlock()
		return gateway(request)
	}, activity.RegisterOptions{Name: AuthorizePaymentWithGatewayActivityName})
	env.RegisterActivityWithOptions(func(_ context.Context, request WarehouseAllocationRequest) (WarehouseAllocation, error) {
		rec.mu.Lock()
		rec.warehouse = append(rec.warehouse, request)
		rec.mu.Unlock()
		return warehouse(request)
	}, activity.RegisterOptions{Name: AllocateShipmentWithWarehouseActivityName})
	return env, rec
}

func approve(GatewayAuthorizationRequest) (GatewayAuthorization, error) {
	return GatewayAuthorization{Approved: true, AuthorizationID: "auth-1"}, nil
}

func decline(GatewayAuthorizationRequest) (GatewayAuthorization, error) {
	return GatewayAuthorization{Reason: "insufficient_funds"}, nil
}

func allocate(WarehouseAllocationRequest) (WarehouseAllocation, error) {
	return WarehouseAllocation{Allocated: true, ShipmentID: "sh-55", WarehouseID: "wh-1"}, nil
}

func unexpectedWarehouse(t *testing.T) warehouseFunc {
	return func(WarehouseAllocationRequest) (WarehouseAllocation, error) {
		t.Error("warehouse must not be called")
		return WarehouseAllocation{}, errors.New("unexpected")
	}
}

func unexpectedGateway(t *testing.T) gatewayFunc {
	return func(GatewayAuthorizationRequest) (GatewayAuthorization, error) {
		t.Error("payment gateway must not be called")
		return GatewayAuthorization{}, errors.New("unexpected")
	}
}

// deferThenAllocate reports sku-1 missing on the first attempt key only.
func deferThenAllocate(request WarehouseAllocationRequest) (WarehouseAllocation, error) {
	if request.IdempotencyKey == "ord-100/allocation-1" {
		return WarehouseAllocation{MissingSKUs: []string{"sku-1"}}, nil
	}
	return allocate(request)
}

func at(env *testsuite.TestWorkflowEnvironment, d time.Duration, fn func()) {
	env.RegisterDelayedCallback(fn, d)
}

func query[T any](t *testing.T, env *testsuite.TestWorkflowEnvironment, workflowID, name string) T {
	t.Helper()
	var (
		encoded converter.EncodedValue
		err     error
		value   T
	)
	if workflowID == "" {
		encoded, err = env.QueryWorkflow(name)
	} else {
		encoded, err = env.QueryWorkflowByID(workflowID, name)
	}
	if err == nil {
		err = encoded.Get(&value)
	}
	if err != nil {
		t.Fatalf("query %q %s: %v", workflowID, name, err)
	}
	return value
}

func completed[T any](t *testing.T, env *testsuite.TestWorkflowEnvironment) T {
	t.Helper()
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	var result T
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func placed(event string) RecordedEvent { return RecordedEvent{Event: event, OrderID: "ord-100"} }

// Sales child

func TestSalesGateWaitsForValidCommandThenConfirmsOnce(t *testing.T) {
	env, _ := newEnv(t, unexpectedGateway(t), unexpectedWarehouse(t))
	at(env, time.Minute, func() {
		assertEqual(t, query[CartSummaryView](t, env, "", CartSummaryQuery), CartSummaryView{CartID: "c-100", CustomerID: "cust-7", Items: lineItems(), Total: 125})
		if status := query[*OrderStatusView](t, env, "", OrderStatusQuery); status != nil {
			t.Errorf("no OrderStatus before placement, got %#v", status)
		}
		invalid := placeOrderCommand()
		invalid.CartID = "c-999"
		env.SignalWorkflow(PlaceOrderSignal, invalid)
	})
	// Both outcomes are buffered while the gate waits, so the duplicate is
	// pending when the first one closes the order.
	at(env, 2*time.Minute, func() {
		authorized := BillingOutcome{Authorized: &SalesPaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1"}}
		env.SignalWorkflow(BillingOutcomeSignal, authorized)
		env.SignalWorkflow(BillingOutcomeSignal, authorized)
	})
	at(env, 3*time.Minute, func() {
		if status := query[*OrderStatusView](t, env, "", OrderStatusQuery); status != nil {
			t.Errorf("invalid command must not place the order, got %#v", status)
		}
		env.SignalWorkflow(PlaceOrderSignal, placeOrderCommand())
	})
	env.ExecuteWorkflow(SalesWorkflow, SalesWorkflowInput{CartPriced: cartPriced()})

	got := completed[SalesResult](t, env)
	assertEqual(t, got, SalesResult{
		CartSummary: CartSummaryView{CartID: "c-100", CustomerID: "cust-7", Items: lineItems(), Total: 125},
		Placement:   placedResult(),
		Confirmations: []OrderConfirmationResult{
			{Event: "OrderConfirmed", OrderID: "ord-100", AuthorizationID: "auth-1"},
			{Event: "OrderConfirmationIgnored", OrderID: "ord-100", Reason: "already_confirmed"},
		},
		OrderStatus: &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: "confirmed"},
		History:     []RecordedEvent{placed("OrderPlaced"), placed("OrderConfirmed")},
	})
}

func TestSalesIgnoresMalformedOutcomeThenCancelsOnDecline(t *testing.T) {
	env, _ := newEnv(t, unexpectedGateway(t), unexpectedWarehouse(t))
	at(env, time.Minute, func() { env.SignalWorkflow(PlaceOrderSignal, placeOrderCommand()) })
	at(env, 2*time.Minute, func() { env.SignalWorkflow(BillingOutcomeSignal, BillingOutcome{}) })
	at(env, 3*time.Minute, func() {
		assertEqual(t, query[*OrderStatusView](t, env, "", OrderStatusQuery), &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: "pending_payment"})
		env.SignalWorkflow(BillingOutcomeSignal, BillingOutcome{Declined: &SalesPaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}})
	})
	env.ExecuteWorkflow(SalesWorkflow, SalesWorkflowInput{CartPriced: cartPriced()})

	got := completed[SalesResult](t, env)
	assertEqual(t, got.Cancellations, []OrderCancellationResult{{Event: "OrderCancelled", OrderID: "ord-100", Reason: "payment_declined"}})
	assertEqual(t, got.OrderStatus, &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: "cancelled"})
	assertEqual(t, got.History, []RecordedEvent{placed("OrderPlaced"), placed("OrderCancelled")})
	if got.Confirmations != nil {
		t.Fatalf("unexpected confirmations %#v", got.Confirmations)
	}
}

func TestSalesIgnoresDuplicateDecline(t *testing.T) {
	env, _ := newEnv(t, unexpectedGateway(t), unexpectedWarehouse(t))
	at(env, time.Minute, func() {
		declined := BillingOutcome{Declined: &SalesPaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}}
		env.SignalWorkflow(BillingOutcomeSignal, declined)
		env.SignalWorkflow(BillingOutcomeSignal, declined)
	})
	at(env, 2*time.Minute, func() { env.SignalWorkflow(PlaceOrderSignal, placeOrderCommand()) })
	env.ExecuteWorkflow(SalesWorkflow, SalesWorkflowInput{CartPriced: cartPriced()})

	got := completed[SalesResult](t, env)
	assertEqual(t, got.Cancellations, []OrderCancellationResult{
		{Event: "OrderCancelled", OrderID: "ord-100", Reason: "payment_declined"},
		{Event: "OrderCancellationIgnored", OrderID: "ord-100", Reason: "already_cancelled"},
	})
	assertEqual(t, got.History, []RecordedEvent{placed("OrderPlaced"), placed("OrderCancelled")})
}

func TestSalesRejectsEmptyCart(t *testing.T) {
	env, _ := newEnv(t, unexpectedGateway(t), unexpectedWarehouse(t))
	at(env, time.Minute, func() { env.SignalWorkflow(PlaceOrderSignal, placeOrderCommand()) })
	env.ExecuteWorkflow(SalesWorkflow, SalesWorkflowInput{CartPriced: CartPriced{CartID: "c-100", CustomerID: "cust-7", Items: []LineItem{}, Total: 0}})

	got := completed[SalesResult](t, env)
	assertEqual(t, got, SalesResult{
		CartSummary: CartSummaryView{CartID: "c-100", CustomerID: "cust-7", Items: []LineItem{}, Total: 0},
		Placement:   PlaceOrderResult{Event: "OrderPlacementRejected", Rejected: &OrderPlacementRejected{OrderID: "ord-100", CartID: "c-100", Reason: "empty_cart"}},
		History:     []RecordedEvent{placed("OrderPlacementRejected")},
	})
}

// Billing child

func TestBillingWorkflowOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		gateway gatewayFunc
		want    BillingResult
	}{
		{
			name:    "gateway approves",
			gateway: approve,
			want: BillingResult{
				Payment:       PaymentAuthorizationResult{Event: "PaymentAuthorized", Authorized: &PaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1", Amount: 125}},
				PaymentStatus: PaymentStatusView{OrderID: "ord-100", Status: "authorized"},
			},
		},
		{
			name:    "gateway declines",
			gateway: decline,
			want: BillingResult{
				Payment:       PaymentAuthorizationResult{Event: "PaymentDeclined", Declined: &PaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}},
				PaymentStatus: PaymentStatusView{OrderID: "ord-100", Status: "declined", Reason: "insufficient_funds"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, rec := newEnv(t, tc.gateway, unexpectedWarehouse(t))
			env.ExecuteWorkflow(BillingWorkflow, BillingWorkflowInput{OrderPlaced: SalesOrderPlaced{OrderID: "ord-100", PaymentMethodID: "pm-9", Total: 125}})
			assertEqual(t, completed[BillingResult](t, env), tc.want)
			assertEqual(t, query[*PaymentStatusView](t, env, "", PaymentStatusQuery), &tc.want.PaymentStatus)
			assertEqual(t, rec.paymentRequests(), []GatewayAuthorizationRequest{{Command: authorizePaymentCommand(), IdempotencyKey: "ord-100"}})
		})
	}
}

func TestBillingRetriesGatewayWithSameKey(t *testing.T) {
	tests := []struct {
		name     string
		failures int
		wantErr  bool
	}{
		{name: "succeeds on the last allowed attempt", failures: MaximumActivityAttempts - 1},
		{name: "fails after retry.max + 1 attempts", failures: MaximumActivityAttempts, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			env, rec := newEnv(t, func(r GatewayAuthorizationRequest) (GatewayAuthorization, error) {
				mu.Lock()
				defer mu.Unlock()
				calls++
				if calls <= tc.failures {
					return GatewayAuthorization{}, errors.New("gateway unavailable")
				}
				return approve(r)
			}, unexpectedWarehouse(t))
			env.ExecuteWorkflow(BillingWorkflow, BillingWorkflowInput{OrderPlaced: SalesOrderPlaced{OrderID: "ord-100", PaymentMethodID: "pm-9", Total: 125}})
			if !env.IsWorkflowCompleted() {
				t.Fatal("workflow did not complete")
			}
			if gotErr := env.GetWorkflowError() != nil; gotErr != tc.wantErr {
				t.Fatalf("workflow error = %v, want error %v", env.GetWorkflowError(), tc.wantErr)
			}
			requests := rec.paymentRequests()
			if len(requests) != min(tc.failures+1, MaximumActivityAttempts) {
				t.Fatalf("attempts = %d", len(requests))
			}
			for _, r := range requests {
				if r.IdempotencyKey != "ord-100" {
					t.Fatalf("retry changed idempotency key: %q", r.IdempotencyKey)
				}
			}
		})
	}
}

func TestBillingRejectsIncompleteGatewayResultWithoutRetry(t *testing.T) {
	env, rec := newEnv(t, func(GatewayAuthorizationRequest) (GatewayAuthorization, error) {
		return GatewayAuthorization{}, nil
	}, unexpectedWarehouse(t))
	env.ExecuteWorkflow(BillingWorkflow, BillingWorkflowInput{OrderPlaced: SalesOrderPlaced{OrderID: "ord-100", PaymentMethodID: "pm-9", Total: 125}})
	if env.GetWorkflowError() == nil {
		t.Fatal("expected incomplete gateway result to fail billing")
	}
	if n := len(rec.paymentRequests()); n != 1 {
		t.Fatalf("decision failure must not re-call the gateway; attempts = %d", n)
	}
}

// Fulfillment child

func fulfillmentInput() FulfillmentWorkflowInput {
	return FulfillmentWorkflowInput{
		OrderPlaced:       FulfillmentOrderPlaced{OrderID: "ord-100", Items: shipmentItems()},
		PaymentAuthorized: FulfillmentPaymentAuthorized{OrderID: "ord-100", Amount: 125},
	}
}

func TestFulfillmentAllocatesImmediately(t *testing.T) {
	env, rec := newEnv(t, unexpectedGateway(t), allocate)
	env.ExecuteWorkflow(FulfillmentWorkflow, fulfillmentInput())

	queued := ShipmentQueueView{ShipmentID: "sh-55", OrderID: "ord-100", WarehouseID: "wh-1", Status: "queued_for_picking"}
	assertEqual(t, completed[FulfillmentResult](t, env), FulfillmentResult{
		PaidOrdersToShip: readyToShip(),
		Allocations:      []ShipmentAllocationResult{{Event: "ShipmentAllocated", Allocated: &ShipmentAllocated{OrderID: "ord-100", ShipmentID: "sh-55", WarehouseID: "wh-1"}}},
		ShipmentQueue:    queued,
		Attempts:         1,
	})
	assertEqual(t, query[PaidOrdersToShipView](t, env, "", PaidOrdersToShipQuery), readyToShip())
	assertEqual(t, query[*ShipmentQueueView](t, env, "", ShipmentQueueQuery), &queued)
	assertEqual(t, rec.warehouseRequests(), []WarehouseAllocationRequest{{Command: allocateShipmentCommand(), IdempotencyKey: "ord-100/allocation-1"}})
}

func TestFulfillmentWaitsForMissingSKUReplenishment(t *testing.T) {
	env, rec := newEnv(t, unexpectedGateway(t), deferThenAllocate)
	waiting := ShipmentQueueView{OrderID: "ord-100", Status: "awaiting_inventory", MissingSKUs: []string{"sku-1"}}
	at(env, time.Minute, func() {
		assertEqual(t, query[*ShipmentQueueView](t, env, "", ShipmentQueueQuery), &waiting)
		env.SignalWorkflow(InventoryReplenishedSignal, InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-9"})
	})
	at(env, 2*time.Minute, func() {
		assertEqual(t, query[*ShipmentQueueView](t, env, "", ShipmentQueueQuery), &waiting)
		if n := len(rec.warehouseRequests()); n != 1 {
			t.Errorf("unrelated SKU must not re-issue AllocateShipment; attempts = %d", n)
		}
		env.SignalWorkflow(InventoryReplenishedSignal, InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-1"})
	})
	env.ExecuteWorkflow(FulfillmentWorkflow, fulfillmentInput())

	assertEqual(t, completed[FulfillmentResult](t, env), FulfillmentResult{
		PaidOrdersToShip: readyToShip(),
		Allocations: []ShipmentAllocationResult{
			deferredResult(),
			{Event: "ShipmentAllocated", Allocated: &ShipmentAllocated{OrderID: "ord-100", ShipmentID: "sh-55", WarehouseID: "wh-1"}},
		},
		ShipmentQueue: ShipmentQueueView{ShipmentID: "sh-55", OrderID: "ord-100", WarehouseID: "wh-1", Status: "queued_for_picking"},
		Attempts:      2,
	})
	assertEqual(t, rec.warehouseRequests(), []WarehouseAllocationRequest{
		{Command: allocateShipmentCommand(), IdempotencyKey: "ord-100/allocation-1"},
		{Command: allocateShipmentCommand(), IdempotencyKey: "ord-100/allocation-2"},
	})
}

func TestFulfillmentRetriesWarehouseWithSameKey(t *testing.T) {
	env, rec := newEnv(t, unexpectedGateway(t), func(WarehouseAllocationRequest) (WarehouseAllocation, error) {
		return WarehouseAllocation{}, errors.New("warehouse unavailable")
	})
	env.ExecuteWorkflow(FulfillmentWorkflow, fulfillmentInput())
	if env.GetWorkflowError() == nil {
		t.Fatal("expected fulfillment to fail after exhausting retries")
	}
	requests := rec.warehouseRequests()
	if len(requests) != MaximumActivityAttempts {
		t.Fatalf("attempts = %d, want %d", len(requests), MaximumActivityAttempts)
	}
	for _, r := range requests {
		if r.IdempotencyKey != "ord-100/allocation-1" {
			t.Fatalf("activity retry must reuse the attempt key, got %q", r.IdempotencyKey)
		}
	}
}

// Parent

func runParent(t *testing.T, env *testsuite.TestWorkflowEnvironment, cart CartPriced) OrderFulfillmentResult {
	t.Helper()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: parentID})
	at(env, time.Minute, func() {
		if err := env.SignalWorkflowByID(parentID+SalesChildSuffix, PlaceOrderSignal, placeOrderCommand()); err != nil {
			t.Errorf("checkout signal: %v", err)
		}
	})
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, OrderFulfillmentInput{CartPriced: cart})
	return completed[OrderFulfillmentResult](t, env)
}

func TestParentHappyPath(t *testing.T) {
	env, rec := newEnv(t, approve, allocate)
	got := runParent(t, env, cartPriced())

	assertEqual(t, got, OrderFulfillmentResult{
		Sales: SalesResult{
			CartSummary:   CartSummaryView{CartID: "c-100", CustomerID: "cust-7", Items: lineItems(), Total: 125},
			Placement:     placedResult(),
			Confirmations: []OrderConfirmationResult{{Event: "OrderConfirmed", OrderID: "ord-100", AuthorizationID: "auth-1"}},
			OrderStatus:   &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: "confirmed"},
			History:       []RecordedEvent{placed("OrderPlaced"), placed("OrderConfirmed")},
		},
		Billing: &BillingResult{
			Payment:       PaymentAuthorizationResult{Event: "PaymentAuthorized", Authorized: &PaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1", Amount: 125}},
			PaymentStatus: PaymentStatusView{OrderID: "ord-100", Status: "authorized"},
		},
		Fulfillment: &FulfillmentResult{
			PaidOrdersToShip: readyToShip(),
			Allocations:      []ShipmentAllocationResult{{Event: "ShipmentAllocated", Allocated: &ShipmentAllocated{OrderID: "ord-100", ShipmentID: "sh-55", WarehouseID: "wh-1"}}},
			ShipmentQueue:    ShipmentQueueView{ShipmentID: "sh-55", OrderID: "ord-100", WarehouseID: "wh-1", Status: "queued_for_picking"},
			Attempts:         1,
		},
	})
	assertEqual(t, rec.paymentRequests(), []GatewayAuthorizationRequest{{Command: authorizePaymentCommand(), IdempotencyKey: "ord-100"}})
}

func TestParentDeclineCancelsInSalesAndSkipsFulfillment(t *testing.T) {
	env, _ := newEnv(t, decline, unexpectedWarehouse(t))
	got := runParent(t, env, cartPriced())

	if got.Fulfillment != nil {
		t.Fatalf("declined payment must not start fulfillment: %#v", got.Fulfillment)
	}
	assertEqual(t, got.Billing, &BillingResult{
		Payment:       PaymentAuthorizationResult{Event: "PaymentDeclined", Declined: &PaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}},
		PaymentStatus: PaymentStatusView{OrderID: "ord-100", Status: "declined", Reason: "insufficient_funds"},
	})
	assertEqual(t, got.Sales.Cancellations, []OrderCancellationResult{{Event: "OrderCancelled", OrderID: "ord-100", Reason: "payment_declined"}})
	assertEqual(t, got.Sales.OrderStatus, &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: "cancelled"})
	assertEqual(t, got.Sales.History, []RecordedEvent{placed("OrderPlaced"), placed("OrderCancelled")})
}

func TestParentRejectedPlacementStopsInSales(t *testing.T) {
	env, _ := newEnv(t, unexpectedGateway(t), unexpectedWarehouse(t))
	got := runParent(t, env, CartPriced{CartID: "c-100", CustomerID: "cust-7", Items: []LineItem{}, Total: 0})

	assertEqual(t, got, OrderFulfillmentResult{Sales: SalesResult{
		CartSummary: CartSummaryView{CartID: "c-100", CustomerID: "cust-7", Items: []LineItem{}, Total: 0},
		Placement:   PlaceOrderResult{Event: "OrderPlacementRejected", Rejected: &OrderPlacementRejected{OrderID: "ord-100", CartID: "c-100", Reason: "empty_cart"}},
		History:     []RecordedEvent{placed("OrderPlacementRejected")},
	}})
}

func TestParentDeferredAllocationResumesOnReplenishment(t *testing.T) {
	env, rec := newEnv(t, approve, deferThenAllocate)
	at(env, 10*time.Minute, func() {
		fulfillmentID := parentID + FulfillmentChildSuffix
		assertEqual(t, query[*ShipmentQueueView](t, env, fulfillmentID, ShipmentQueueQuery), &ShipmentQueueView{OrderID: "ord-100", Status: "awaiting_inventory", MissingSKUs: []string{"sku-1"}})
		assertEqual(t, query[*OrderStatusView](t, env, parentID+SalesChildSuffix, OrderStatusQuery), &OrderStatusView{OrderID: "ord-100", CustomerID: "cust-7", Status: "confirmed"})
		if err := env.SignalWorkflowByID(fulfillmentID, InventoryReplenishedSignal, InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-1"}); err != nil {
			t.Errorf("replenish signal: %v", err)
		}
	})
	got := runParent(t, env, cartPriced())

	if got.Fulfillment == nil || got.Fulfillment.Attempts != 2 {
		t.Fatalf("expected two allocation attempts, got %#v", got.Fulfillment)
	}
	assertEqual(t, got.Fulfillment.ShipmentQueue, ShipmentQueueView{ShipmentID: "sh-55", OrderID: "ord-100", WarehouseID: "wh-1", Status: "queued_for_picking"})
	assertEqual(t, len(rec.warehouseRequests()), 2)
}

// Demo adapters

func TestDemoAdaptersAreIdempotentPerKey(t *testing.T) {
	gateway := &DemoPaymentGateway{Result: GatewayAuthorization{Approved: true, AuthorizationID: "auth-1"}}
	first, err := gateway.Authorize(t.Context(), authorizePaymentCommand(), "ord-100")
	if err != nil {
		t.Fatal(err)
	}
	gateway.Result = GatewayAuthorization{Reason: "insufficient_funds"}
	replayed, err := gateway.Authorize(t.Context(), authorizePaymentCommand(), "ord-100")
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, replayed, first)

	warehouse := &DemoWarehouse{UnavailableAttempts: 1, MissingSKUs: []string{"sku-1"}, ShipmentID: "sh-55", WarehouseID: "wh-1"}
	var got []WarehouseAllocation
	for _, key := range []string{"ord-100/allocation-1", "ord-100/allocation-1", "ord-100/allocation-2"} {
		result, err := warehouse.Allocate(t.Context(), allocateShipmentCommand(), key)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, result)
	}
	assertEqual(t, got, []WarehouseAllocation{
		{MissingSKUs: []string{"sku-1"}},
		{MissingSKUs: []string{"sku-1"}},
		{Allocated: true, ShipmentID: "sh-55", WarehouseID: "wh-1"},
	})
}

func TestActivitiesRejectIncompleteResults(t *testing.T) {
	a := &Activities{
		PaymentGateway: &DemoPaymentGateway{Result: GatewayAuthorization{Approved: true}},
		Warehouse:      &DemoWarehouse{ShipmentID: "sh-55"},
	}
	if _, err := a.AuthorizePaymentWithGateway(t.Context(), GatewayAuthorizationRequest{Command: authorizePaymentCommand(), IdempotencyKey: "ord-100"}); err == nil {
		t.Fatal("approved result without authorizationId must fail")
	}
	if _, err := a.AllocateShipmentWithWarehouse(t.Context(), WarehouseAllocationRequest{Command: allocateShipmentCommand(), IdempotencyKey: "ord-100/allocation-1"}); err == nil {
		t.Fatal("allocated result without warehouseId must fail")
	}
	if _, err := a.AuthorizePaymentWithGateway(t.Context(), GatewayAuthorizationRequest{Command: authorizePaymentCommand()}); err == nil {
		t.Fatal("missing idempotency key must fail")
	}
}
