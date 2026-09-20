# Compile report: flight-arrival-post-flight-settlement / finance

Version 1.0.0. Seven deterministic nodes cover Finance translation, full/partial/rejected approval, payout issue, settlement projection, and payout reversal/reissue. Exact GWT payloads include the compound `PayoutReversed` + `PayoutReissued` result. `FinanceSettlementReport28` and payout recovery are exact projection tests. Input `Compensation.CompensationCalculated` produces `PayoutIssued` for Marketing; partial rejection feeds Compensation recovery. No HITL, LLM, external call, or open question remains.

## Changes

`0.1.0 → 1.0.0`: removed incorrectly owned Compensation recovery nodes, replaced seven stubs, and rebuilt typed edges/bindings. The incompatible shape requires a major version.
