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
- Four notation features from the eventmodelers.ai cheat sheet have no DSL
  equivalent yet: hotspots, actor lanes, chapters, slice status tags. Grammar
  sketches and rationale live in `EVENT_MODELING.md` §12 — read that before
  proposing new keywords for any of these.

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

### Workshop UX (learned 2026-09, domain-expert sessions)

The app is built for live sessions with non-technical domain experts. The
core loop is **staging, not auto-applying** — nothing changes the expert's
model without an explicit click:

- **Staged proposals.** A chat turn that yields valid `.evml` does NOT
  overwrite the draft. It's stored as `DraftVersion.PendingEvml/PendingSVG`;
  the diagram shows a *preview* of the proposal and an amber banner offers
  **Accept** (`POST .../accept`) / **Reject** (`POST .../reject`). Parse
  errors get one automatic LLM retry fed with the validation error before
  surfacing to the expert (`streamAssistantReply` in handlers_chat.go).
- **Source tab.** The left pane has Diagram | Source view tabs (client-side
  `$leftView` signal). Source shows the draft's `.evml` in a textarea
  (`data-bind:evml-source`); **Save** posts to `POST .../source`, which
  validates before applying — invalid source sets `ParseError` and never
  touches the diagram.
- **Draft management.** Tabs show label + date; per-draft **Rename**
  (`POST .../rename`, `draftLabel` signal), **Delete** (`POST .../delete`,
  JS `confirm()`; deleting the last draft forks a fresh one from the
  baseline), **Export .evml/.svg** (`GET .../export.evml|svg`, plain
  downloads). Custom labels live in `DraftVersion.Label` (sidecar JSON).
- **Safer Activate.** The Activate button JS-confirms first; the handler
  backs up the current fixture to `<state-dir>/_backups/<flow>/<timestamp>.evml`
  before overwriting `testdata/fixtures/<flow>.evml`.
- **Per-draft locking.** `DraftVersion.mu` serializes chat/edit mutations —
  two browser tabs chatting on the same draft no longer race.
- `PendingEvml` persists in the draft sidecar; `PendingSVG` is derived on
  load (a stale/invalid proposal is silently dropped), same pattern as
  `SVG` from `EvmlSource`.

New routes (all under `/flow/{flow}/draft/{id}`): `accept`, `reject`,
`source`, `rename`, `delete`, `export.evml`, `export.svg`. Handlers share
the `lookupDraft` helper and still funnel through `patchWorkspaceSSE`.

### Runtime notes

- **llama.cpp version pinning:** `-llama-version <tag>` flag or
  `EVMLWEB_LLAMA_VERSION` env var overrides the kronk SDK's default
  llama.cpp build. Use it when the SDK default requires a newer OS than the
  machine has (e.g. kronk v1.29.9 defaults to b10195, whose prebuilt macOS
  binaries target macOS 26 and fail to `dlopen` on macOS 15). Note the
  pinned version must still export every symbol the kronk SDK loads —
  too-old builds (e.g. b9849) fail with `dlsym ... symbol not found`.
  Pick the newest tag whose macOS binary still targets your OS.

### FinTech example fixtures (added 2026-09)

Three complex, multi-bounded-context models for workshop demos and parser
coverage — `fintech-card-payment-settlement.evml` (auth → clearing →
ledger → reconciliation, decline/reversal/mismatch branches),
`fintech-fraud-review.evml` (real-time risk scoring → analyst case work →
SAR filing), `fintech-kyc-onboarding.evml` (application → external IDV →
sanctions screening → account provisioning). All follow the same patterns:
contexts crossed only via `rf` facts + translator processors, every branch
ends in a recorded outcome read model, GWT scenarios for the key rules.

---

## Roadmap — known gaps (as of 2026-09)

Ordered roughly by value for live domain-expert workshops.

### Web app (`cmd/evmlweb`)

1. **Proposal diff view.** Accept/Reject shows only the rendered SVG
   preview; there's no textual diff of `PendingEvml` vs `EvmlSource`, so
   experts can't see *what* the assistant changed line-by-line before
   accepting. A simple line diff in the Source tab (or a third "Compare"
   view) closes this.
2. **Clickable diagram nodes.** `render.go` emits inert SVG — frame boxes
   carry no `id`/`data-*` attributes. Adding them would let the Datastar
   frontend offer click-a-box → edit-panel (rename, change payload, delete)
   without going through chat.
3. **AST → `.evml` serializer (root library).** The library parses and
   renders but cannot write a `Model` back out as `.evml` text. This is the
   prerequisite for any true GUI editing (mutate AST → validate → write →
   render) instead of having the LLM re-emit the whole document.
4. **Chat context growth.** Every turn resends the full system prompt (two
   long docs) + entire transcript + full document; `max_tokens=4096` can
   silently truncate large complete-document responses mid-fence (no fence
   → misread as a "clarifying answer"). Needs transcript compaction and/or
   a larger token budget scaled to document size.
5. **Streaming cost.** Every token delta re-renders and outer-morphs the
   entire `#chat-log` — O(transcript) per token and resets scroll. Patch
   only the in-flight assistant bubble instead.
6. **Workspace decoupling.** The app only reads `testdata/fixtures/` and
   Activate writes into the git working tree. Support pointing at an
   arbitrary folder of `.evml` files so workshops don't mutate the repo.
7. **Single-user.** Cookie sessions are per-browser; two people can't see
   or collaborate on each other's drafts. No sharing/export of a session.
8. **In-app model download.** Requires locally pre-downloaded Kronk models;
   no picker-driven download.
9. **llama.cpp runtime on macOS 15 (blocked, see "Runtime notes").** kronk
   v1.29.9's default b10195 targets macOS 26; older builds lack symbols the
   SDK loads. Until a compatible pin is found, the app can't boot on this
   machine and the Playwright UI regression can't run.

### DSL / core library

10. **§12 proposed extensions** (`EVENT_MODELING.md`): `hotspot`, `actor`
    lanes + `@Actor`, `chapter`, `slice … status`. The types already exist
    in `model.go` (`Hotspot`, `Chapter`, `Slice`, `SliceStatus`,
    `Frame.Actor`) but the parser never populates them and `render.go`
    ignores them; `model.go` doc comments even reference `OpenHotspots`/
    `Lint` functions that don't exist yet. Slice statuses (`live` /
    `staging` / `future` / `blocked` / `deprecated`) are exactly the
    vocabulary for "what fits reality vs future goals" in workshops, so
    this pairs naturally with the web app's staging flow. Read §12's
    grammar sketches before implementing — parser, renderer, validation,
    and docs must move together.

NOTE: Kimi K3 - command-code --resume 10d16794-0aab-4b42-8ee2-89025913a493

