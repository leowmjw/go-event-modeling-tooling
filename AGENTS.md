# AGENTS.md — Coding guide for AI agents working on this repository

This file captures the conventions used in `go-event-modeling-tooling` so that
any AI agent (or human) can produce idiomatic, consistent contributions without
needing to read every source file first.

---

## Repository layout

```
.
├── cmd/evml/        CLI entry-point (main package)
├── testdata/
│   └── fixtures/    *.evml sample files used by render_test.go
├── model.go         Domain types (Model, Frame, DataEntity, …)
├── parse.go         Hand-written recursive-descent parser
├── render.go        SVG renderer + layout helpers
├── validate.go      Post-parse validation helpers
├── json.go          ModelJSON — evml-model/v1 JSON serializer for tooling
├── cli.go           CLI wiring (Cobra / flag parsing)
├── cli_test.go
├── parse_test.go
├── render_test.go
├── .mise.toml       Toolchain + task runner (mise)
└── .air.toml        Air hot-reload config
```

---

## Toolchain

- **Go 1.26** (declared in `go.mod` and `.mise.toml`).
- **mise** manages the Go toolchain and the `air` hot-reload binary.
- No external frameworks — the standard library only.

### Common commands

| Goal | Command |
|------|---------|
| Build CLI | `mise run build` |
| Run all tests | `mise run test` |
| Dev (hot reload) | `mise run dev` |
| Re-render changed fixture SVGs into `out/` | `mise run svg` (`-- --all` forces every fixture) |
| Direct test run | `go test ./...` |
| Direct build | `go build -o bin/evml ./cmd/evml` |
| Emit model JSON (for the compile skill) | `go run ./cmd/evml json <file> [-o out]` |

---

## Go style guide (this repo)

### Package organisation
- One package per responsibility: `evml` (library) and `main` (CLI wrapper).
- No sub-packages inside the library — keep the surface flat.
- All exported names live at the package root.

### Naming
- Types: `PascalCase` — `Frame`, `DataEntity`, `GWT`.
- Functions / methods: `PascalCase` if exported, `camelCase` otherwise.
- Constants / enum-like strings: `PascalCase` for exported, e.g. `EntityUI`.
- Single-letter receivers are fine for small types (`f *Frame`, `p *parser`).
- Error types follow the `XxxError` convention (`ParseError`).

### Error handling
- Return `error` as the last value; never panic for user-visible errors.
- Use the custom `ParseError{Line, Msg}` type for parser errors so callers can
  report line numbers.
- Wrap with `fmt.Errorf("context: %w", err)` only when adding context.
- Use **`errors.AsType[T]`** (Go 1.26) instead of the old two-step
  `var pe *T; errors.As(err, &pe)` pattern when type-asserting errors.

### Tests
- File: `*_test.go` next to the file under test, same package (`package evml`).
- Use `t.Fatalf` (not `t.Errorf`) when further steps cannot proceed.
- Table-driven tests where there are multiple cases.
- Fixture files go in `testdata/fixtures/*.evml`; `TestRenderFixturesToSVG`
  picks them up automatically via `filepath.Glob`.

### Adding a new fixture
1. Create `testdata/fixtures/<name>.evml`.
2. No code changes required — the glob test covers it automatically.
3. Add a focused `TestRender<Name>` function in `render_test.go` only if you
   need to assert specific SVG content beyond the generic smoke test.

### SVG rendering colours (do not change without updating this file)

| Entity type | Fill | Stroke |
|---|---|---|
| `ui` / `pcr` | `#f8d4bc` | `#d38e5f` |
| `cmd` / `rmo` | `#bcd6fe` | `#679ac3` |
| `evt` | `#d3f1a2` | `#84af49` |

### String helpers
- `StripOuterBraces(s)` — removes wrapping `{ }` from data payloads.
- `StripQuotes(s)` — removes wrapping `"` or `'` from quoted strings.
- `esc(s)` — HTML-escapes text before embedding in SVG.

### Adding a new entity type
1. Add the `EntityType` constant in `model.go`.
2. Add the keyword to `parseEntityType` in `parse.go`.
3. Add a `case` in `frameColors` in `render.go`.
4. Add a `case` in `SwimlaneBand` in `model.go`.
5. Add at least one fixture and a targeted test.

### Adding a new frame annotation (sibling of `note` / `hotspot`)
Annotations attach to a frame by id and are *not* `EntityType`s, so
`allowedSources` is untouched. Follow the `hotspot` implementation:
1. Add the `XxxEntity` struct + `Model.XxxEntities` slice in `model.go`.
2. Add `parseXxxEntity` in `parse.go` (mirror `parseNoteEntity`), a
   `hasKeyword(trimmed, "xxx")` case in `parse`, the keyword in
   `isTopLevel`, and a resolution loop in `resolveReferences`.
3. Add the `xxxJSON` type + array to `modelJSON` in `json.go` —
   `make(...)` it so the JSON emits `[]` and not `null`.
4. Add `xxxStackHeight` + `renderXxx` in `render.go` and one more
   `stackBand` call in `RenderSVG`.
5. Include it in the compile skill's provenance hash (`frame_content` in
   `.agents/skills/compile-evml-rote-ir/scripts/evml_provenance.py`) if a
   change to it should invalidate compiled IR.
6. Document it in `EVENT_MODELING.md` (own section + BNF `Statement`
   alternative), and add a fixture plus parser/JSON tests.

---

## Validation semantics (learned 2026-08, cross-checked against eventmodelers.ai)

- `ValidateConnections` in `validate.go` is the single source of truth for
  which entity types may feed which — see `allowedSources`. It shipped with
  a bug (processor only accepted `rmo` sources, rejecting the documented
  `evt → pcr` shorthand); fixed to accept **both** `evt` and `rmo` as
  processor sources, since both shapes are legitimate:
  - Canonical (per the official cheatsheet): `evt → rmo → pcr → cmd → evt` —
    the processor watches a read model.
  - Shorthand (used throughout this repo's fixtures): `evt → pcr → cmd →
    evt` — the processor watches the raw event directly.
- **"No question, no read model"** — do not model `rmo` frames that aren't
  answering a specific, nameable question, and do not use `rmo` as a
  generic placeholder for "external data entering the system" (that's what
  `rf ... evt ...` is for). See `SKILL.md` §"Anti-Patterns to Spot" and the
  Read Model Naming Rules for the full rule and the fixture cleanup example
  (`testdata/fixtures/agent-workflow-from-discord.evml`).
- When touching `allowedSources` or the four-pattern descriptions, update
  both `validate.go`'s error strings and the corresponding prose in
  `EVENT_MODELING.md` / `SKILL.md` together — they're expected to agree.
- Of the four notation features from the eventmodelers.ai cheat sheet that
  had no DSL equivalent, **`hotspot` is now implemented** (`EVENT_MODELING.md`
  §6): a sibling of `note` for *unresolved* questions, rendered in a red box
  and included in the compile skill's provenance hash. Three remain
  unimplemented: actor lanes, chapters, slice status tags. Grammar sketches
  and rationale live in `EVENT_MODELING.md` §13 — read that before proposing
  new keywords for any of these.

---

## Do not
- Import third-party packages (keep zero non-stdlib dependencies in the library).
- Add a `//nolint` directive without a comment explaining why.
- Commit binary output (`bin/`, `tmp/`) — they are gitignored.
- Change the `.evml` DSL grammar without updating `EVENT_MODELING.md`.

---

## `.agents/skills/compile-evml-rote-ir`

Skill that compiles a `.evml` model into rote IR — one
`COMPILED/<model>/<context>/pipeline.yaml` per bounded context (section
banner). Intake is `evml json` output (`COMPILED/<model>/model.json`);
staleness/provenance is tracked by `scripts/evml_provenance.py`.
`COMPILED/` is committed.

## `.devin/skills/compile-evml-go-temporal`

Combined compiler/emitter that first produces the authoritative rote IR above, then emits a
**standalone Go + Temporal application** at `COMPILED/go-<model>/`. Use it for requests to
compile an event model directly to runnable Go workflows, refresh Go output after IR changes,
or demo a model on local Temporal. Generated apps are nested Go modules, so the root library
remains zero-dependency; they may depend on the public Temporal Go SDK.

Load-bearing rules:

- `.evml` → validated `model.json` → `pipeline.yaml` → Go; never bypass or hand-diverge from
  the IR. Provenance must be current before emission.
- Every IR node, edge, binding, entry/exit, retry, timeout, mandatory flag, `gwt`, and referenced
  `data` block needs a typed Go mapping and executable test. The generated README carries the
  exact node-coverage table; unmapped/report-only nodes block emission.
- One child workflow per bounded context; a parent coordinates cross-context facts. External I/O
  is activity-only. Feedback edges use durable Signals/Updates, waiting read models use Queries,
  and idempotency comes from workflow state/prior events—not caller booleans.
- Workflow-reachable code must be deterministic and use only public Temporal SDK packages.
  Version workflow types for incompatible behavior unless public-replayer coverage proves
  compatibility with saved histories.
- Generated tests use `go.temporal.io/sdk/testsuite`; prefer anonymous activities registered with
  `RegisterActivityWithOptions`. Assert complete event payloads and exact data-block projections,
  not only event names.
- Generated apps use the latest stable Go (`go = "latest"` in local mise; current language version
  in `go.mod`), mise-managed Overmind, and `$HOME/go/bin/temporal` (not assumed on `PATH`). They
  include `mise run doctor`, `check`, `demo`, `demo:stop`, scenario, signal, and query tasks plus a
  `Procfile` and demo README.
- Completion requires `gofmt`, `go mod tidy -diff`, `go test -race ./...`, `go vet ./...`, current
  provenance, and a bounded live Overmind demo.

Reference implementation/demo:
`COMPILED/go-bounded-context-order-fulfillment/` (parent + Sales/Billing/Fulfillment children,
read-model projections, inventory-replenishment Signal/Query feedback, worker/starter commands).

---

## `cmd/evmlweb` — local web app (nested module, dependency exception)

`cmd/evmlweb` is a standalone local web app (Go + [Datastar](https://data-star.dev) +
the [Kronk](https://www.kronkai.com) SDK for local LLM inference) that lets non-technical
domain experts build and iterate on `.evml` event models conversationally. It has its
**own `go.mod`** (`github.com/leowmjw/go-event-modeling-tooling/cmd/evmlweb`, with a
`replace` pointing at the repo root) specifically so it can depend on
`github.com/ardanlabs/kronk` and `github.com/starfederation/datastar-go` without
pulling either into the root module's dependency graph — `go get
github.com/leowmjw/go-event-modeling-tooling` (the `evml` library) stays zero-dependency.
The "no third-party packages" rule above applies to the root module only; `cmd/evmlweb`
manages its own dependencies via its own `go.mod`/`go.sum`.

Build/run it independently of the root toolchain: `cd cmd/evmlweb && go run .`. It reuses
`evml.Parse` / `evml.ValidateConnections` / `evml.RenderSVG` unchanged and writes activated
drafts straight into `testdata/fixtures/`, so it never needs to modify the core library.

### Datastar (client + server)

- **Server SDK:** `github.com/starfederation/datastar-go` (see `cmd/evmlweb/go.mod`).
- **Client bundle:** `cmd/evmlweb/static/datastar.js` — currently **v1.0.2** (the latest
  published JS release). The Go SDK version can be newer; the SSE patch protocol is
  compatible. Do **not** assume the client version matches `datastar-go` tag-for-tag.

#### Attribute syntax — colon, not hyphen (learned 2026-08)

Datastar v1.0+ resolves plugins from the attribute **key** using a **colon** separator.
Hyphenated spellings silently fail: the plugin is never registered, handlers never attach,
and native HTML behaviour takes over (e.g. a `<form>` GET-submits and reloads the page).

| Wrong (no-op) | Correct |
|---|---|
| `data-on-click` | `data-on:click` |
| `data-on-submit__prevent` | `data-on:submit__prevent` |
| `data-bind-model` | `data-bind:model` |
| `data-signals-model` | `data-signals:model` |
| `data-attr-style` | `data-attr:style` |

`data-show` is unchanged (no key suffix). Modifiers still use double-underscore:
`data-on:click__prevent`, `data-on:mouseup__window`, etc.

**Symptom checklist** when actions "do nothing":
1. Browser console: no `[evmlweb:debug] fetch ->` lines on click/submit.
2. Network tab: no `POST` to `/model` or `/flow/select`; instead a full-page `GET /?`.
3. `datastar-ready` fires but `$model` / `$flow` signals stay empty.

Reference: [data-star.dev attributes](https://data-star.dev/reference/attributes).

#### SSE patching — never morph inline SVG

`PatchElements` with morph/`outer` mode drops inline `<svg>` inside large HTML fragments
(Datastar `DOMParser` limitation). After flow/chat actions, patch in **two** steps:

1. `PatchElements(workspaceFrag, WithSelectorID("workspace-inner"), WithModeReplace())` —
   tabs, chat, empty `#svg-container` placeholder (`PatchSVG` flag in workspace template).
2. `PatchElements(svgFrag, WithSelectorID("svg-container"), WithModeInner())` — SVG only.

Full page load (Go template render) is unaffected; only the SSE patch path needs the split.

#### Session persistence

Per-browser state (model, active flow, active draft per flow) is keyed by the
`evmlweb_session` cookie token and written to `<state-dir>/_sessions/<token>.json`.
`PersistSelection` is called after model/flow/draft-tab changes — not after in-draft edits
(draft content is saved separately by `DraftStore.Save`).

`handleSelectFlow` must read **both** `model` and `flow` from signals (Open is the atomic
commit). `resumeActiveFlow` must call `NewDraft` when `DraftOrder == 0`, same as Open.

#### Debugging client ↔ server

- **Browser:** append `?debug=1` to enable `static/debug.js` (logs fetch bodies and Datastar
  events as `[evmlweb:debug]`).
- **Server:** structured logs include `session=<token>` — grep the token from either side.
  Key lines: `action: flow select requested`, `action: flow opened`, `action: workspace patched`.

#### Tests

```bash
cd cmd/evmlweb && go test ./...
# UI regression (evmlweb must be running on :8080 — start with `mise run webapp`):
mise run test:ui-model-flow-selection
```

Browser debug logging: append `?debug=1` to the URL.
