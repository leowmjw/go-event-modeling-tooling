# Compile report: agent-workflow-from-discord / conversation-agent

## Classification
| Frames | Node | Kind | Rationale |
|---|---|---|---|
| 02 | `decide_ask_question` | pure_function | Complete GWT command result. |
| 04 | `judge_select_invoice_tool` | llm_judge | Expert-selected bounded typed judgment. |
| 05 | `decide_request_invoice_search` | pure_function | Complete GWT command result. |
| 14 | `format_invoice_answer` | pure_function | Enumerable formatting rule. |
| 15 | `decide_record_answer` | pure_function | Complete GWT command result. |
| 17 | `project_answers` | pure_function | Exact data-block projection. |

## Crystallization
Five deterministic implementations and one typed judge signature are working and tested. There are no stubs.

## Cross-context links
`Conversation.InvoiceSearchRequested` exits to invoice-search. `InvoiceSearch.InvoicesFound` re-enters the continuation slice.

## Judgment calls
Two entry slices intentionally share one context pipeline and are connected by the parent workflow through invoice-search.

## Open questions
None.

## Changes
Initial post-split major version `1.0.0`; the obsolete single-context placeholder pipeline was replaced.
