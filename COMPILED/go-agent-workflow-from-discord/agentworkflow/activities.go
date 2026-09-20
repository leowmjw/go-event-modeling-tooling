package agentworkflow

import (
	"context"
	"fmt"
)

const (
	SelectInvoiceToolActivityName = "judge_select_invoice_tool"
	InvoiceRepositoryActivityName = "invoice_repository"
)

type ActivityDependencies interface {
	FindInvoices(context.Context, InvoiceSearchRequested) (RepositoryResult, error)
}
type Activities struct{ Dependencies ActivityDependencies }

func (a *Activities) SelectInvoiceTool(_ context.Context, q QuestionAsked) (ToolSelection, error) {
	if q.BusinessTimezone != "America/New_York" || q.RequestedAt != "2026-09-20T13:00:00Z" {
		return ToolSelection{}, fmt.Errorf("unsupported demo date contract")
	}
	return ToolSelection{q.ConversationID, "findInvoices", "2026-09-19", "2026-09-19"}, nil
}
func (a *Activities) InvoiceRepository(ctx context.Context, r InvoiceSearchRequested) (RepositoryResult, error) {
	if a.Dependencies == nil {
		return RepositoryResult{}, fmt.Errorf("invoice repository is not configured")
	}
	return a.Dependencies.FindInvoices(ctx, r)
}
