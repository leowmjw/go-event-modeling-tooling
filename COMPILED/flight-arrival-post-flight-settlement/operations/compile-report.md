# Compile report: flight-arrival-post-flight-settlement / operations

Version 1.0.0. Seven `pure_function` nodes map frames 02, 03, 05, 07, 52, 53, and 55. GWTs cover wheels-down, gate-arrival, and on-time closure success/rejection. `FlightStatus02`, `ArrivalBoard07`, and the closure record are exact projection tests. Cross-context exits are `GateOpened` and `FlightClosedOnTime`; input `Operations.WheelsDownRecorded` drives the on-time automation slice. No HITL, LLM, external call, or open question remains.

## Changes

`0.1.0 → 1.0.0`: replaced seven stubs, corrected bindings/edges, and renamed node IDs to snake case. This incompatible workflow shape requires a major version.
