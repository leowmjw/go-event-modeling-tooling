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
9. [Chapters (`chapter`)](#9-chapters-chapter)
10. [Slices (`slice`)](#10-slices-slice)
11. [Comments](#11-comments)
12. [Identifier rules](#12-identifier-rules)
13. [Payload rules](#13-payload-rules)
14. [Full formal grammar (BNF-style)](#14-full-formal-grammar-bnf-style)
15. [Proposed future extensions (not yet implemented)](#15-proposed-future-extensions-not-yet-implemented)

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

## 6. Hotspots (`hotspot`)

```
hotspot <frameId> [`<dataType>`]? [status <open|resolved>]? {
  <free-form question or blocker text>
}
```

- A hotspot is a sibling of `note`, but semantically distinct: it marks an
  **unresolved** question the workshop owes an answer to, not a finished
  annotation. Rendered with a **red pin** in the top-right corner of the
  parent frame, and a red sticky-note-style block below the swimlane.
- Use `hotspot` whenever a discussion surfaces "what happens when…?"
  or "operations wants X, compliance wants Y" — anything that would
  otherwise live as a `//` comment and get forgotten.
- Optional `status resolved` keyword marks the hotspot as answered. The
  pin turns grey and the sticky becomes dimmed.
- Validation: every hotspot's `frameId` must reference a declared frame.

### Example

```evml
hotspot 12 {
  At DTI > 43%, do we decline or offer reduced principal? Credit policy
  differs from product team expectation.
}

hotspot 08 status resolved {
  Sanctions list version: previously daily OFAC; now real-time.
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

## 9. Chapters (`chapter`)

```
chapter "<Name>" <startFrameId>-<endFrameId>
```

- A chapter is a labelled, contiguous frame range that represents a
  **bounded context** across the timeline (e.g. "Operations",
  "Compensation", "Finance"). Rendered as a coloured band across the top
  of the diagram, with the frame-ID range printed on the right edge.
- Validation: chapter `startFrameId` and `endFrameId` must both exist;
  the end must be at or after the start; chapters must not overlap one
  another (slices, in contrast, may freely overlap — they model
  build-stage decomposition).
- Use `chapter` to replace the `// ── Operations bounded context ──`
  banner convention that previously lived only in source comments and
  rendered as nothing.

### Example

```evml
chapter "Operations"    01-07
chapter "Compensation"  08-21
chapter "Finance"       22-39
chapter "Marketing"     40-55
```

---

## 10. Slices (`slice`)

```
slice "<Name>" <startFrameId>-<endFrameId> status <StatusKeyword>
```

`<StatusKeyword>` ∈ `Created | Planned | Assigned | InProgress | Review
| Done | Blocked | Informational`.

- A slice is a named, framed range of the timeline with an explicit build
  status — the answer to "what will ship in MVP, what is next, what is
  speculative?". Rendered as a coloured bar below the swimlanes with
  the status keyword as a small badge.
- Status keywords are case-sensitive and PascalCase; unknown keywords
  fail to parse.
- Slices **may** overlap. The same frame can belong to "Auth + capture"
  (Done) and to "Refund flow" (Planned) at the same time — the read
  model is "build stage per feature, not a partition".
- A chapter groups multiple named slices naturally; a slice is finer
  than a chapter.

### Example

```evml
slice "Happy-path instant approval"  01-20 status Done
slice "Manual review queue"          12-19 status InProgress
slice "PEP / sanctions escalation"   09-19 status Planned
slice "Re-KYC at 12 months"          22-30 status Informational
```

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

## 11. Comments

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

## 12. Identifier rules

| Role | Pattern | Examples |
|---|---|---|
| Frame ID (`EM_FID`) | 1–3 digits | `01`, `7`, `123` |
| Name / reference (`EM_EID`) | `[_a-zA-Z][\w_]*` | `Cart`, `AddItem01`, `Pending_Questions` |
| Qualified name | `Name(.Name)*` | `Cart.ItemAdded`, `External.Inventory` |

Frame IDs must be **unique** across all `tf`/`rf` declarations in a file.

---

## 13. Payload rules

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

## 14. Full formal grammar (BNF-style)

```
EventModel  ::= 'eventmodeling' Statement*

Statement   ::= TimeFrame
             |  ResetFrame
             |  DataEntity
             |  NoteEntity
             |  HotspotEntity
             |  Chapter
             |  Slice
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

HotspotEntity::= 'hotspot' FrameId TypeHint? ('status' ('open'|'resolved'))? DataBlock

Chapter     ::= 'chapter' QuotedString FrameId '-' FrameId

Slice       ::= 'slice' QuotedString FrameId '-' FrameId
                'status' SliceStatus

SliceStatus ::= 'Created'|'Planned'|'Assigned'|'InProgress'
             |  'Review'|'Done'|'Blocked'|'Informational'

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

## 15. Proposed future extensions (not yet implemented)

One notation feature remains on the eventmodelers.ai cheat sheet that this
DSL has no equivalent for today: **actor lanes**. None of the others
(hotspots, chapters, slice status) are unimplemented any more — they live
in §6, §9, §10 above.

### 15.1 Actor lanes — `actor` + `@ActorName`

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
