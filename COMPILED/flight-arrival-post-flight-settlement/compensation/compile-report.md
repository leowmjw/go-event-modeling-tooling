# Compile report: flight-arrival-post-flight-settlement / compensation

Version 1.0.0. Eleven deterministic nodes cover delay evaluation, eligibility, calculation, claim projection, on-time closure, and partial-disbursement recovery. All command branches are GWT tests; `DelayClaimSummary16`, closed-claim, and escalation projections are executable. Inputs are `Operations.GateOpened`, `Operations.FlightClosedOnTime`, and `Finance.PartialDisbursementRejected`; `CompensationCalculated` exits to Finance. Frames 35–39 are explicitly owned by `Compensation: disbursement recovery`. No HITL, LLM, external call, or open question remains.

## Changes

`0.1.0 → 1.0.0`: moved recovery ownership from Finance, added complete typed flows, replaced eleven stubs, and rebuilt edges/bindings. The context ownership and workflow shape changes require a major version.
