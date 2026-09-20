# Compile report: agent-workflow-from-discord / support-operations

## Classification
| Frames | Node | Kind | Rationale |
|---|---|---|---|
| 19 | `project_support_review_inbox` | pure_function | Exact failure-event projection into the modeled data block. |

## Crystallization
The projection is working and exactly tested; no stubs.

## Cross-context links
Consumes `InvoiceSearch.InvoiceSearchFailed` from invoice-search.

## Judgment calls
Morning review is operator cadence, not a workflow timer. The inbox is returned as durable workflow state.

## Open questions
None.

## Changes
Initial post-split major version `1.0.0`.
