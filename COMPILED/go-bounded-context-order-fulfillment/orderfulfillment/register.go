package orderfulfillment

import (
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func Register(w worker.Worker, activities *Activities) error {
	if w == nil || activities == nil || activities.PaymentGateway == nil || activities.Warehouse == nil {
		return fmt.Errorf("worker, payment gateway, and warehouse inventory are required")
	}
	w.RegisterWorkflowWithOptions(OrderFulfillmentWorkflow, workflow.RegisterOptions{Name: OrderFulfillmentWorkflowName})
	w.RegisterWorkflowWithOptions(SalesWorkflow, workflow.RegisterOptions{Name: SalesWorkflowName})
	w.RegisterWorkflowWithOptions(BillingWorkflow, workflow.RegisterOptions{Name: BillingWorkflowName})
	w.RegisterWorkflowWithOptions(FulfillmentWorkflow, workflow.RegisterOptions{Name: FulfillmentWorkflowName})
	w.RegisterActivityWithOptions(activities.AuthorizePaymentWithGateway, activity.RegisterOptions{Name: AuthorizePaymentWithGatewayActivityName})
	w.RegisterActivityWithOptions(activities.AllocateShipmentWithWarehouse, activity.RegisterOptions{Name: AllocateShipmentWithWarehouseActivityName})
	return nil
}
