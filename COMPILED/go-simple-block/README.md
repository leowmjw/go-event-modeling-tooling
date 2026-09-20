# Simple Block — Go Temporal

Standalone Temporal implementation of `simple-block` model snapshot `d40f88b7`, pipeline `simple-block-main` v1.0.0.

- Task queue: `simple-block-v1`
- Workflow: `SimpleBlockWorkflowV1`
- IR: [`../simple-block/main/pipeline.yaml`](../simple-block/main/pipeline.yaml)
- Provenance: [`../simple-block/main/provenance.json`](../simple-block/main/provenance.json)
- Compile report: [`../simple-block/main/compile-report.md`](../simple-block/main/compile-report.md)

## Node coverage

| IR node | Go symbol | Shape |
|---|---|---|
| `decide_add_item` | `DecideAddItem` | deterministic decision |
| `project_item_catalog` | `ProjectItemCatalog` | deterministic projection/query state |

Both IR nodes are executed by `Workflow`; the projection is returned and queryable as `item_catalog`. There are no external activity contracts.

## Scenarios

- Valid item emits the complete `ItemAdded` payload and projects the exact catalog data block.
- Non-positive price emits `ItemAddRejected` with `price_must_be_positive`.

## Commands

```sh
mise run doctor
mise run check
mise run demo
mise run start
mise run query
mise run demo:stop
```

## Demo

Start `mise run demo`, run `mise run start` in another terminal, and query the completed workflow with `mise run query`. The starter prints the complete decision and catalog projection.

## Verification

Provenance is checked against the fresh intake before emission. `mise run check` performs formatting, `go mod tidy -diff`, race tests, and vet. This implementation has no Python-emitter parity difference for retries, activities, or HITL because this pipeline contains only deterministic pure functions.
