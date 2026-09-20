# agent-workflow-from-discord compiled IR

| Context | Pipeline | Version | Input | Exit events/views |
|---|---|---:|---|---|
| Conversation Agent | `conversation-agent/pipeline.yaml` | 1.0.0 | AskQuestion or InvoicesFound | InvoiceSearchRequested, Answers |
| Invoice Search | `invoice-search/pipeline.yaml` | 1.0.0 | InvoiceSearchRequested | InvoicesFound, InvoiceSearchFailed, InvoiceSearchResult |
| Support Operations | `support-operations/pipeline.yaml` | 1.0.0 | InvoiceSearchFailed | SupportReviewInbox |
