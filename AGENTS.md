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
├── model.go         Domain types (Model, Frame, Slice, Chapter, Hotspot, Stage, …)
├── parse.go         Hand-written recursive-descent parser (records Line/LineCount for editors)
├── render.go        SVG renderer + layout helpers (chapters, slices, hotspots, stage styling)
├── validate.go      Validate / ValidateConnections / ValidateRanges / Lint
├── filter.go        FilterStages — the as-is / staging / future "lens"
├── diff.go          Diff — semantic comparison of two versions by frame ID
├── cli.go           CLI wiring: svg [--stage], lint [--strict], diff
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
| Lint every fixture | `mise run lint` (`-- --strict` fails on any finding) |
| Web app without an LLM | `mise run webapp:nollm` |
| Direct test run | `go test ./...` |
| Direct build | `go build -o bin/evml ./cmd/evml` |

> Sandboxed agents: if `go build`/`go test` fails with `operation not permitted`
> on `~/Library/Caches/go-build`, set `GOCACHE=$TMPDIR/gocache`. The
> `writing stat cache … operation not permitted` warning from the module cache
> is harmless.

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

### Adding a new top-level keyword (like `hotspot`, `slice`)
1. Parse it in the `switch` in `parser.parse` **and** add it to `isTopLevel`
   (otherwise a `gwt` block swallows it as a statement).
2. Record `Line`/`LineCount` on the new node — `cmd/evmlweb/internal/webapp/edit.go`
   relies on them for text-level edits.
3. Resolve references in `resolveReferences`; range checks go in `ValidateRanges`.
4. Make `FilterStages` (filter.go) carry the node across when its frame survives.
5. Update `EVENT_MODELING.md` §11 (BNF) and the relevant §12–14 section.

### Stages (`current` / `staging` / `future`)
- `Model.FrameStage(f)` is the only place that resolves a frame's effective
  stage (explicit `#tag` → enclosing slice → current). Never re-derive it.
- `Lint` skips `uncovered-command` for future-stage commands on purpose.
- `FilterStages` with all three stages returns the *same* pointer; callers
  may rely on that for cheap no-op lenses.

### Parser leniency to preserve
- Modifiers `@Actor` / `#stage` are accepted before `->>` sources **and** after
  a single-line `{ … }` payload, but not after a quoted payload (pre-existing
  behaviour: trailing junk after a quoted string is ignored).
- Multi-line prose blocks (`note`, `hotspot`, `data`) retry without
  quote-awareness when the quote-aware pass fails, so a lone apostrophe
  ("the bank's clock") doesn't produce `unbalanced payload braces`.
- `gwt` `LineCount` excludes trailing blank lines.

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
- Hotspots, actors, chapters and slices (with status and stage) are
  implemented — see `EVENT_MODELING.md` §12–14 and the
  `staging-lens-hotspots.evml` fixture. Validation rules: chapter ranges
  must not overlap; slices must not straddle a chapter boundary; ranges run
  forwards in *declaration* order (frame IDs are labels, not positions).
- Lint findings are advisory (`evml lint`), not validation errors; only
  `--strict` turns them into a non-zero exit. Keep it that way so a model with
  open questions still renders in a session.

---

## Do not
- Import third-party packages (keep zero non-stdlib dependencies in the library).
- Add a `//nolint` directive without a comment explaining why.
- Commit binary output (`bin/`, `tmp/`) — they are gitignored.
- Change the `.evml` DSL grammar without updating `EVENT_MODELING.md`.

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

Build/run it independently of the root toolchain: `cd cmd/evmlweb && go run .` (or
`go run . -llm=false` to skip Kronk entirely — every editing feature works without a model;
only the Assistant tab is disabled). It reuses `evml.Parse` / `evml.Validate` /
`evml.FilterStages` / `evml.Diff` / `evml.Lint` / `evml.RenderSVG` unchanged and writes
promoted drafts straight into `testdata/fixtures/`, so it never needs to modify the core library.

### Workshop editing model (learned 2026-09)

Domain experts never see the DSL. Every form in the side panel (`Steps`, `Questions`,
`Scenarios`, `Slices`) posts a few signals to a handler in `handlers_edit.go`, which:

1. Formats the change as DSL text (`edit.go`: `FormatFrameLine`, `FormatScenario`,
   `FormatHotspot`, `FormatSlice`, …).
2. Splices it into the draft's **source text** using the parser's `Line`/`LineCount`
   positions (`InsertAfterFrame`, `ReplaceLines`, `RemoveFrame`, `SetFrameStage`) — never
   by re-serialising the AST, so comments and the expert's formatting survive.
3. Re-parses + validates via `commitSource`. A rejected edit leaves the draft untouched and
   sets `Session.LastError` (transient, shown once as the red banner); an accepted edit
   appends a `system` transcript message — the transcript doubles as the **session log**
   shown in the Compare tab.

Invariants to keep:
- New steps default to `#staging`; `SetFrameStage` writes an explicit `#current` only when
  the frame would otherwise inherit a different stage from a slice.
- `RemoveFrame` must leave a parseable file: it drops anchored notes/hotspots/scenarios,
  strips `->> id` references from other frames, and shrinks or removes chapters/slices
  whose range starts or ends on the frame. Add a test in `edit_test.go` when extending it.
- The diagram is rendered **through the lens** (`Session.Lens`, persisted in the session
  snapshot) on every request; `DraftVersion.SVG` remains the full render used as a
  fallback when the source doesn't parse.
- `handlers_edit_test.go` drives `App.Routes()` in-process with one cookie — extend that
  test for any new endpoint. The sandbox cannot bind ports, so this is the smoke test.

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

#### Signals & client-side state

- Panel/selection state lives in signals declared once on `#workspace-inner` with
  `data-signals__ifmissing` so an SSE replace doesn't reset the open tab or selected step.
  `lens` and `source` are server-owned: `lens` is re-declared on every patch and `source`
  is pushed with `PatchSignals` after each workspace patch.
- Forms send only their own signals: `@post(url, {filterSignals: {include: /^step/}})`.
  Server handlers read camelCase JSON keys (`stepType`) for kebab-case bindings
  (`data-bind:step-type`).
- Clicking a frame in the SVG works because the renderer emits `data-frame="<id>"` on each
  `g.box`; `#svg-container`'s click handler sets `$sel`. A `data-effect` toggles the
  `.selected` class on `#frame-<id>`.
- `$moved` guards against a pan being read as a click.

#### Session persistence

Per-browser state (model, active flow, active draft per flow, lens) is keyed by the
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
cd cmd/evmlweb && go test ./...          # includes the in-process editing flow test
# UI regression (evmlweb must be running on :8080 — `mise run webapp` or `mise run webapp:nollm`):
mise run test:ui-model-flow-selection    # flow open, lens, click-to-select, add step, reload
```

Browser debug logging: append `?debug=1` to the URL.
