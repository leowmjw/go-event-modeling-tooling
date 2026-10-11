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

2026-10-03 (provenance-only refresh, version unchanged): `tf 11` and `tf 12` declared their `->> 09` / `->> 10`
sources after the inline payload, which the parser silently dropped. The `.evml` now orders clauses per the
grammar (`->>`, then `[[ref]]`, then payload) and the parser rejects the old order. The IR already carried the
equivalent `decide_record_invoice_search_result → project_invoice_search_result` edge (event frames are not
nodes), so no node, edge, or binding changed; only the section hashes in `provenance.json` were rewritten.
