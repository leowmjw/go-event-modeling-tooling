# Agent workflow from Discord — Go + Temporal

Standalone Temporal application generated from `agent-workflow-from-discord` model intake sha256 `27099d2a7a588bafc82cca25644a9b02e7752d51d31108247b59fec23671c0a9`.

## Runtime contract

- Model/context pipeline versions: `conversation-agent`, `invoice-search`, `support-operations` v1.0.0
- Parent workflow: `AgentWorkflowFromDiscordParentV1`
- Child workflows: `ConversationAgentWorkflowV1`, `InvoiceSearchWorkflowV1`, `SupportOperationsWorkflowV1`
- Task queue: `agent-workflow-from-discord`
- Activities: `judge_select_invoice_tool`, `invoice_repository`
- IR: [conversation-agent](../agent-workflow-from-discord/conversation-agent/pipeline.yaml), [invoice-search](../agent-workflow-from-discord/invoice-search/pipeline.yaml), [support-operations](../agent-workflow-from-discord/support-operations/pipeline.yaml)

## Node coverage

| IR node | Go symbol | Shape |
|---|---|---|
| `decide_ask_question` | `DecideAskQuestion` | decision |
| `judge_select_invoice_tool` | `Activities.SelectInvoiceTool` | activity / bounded judge |
| `decide_request_invoice_search` | `DecideRequest` | decision |
| `format_invoice_answer` | `FormatInvoiceAnswer` | decision |
| `decide_record_answer` | `ConversationAgentWorkflow` | child-workflow decision |
| `project_answers` | `ProjectAnswers` | projection/result |
| `invoice_repository` | `Activities.InvoiceRepository` | external activity |
| `decide_record_invoice_search_result` | `DecideRepositoryResult` | typed result union |
| `project_invoice_search_result` | `InvoiceSearchWorkflow` result | projection/result |
| `project_support_review_inbox` | `ProjectSupport` | child-workflow projection/result |

All 10 IR node IDs are mapped. Cross-context events are typed child-workflow inputs. Repository retry uses `MaximumAttempts: 3` for IR `max: 2`; timeout is 30 seconds. Exhaustion emits `InvoiceSearch.InvoiceSearchFailed` and starts Support Operations. No caller idempotency boolean, workflow-reachable wall clock, randomness, filesystem, network, goroutine, native channel, or mutable global is used.

## Scenarios

- Successful bounded tool selection, invoice lookup, exact answer projection.
- Repository retry exhaustion after exactly three attempts, exact support inbox projection.
- Exact `Answers17` data-block projection.

## Commands

```bash
mise run doctor
mise run check
mise run demo
mise run scenario
mise run demo:stop
```

The configured Temporal CLI is `$HOME/go/bin/temporal`. `mise run demo` starts the local dev server and worker using mise-managed Overmind. Run `mise run scenario` in another terminal, then stop services with `mise run demo:stop`.

## External contract

`ActivityDependencies.FindInvoices` receives `{conversationId, fromDate, toDate}` and returns `{conversationId, count, customers, failureReason}`. Production dependencies are injected into `Activities`; workflow code performs no external I/O.

## Verification

- IR schema loading: passed for all contexts.
- Compiler Python tests: passed for all contexts.
- Provenance: current for all contexts.
- `go test -race ./...`: passed.
- `go vet ./...`: passed.
- `go mod tidy -diff`: passed with no diff.
- `mise run doctor`: passed.
- Bounded local Overmind demo: passed; the parent completed the successful cross-context path and services were stopped cleanly.

## Temporal emitter parity

The Go runtime preserves upstream timeout, retry arithmetic, stable explicit activity names, mandatory scheduling, typed bindings, and exit results. Intentional extension: unlike the Python emitter's single-pipeline workflow, this application adds an explicit parent and one child workflow per EVML bounded context, with typed cross-context inputs and support-failure routing.
