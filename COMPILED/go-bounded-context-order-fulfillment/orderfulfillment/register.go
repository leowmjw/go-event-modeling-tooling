package orderfulfillment

import (
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func Register(w worker.Worker, activities *Activities) error {
	if w == nil || activities == nil || activities.PaymentGateway == nil || activities.Inventory == nil {
		return fmt.Errorf("worker, payment gateway, and inventory service are required")
	}
	w.RegisterWorkflowWithOptions(OrderFulfillmentWorkflow, workflow.RegisterOptions{Name: OrderFulfillmentWorkflowName})
	w.RegisterWorkflowWithOptions(SalesWorkflow, workflow.RegisterOptions{Name: SalesWorkflowName})
	w.RegisterWorkflowWithOptions(BillingWorkflow, workflow.RegisterOptions{Name: BillingWorkflowName})
	w.RegisterWorkflowWithOptions(FulfillmentWorkflow, workflow.RegisterOptions{Name: FulfillmentWorkflowName})
	w.RegisterWorkflowWithOptions(SalesRecoveryWorkflow, workflow.RegisterOptions{Name: SalesRecoveryWorkflowName})
	w.RegisterActivityWithOptions(activities.AuthorizePayment, activity.RegisterOptions{Name: AuthorizePaymentActivityName})
	w.RegisterActivityWithOptions(activities.CheckInventory, activity.RegisterOptions{Name: CheckInventoryActivityName})
	return nil
}
