package orderfulfillment

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestSalesWorkflowScenarios(t *testing.T) {
	tests := []struct {
		name  string
		total float64
		event string
	}{
		{name: "place order from a priced cart", total: 125, event: "OrderPlaced"},
		{name: "reject order when cart has no priced items", total: 0, event: "OrderPlacementRejected"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
			env.ExecuteWorkflow(SalesWorkflow, SalesWorkflowInput{
				Cart: CartPriced{
					CartID: "c-100", CustomerID: "cust-7", Total: tc.total,
					Items: []LineItem{{SKU: "sku-1", Quantity: 1, UnitPrice: tc.total}},
				},
				Command: PlaceOrderCommand{CartID: "c-100", CustomerID: "cust-7", Total: tc.total},
			})
			assertWorkflowCompleted(t, env)
			var result SalesResult
			if err := env.GetWorkflowResult(&result); err != nil {
				t.Fatal(err)
			}
			if result.Event != tc.event {
				t.Fatalf("event = %q, want %q", result.Event, tc.event)
			}
			if result.CartSummary.CartID != "c-100" || result.OrderStatus.Status == "" {
				t.Fatalf("missing projections: %#v", result)
			}
		})
	}
}

func TestProjectCartSummaryMatchesDataBlock(t *testing.T) {
	input := validOrderInput().Cart
	view, err := ProjectCartSummary(input)
	if err != nil {
		t.Fatal(err)
	}
	if view.CartID != "c-100" || view.CustomerID != "cust-7" || len(view.Items) != 2 || view.Total != 125 {
		t.Fatalf("projection does not match CartSummary02: %#v", view)
	}
}

func TestDecidePlaceOrderRejectsInvalidIdentifiersAndTotals(t *testing.T) {
	tests := []PlaceOrderCommand{
		{CartID: "cart100", CustomerID: "cust-7", Total: 125},
		{CartID: "c-100", CustomerID: "cust-7", Total: math.NaN()},
		{CartID: "c-100", CustomerID: "cust-7", Total: math.Inf(1)},
	}
	for _, command := range tests {
		if _, err := DecidePlaceOrder(command); err == nil {
			t.Fatalf("expected validation error for %#v", command)
		}
	}
}

func TestBillingWorkflowScenarios(t *testing.T) {
	tests := []struct {
		name     string
		response PaymentGatewayResponse
		event    string
	}{
		{name: "authorize payment for a newly placed order", response: PaymentGatewayResponse{Authorized: true, AuthorizationID: "auth-1"}, event: "PaymentAuthorized"},
		{name: "decline payment when issuer rejects the charge", response: PaymentGatewayResponse{DeclineReason: "issuer_declined"}, event: "PaymentDeclined"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
			calls := 0
			env.RegisterActivityWithOptions(func(context.Context, PaymentGatewayRequest) (PaymentGatewayResponse, error) {
				calls++
				return tc.response, nil
			}, activity.RegisterOptions{Name: AuthorizePaymentActivityName})
			env.ExecuteWorkflow(BillingWorkflow, BillingWorkflowInput{
				Order: OrderPlaced{OrderID: "ord-100", CustomerID: "cust-7", Total: 125}, PaymentMethodID: "pm-9",
			})
			assertWorkflowCompleted(t, env)
			var result PaymentResult
			if err := env.GetWorkflowResult(&result); err != nil {
				t.Fatal(err)
			}
			if result.Event != tc.event {
				t.Fatalf("event = %q, want %q", result.Event, tc.event)
			}
			if result.Status.OrderID != "ord-100" || result.Status.Status == "" {
				t.Fatalf("missing payment projection: %#v", result)
			}
			if calls != 1 {
				t.Fatalf("activity calls = %d, want 1", calls)
			}
		})
	}
}

func TestFulfillmentWorkflowAllocatesImmediately(t *testing.T) {
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, InventoryRequest) (InventoryResponse, error) {
		calls++
		return InventoryResponse{Available: true, ShipmentID: "sh-55"}, nil
	}, activity.RegisterOptions{Name: CheckInventoryActivityName})
	env.ExecuteWorkflow(FulfillmentWorkflow, fulfillmentInput())
	assertWorkflowCompleted(t, env)
	var result ShipmentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Event != "ShipmentAllocated" || result.Queue.Status != "queued_for_picking" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if calls != 1 {
		t.Fatalf("activity calls = %d, want 1", calls)
	}
}

func TestFulfillmentWorkflowDefersThenRetriesOnReplenishment(t *testing.T) {
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, InventoryRequest) (InventoryResponse, error) {
		calls++
		if calls == 1 {
			return InventoryResponse{}, nil
		}
		return InventoryResponse{Available: true, ShipmentID: "sh-55"}, nil
	}, activity.RegisterOptions{Name: CheckInventoryActivityName})
	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(ShipmentQueueQuery)
		if err != nil {
			t.Fatal(err)
		}
		var queue ShipmentQueueView
		if err := encoded.Get(&queue); err != nil {
			t.Fatal(err)
		}
		if queue.Status != "deferred:inventory_unavailable" {
			t.Fatalf("queue status = %q, want deferred", queue.Status)
		}
		env.SignalWorkflow(InventoryReplenishedSignal, InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-1"})
	}, time.Second)
	env.ExecuteWorkflow(FulfillmentWorkflow, fulfillmentInput())
	assertWorkflowCompleted(t, env)
	var result ShipmentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Event != "ShipmentAllocated" || calls != 2 {
		t.Fatalf("unexpected result %#v after %d calls", result, calls)
	}
}

func TestSalesRecoveryWorkflowScenarios(t *testing.T) {
	tests := []struct {
		name        string
		priorEvents []OrderEvent
		event       string
	}{
		{name: "cancel order after payment declines", event: "OrderCancelled"},
		{name: "ignore cancellation when order is already cancelled", priorEvents: []OrderEvent{{Type: "OrderCancelled", OrderID: "ord-100"}}, event: "OrderCancellationIgnored"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
			env.ExecuteWorkflow(SalesRecoveryWorkflow, SalesRecoveryWorkflowInput{
				Declined: PaymentDeclined{OrderID: "ord-100", Reason: "insufficient_funds"}, PriorEvents: tc.priorEvents,
			})
			assertWorkflowCompleted(t, env)
			var result CancellationResult
			if err := env.GetWorkflowResult(&result); err != nil {
				t.Fatal(err)
			}
			if result.Event != tc.event {
				t.Fatalf("event = %q, want %q", result.Event, tc.event)
			}
			if result.Status.Status != "cancelled" {
				t.Fatalf("missing cancellation projection: %#v", result)
			}
		})
	}
}

func TestOrderFulfillmentRejectsBeforeExternalCalls(t *testing.T) {
	env := newParentEnvironment()
	input := validOrderInput()
	input.Cart.Total = 0
	input.Command.Total = 0
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, input)
	assertWorkflowCompleted(t, env)
	var result OrderFulfillmentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Sales.Event != "OrderPlacementRejected" || result.Payment != nil || result.Shipment != nil {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestOrderFulfillmentAuthorizedPath(t *testing.T) {
	env := newParentEnvironment()
	paymentCalls := 0
	inventoryCalls := 0
	env.RegisterActivityWithOptions(func(context.Context, PaymentGatewayRequest) (PaymentGatewayResponse, error) {
		paymentCalls++
		return PaymentGatewayResponse{Authorized: true, AuthorizationID: "auth-1"}, nil
	}, activity.RegisterOptions{Name: AuthorizePaymentActivityName})
	env.RegisterActivityWithOptions(func(context.Context, InventoryRequest) (InventoryResponse, error) {
		inventoryCalls++
		return InventoryResponse{Available: true, ShipmentID: "sh-55"}, nil
	}, activity.RegisterOptions{Name: CheckInventoryActivityName})
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, validOrderInput())
	assertWorkflowCompleted(t, env)
	var result OrderFulfillmentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Payment == nil || result.Payment.Event != "PaymentAuthorized" || result.Shipment == nil || result.Shipment.Event != "ShipmentAllocated" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if paymentCalls != 1 || inventoryCalls != 1 {
		t.Fatalf("activity calls = payment:%d inventory:%d, want 1 each", paymentCalls, inventoryCalls)
	}
}

func TestOrderFulfillmentDeferredThenReplenishedPath(t *testing.T) {
	env := newParentEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: "order-test"})
	env.RegisterActivityWithOptions(func(context.Context, PaymentGatewayRequest) (PaymentGatewayResponse, error) {
		return PaymentGatewayResponse{Authorized: true, AuthorizationID: "auth-1"}, nil
	}, activity.RegisterOptions{Name: AuthorizePaymentActivityName})
	inventoryCalls := 0
	env.RegisterActivityWithOptions(func(context.Context, InventoryRequest) (InventoryResponse, error) {
		inventoryCalls++
		if inventoryCalls == 1 {
			return InventoryResponse{}, nil
		}
		return InventoryResponse{Available: true, ShipmentID: "sh-55"}, nil
	}, activity.RegisterOptions{Name: CheckInventoryActivityName})
	env.RegisterDelayedCallback(func() {
		if err := env.SignalWorkflowByID("order-test/fulfillment", InventoryReplenishedSignal, InventoryReplenished{WarehouseID: "wh-1", SKU: "sku-1"}); err != nil {
			t.Fatal(err)
		}
	}, time.Second)
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, validOrderInput())
	assertWorkflowCompleted(t, env)
	var result OrderFulfillmentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Shipment == nil || result.Shipment.Event != "ShipmentAllocated" || inventoryCalls != 2 {
		t.Fatalf("unexpected result %#v after %d inventory calls", result, inventoryCalls)
	}
}

func TestOrderFulfillmentDeclinedPath(t *testing.T) {
	env := newParentEnvironment()
	paymentCalls := 0
	env.RegisterActivityWithOptions(func(context.Context, PaymentGatewayRequest) (PaymentGatewayResponse, error) {
		paymentCalls++
		return PaymentGatewayResponse{DeclineReason: "issuer_declined"}, nil
	}, activity.RegisterOptions{Name: AuthorizePaymentActivityName})
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, validOrderInput())
	assertWorkflowCompleted(t, env)
	var result OrderFulfillmentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Cancellation == nil || result.Cancellation.Event != "OrderCancelled" || result.Shipment != nil {
		t.Fatalf("unexpected result: %#v", result)
	}
	if paymentCalls != 1 {
		t.Fatalf("payment activity calls = %d, want 1", paymentCalls)
	}
}

func TestBillingActivityRetries(t *testing.T) {
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	attempts := 0
	env.RegisterActivityWithOptions(func(context.Context, PaymentGatewayRequest) (PaymentGatewayResponse, error) {
		attempts++
		if attempts == 1 {
			return PaymentGatewayResponse{}, errors.New("temporary gateway failure")
		}
		return PaymentGatewayResponse{Authorized: true, AuthorizationID: "auth-1"}, nil
	}, activity.RegisterOptions{Name: AuthorizePaymentActivityName})
	env.ExecuteWorkflow(BillingWorkflow, BillingWorkflowInput{
		Order: OrderPlaced{OrderID: "ord-100", Total: 125}, PaymentMethodID: "pm-9",
	})
	assertWorkflowCompleted(t, env)
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func newParentEnvironment() *testsuite.TestWorkflowEnvironment {
	env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(SalesWorkflow, workflow.RegisterOptions{Name: SalesWorkflowName})
	env.RegisterWorkflowWithOptions(BillingWorkflow, workflow.RegisterOptions{Name: BillingWorkflowName})
	env.RegisterWorkflowWithOptions(FulfillmentWorkflow, workflow.RegisterOptions{Name: FulfillmentWorkflowName})
	env.RegisterWorkflowWithOptions(SalesRecoveryWorkflow, workflow.RegisterOptions{Name: SalesRecoveryWorkflowName})
	return env
}

func fulfillmentInput() FulfillmentWorkflowInput {
	return FulfillmentWorkflowInput{
		Payment: PaymentAuthorized{OrderID: "ord-100", AuthorizationID: "auth-1", Amount: 125}, WarehouseID: "wh-1",
	}
}

func validOrderInput() OrderFulfillmentInput {
	return OrderFulfillmentInput{
		Cart: CartPriced{
			CartID: "c-100", CustomerID: "cust-7", Total: 125,
			Items: []LineItem{
				{SKU: "sku-1", Quantity: 1, UnitPrice: 75},
				{SKU: "sku-2", Quantity: 1, UnitPrice: 50},
			},
		},
		Command:         PlaceOrderCommand{CartID: "c-100", CustomerID: "cust-7", Total: 125},
		PaymentMethodID: "pm-9",
		WarehouseID:     "wh-1",
	}
}

func assertWorkflowCompleted(t *testing.T, env *testsuite.TestWorkflowEnvironment) {
	t.Helper()
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
}
