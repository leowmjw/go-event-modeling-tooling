package agentworkflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue                     = "agent-workflow-from-discord"
	ParentWorkflowName            = "AgentWorkflowFromDiscordParentV1"
	ConversationWorkflowName      = "ConversationAgentWorkflowV1"
	InvoiceSearchWorkflowName     = "InvoiceSearchWorkflowV1"
	SupportOperationsWorkflowName = "SupportOperationsWorkflowV1"
)

func ConversationAgentWorkflow(ctx workflow.Context, input ConversationInput) (ConversationResult, error) {
	if input.Question != nil {
		asked, err := DecideAskQuestion(*input.Question)
		if err != nil {
			return ConversationResult{}, err
		}
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
		var selection ToolSelection
		if err := workflow.ExecuteActivity(ctx, SelectInvoiceToolActivityName, asked).Get(ctx, &selection); err != nil {
			return ConversationResult{}, err
		}
		request, err := DecideRequest(selection)
		if err != nil {
			return ConversationResult{}, err
		}
		return ConversationResult{Request: &request}, nil
	}
	if input.SearchResult != nil {
		command, err := FormatInvoiceAnswer(*input.SearchResult)
		if err != nil {
			return ConversationResult{}, err
		}
		recorded := AnswerRecorded{"AnswerRecorded", command.ConversationID, command.Answer}
		view := ProjectAnswers(recorded)
		return ConversationResult{Answers: &view}, nil
	}
	return ConversationResult{}, temporal.NewNonRetryableApplicationError("unsupported conversation input", "InvalidInput", nil)
}

func InvoiceSearchWorkflow(ctx workflow.Context, input InvoiceSearchRequested) (InvoiceSearchResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3, BackoffCoefficient: 2}})
	var result RepositoryResult
	if err := workflow.ExecuteActivity(ctx, InvoiceRepositoryActivityName, input).Get(ctx, &result); err != nil {
		result = RepositoryResult{ConversationID: input.ConversationID, FailureReason: "repository_unavailable"}
	}
	return DecideRepositoryResult(result), nil
}
func SupportOperationsWorkflow(_ workflow.Context, input InvoiceSearchResult) (SupportReviewInbox, error) {
	return ProjectSupport(input), nil
}

func ParentWorkflow(ctx workflow.Context, input AskQuestion) (ParentResult, error) {
	opts := func(id string) workflow.ChildWorkflowOptions { return workflow.ChildWorkflowOptions{WorkflowID: id} }
	var start ConversationResult
	if err := workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, opts("conversation-"+input.ConversationID)), ConversationWorkflowName, ConversationInput{Question: &input}).Get(ctx, &start); err != nil {
		return ParentResult{}, err
	}
	var search InvoiceSearchResult
	if err := workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, opts("invoice-search-"+input.ConversationID)), InvoiceSearchWorkflowName, *start.Request).Get(ctx, &search); err != nil {
		return ParentResult{}, err
	}
	if search.Event == "InvoiceSearch.InvoiceSearchFailed" {
		var inbox SupportReviewInbox
		if err := workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, opts("support-"+input.ConversationID)), SupportOperationsWorkflowName, search).Get(ctx, &inbox); err != nil {
			return ParentResult{}, err
		}
		return ParentResult{SupportInbox: &inbox}, nil
	}
	var answer ConversationResult
	if err := workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, opts("answer-"+input.ConversationID)), ConversationWorkflowName, ConversationInput{SearchResult: &search}).Get(ctx, &answer); err != nil {
		return ParentResult{}, err
	}
	return ParentResult{Answer: answer.Answers}, nil
}
