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
6. [Given-When-Then scenarios (`gwt`)](#6-given-when-then-scenarios-gwt)
7. [Entity declarations (`entity`)](#7-entity-declarations-entity)
8. [Comments](#8-comments)
9. [Identifier rules](#9-identifier-rules)
10. [Payload rules](#10-payload-rules)
11. [Full formal grammar (BNF-style)](#11-full-formal-grammar-bnf-style)
12. [Proposed future extensions (not yet implemented)](#12-proposed-future-extensions-not-yet-implemented)

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
- `payload` — inline `{ ... }` block or quoted string (see §10).

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

- Attaches an annotation to an existing frame.
- Rendered below the swimlane area in a yellow box.

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

## 6. Given-When-Then scenarios (`gwt`)

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

## 7. Entity declarations (`entity`)

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

## 8. Comments

```evml
// single-line comment (C-style)
%% single-line comment (Mermaid-style)
/* multi-line
   comment */
```

All comment styles are ignored by the parser.

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

## 9. Identifier rules

| Role | Pattern | Examples |
|---|---|---|
| Frame ID (`EM_FID`) | 1–3 digits | `01`, `7`, `123` |
| Name / reference (`EM_EID`) | `[_a-zA-Z][\w_]*` | `Cart`, `AddItem01`, `Pending_Questions` |
| Qualified name | `Name(.Name)*` | `Cart.ItemAdded`, `External.Inventory` |

Frame IDs must be **unique** across all `tf`/`rf` declarations in a file.

---

## 10. Payload rules

A payload is data attached to a frame, GWT statement, data block, or note.

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

## 11. Full formal grammar (BNF-style)

```
EventModel  ::= 'eventmodeling' Statement*

Statement   ::= TimeFrame
             |  ResetFrame
             |  DataEntity
             |  NoteEntity
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

## 12. Proposed future extensions (not yet implemented)

Four notation features exist on the eventmodelers.ai cheat sheet that this
DSL has no equivalent for today: **hotspots**, **actor lanes**, **chapters**,
and **slice status tags**. None of these are implemented — this section is a
grammar sketch to work from when they are. Do not treat any syntax below as
valid `.evml` until the parser, `model.go`, and `render.go` are updated to
match, and this section is promoted out of "proposed."

### 12.1 Hotspots — `hotspot`

```
hotspot <frameId> {
  <free-form question or blocker text>
}
```

- Sibling of `note`, but semantically distinct: a hotspot marks an
  **unresolved** question or blocker, not a finished annotation. Rendered as
  a red sticky (🔴) rather than `note`'s yellow.
- A new `EntityStatus`-style flag, not an `EntityType` — it attaches to a
  frame the same way `note` does, so no changes to `allowedSources` are
  needed.
- Enables a `evml lint --hotspots` (or `--strict`) mode that exits non-zero
  if any hotspot remains, so "all open questions resolved" becomes a CI gate
  instead of a convention nobody checks.

### 12.2 Actor lanes — `actor` + `@ActorName`

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

### 12.3 Chapters — `chapter`

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

### 12.4 Slice status tags — `status`

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

### What these unlock — 3 scenarios not modelable today

**Scenario 1 — Hotspots: making unresolved rules impossible to lose.**
Today, an open question like *"what happens if two guests book the same
room simultaneously?"* can only be captured as a `//` comment — which the
parser ignores, which never renders, and which nothing can enforce. With
`hotspot 06 { concurrent booking on the same room: last write wins, or
reject? }` attached to `tf 06 cmd BookRoom`, the question is visible in the
SVG and queryable by tooling. A CI gate (`evml lint --hotspots`) can then
block a merge until every hotspot is either resolved (converted to a `note`
or removed) or explicitly accepted — turning "we forgot to decide this"
from a silent failure mode into a build failure.

**Scenario 2 — Actor lanes: seeing who does what without reading labels.**
`what-is-event-modeling.evml` mixes guest self-service (`SearchRoomsScreen`,
`BookRoomScreen`) with staff-operated screens (`CheckInDesk`,
`CheckOutDesk`) in the same UI swimlane — today you can only tell them apart
by reading each frame's name. Tagging `tf 09 ui CheckInDesk @FrontDeskStaff`
vs. `tf 01 ui SearchRoomsScreen @Guest` and rendering a colour-coded actor
band makes the guest/staff split immediately visible, which matters for
staffing and training conversations, and surfaces the "Bed" anti-pattern
per-actor (e.g. "FrontDeskStaff fires four unrelated commands from one
screen").

**Scenario 3 — Chapters + slice status: a build tracker that lives in the
diagram.** `flight-arrival-post-flight-settlement.evml` is 55 frames across
four bounded contexts (Operations → Compensation → Finance → Marketing);
today that boundary structure exists only as a `//` comment header nobody
can query. Wrapping each context in a `chapter` with named `slice`s inside —
`slice "Evaluate delay" 09-11 status Done`, `slice "Escalate unresolved
claim" 36-39 status InProgress` — turns the model into a live progress view:
which slices are shipped, which are in review, which are blocked. This
closes the gap between "the diagram" and "the sprint board" instead of
requiring both to be maintained separately and kept in sync by hand.
