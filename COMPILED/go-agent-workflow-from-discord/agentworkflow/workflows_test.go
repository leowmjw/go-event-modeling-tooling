package agentworkflow

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
)

func registerWorkflows(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterWorkflowWithOptions(ParentWorkflow, workflow.RegisterOptions{Name: ParentWorkflowName})
	env.RegisterWorkflowWithOptions(ConversationAgentWorkflow, workflow.RegisterOptions{Name: ConversationWorkflowName})
	env.RegisterWorkflowWithOptions(InvoiceSearchWorkflow, workflow.RegisterOptions{Name: InvoiceSearchWorkflowName})
	env.RegisterWorkflowWithOptions(SupportOperationsWorkflow, workflow.RegisterOptions{Name: SupportOperationsWorkflowName})
	env.RegisterActivityWithOptions(func(context.Context, QuestionAsked) (ToolSelection, error) {
		return ToolSelection{"conv-1001", "findInvoices", "2026-09-19", "2026-09-19"}, nil
	}, activity.RegisterOptions{Name: SelectInvoiceToolActivityName})
}
func question() AskQuestion {
	return AskQuestion{"conv-1001", "How many invoices were sent out yesterday?", "2026-09-20T13:00:00Z", "America/New_York"}
}
func TestParentSuccess(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	registerWorkflows(env)
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, InvoiceSearchRequested) (RepositoryResult, error) {
		calls++
		return RepositoryResult{"conv-1001", 2, []string{"X", "Y"}, ""}, nil
	}, activity.RegisterOptions{Name: InvoiceRepositoryActivityName})
	env.ExecuteWorkflow(ParentWorkflowName, question())
	require.NoError(t, env.GetWorkflowError())
	var got ParentResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, "2 invoices were sent out yesterday to customers X and Y.", got.Answer.Answer)
	require.Equal(t, 1, calls)
}
func TestRetryExhaustionRoutesSupport(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	registerWorkflows(env)
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, InvoiceSearchRequested) (RepositoryResult, error) {
		calls++
		return RepositoryResult{}, errors.New("down")
	}, activity.RegisterOptions{Name: InvoiceRepositoryActivityName})
	env.ExecuteWorkflow(ParentWorkflowName, question())
	require.NoError(t, env.GetWorkflowError())
	var got ParentResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, []SupportItem{{"conv-1001", "repository_unavailable"}}, got.SupportInbox.Items)
	require.Equal(t, 3, calls)
}
func TestDataProjectionExact(t *testing.T) {
	require.Equal(t, AnswersView{"conv-1001", "2 invoices were sent out yesterday to customers X and Y."}, ProjectAnswers(AnswerRecorded{"AnswerRecorded", "conv-1001", "2 invoices were sent out yesterday to customers X and Y."}))
}
