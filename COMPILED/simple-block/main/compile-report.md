# Compile report: simple-block / main

## Summary

Pipeline `simple-block-main` v1.0.0 contains two deterministic pure-function nodes.

| Frame | Node | Kind | Classification |
|---|---|---|---|
| `tf 02 cmd AddItem` | `decide_add_item` | `pure_function` | GWTs fully define success and non-positive-price rejection. |
| `tf 04 rmo ItemCatalog` | `project_item_catalog` | `pure_function` | The data block exactly defines the `ItemAdded` projection. |

## Crystallization

Both implementations in `extracted/main.py` are working code. The two frame-02 GWTs are executable tests; `ItemCatalog04` is asserted exactly in the success test.

## Cross-context links

None.

## HITL gates

None; the initial UI command starts the workflow.

## Judgment calls and open questions

Item IDs are command inputs rather than workflow-generated values, preserving determinism. No open questions remain.

## Changes

`0.1.0 → 1.0.0`: added the UI, result event, catalog projection, success/rejection GWTs, real implementations, and executable data flow. This replaces the prior single stub and changes the workflow shape, requiring a major version.
