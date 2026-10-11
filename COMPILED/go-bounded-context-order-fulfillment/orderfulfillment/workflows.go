package orderfulfillment

import (
	"errors"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	PaymentGatewayTimeout     = 10 * time.Second
	WarehouseInventoryTimeout = 30 * time.Second
	// MaximumActivityAttempts is IR retry.max (3 retries) plus the first attempt.
	MaximumActivityAttempts = 4
)

func externalCallOptions(ctx workflow.Context, timeout time.Duration) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: timeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    MaximumActivityAttempts,
		},
	})
}

// OrderFulfillmentWorkflow coordinates the cross-context links: rf 06/28 from
// Sales OrderPlaced, rf 11/30 and rf 16 from the Billing outcome.
func OrderFulfillmentWorkflow(ctx workflow.Context, input OrderFulfillmentInput) (OrderFulfillmentResult, error) {
	id := workflow.GetInfo(ctx).WorkflowExecution.ID
	sales := workflow.ExecuteChildWorkflow(childOptions(ctx, id+SalesChildSuffix), SalesWorkflowName, SalesWorkflowInput{CartPriced: input.CartPriced})
	if err := sales.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
		return OrderFulfillmentResult{}, fmt.Errorf("start sales: %w", err)
	}

	var (
		result     OrderFulfillmentResult
		placement  PlaceOrderResult
		salesDone  bool
		salesErr   error
		placements = workflow.GetSignalChannel(ctx, OrderPlacementSignal)
	)
	selector := workflow.NewSelector(ctx)
	selector.AddReceive(placements, func(c workflow.ReceiveChannel, _ bool) { c.Receive(ctx, &placement) })
	selector.AddFuture(sales, func(f workflow.Future) {
		salesDone = true
		salesErr = f.Get(ctx, &result.Sales)
	})
	selector.Select(ctx)
	if salesDone {
		if salesErr != nil {
			return result, fmt.Errorf("sales: %w", salesErr)
		}
		placement = result.Sales.Placement
	}

	if placement.Placed == nil {
		if !salesDone {
			if err := sales.Get(ctx, &result.Sales); err != nil {
				return result, fmt.Errorf("sales: %w", err)
			}
		}
		return result, nil
	}
	placed := *placement.Placed

	var billing BillingResult
	err := workflow.ExecuteChildWorkflow(childOptions(ctx, id+BillingChildSuffix), BillingWorkflowName, BillingWorkflowInput{
		OrderPlaced: SalesOrderPlaced{OrderID: placed.OrderID, PaymentMethodID: placed.PaymentMethodID, Total: placed.Total},
	}).Get(ctx, &billing)
	if err != nil {
		return result, fmt.Errorf("billing: %w", err)
	}
	result.Billing = &billing

	var outcome BillingOutcome
	if a := billing.Payment.Authorized; a != nil {
		outcome.Authorized = &SalesPaymentAuthorized{OrderID: a.OrderID, AuthorizationID: a.AuthorizationID}
	} else {
		outcome.Declined = &SalesPaymentDeclined{OrderID: billing.Payment.Declined.OrderID, Reason: billing.Payment.Declined.Reason}
	}
	if err := sales.SignalChildWorkflow(ctx, BillingOutcomeSignal, outcome).Get(ctx, nil); err != nil {
		return result, fmt.Errorf("signal sales billing outcome: %w", err)
	}

	var fulfillment workflow.ChildWorkflowFuture
	if a := billing.Payment.Authorized; a != nil {
		items := make([]ShipmentItem, len(placed.Items))
		for i, item := range placed.Items {
			items[i] = ShipmentItem{SKU: item.SKU, Qty: item.Qty}
		}
		fulfillment = workflow.ExecuteChildWorkflow(childOptions(ctx, id+FulfillmentChildSuffix), FulfillmentWorkflowName, FulfillmentWorkflowInput{
			OrderPlaced:       FulfillmentOrderPlaced{OrderID: placed.OrderID, Items: items},
			PaymentAuthorized: FulfillmentPaymentAuthorized{OrderID: a.OrderID, Amount: a.Amount},
		})
	}

	if err := sales.Get(ctx, &result.Sales); err != nil {
		return result, fmt.Errorf("sales: %w", err)
	}
	if fulfillment != nil {
		var shipped FulfillmentResult
		if err := fulfillment.Get(ctx, &shipped); err != nil {
			return result, fmt.Errorf("fulfillment: %w", err)
		}
		result.Fulfillment = &shipped
	}
	return result, nil
}

func childOptions(ctx workflow.Context, workflowID string) workflow.Context {
	return workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{WorkflowID: workflowID})
}

// SalesWorkflow runs the Sales context: CartSummary, the CheckoutScreen gate,
// the placement decision, and closing the order from the Billing outcome.
func SalesWorkflow(ctx workflow.Context, input SalesWorkflowInput) (SalesResult, error) {
	logger := workflow.GetLogger(ctx)
	cart, err := ProjectCartSummary(input.CartPriced)
	if err != nil {
		return SalesResult{}, temporal.NewNonRetryableApplicationError(err.Error(), "InvalidCartPriced", err)
	}
	result := SalesResult{CartSummary: cart, History: []RecordedEvent{}}
	if err := workflow.SetQueryHandler(ctx, CartSummaryQuery, func() (CartSummaryView, error) { return result.CartSummary, nil }); err != nil {
		return result, err
	}
	if err := workflow.SetQueryHandler(ctx, OrderStatusQuery, func() (*OrderStatusView, error) { return result.OrderStatus, nil }); err != nil {
		return result, err
	}

	commands := workflow.GetSignalChannel(ctx, PlaceOrderSignal)
	for result.Placement.Event == "" {
		var command PlaceOrderCommand
		commands.Receive(ctx, &command)
		placement, err := DecidePlaceOrder(command, cart)
		if err != nil {
			logger.Warn("PlaceOrder rejected as invalid input; gate keeps waiting", "error", err)
			continue
		}
		result.Placement = placement
	}
	if p := result.Placement.Placed; p != nil {
		result.History = append(result.History, RecordedEvent{Event: result.Placement.Event, OrderID: p.OrderID})
	} else {
		result.History = append(result.History, RecordedEvent{Event: result.Placement.Event, OrderID: result.Placement.Rejected.OrderID})
	}
	result.OrderStatus = ProjectOrderStatus(result.Placement, nil, nil)

	if parent := workflow.GetInfo(ctx).ParentWorkflowExecution; parent != nil {
		if err := workflow.SignalExternalWorkflow(ctx, parent.ID, parent.RunID, OrderPlacementSignal, result.Placement).Get(ctx, nil); err != nil {
			return result, fmt.Errorf("report placement: %w", err)
		}
	}
	if result.Placement.Placed == nil {
		return result, nil
	}

	var confirmed *OrderConfirmationResult
	var cancelled *OrderCancellationResult
	apply := func(outcome BillingOutcome) error {
		switch {
		case outcome.Authorized != nil && outcome.Declined == nil:
			command, err := TranslateSalesConfirmation(*outcome.Authorized)
			if err != nil {
				return err
			}
			decision, err := DecideConfirmOrder(command, result.Placement, result.History)
			if err != nil {
				return err
			}
			result.Confirmations = append(result.Confirmations, decision)
			if decision.Event == "OrderConfirmed" {
				confirmed = &decision
				result.History = append(result.History, RecordedEvent{Event: decision.Event, OrderID: decision.OrderID})
			}
		case outcome.Declined != nil && outcome.Authorized == nil:
			command, err := TranslateSalesRecovery(*outcome.Declined)
			if err != nil {
				return err
			}
			decision, err := DecideCancelOrder(command, result.Placement, result.History)
			if err != nil {
				return err
			}
			result.Cancellations = append(result.Cancellations, decision)
			if decision.Event == "OrderCancelled" {
				cancelled = &decision
				result.History = append(result.History, RecordedEvent{Event: decision.Event, OrderID: decision.OrderID})
			}
		default:
			return errors.New("billing outcome must carry exactly one of authorized or declined")
		}
		result.OrderStatus = ProjectOrderStatus(result.Placement, confirmed, cancelled)
		return nil
	}

	outcomes := workflow.GetSignalChannel(ctx, BillingOutcomeSignal)
	for confirmed == nil && cancelled == nil {
		var outcome BillingOutcome
		outcomes.Receive(ctx, &outcome)
		if err := apply(outcome); err != nil {
			logger.Warn("billing outcome ignored", "error", err)
		}
	}
	for {
		var outcome BillingOutcome
		if !outcomes.ReceiveAsync(&outcome) {
			break
		}
		if err := apply(outcome); err != nil {
			logger.Warn("billing outcome ignored", "error", err)
		}
	}
	return result, nil
}

// BillingWorkflow runs the Billing context.
func BillingWorkflow(ctx workflow.Context, input BillingWorkflowInput) (BillingResult, error) {
	command, err := TranslateBilling(input.OrderPlaced)
	if err != nil {
		return BillingResult{}, temporal.NewNonRetryableApplicationError(err.Error(), "InvalidOrderPlaced", err)
	}
	var status *PaymentStatusView
	if err := workflow.SetQueryHandler(ctx, PaymentStatusQuery, func() (*PaymentStatusView, error) { return status, nil }); err != nil {
		return BillingResult{}, err
	}

	var gateway GatewayAuthorization
	err = workflow.ExecuteActivity(externalCallOptions(ctx, PaymentGatewayTimeout), AuthorizePaymentWithGatewayActivityName,
		GatewayAuthorizationRequest{Command: command, IdempotencyKey: command.OrderID}).Get(ctx, &gateway)
	if err != nil {
		return BillingResult{}, fmt.Errorf("authorize payment: %w", err)
	}
	payment, err := DecideAuthorizePayment(command, gateway)
	if err != nil {
		return BillingResult{}, err
	}
	projected := ProjectPaymentStatus(payment)
	status = &projected
	return BillingResult{Payment: payment, PaymentStatus: projected}, nil
}

// FulfillmentWorkflow runs the Fulfillment context, including the
// AllocationRetrier feedback loop driven by InventoryReplenished signals.
func FulfillmentWorkflow(ctx workflow.Context, input FulfillmentWorkflowInput) (FulfillmentResult, error) {
	logger := workflow.GetLogger(ctx)
	view, err := ProjectPaidOrdersToShip(input.OrderPlaced, &input.PaymentAuthorized)
	if err != nil {
		return FulfillmentResult{}, temporal.NewNonRetryableApplicationError(err.Error(), "InvalidOrderPlaced", err)
	}
	result := FulfillmentResult{PaidOrdersToShip: view, Allocations: []ShipmentAllocationResult{}}
	var queue *ShipmentQueueView
	if err := workflow.SetQueryHandler(ctx, PaidOrdersToShipQuery, func() (PaidOrdersToShipView, error) { return result.PaidOrdersToShip, nil }); err != nil {
		return result, err
	}
	if err := workflow.SetQueryHandler(ctx, ShipmentQueueQuery, func() (*ShipmentQueueView, error) { return queue, nil }); err != nil {
		return result, err
	}
	command, err := TranslateFulfillment(view)
	if err != nil {
		return result, temporal.NewNonRetryableApplicationError(err.Error(), "OrderNotReady", err)
	}

	activityCtx := externalCallOptions(ctx, WarehouseInventoryTimeout)
	replenishments := workflow.GetSignalChannel(ctx, InventoryReplenishedSignal)
	for {
		result.Attempts++
		var allocation WarehouseAllocation
		err := workflow.ExecuteActivity(activityCtx, AllocateShipmentWithWarehouseActivityName, WarehouseAllocationRequest{
			Command: command, IdempotencyKey: AllocationIdempotencyKey(command.OrderID, result.Attempts),
		}).Get(ctx, &allocation)
		if err != nil {
			return result, fmt.Errorf("allocate shipment: %w", err)
		}
		decision, err := DecideAllocateShipment(command, allocation)
		if err != nil {
			return result, err
		}
		result.Allocations = append(result.Allocations, decision)
		result.ShipmentQueue = ProjectShipmentQueue(decision)
		queue = &result.ShipmentQueue
		if decision.Allocated != nil {
			return result, nil
		}
		for {
			var replenished InventoryReplenished
			replenishments.Receive(ctx, &replenished)
			if next := RetryAllocation(replenished, decision, result.PaidOrdersToShip); next != nil {
				command = *next
				break
			}
			logger.Info("replenishment does not cover a missing SKU; still waiting", "sku", replenished.SKU)
		}
	}
}
