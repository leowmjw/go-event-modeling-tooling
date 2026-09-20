package agentworkflow

type AskQuestion struct{ ConversationID, Question, RequestedAt, BusinessTimezone string }
type QuestionAsked struct{ Event, ConversationID, Question, RequestedAt, BusinessTimezone string }
type ToolSelection struct{ ConversationID, Tool, FromDate, ToDate string }
type InvoiceSearchRequested struct{ ConversationID, FromDate, ToDate string }
type RepositoryResult struct {
	ConversationID string
	Count          int
	Customers      []string
	FailureReason  string
}
type InvoiceSearchResult struct {
	Event, ConversationID string
	Count                 int
	Customers             []string
	Reason                string
}
type RecordAnswer struct{ ConversationID, Answer string }
type AnswerRecorded struct{ Event, ConversationID, Answer string }
type AnswersView struct{ ConversationID, Answer string }
type SupportItem struct{ ConversationID, Reason string }
type SupportReviewInbox struct{ Items []SupportItem }
type ConversationInput struct {
	Question     *AskQuestion
	SearchResult *InvoiceSearchResult
}
type ConversationResult struct {
	Request *InvoiceSearchRequested
	Answers *AnswersView
}
type ParentResult struct {
	Answer       *AnswersView
	SupportInbox *SupportReviewInbox
}
