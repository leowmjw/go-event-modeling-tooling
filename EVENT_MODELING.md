# EVENT_MODELING.md — `.evml` DSL Reference

> Grammar derived from the official Langium grammar at
> `lgazo/event-modeling-tools` and cross-checked against every fixture in
> `testdata/fixtures/`.  Do **not** change the DSL without updating this file.

---

## Table of contents

1. [File structure](#1-file-structure)
2. [Entity types & SVG colours](#2-entity-types--svg-colours)
3. [Frame declarations (`tf` / `rf`)](#3-frame-declarations-tf--rf)
4. [Data blocks (`data`)](#4-data-blocks-data)
5. [Notes (`note`)](#5-notes-note)
6. [Hotspots (`hotspot`)](#6-hotspots-hotspot)
7. [Given-When-Then scenarios (`gwt`)](#7-given-when-then-scenarios-gwt)
8. [Entity declarations (`entity`)](#8-entity-declarations-entity)
9. [Comments](#9-comments)
10. [Identifier rules](#10-identifier-rules)
11. [Payload rules](#11-payload-rules)
12. [Full formal grammar (BNF-style)](#12-full-formal-grammar-bnf-style)
13. [Proposed future extensions (not yet implemented)](#13-proposed-future-extensions-not-yet-implemented)

---

## 1. File structure

Every `.evml` file **must** start with the literal keyword `eventmodeling` on
its own line (no leading whitespace).  Everything that follows is optional and
order-independent, though conventional ordering is:

```
eventmodeling

// frames (left to right, chronologically)
tf 01 ui  ...
tf 02 cmd ...
tf 03 evt ...
...

// data blocks (referenced by frames)
data SomeName { ... }

// notes (annotations on frames)
note 02 { ... }

// GWT scenarios
gwt 03 "scenario label"
  given ...
  when  ...
  then  ...
```

---

## 2. Entity types & SVG colours

| Keyword(s) | Meaning | SVG fill | SVG stroke | Swimlane band |
|---|---|---|---|---|
| `ui` · `scn` · `screen` | Wireframe / UI screen | `#f8d4bc` salmon | `#d38e5f` | UI/Automation |
| `cmd` · `command` | Command / intention | `#bcd6fe` blue | `#679ac3` | Command/Read Model |
| `evt` · `event` | Domain event | `#d3f1a2` green | `#84af49` | Events |
| `rmo` · `readmodel` | Read model / projection | `#bcd6fe` blue | `#679ac3` | Command/Read Model |
| `pcr` · `processor` | Automation / processor | `#f8d4bc` salmon | `#d38e5f` | UI/Automation |

---

## 3. Frame declarations (`tf` / `rf`)

### Time frame

```
tf <id> <type> <Name> [->> <sourceId>]* [[[dataRef]]]? [payload]?
```

- `tf` (or `timeframe`) — a regular step in the timeline.
- `<id>` — 1-3 digit numeric identifier (e.g. `01`, `7`, `123`).
- `<type>` — one of the entity type keywords from §2.
- `<Name>` — a qualified identifier: `PascalCase` or `Namespace.Name`.
- `->> <sourceId>` — explicit source frame(s); can be repeated for multiple
  sources.  When omitted the renderer infers the nearest compatible predecessor.
- `[[dataRef]]` — reference to a `data` block by its `EM_EID` name.
- `payload` — inline `{ ... }` block or quoted string (see §11).

### Reset frame

```
rf <id> <type> <Name> [->> <sourceId>]* [[[dataRef]]]? [payload]?
```

- `rf` (or `resetframe`) — marks a boundary; automatic edge inference stops
  here.  Use when an external event enters the system or a new bounded context
  begins.

### Examples

```evml
// minimal
tf 01 ui CartScreen

// with inline payload
tf 02 cmd AddToCart { id: "p-1" }

// event references a command as its source
tf 03 evt CartUpdated ->> 02

// read model references a data block
tf 04 rmo RoomList [[RoomList04]]

// multiple explicit sources
tf 07 pcr Agent_A ->> 05 ->> 06

// reset frame — external event enters the system
rf 04 evt External.InventoryChanged

// qualified name (Namespace.Event)
tf 05 evt Cart.ItemAdded
```

---

## 4. Data blocks (`data`)

```
data <Name> [`<dataType>`]? {
  <free-form content>
}
```

- `<Name>` — matches `EM_EID` (starts with letter/underscore, alphanumeric).
- Optional backtick-quoted data type hint: `json` | `jsobj` | `figma` | `salt`
  | `uri` | `md` | `html` | `text`.
- Body is free-form text between balanced `{ }`.  Can span multiple lines.
- Referenced from frames with `[[Name]]`.

### Examples

```evml
data AddItem01 {
  description: 'john'
  image: 'avatar_john'
  price: 20.4
}

data CartUpdated02 {
  items: [
    { id: "p-1", qty: 1 }
  ]
}

data Note01 `md` {
  # Heading
  Some **markdown** content.
}
```

---

## 5. Notes (`note`)

```
note <frameId> [`<dataType>`]? {
  <content>
}
```

- Attaches a **settled** annotation to an existing frame — a decision that has
  been made. For an *unresolved* question use `hotspot` (§6) instead.
- Rendered below the swimlane area in a yellow box.
- May also carry **runtime-knob config** for the frame when the payload is a
  key/value block: `timeout`, `retry`, `mcp`, `hitl`, `eval_set`. See
  `.agents/skills/compile-evml-rote-ir/SKILL.md` §"Runtime-knob convention
  for `note`".

### Example

```evml
note 02 `md` {
  # head 1
  this is a markdown note
}

note 05 {
  This is whatever <b>you</b> want
  On multiple lines
}
```

---

## 6. Hotspots (`hotspot`)

```
hotspot <frameId> [`<dataType>`]? {
  <free-form question or blocker text>
}
```

- Syntactically a sibling of `note` (§5) — same frame reference, same optional
  backtick type hint, same payload rules — but semantically the opposite: a
  hotspot marks an **unresolved** question or blocker, not a settled decision.
- Rendered below the GWT scenarios in a **red** box (`#ffcccc` fill,
  `#c95c5c` stroke) so open questions are visually distinct from `note`'s
  yellow.
- Multiple hotspots may reference the same frame.
- Surfaced in `evml json` under `hotspots`, and included in the
  `compile-evml-rote-ir` provenance hash for its frame — so **resolving a
  hotspot marks the compiled context stale** and forces the affected node to
  be re-derived.

### Example

```evml
hotspot 06 {
  concurrent booking on the same room: last write wins, or reject?
}

hotspot 06 `md` {
  Also unresolved: does an **overbooking** emit a rejection event, or is it
  silently queued for the front desk?
}
```

---

## 7. Given-When-Then scenarios (`gwt`)

```
gwt <frameId> ["label"]?
  given
    <statement>+
  [when
    <statement>+]?
  then
    <statement>+
```

- `<frameId>` — references the frame this scenario is associated with.
- `label` — optional quoted string (single or double quotes).
- `given` and `then` are required; `when` is optional.
- Each `<statement>` is indented and has the form:

```
  <type> <Name> [payload]?
```

- Payloads in statements can be inline `{ ... }` (single-line or multi-line)
  or a backtick-typed block (same rules as frame payloads).
- Multiple `gwt` blocks can reference the **same** frame (multiple scenarios
  for one command/event).

### Examples

```evml
gwt 01 "happy path"
  given
    evt CartCreated
  when
    cmd AddToCart { id: "p-1" }
  then
    evt CartUpdated { items: [ { id: "p-1", qty: 1 } ] }

gwt 01 "duplicate add increments qty"
  given
    evt CartUpdated { items: [ { id: "p-1", qty: 1 } ] }
  when
    cmd AddToCart { id: "p-1" }
  then
    evt CartUpdated { items: [ { id: "p-1", qty: 2 } ] }

// when is optional (state-change only)
gwt 02 'audit'
  given
    evt CartUpdated
  then
    rmo CartReadModel

// multi-line nested payload
gwt 03 "nested payload"
  given
    evt Start `jsobj` {
      a: {
        b: { c: 1 }
      }
    }
  then
    evt Done { result: { ok: true } }
```

---

## 8. Entity declarations (`entity`)

```
entity <Name>
```

- Declares a named domain entity (e.g. an aggregate root).
- Used for documentation / tooling; not rendered in the SVG by default.

### Example

```evml
entity Cart
entity Hotel.Room
```

---

## 9. Comments

```evml
// single-line comment (C-style)
%% single-line comment (Mermaid-style)
/* multi-line
   comment */
```

All comment styles are ignored by the parser.

**Section banners.** A comment line of the form `// ── <Name> ──────` (one or
more `─` U+2500 box-drawing dashes on each side of a name) is still ignored by
the grammar, but tooling records it as a *section boundary*: `evml json` lists
it under `sections` and tags every following frame with `section: "<Name>"`.
The `compile-evml-rote-ir` skill uses these banners as bounded-context
boundaries when splitting a model into per-context `pipeline.yaml` IR.

> **Parser restriction:** comments are only valid at the *top level* — between
> top-level declarations (`tf`, `rf`, `data`, `gwt`, etc.).  Do **not** place
> a comment line between two `gwt` blocks or anywhere inside a `gwt` block body.
> The parser will attempt to parse the comment as a frame declaration and emit
> `unknown entity type "//"`.  Annotate `gwt` blocks with label strings instead:
>
> ```evml
> gwt 12 "happy path — all passengers eligible"
>   given ...
>   then  ...
>
> gwt 12 "edge case — passenger already compensated"
>   given ...
>   then  ...
> ```

---

## 10. Identifier rules

| Role | Pattern | Examples |
|---|---|---|
| Frame ID (`EM_FID`) | 1–3 digits | `01`, `7`, `123` |
| Name / reference (`EM_EID`) | `[_a-zA-Z][\w_]*` | `Cart`, `AddItem01`, `Pending_Questions` |
| Qualified name | `Name(.Name)*` | `Cart.ItemAdded`, `External.Inventory` |

Frame IDs must be **unique** across all `tf`/`rf` declarations in a file.

---

## 11. Payload rules

A payload is data attached to a frame, GWT statement, data block, note, or
hotspot.

### Inline `{ ... }`

```
{ <any text, balanced braces> }
```

- Can contain nested `{ }` as long as braces are balanced.
- Quoted strings inside (`"..."` or `'...'`) may contain unbalanced braces.
- Single-line: `{ id: "p-1", qty: 1 }`
- Multi-line: opening `{` may be on the same line as the frame declaration;
  closing `}` on its own line.

### Quoted string

```
"text"  or  'text'
```

Backslash escapes are honoured: `\"`, `\'`, `\\`.

### Optional data type hint

Any payload (inline or block) may be prefixed with a backtick type tag:

```
`json` { ... }   `md` { ... }   `jsobj` { ... }
```

Supported types: `json`, `jsobj`, `figma`, `salt`, `uri`, `md`, `html`, `text`.

---

## 12. Full formal grammar (BNF-style)

```
EventModel  ::= 'eventmodeling' Statement*

Statement   ::= TimeFrame
             |  ResetFrame
             |  DataEntity
             |  NoteEntity
             |  HotspotEntity
             |  GWT
             |  EntityDecl

TimeFrame   ::= ('tf'|'timeframe') FrameId EntityType QualifiedName
                SourceRef* DataRef? Payload?

ResetFrame  ::= ('rf'|'resetframe') FrameId EntityType QualifiedName
                SourceRef* DataRef? Payload?

SourceRef   ::= '->>' FrameId

DataRef     ::= '[[' EID ']]'

DataEntity  ::= 'data' EID TypeHint? DataBlock

NoteEntity  ::= 'note' FrameId TypeHint? DataBlock

HotspotEntity ::= 'hotspot' FrameId TypeHint? DataBlock

GWT         ::= 'gwt' FrameId QuotedString?
                'given' GWTStatement+
                ('when' GWTStatement+)?
                'then' GWTStatement+

GWTStatement::= EntityType QualifiedName Payload?

EntityDecl  ::= 'entity' QualifiedName

EntityType  ::= 'ui'|'scn'|'screen'|'cmd'|'command'
             |  'evt'|'event'|'rmo'|'readmodel'|'pcr'|'processor'

Payload     ::= TypeHint? (DataBlock | InlineBlock | QuotedString)

DataBlock   ::= '{' (multi-line, balanced) '}'
InlineBlock ::= '{' (single-line, balanced) '}'
QuotedString::= '"' .* '"' | "'" .* "'"

TypeHint    ::= '`' DataType '`'
DataType    ::= 'json'|'jsobj'|'figma'|'salt'|'uri'|'md'|'html'|'text'

QualifiedName ::= EID ('.' EID)*
FrameId     ::= [0-9]{1,3}
EID         ::= [_a-zA-Z][_\w]*
```

---

## 13. Proposed future extensions (not yet implemented)

Of the four notation features on the eventmodelers.ai cheat sheet that this
DSL originally lacked, **hotspots are now implemented** (§6). Three remain
proposed: **actor lanes**, **chapters**, and **slice status tags**. The
sections below are grammar sketches only — do not treat any syntax below as
valid `.evml` until the parser, `model.go`, and `render.go` are updated to
match and the feature is promoted out of this section.

Still outstanding for hotspots: an `evml lint --hotspots` (or `--strict`)
mode that exits non-zero while any hotspot remains, so "all open questions
resolved" becomes a CI gate rather than a convention nobody checks.

### 13.1 Actor lanes — `actor` + `@ActorName`

```
actor <Name>
...
tf <id> ui <QualifiedName> @<ActorName> [payload]?
```

- `actor Guest`, `actor FrontDeskStaff` declare personas up front (parallel
  to `entity`).
- `@ActorName` is an optional suffix on `ui` (and possibly `pcr`, for
  automated "actors") frames — orthogonal to `EntityType`, so it doesn't
  interact with `allowedSources` either.
- Rendering adds a **secondary vertical banding** across the UI swimlane,
  colour-coded per actor — independent of the existing entity-type
  swimlanes, which stay horizontal.

### 13.2 Chapters — `chapter`

```
chapter <Name> {
  <frameId>-<frameId>
}
```

or, more simply, a range attached directly to a declaration:

```
chapter "Operations" 01-07
chapter "Compensation" 08-21
```

- Purely a rendering/navigation concern: a **wide labelled arrow or bracket**
  spanning the given frame-ID range, drawn above the swimlanes. No effect on
  parsing semantics of the frames themselves.
- Frame ranges must be non-overlapping and reference declared `tf`/`rf` IDs;
  validated the same way `->>` source IDs are today (existence check only,
  in `ValidateConnections` or a sibling `ValidateChapters`).
- Turns the `//` section-comment convention already used in fixtures like
  `flight-arrival-post-flight-settlement.evml` (bounded-context banners) into
  something that actually renders, instead of living only in source comments.

### 13.3 Slice status tags — `status`

```
slice <Name> [<startFrameId>-<endFrameId>] status <StatusKeyword>
```

Where `StatusKeyword` ∈ `Created | Planned | Assigned | InProgress | Review
| Done | Blocked | Informational`.

- A `slice` is the vertical cut already described conceptually in `SKILL.md`
  §"Slices & Scenarios" — this gives it an explicit DSL declaration instead
  of being an implicit grouping.
- Status renders as a small badge on the slice's frame range; `Blocked`
  could additionally render a red border to align visually with hotspots.
- Natural pairing with **chapters**: a chapter groups multiple named slices,
  each with its own status, giving a build-progress view without leaving
  the model.

### What these unlock — 2 scenarios not modelable today

> **Already delivered by §6 hotspots:** an open question like *"what happens
> if two guests book the same room simultaneously?"* used to be capturable
> only as a `//` comment — ignored by the parser, never rendered, enforceable
> by nothing. `hotspot 06 { … }` on `tf 06 cmd BookRoom` now renders in the
> SVG, appears in `evml json`, and marks the compiled IR context stale when
> resolved. The remaining gap is the `evml lint --hotspots` CI gate.

**Scenario 1 — Actor lanes: seeing who does what without reading labels.**
`what-is-event-modeling.evml` mixes guest self-service (`SearchRoomsScreen`,
`BookRoomScreen`) with staff-operated screens (`CheckInDesk`,
`CheckOutDesk`) in the same UI swimlane — today you can only tell them apart
by reading each frame's name. Tagging `tf 09 ui CheckInDesk @FrontDeskStaff`
vs. `tf 01 ui SearchRoomsScreen @Guest` and rendering a colour-coded actor
band makes the guest/staff split immediately visible, which matters for
staffing and training conversations, and surfaces the "Bed" anti-pattern
per-actor (e.g. "FrontDeskStaff fires four unrelated commands from one
screen").

**Scenario 2 — Chapters + slice status: a build tracker that lives in the
diagram.** `flight-arrival-post-flight-settlement.evml` is 55 frames across
four bounded contexts (Operations → Compensation → Finance → Marketing);
today that boundary structure exists only as a `//` comment header nobody
can query. Wrapping each context in a `chapter` with named `slice`s inside —
`slice "Evaluate delay" 09-11 status Done`, `slice "Escalate unresolved
claim" 36-39 status InProgress` — turns the model into a live progress view:
which slices are shipped, which are in review, which are blocked. This
closes the gap between "the diagram" and "the sprint board" instead of
requiring both to be maintained separately and kept in sync by hand.
