---
name: compile-evml-rote-ir
description: >-
  Compile an Event Modeling `.evml` file (testdata/fixtures/*.evml or any
  path) into runtime-agnostic rote IR (`pipeline.yaml`) under
  COMPILED/<model>/<bounded-context>/, ready for later Temporal (or other
  workflow engine) emission. Two modes: (1) targeted — compile one named
  model; (2) top-level — scan every fixture and recompile only the bounded
  contexts whose source frames changed since the last compile (or were never
  compiled). Stops at the IR; it does NOT emit runtime code. Trigger phrases:
  "compile this event model to IR", "compile <model> to rote IR", "recompile
  event models", "refresh the compiled IR", "which IR is stale", "build
  pipeline.yaml for <model>", "turn this .evml into a workflow".
---

# Compile `.evml` Event Model → rote IR

Turn an Event Model (`.evml`) into rote intermediate representation
(`pipeline.yaml`), one pipeline per **bounded context**. This is Phases 1–5
of the rote compiler run as agent work — no `rote compile`, no LLM-driven
compile run, no cost. Emitting runtime code (Temporal, DBOS, …) is a separate,
later step: `rote emit` consumes the IR this skill produces.

**Why:** the domain expert owns the `.evml` (the business process, in
ubiquitous language). The IR is the durable-workflow shape of that same
process. Keeping the two in lock-step — with per-frame provenance so only the
changed slices are re-derived, and semver on the pipeline so in-flight
workflows are never orphaned silently — is the whole point of this skill.

**Authoritative references (read before compiling):**
- `EVENT_MODELING.md` — the `.evml` grammar. `SKILL.md` (repo root) — the
  four patterns, anti-patterns, naming rules.
- `ROTE/rote/skills/rote-compile/SKILL.md` — the seven-phase rote method.
- `ROTE/rote/skills/rote-compile/references/node-kinds.md` — the five node
  kinds.
- `ROTE/rote/skills/rote-compile/references/ir-schema.md` — the exact
  `pipeline.yaml` schema (mirrors `ROTE/rote/src/rote/ir.py`, which wins on
  disagreement).
- `ROTE/rote/skills/rote-compile/references/implementation.md`,
  `crystallization-heuristics.md`, `llm-judge-extraction.md`,
  `eval-estimates.md` — stubs vs working code, `llm_judge` signatures,
  `eval.yaml`.
- `ROTE/rote/examples/bdr-outreach/expected/pipeline.yaml` — a complete
  reference IR to imitate for shape and style.

`ROTE/` is a gitignored sibling checkout. Read it with shell tools (`cat`,
`ls`, `sed`) if the editor's file tools refuse ignored paths. Never edit it.

## Conventions

- **Source models:** `testdata/fixtures/<model>.evml` by default; any `.evml`
  path is accepted. `<model>` is the file stem.
- **Intake snapshot:** `COMPILED/<model>/model.json` — output of
  `go run ./cmd/evml json <file>`. This is the compiler's input, not the raw
  text: it is validated (`Parse` + `ValidateConnections`), carries source
  line numbers, section banners, and the file sha256. Regenerate it at the
  start of every compile; it is committed so a reviewer can diff intakes.
- **IR output:** `COMPILED/<model>/<context>/pipeline.yaml`, one directory
  per bounded context (see "Bounded contexts"). `<context>` is the kebab-case
  slug of the section banner name with the words "bounded context" dropped
  (`// ── Billing bounded context ──` → `billing`). Models with no banners
  and no `rf` frames compile to a single context `main`.
- **Implementation workspace:** `impl: extracted/<context>.py:decide_place_order`
  and `signature: signatures/<node>.py:<Class>` are import targets relative
  to `COMPILED/<model>/<context>/`. The file and symbol must exist.
- **Provenance:** `COMPILED/<model>/<context>/provenance.json` records the
  sha256 of every source **frame section** the context was compiled from
  (frame + its `gwt` blocks + referenced `data` block + `note`s +
  `hotspot`s) and, per node, which frame section it came from. Written by
  `scripts/evml_provenance.py write`; checked by `... check`.
- **Tooling:** Go via `go run ./cmd/evml …`; Python only via
  `mise exec -- uv run …` (never a raw `.venv`). One-off scripts and scratch
  output go under `/tmp` and are deleted afterwards — never into `COMPILED/`.

## Entry Modes

| Invocation | Do this |
|---|---|
| A model is named ("compile `<model>` to IR", a `.evml` path is given) | **Targeted mode** — compile that model's stale/missing contexts (all contexts on first compile). |
| No model named ("recompile event models", "which IR is stale") | **Top-level mode** — scan every `testdata/fixtures/*.evml`, recompile only stale/missing contexts. |

---

## Mapping rules: Event Model → rote IR

These are the load-bearing decisions. Apply them mechanically; record every
deviation in `compile-report.md` §Judgment calls.

### Bounded contexts → pipelines

1. Partition the frames of the model into contexts using, in order of
   preference: (a) the `section` field on each frame in `model.json`
   (section banners); (b) if no banners exist, every `rf` frame starts a new
   context, and frames before the first `rf` form context `main`;
   (c) if the model has neither, everything is `main`.
   Not every banner is a context. Under rule (a):
   - A banner whose name contains **"bounded context"** (case-insensitive)
     defines a context: `// ── Billing bounded context ──` ⇒ `billing`.
   - A banner named `<Context>: <anything>` where `<Context>` matches an
     already-defined context assigns its frames to that context
     (`// ── Operations: no-delay path ──`).
   - Any other banner (`// ── GWT scenarios ──`, `// ── Data blocks ──`) is
     a sub-section: its frames inherit the enclosing context — **unless** its
     first frame is an `rf`, which makes it an automation slice with no
     declared owner. Stop and ask the expert which context owns it
     (flight-arrival's `No-delay path: Operations closes… (Gap-1)` slice,
     `rf 51 Operations.WheelsDownRecorded → pcr OntimeVerifier`, belongs to
     Operations), then ask them to rename the banner to the
     `<Context>: …` form so the next compile needs no question.
2. Under rule (b) the context has no human-given name. **Stop and ask the
   domain expert** for the context names before compiling (propose names
   from the `rf` namespace and the translator processor, e.g.
   `Billing.PaymentDeclined` → `pcr SalesRecoveryTranslator` ⇒ `sales-recovery`).
   Then ask them to add matching `// ── <Name> bounded context ──` banners to
   the `.evml` so the next compile is deterministic.
   When a context spans several banners, pass the context's **defining**
   banner as `--section-name` to `evml_provenance.py write`; frames later
   added under a sub-banner surface in `check` as `unassigned`, which the
   skill treats as stale (see Top-Level Mode).
3. One `pipeline.yaml` per context: `name: <model>-<context>`. Each context
   becomes its own workflow at emission time; contexts talk **only** through
   events crossing an `rf` boundary (recorded in the report's "Cross-context
   links" table — the producing context's exit node and the consuming
   context's pipeline input).

### Frames → nodes

| `.evml` frame | IR | Node id | Notes |
|---|---|---|---|
| `ui` that is the **first** frame of a context and feeds a `cmd` | **pipeline `input`** — no node. `input.type` = `<Cmd>Command`; `input_schema` = JSON Schema inferred from the `cmd` payload. | — | The human's action *is* the workflow start. |
| `ui` **mid-flow** (sourced from an `rmo`, feeding a `cmd`) | `hitl_gate` | `gate_<snake(cmd)>` | `signal: <snake(cmd)>`; the gate's output is the command payload the human submits; the downstream `cmd` node binds `inputs:` to `gate_x.output`. `timeout` from `config.hitl.default_timeout` (7d) unless the model's `note` says otherwise. |
| `cmd` | `pure_function` | `decide_<snake(cmd)>` | The command handler: given prior state + the command, return the resulting event(s) or a rejection event. `impl: extracted/<context>.py:decide_<snake>`. `mandatory: true` when any `gwt` on the frame has a rejection `then`. |
| `evt` | **not a node** — it is the `output` type of the `cmd` node that produces it | — | Output type name = the event name; multiple `then` events from one command (e.g. `PayoutReversed` + `PayoutReissued`) → an output object with one field per event. Success/rejection alternatives → `decision` enum + optional event fields. The event's payload fields become the output JSON Schema. Durability of the event log is the runtime's job (Temporal history); if the expert wants an explicit event store, add one `external_call append_<snake(evt)>` per event and say so in the report. |
| `rmo` | `pure_function` | `project_<snake(rmo)>` | The projection: fold the events it sources into the view. `inputs:` bind to the producing `cmd` nodes' outputs (`decide_x.output`). If the frame has a `data` block, that block is the projection's expected output shape and the golden test's expected value. |
| `pcr` — translator (`*Translator`, sourced from an `rf`) | `pure_function` | `translate_<snake(pcr)>` | Maps the external schema to the internal command payload. Input = the `rf` event (= pipeline input of this context); output = the downstream command payload. |
| `pcr` — deterministic rule/robot (sourced from `evt`/`rmo`, fixed logic) | `pure_function` | `<snake(pcr)>` | Prefer this classification whenever the `gwt`s or payload show the decision is enumerable. |
| `pcr` — calls a named external system / MCP tool | `external_call` | `<snake(pcr)>` | Add `mcp:` **only** when the model names a real MCP server+tool (in a `note` or payload); never invent one. `retry`/`timeout` from notes or defaults (`max: 3`, `exponential`, `60s`). |
| `pcr` — LLM-flavoured (`Agent_*`, `InvokeModel`, "classify", "summarise", free-text payloads, `data` block containing prose) | **Stop and ask** | — | Present the frame, its sources, and the three options: `llm_judge` (bounded typed output — preferred), `agent_loop` (iterative tool use; needs `tools:` + `tool_servers:` + `termination`), or `pure_function` (the expert confirms the decision is actually enumerable). Do not guess. Record the answer in the report and as a `note` suggestion for the `.evml`. |
| `rf` | **pipeline `input`** of the consuming context — no node. `input.type` = the event name; `input_schema` from its payload. | — | In the producing context, the `cmd` node that emits this event is an `exit_node`; record the link in the report. If the same external event is produced by nothing in the model (a truly external system), say so. |
| `data` | JSON Schema for the frame's payload (`$defs/<Name>`) | — | Infer types from the literal (`"…"` → string, `12.5` → number, `[...]` → array, ISO dates → `format: date-time`). |
| `note` | Appended to the node `description`; also the place to look for timeouts, retries, MCP names, HITL channels. If the payload is a key/value block, it becomes the node's `timeout`, `retry`, `mcp`, `hitl`, `eval_set` config. | — | |
| `gwt` | Golden tests for the `decide_*` node + seed examples | — | `given` → prior events (fixture state), `when` → command payload, `then` → expected output. Every `gwt` becomes one test case in `tests/test_<context>.py`; for `llm_judge` nodes it also becomes one `evals/<node>.jsonl` line. |
| `hotspot` | **Blocks the node** — no IR guess | — | An unresolved business question on that frame. Do **not** invent a decision to fill it. Carry it verbatim into `compile-report.md` §Open questions, and set `mandatory: true` on the node so the gap is loud at runtime rather than silently defaulted. If the hotspot makes the node's contract undecidable, stop and ask the expert before emitting the context. |
| `entity` | Report-only | — | Candidate workflow id / correlation key (`entity Cart` ⇒ `cartId`). |

### Runtime-knob convention for `note`

When a `note` payload is a key/value block (rather than prose), the
compiler treats it as the authoritative runtime config for the node it
annotates. Recognised keys:

| Key | Maps to | Example |
|---|---|---|
| `timeout` | `node.timeout` | `timeout: "60s"` or `timeout: "7d"` |
| `retry` | `node.retry` | `retry: { max: 3, backoff: exponential }` |
| `mcp` | `external_call` `mcp:` binding | `mcp: { server: hubspot, tool: hubspot_batch_upsert }` |
| `hitl` | `hitl_gate` channel / signal | `hitl: "#billing-reviews"` |
| `eval_set` | `llm_judge` / `agent_loop` evals path | `eval_set: "evals/vet_contact.jsonl"` |

Unrecognised keys are copied into `constants:` and logged in the report.
This keeps business rules in the `.evml` and runtime tuning in the
`.evml` too, instead of hand-editing `pipeline.yaml`.

### Edges and data flow

- `->>` sources and the auto-inferred nearest-predecessor chain (see
  `EVENT_MODELING.md` §3) become `edges:` between the corresponding nodes.
  Frames that map to *no node* (`ui` entry, `evt`, `rf`) collapse: an edge
  `cmd → evt → rmo` becomes `decide_x → project_y`.
- Bind `inputs:` on every node using only the four reference forms
  (`pipeline.input[.field]`, `<node>.output[.field]`). A `pcr` with several
  `->>` sources binds one parameter per source.
- `entry_nodes` = nodes fed only by the pipeline input; `exit_nodes` =
  terminal `project_*` nodes and any `decide_*` whose event crosses an `rf`
  boundary.
- Never emit `fan_out` unless the payload is explicitly a list processed
  per element (e.g. `passengerIds: [...]` with a per-passenger rule in the
  `gwt`s).

### Node ids are derived from **names, not frame numbers**

`decide_place_order` comes from `cmd PlaceOrder`, never from `tf 03`.
Renumbering frames therefore re-derives the affected nodes (their section
hash changes) but keeps ids stable, so emitted workflow types and in-flight
runs survive. Renaming a command/read model **is** an id change — see
"Versioning".

### Types

Put every event/command/read-model schema under the pipeline's
`input.input_schema.$defs` and each node's `signature_spec`/`input`/`output`
by name. Type names are the `.evml` identifiers (`OrderPlaced`,
`CartSummary`); namespaced names drop the namespace inside the consuming
context (`Sales.OrderPlaced` → `OrderPlaced`) and keep it in the report.

---

## Targeted Mode

Compile exactly one model.

1. **Intake.** Resolve `<model>` → `testdata/fixtures/<model>.evml` (or the
   given path). Run
   ```sh
   mkdir -p COMPILED/<model>
   go run ./cmd/evml json <path>.evml -o COMPILED/<model>/model.json
   ```
   A parse/validation error here is the domain expert's to fix — report it
   with the line number and stop. Read `model.json` fully; also read the
   `.evml` for comments the JSON drops (only banners are preserved).
   Partition into contexts (rules above). Restate each context as a
   sentence ("Billing: on `Sales.OrderPlaced`, authorize payment; emit
   `PaymentAuthorized` or `PaymentDeclined`") before going on.
2. **Staleness.** If `COMPILED/<model>/` already has context directories,
   run the check and compile only `stale`/`missing` contexts:
   ```sh
   mise exec -- uv run --no-project .agents/skills/compile-evml-rote-ir/scripts/evml_provenance.py \
     check COMPILED/<model>/model.json --compiled COMPILED/<model>
   ```
   For a stale context, the `preserved_nodes` list is binding: copy those
   nodes into the new `pipeline.yaml` **verbatim** (same id, same fields)
   unless a neighbour forces an `inputs:`/edge change — explain any such
   deviation in the report. Re-derive only `stale_nodes` plus nodes for
   `added` frames. Do not rewrite `extracted/`/`signatures/` files whose
   nodes are preserved (the expert may have filled them in). `check`
   compares the source section hashes stored in `provenance.json` against
   the current `model.json`; it does **not** diff the live `pipeline.yaml`.
   If the IR was hand-edited (renamed or deleted nodes) without rewriting
   `provenance.json`, the preserved/stale list will still reflect the old
   stamp — fix the model and rerun `write` rather than editing the IR.
3. **Phases 2–5** per context, following the rote references and the
   mapping rules above:
   - Phase 2: classification table (frame → node kind → one-sentence
     justification). Stop for every "ask" row before continuing.
   - Phase 3: create every `extracted/*.py` named by an `impl:`. Default is
     a contract-documented `NotImplementedError` stub whose docstring states
     the input/output contract and lists the `gwt` cases; **but** when the
     `gwt`s fully determine the decision (every branch is a payload
     comparison), implement `decide_*` for real and make the golden tests
     pass — that is ground truth, Regime 2 in `implementation.md`.
     `project_*` with a `data` block likewise gets a real implementation
     when the fold is obvious.
   - Phase 4: every `llm_judge` (only after the expert confirmed it) gets
     `signatures/<node>.py` **and** a `signature_spec` with JSON Schemas,
     plus `evals/<node>.jsonl` seeded from the `gwt`s.
   - Phase 5: assemble `pipeline.yaml` (schema per `ir-schema.md`; start
     from its minimal skeleton). Every node carries
     `source.section: "<kind> <id> <type> <Name>"` exactly as
     `evml_provenance.py hash` prints the keys (e.g. `"tf 03 cmd PlaceOrder"`).
     Do **not** set `content_hash`. Write `eval.yaml` beside it.
   - Never silently drop a rule. A `gwt` you cannot express becomes a
     `mandatory: true` node or an open question — never an omission.
4. **Validate the workspace.**
   ```sh
   mise exec -- uv run --project ROTE/rote python -c \
     "from rote.ir import load_pipeline; load_pipeline('COMPILED/<model>/<context>/pipeline.yaml'); print('OK')"
   ```
   Fix every validation error. Then, from `COMPILED/<model>/<context>/`,
   import every `impl:` and `signature:` target and assert the symbol
   exists; run `tests/` when any implementation is real:
   ```sh
   cd COMPILED/<model>/<context> && mise exec -- uv run --no-project --with pytest --with pydantic pytest tests -q
   ```
   Optional free smoke test of adapter-readiness (writes only to `/tmp`):
   ```sh
   mise exec -- uv run --project ROTE/rote rote emit COMPILED/<model>/<context>/pipeline.yaml \
     --runtime temporal --out /tmp/evml-emit-<context> && rm -rf /tmp/evml-emit-<context>
   ```
   A schema-valid pipeline with a dangling `impl:` is **not** compiled.
5. **Provenance.** For each compiled context:
   ```sh
   mise exec -- uv run --no-project .agents/skills/compile-evml-rote-ir/scripts/evml_provenance.py \
     write COMPILED/<model>/model.json --model <model> --context <context> \
     --section-name "<banner name or ''>" --frames 06,07,08,09,10 \
     --pipeline COMPILED/<model>/<context>/pipeline.yaml \
     --out COMPILED/<model>/<context>/provenance.json
   ```
   It exits non-zero if any node's `source.section` does not name a frame
   in `--frames` — fix the IR, not the tool.
6. **Report.** Write `COMPILED/<model>/<context>/compile-report.md` with:
   node count by kind; the Phase 2 classification table; HITL gates and
   their signals; the crystallization log (frame → `impl:` → stub/working,
   with the `gwt`s that became tests); cross-context links (events
   produced here that are `rf` inputs elsewhere and vice-versa); judgment
   calls and open questions for the expert; and a **Changes** section (see
   Versioning) when this is a recompile. Also (re)write
   `COMPILED/<model>/README.md`: one table `context | pipeline | version |
   input event/command | exit events` so the model-level picture is visible
   without opening each directory.

---

## Top-Level Mode (incremental refresh)

1. **List sources:** every `testdata/fixtures/*.evml`.
2. **Intake each** with `evml json` into `COMPILED/<model>/model.json` (a
   fresh snapshot is required for a correct check; it is cheap).
3. **Classify each context** with `evml_provenance.py check` as
   `missing` / `stale` / `current`. A model with no `COMPILED/<model>/`
   directory at all is `missing` in its entirety. Frames reported as
   `unassigned` mean the partitioning changed — treat the model as stale.
4. **Recompile only missing + stale contexts** via Targeted Mode steps 3–6.
   Leave current contexts untouched (do not even rewrite their reports).
5. **Report a table:** model | context | status | action
   (compiled/skipped) | notes. Say explicitly which contexts were skipped as
   current, so a no-op run is visible rather than silent.

---

## Versioning (keeping the IR in step with the model)

`pipeline.version` is semver, bumped per context on every recompile that
changes the IR, and the reason goes into `compile-report.md` §Changes
(`previous_version → version`, changed/added/removed frame sections,
preserved vs re-derived nodes — the `check` output gives you this list):

| Change in the `.evml` | Bump | Why |
|---|---|---|
| Payload fields, `data` blocks, `gwt`s, `note`s only — no node added/removed/renamed | **patch** | Same workflow shape; stubs/tests/schemas change. |
| New frames → new nodes or edges; new `gwt` adds a branch | **minor** | Additive; in-flight runs unaffected. |
| A frame removed or a `cmd`/`rmo`/`pcr` **renamed** (node id changes), a `ui` moved so a gate appears/disappears, a context split/merged | **major** | Node ids are function/activity names and feed the emitted workflow's type hash — in-flight durable runs of the old version must drain on the old code. Say so in the report. |

Never reuse a node id for a different step. When the expert renames a
command, propose keeping the old id as a deprecated alias only if they
confirm runs are in flight; otherwise take the major bump.

---

## provenance.json format

Written by `scripts/evml_provenance.py write`; documented here so a reader
can verify it by hand.

```json
{
  "schema": "evml-rote-ir-provenance/v1",
  "source_model": "testdata/fixtures/bounded-context-order-fulfillment.evml",
  "source_sha256": "<sha256 of the .evml file>",
  "model": "bounded-context-order-fulfillment",
  "context": "billing",
  "section_name": "Billing bounded context",
  "frames": ["06", "07", "08", "09", "10"],
  "ir": "pipeline.yaml",
  "ir_schema_ref": "ROTE/rote/skills/rote-compile/references/ir-schema.md",
  "compiled_at": "YYYY-MM-DD",
  "pipeline_version": "0.1.0",
  "sections": {
    "rf 06 evt Sales.OrderPlaced": "<sha256>",
    "tf 07 pcr BillingTranslator": "<sha256>"
  },
  "nodes": {
    "translate_billing": { "section": "tf 07 pcr BillingTranslator", "content_hash": "<sha256>" }
  }
}
```

A section hash covers the frame declaration plus every `gwt` anchored to
it, the `data` block it references, and its `note`s and `hotspot`s — with
source line numbers excluded, so reformatting does not churn hashes but
renumbering a frame does (its key and `->>` references change). Resolving
a hotspot therefore marks its context stale, which is the point: the
open question the compiler asked about has been answered.

## Invariants

- **Compiler workspace only.** Emit the IR and its compiler artifacts
  (`extracted/`, `signatures/`, `evals/`, `eval.yaml`, `tests/`,
  `compile-report.md`, `provenance.json`, `model.json`, `README.md`). Never
  write Temporal/DBOS/Cloudflare code into `COMPILED/`; `rote emit` is the
  next step and runs from the IR.
- **The `.evml` is the source of truth for the business; the IR is the
  source of truth for the workflow.** Never "fix" the IR by hand in a way
  the model does not express — change the model (with the expert), then
  recompile. If the compile reveals a modelling gap (a read model no event
  can build, a command with no rejection scenario, a processor whose
  decision is unclear), report it as an open question against the `.evml`,
  citing the frame id.
- **Ask, don't guess, on LLM-flavoured processors and unnamed contexts.**
- **Deterministic over agentic.** `gwt`s are executable specs — use them.
- **Validate before declaring done.** `load_pipeline` passes, every
  `impl:`/`signature:` resolves, tests pass, provenance written.
- **Provenance is mandatory** and always regenerated after the IR changes.

## Related

- `ROTE/rote/src/rote/adapters/temporal.py` — how the IR becomes a Temporal
  workflow (activities per node, signals for gates, class name versioned by
  pipeline hash). Read-only; useful to sanity-check that a context maps to
  a sensible workflow.
- `ROTE/rote/AGENTS.md` — driving the `rote` CLI (`rote emit`, `rote
  analyze --json`) once the IR exists.
- `scripts/evml_provenance.py` — `hash` / `write` / `check`; tests in
  `scripts/tests/`.
