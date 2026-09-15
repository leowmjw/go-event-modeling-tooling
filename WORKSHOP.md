# WORKSHOP.md — Running an event modeling session with domain experts

This guide is for the facilitator. It assumes `mise run webapp:nollm` (or
`mise run webapp`) is running on a laptop plugged into the room's screen, and
that nobody in the room needs to know the `.evml` notation.

---

## 1. Before the session

- **Pick the flow.** Open an existing flow (the process *as it runs today*) or
  start a new one. A new flow begins empty; that is fine for a brainstorm.
- **Set the lens to "As-is".** Start every session from reality. Proposals
  come later.
- **Invite the right people**: those with questions, those with answers, and
  you. Two to six domain experts is the sweet spot.
- **Health check** (Compare tab): see which commands have no scenario and
  which questions are still open from last time. Those are your first topics.

## 2. The six workshop steps, mapped to the app

| Cheat-sheet step | What you do in the app |
|---|---|
| **Brainstorm** — "what happened?" | Add steps of type **Event** only, in any order, as *staging*. Past tense: `PaymentSettled`, not `SettlePayment`. |
| **The plot** — order in time | Drag is not needed: use *Insert after* when adding, or remove and re-add. Ask "what's first?" and "what's last?". |
| **Storyboard** — who sees what | Add **Screens** with an actor (`@Customer`, `@Analyst`) in front of each command. |
| **Input / output** | Add the **Command** before each event and the **Read model** after. A read model must answer *one* question; if you can't name it, don't add it. |
| **Swimlanes / contexts** | Mark **Chapters** for bounded contexts. Where another system's event enters, add it as an *external event* (Advanced → tick the box) followed by an **Automation** that translates it. |
| **Scenarios** | On each command: happy path first, then "is there any rule we didn't cover?" — three times. Rejections are events too (`TopUpRejected { reason }`). |

## 3. As-is, staging, future

Every step and slice has a **stage**:

| Stage | Say this in the room | Looks like |
|---|---|---|
| **Current** | "This is how it works today." | Solid |
| **Staging** | "We think it should work like this — let's check it against reality." | Amber dashed, `STAGING` badge |
| **Future** | "We want this eventually; parking it so we don't lose it." | Grey dotted, faded |

New steps default to **staging**. When the room agrees a staging step matches
reality (or has been built), click **current** on it. When a proposal turns out
to be a longer-term goal, click **future**. The **lens** buttons hide future,
or future and staging, so the diagram on the wall never mixes wish and fact
without everyone knowing.

## 4. Open questions (hotspots)

When the room stalls — a disputed rule, an unknown threshold, "we need Legal
for that" — **park it**: Questions tab → pick the step → write the question.
It renders as a red sticky and a red count on the step, and it will still be
there next session. Never guess.

Resolve it later by **recording the decision** (it becomes a note on the model:
"Q: … / Decision: …") or **drop** it if it no longer applies.

## 5. Drafts, compare, promote

- Each session works in a **draft** (dated, numbered). Fork a new draft when
  you want to try a different shape without losing the current one.
- **Compare** shows what changed versus the saved baseline: steps added,
  removed, renamed, stage changes, scenarios and questions opened or resolved.
  Read it aloud at the end of the session; it is the meeting summary.
- **Promote to baseline** writes the draft into `testdata/fixtures/<flow>.evml`.
  Do this when the as-is part is agreed. Staging and future stay in the file;
  they are part of the shared picture.
- **Export** the `.evml` or `.svg` for the meeting notes.

## 6. Facilitation rules that survive contact with reality

1. Model behaviour, not data structures. If someone says "we need a table",
   ask "what happened just before?".
2. One decision = one command + one event. If a command "checks eligibility
   and calculates the fee", split it.
3. Every command needs at least two scenarios before you call it understood.
4. `*Sent` is not an outcome. What does the recipient do next?
5. When an outside system answers late or never (scheme timeout, ACH return,
   arbitration), model that path now; it is where the money is lost.
6. Regulatory deadlines are read models ("how much time is left?") watched by
   an automation that acts when the clock lapses.
7. Park, don't argue. A red hotspot is progress; a twenty-minute debate isn't.
8. End with the Compare tab and the list of open questions. Assign owners.

## 7. Command line equivalents

```bash
evml lint  <file> --strict     # fail while any question is open — a CI gate for "decided"
evml svg   <file> --stage current,staging   # print-out of what we intend to build next
evml diff  <before> <after>    # what changed between two sessions
```
