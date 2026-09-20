package agentworkflow

import (
	"fmt"
	"strings"
)

func DecideAskQuestion(c AskQuestion) (QuestionAsked, error) {
	if c.ConversationID == "" || c.Question == "" || c.RequestedAt == "" || c.BusinessTimezone == "" {
		return QuestionAsked{}, fmt.Errorf("all question fields are required")
	}
	return QuestionAsked{"QuestionAsked", c.ConversationID, c.Question, c.RequestedAt, c.BusinessTimezone}, nil
}
func DecideRequest(s ToolSelection) (InvoiceSearchRequested, error) {
	if s.Tool != "findInvoices" || s.FromDate == "" || s.ToDate == "" {
		return InvoiceSearchRequested{}, fmt.Errorf("invalid tool selection")
	}
	return InvoiceSearchRequested{s.ConversationID, s.FromDate, s.ToDate}, nil
}
func DecideRepositoryResult(r RepositoryResult) InvoiceSearchResult {
	if r.FailureReason != "" {
		return InvoiceSearchResult{Event: "InvoiceSearch.InvoiceSearchFailed", ConversationID: r.ConversationID, Reason: r.FailureReason}
	}
	return InvoiceSearchResult{Event: "InvoiceSearch.InvoicesFound", ConversationID: r.ConversationID, Count: r.Count, Customers: r.Customers}
}
func FormatInvoiceAnswer(r InvoiceSearchResult) (RecordAnswer, error) {
	if r.Event != "InvoiceSearch.InvoicesFound" {
		return RecordAnswer{}, fmt.Errorf("cannot format %s", r.Event)
	}
	noun, verb := "invoices", "were"
	if r.Count == 1 {
		noun, verb = "invoice", "was"
	}
	names := "no customers"
	if len(r.Customers) == 1 {
		names = r.Customers[0]
	} else if len(r.Customers) > 1 {
		names = strings.Join(r.Customers[:len(r.Customers)-1], ", ") + " and " + r.Customers[len(r.Customers)-1]
	}
	return RecordAnswer{r.ConversationID, fmt.Sprintf("%d %s %s sent out yesterday to customers %s.", r.Count, noun, verb, names)}, nil
}
func ProjectAnswers(a AnswerRecorded) AnswersView { return AnswersView{a.ConversationID, a.Answer} }
func ProjectSupport(r InvoiceSearchResult) SupportReviewInbox {
	return SupportReviewInbox{[]SupportItem{{r.ConversationID, r.Reason}}}
}
