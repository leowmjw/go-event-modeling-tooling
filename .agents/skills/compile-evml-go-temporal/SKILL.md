---
name: compile-evml-go-temporal
description: >-
  Compile a validated .evml event model into rote pipeline IR and a standalone,
  idiomatic Go Temporal application. Produces one child workflow per bounded
  context, a parent orchestration workflow for cross-context links, activities,
  worker/starter commands, provenance, reports, and Temporal testsuite tests.
  Use for requests to compile event models directly to Go + Temporal, combine
  EVML compilation and Temporal emission, or refresh generated Go workflows.
---

# Compile EVML to rote IR and Go Temporal

Compile the model once, then emit both reviewable rote IR and standalone Go code. The `.evml` remains the business source of truth; `model.json` is the validated intake; `pipeline.yaml` is the runtime-neutral workflow contract; Go is generated from that contract plus the crystallized business rules.

## Authoritative references

Read before writing:

- `.agents/skills/compile-evml-rote-ir/SKILL.md` for intake, context partitioning, frame mapping, provenance, versioning, and reports.
- `ROTE/emit-rote-ir-to-temporal/SKILL.md` and `references/parity-checklist.md` for Temporal semantics.
- `ROTE/rote/src/rote/adapters/temporal.py` as read-only parity reference.
- Temporal Go SDK public packages only: `go.temporal.io/sdk/{activity,client,temporal,testsuite,worker,workflow}`.

Never import internal SDK packages. Never edit `ROTE/`.

## Output

For `<model>`, retain the normal IR at `COMPILED/<model>/`. Emit Go at:

```text
COMPILED/go-<model>/
├── go.mod
├── go.sum
├── README.md
├── .mise.toml
├── Procfile
├── model.json                # copied validated intake snapshot
├── <package>/
│   ├── types.go
│   ├── decisions.go
│   ├── activities.go
│   ├── workflows.go
│   └── workflows_test.go
└── cmd/
    ├── worker/main.go
    ├── starter/main.go
    └── <signal-or-update>/main.go
```

The Go folder is standalone: it must not import the repository root or rote.

## Procedure

1. Run the complete `compile-evml-rote-ir` targeted procedure, including its fresh-intake preflight completeness gate even when provenance says `current`. Mechanically repair stale/invalid generated artifacts and EVML omissions only when the intended rule is already explicit elsewhere in the model. Stop—without emitting Go—on unresolved context ownership, LLM classification, hotspot, under-specified command branch, non-derivable read-model field, hidden identifier/timestamp source, missing implementation, placeholder test/report, or invalid/structurally disconnected IR. Report each blocker by frame and state exactly what model information is required; never invent the answer.
2. Copy `model.json` into the Go output. Keep each context's authoritative `pipeline.yaml`, `provenance.json`, and `compile-report.md` in the sibling `COMPILED/<model>/<context>/`; do not duplicate artifacts that can drift. Link those paths from the Go README and verify their provenance immediately before emission. Go behavior must agree with them.
3. Build a node coverage table before emission: every IR node ID must map to a Go decision, projection, activity, query, signal/update handler, or child workflow. Stop if any node is unmapped. Map every bounded context to a child workflow. Create a parent workflow when the model has cross-context links. The parent uses `workflow.ExecuteChildWorkflow`; child options use stable workflow IDs and public SDK APIs.
4. Map deterministic `pure_function` nodes to plain Go decision/projection functions. Map calls to external systems to activity interfaces and registered activity methods. Workflows contain no I/O, wall-clock access, randomness, goroutines, or native channels.
5. Map `timeout` and `retry` to `workflow.ActivityOptions` and `temporal.RetryPolicy`; `retry.max` means `MaximumAttempts = max + 1`.
6. Preserve success/rejection alternatives as typed result unions with an explicit event discriminator. Do not silently default a mandatory result.
7. Cross-context event links become typed child-workflow inputs. A retry feedback edge that would make the IR cyclic becomes a signal/update to a stateful entity child or a new child-workflow execution, never a cyclic workflow graph. Register query handlers for modeled read models that must be observable while a workflow is waiting. Derive idempotency from workflow state or prior domain events—never from a caller-supplied boolean.
8. Generate `cmd/worker` registering every workflow and activity on one task queue. Generate `cmd/starter` that starts the parent workflow through `client.ExecuteWorkflow`.
9. Use the Temporal Go testsuite. Prefer anonymous replacement functions registered with `env.RegisterActivityWithOptions` over interface mocks or `env.OnActivity`; capture counters/inputs in closures to assert calls. Exercise every gwt success/rejection branch and each cross-context parent path. Tests must not require a Temporal server.
10. Add at least one retry attempt-count test when an activity has retry configuration. When regenerating an existing workflow type, run Temporal's public workflow replayer against saved production/test history before declaring the change replay-compatible; otherwise version the workflow type.
11. Run `gofmt`, `go mod tidy`, `go test ./...`, and `go vet ./...`. Inspect `go list -m all` and pin the Temporal SDK to a release at least seven days old.
12. Generate a standalone `.mise.toml` and `Procfile`: declare `go = "latest"` and Overmind as mise tools, while setting `go.mod` to the current stable Go language version; `mise run doctor` verifies Go, mise-managed Overmind, tmux, the configured Temporal CLI, module integrity, and required files; `mise run demo` starts a local Temporal dev server plus worker through `mise exec -- overmind`; separate tasks start scenarios, send modeled signals, query live read models, stop services, and run all checks. In this repo the Temporal CLI is `$HOME/go/bin/temporal`; do not assume it is on `PATH`. Ignore only local Temporal/Overmind state.
13. Refresh the Go README with model version, workflow names, task queue, commands, scenario list, cross-context links, external activity contracts, a reproducible demo walkthrough, and verification results.

## Deterministic parity gate

Emission is incomplete until all checks below pass:

1. Compare the fresh `model.json` sha256 with every context's `provenance.json`; stop on stale/missing/unassigned contexts.
2. Parse every `pipeline.yaml` and inventory all node IDs, edges, entry/exit nodes, bindings, constants, mandatory flags, timeouts, retries, signals, and output types.
3. Produce a node-coverage table in the Go README or compile report. Every node must identify its Go symbol and execution shape (decision, projection, activity, child workflow, query, signal, or update). Counts and IDs must match exactly; no report-only omission is allowed.
4. Trace every IR edge and binding into typed Go data flow. Reject generated code that invents a zero value, caller boolean, or default branch to replace missing workflow state.
5. Trace every exit node into a workflow result, durable query state, or emitted cross-context event. Read models must be returned or queryable; feedback edges must be represented by a durable signal/update or a new workflow execution.
6. Verify every `gwt` as an executable testsuite case, including all `given` prior state, `when` command payload, and complete `then` event payload—not only the event discriminator. Verify every referenced `data` block with an exact projection test so demo fixtures cannot silently omit fields.
7. Verify retry arithmetic (`max + 1`), timeout units, backoff, mandatory scheduling, and external-I/O activity boundaries.
8. Reject workflow-reachable nondeterminism: native goroutines/channels, `time.Now`, timers outside workflow APIs, randomness, map iteration that affects commands, filesystem/network/database calls, mutable globals, or standard `context.Context`.
9. Register stable explicit workflow/activity names. If behavior changes incompatibly, emit a new workflow type/version and preserve the old worker until in-flight runs drain; otherwise prove replay compatibility with the public replayer.
10. Run `gofmt` check, `go mod tidy -diff`, `go test -race ./...`, `go vet ./...`, provenance check, and a bounded local Overmind demo before calling the output runnable.

## Go conventions

- Use small typed structs; avoid `map[string]any` for domain contracts.
- Return `(T, error)` from activities and validate required fields at boundaries.
- Keep business decisions as ordinary deterministic functions so tests can cover them without Temporal.
- Name registered workflow/activity types explicitly via registration options when compatibility requires a stable name.
- Use `workflow.Context`, `workflow.ExecuteActivity`, `workflow.ExecuteChildWorkflow`, `workflow.Await`, and SDK futures/channels only inside workflows.
- Use standard `context.Context` only in activities and client/worker commands.
- Inject production activity dependencies through a struct; never capture clients or mutable global state in workflow code.
- A generated application is not complete if any implementation panics, returns a placeholder, or contains `TODO`/`NotImplemented`.

## Safe regeneration

Before replacing generated Go, compare it with the last committed/generated version. Preserve hand-written code unless provenance proves it is generated and unchanged. If ownership is ambiguous, stop and ask. Context merges/splits or workflow-name changes are major versions because in-flight workflows must drain on the previous worker.

## Completion report

Report model and pipeline versions, parent/child workflow names, activities, task queue, test scenarios, `go test`/`go vet` results, provenance status, and any parity difference from the Python Temporal emitter.