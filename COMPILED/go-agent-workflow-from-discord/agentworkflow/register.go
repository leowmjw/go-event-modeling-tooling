package agentworkflow

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func Register(w worker.Worker, a *Activities) {
	w.RegisterWorkflowWithOptions(ParentWorkflow, workflow.RegisterOptions{Name: ParentWorkflowName})
	w.RegisterWorkflowWithOptions(ConversationAgentWorkflow, workflow.RegisterOptions{Name: ConversationWorkflowName})
	w.RegisterWorkflowWithOptions(InvoiceSearchWorkflow, workflow.RegisterOptions{Name: InvoiceSearchWorkflowName})
	w.RegisterWorkflowWithOptions(SupportOperationsWorkflow, workflow.RegisterOptions{Name: SupportOperationsWorkflowName})
	w.RegisterActivityWithOptions(a.SelectInvoiceTool, activity.RegisterOptions{Name: SelectInvoiceToolActivityName})
	w.RegisterActivityWithOptions(a.InvoiceRepository, activity.RegisterOptions{Name: InvoiceRepositoryActivityName})
}
