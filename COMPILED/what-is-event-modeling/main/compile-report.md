# Compile report: what-is-event-modeling / main

Pipeline `what-is-event-modeling-main` v1.0.0 contains ten deterministic functions and three HITL gates (`book_room`, `check_in`, `check_out`). All ten GWT scenarios are executable and all four data blocks have exact projection assertions.

## Classification

Commands map to `decide_*` pure functions; read models map to `project_*`; `BillingProcessor` is a deterministic translation; the three mid-flow UIs are durable HITL gates. No external or LLM nodes exist.

## Cross-context links

None.

## Judgment calls

Room inventory is represented by the complete `RoomsSearched` event. Booking IDs and dates are command inputs. Checkout carries nights, rate, and amount, allowing billing and invoice projection without hidden I/O.

## Open questions

None.

## Changes

`0.1.0 → 1.0.0`: completed payload flow, added rejection GWTs, replaced ten stubs, corrected all edges/bindings, and renamed nodes to stable snake-case IDs. The incompatible workflow-shape change requires a major version.
