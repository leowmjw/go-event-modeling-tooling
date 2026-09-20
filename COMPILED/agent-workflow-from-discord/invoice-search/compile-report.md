# Compile report: agent-workflow-from-discord / invoice-search

## Classification
| Frames | Node | Kind | Rationale |
|---|---|---|---|
| 08 | `invoice_repository` | external_call | Named InvoiceRepository I/O contract with timeout and retry. |
| 09 | `decide_record_invoice_search_result` | pure_function | Complete success and rejection GWTs. |
| 12 | `project_invoice_search_result` | pure_function | Typed result projection. |

## Crystallization
All implementations are working and tested; no stubs.

## Cross-context links
Consumes `Conversation.InvoiceSearchRequested`; emits `InvoiceSearch.InvoicesFound` or `InvoiceSearch.InvoiceSearchFailed`.

## Judgment calls
`retry.max: 2` maps to three Temporal attempts. Exhaustion is converted to the modeled failure event.

## Open questions
None.

## Changes
Initial post-split major version `1.0.0`.
