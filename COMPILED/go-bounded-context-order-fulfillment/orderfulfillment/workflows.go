package orderfulfillment

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func SalesWorkflow(_ workflow.Context, input SalesWorkflowInput) (SalesResult, error) {
	cartSummary, err := ProjectCartSummary(input.Cart)
	if err != nil {
		return SalesResult{}, err
	}
	result, err := DecidePlaceOrder(input.Command)
	if err != nil {
		return SalesResult{}, err
	}
	result.CartSummary = cartSummary
	result.OrderStatus = ProjectOrderStatus(result)
	return result, nil
}

func BillingWorkflow(ctx workflow.Context, input BillingWorkflowInput) (PaymentResult, error) {
	command, err := TranslateBilling(input.Order, input.PaymentMethodID)
	if err != nil {
		return PaymentResult{}, err
	}
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    4,
			BackoffCoefficient: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	var response PaymentGatewayResponse
	if err := workflow.ExecuteActivity(ctx, AuthorizePaymentActivityName, PaymentGatewayRequest{Command: command}).Get(ctx, &response); err != nil {
		return PaymentResult{}, err
	}
	return DecidePayment(command, response)
}

func FulfillmentWorkflow(ctx workflow.Context, input FulfillmentWorkflowInput) (ShipmentResult, error) {
	command, err := TranslateFulfillment(input.Payment, input.WarehouseID)
	if err != nil {
		return ShipmentResult{}, err
	}
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    4,
			BackoffCoefficient: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	var current ShipmentResult
	if err := workflow.SetQueryHandler(ctx, ShipmentQueueQuery, func() (ShipmentQueueView, error) {
		return current.Queue, nil
	}); err != nil {
		return ShipmentResult{}, err
	}
	replenished := workflow.GetSignalChannel(ctx, InventoryReplenishedSignal)
	for {
		var response InventoryResponse
		if err := workflow.ExecuteActivity(ctx, CheckInventoryActivityName, InventoryRequest{Command: command}).Get(ctx, &response); err != nil {
			return ShipmentResult{}, err
		}
		current, err = DecideShipment(command, response)
		if err != nil || current.Allocated != nil {
			return current, err
		}
		for {
			var event InventoryReplenished
			replenished.Receive(ctx, &event)
			if event.WarehouseID == command.WarehouseID {
				break
			}
		}
	}
}

func SalesRecoveryWorkflow(_ workflow.Context, input SalesRecoveryWorkflowInput) (CancellationResult, error) {
	command, err := TranslateSalesRecovery(input.Declined)
	if err != nil {
		return CancellationResult{}, err
	}
	return DecideCancelOrder(command, input.PriorEvents)
}

func OrderFulfillmentWorkflow(ctx workflow.Context, input OrderFulfillmentInput) (OrderFulfillmentResult, error) {
	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID
	var sales SalesResult
	if err := executeChild(ctx, workflowID+"/sales", SalesWorkflowName, SalesWorkflowInput{Cart: input.Cart, Command: input.Command}, &sales); err != nil {
		return OrderFulfillmentResult{}, err
	}
	result := OrderFulfillmentResult{Sales: sales}
	if sales.Rejected != nil {
		return result, nil
	}
	if sales.Placed == nil {
		return OrderFulfillmentResult{}, fmt.Errorf("sales returned no placed or rejected event")
	}

	var payment PaymentResult
	if err := executeChild(ctx, workflowID+"/billing", BillingWorkflowName, BillingWorkflowInput{
		Order: *sales.Placed, PaymentMethodID: input.PaymentMethodID,
	}, &payment); err != nil {
		return OrderFulfillmentResult{}, err
	}
	result.Payment = &payment
	if payment.Declined != nil {
		var cancellation CancellationResult
		if err := executeChild(ctx, workflowID+"/sales-recovery", SalesRecoveryWorkflowName, SalesRecoveryWorkflowInput{
			Declined: *payment.Declined,
		}, &cancellation); err != nil {
			return OrderFulfillmentResult{}, err
		}
		result.Cancellation = &cancellation
		return result, nil
	}
	if payment.Authorized == nil {
		return OrderFulfillmentResult{}, fmt.Errorf("billing returned no authorized or declined event")
	}

	var shipment ShipmentResult
	if err := executeChild(ctx, workflowID+"/fulfillment", FulfillmentWorkflowName, FulfillmentWorkflowInput{
		Payment: *payment.Authorized, WarehouseID: input.WarehouseID,
	}, &shipment); err != nil {
		return OrderFulfillmentResult{}, err
	}
	result.Shipment = &shipment
	return result, nil
}

func executeChild(ctx workflow.Context, workflowID, workflowName string, input, output any) error {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{WorkflowID: workflowID})
	return workflow.ExecuteChildWorkflow(childCtx, workflowName, input).Get(childCtx, output)
}
