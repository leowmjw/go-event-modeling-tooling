# Flight Arrival & Post-Flight Settlement — Go Temporal

Standalone implementation of four v1.0.0 pipelines. Task queue: `flight-settlement-v1`.

## Workflows

- Parent: `FlightSettlementWorkflowV1`
- Children: `OperationsWorkflowV1`, `CompensationWorkflowV1`, `FinanceWorkflowV1`, `MarketingWorkflowV1`
- Signal: `respond_to_recovery_offer`
- Queries: `marketing_campaign_dashboard`, `marketing_conversion_report`

Authoritative IR and provenance remain in [`../flight-arrival-post-flight-settlement/`](../flight-arrival-post-flight-settlement/).

## Node coverage

| Context | IR node IDs | Go symbols / shape |
|---|---|---|
| Operations | `project_flight_status`, `decide_record_wheels_down`, `decide_record_gate_arrival`, `project_arrival_board`, `translate_ontime_verifier`, `decide_close_flight_on_time`, `project_flight_closure_record` | Same PascalCase symbols in `decisions.go`; deterministic decisions/projections executed by `OperationsWorkflow` |
| Compensation | `translate_compensation_processor`, `decide_evaluate_delay`, `decide_verify_passenger_eligibility`, `decide_calculate_compensation`, `project_delay_claim_summary`, `translate_compensation_closure_processor`, `decide_close_compensation_case`, `project_closed_claim_record`, `translate_compensation_recovery`, `decide_escalate_unresolved_claim`, `project_escalated_claims_queue` | Same PascalCase symbols; deterministic decisions/projections executed by `CompensationWorkflow` |
| Finance | `translate_finance`, `decide_approve_disbursement`, `decide_issue_payout`, `project_finance_settlement_report`, `translate_payout_recovery`, `decide_reverse_and_reissue`, `project_payout_recovery_report` | Same PascalCase symbols; deterministic decisions/projections executed by `FinanceWorkflow` |
| Marketing | `translate_marketing`, `decide_tag_delayed_travelers`, `decide_send_recovery_offer`, `project_marketing_campaign_dashboard`, `gate_recovery_offer_response`, `decide_respond_to_recovery_offer`, `project_marketing_conversion_report` | PascalCase decisions/projections plus durable signal; executed by `MarketingWorkflow`, with both views queryable |

All 32 node IDs are mapped. There are no external activity contracts: the model now carries schedule, account, audience, identifier, and timestamp information explicitly. Cross-context facts are typed child inputs. The parent has delayed-settlement and on-time-closure paths; partial finance rejection starts a new compensation-recovery child.

## Scenarios and tests

Tests cover wheels-down and gate success/rejection, delayed/on-time evaluation, passenger eligibility, exact USD 170 projections, full/partial disbursement, payout reversal/reissue, traveler tagging/skip, accepted/declined responses, and both parent cross-context paths. Complete payloads and data-block projections are asserted.

## Demo

```sh
mise run doctor
mise run check
mise run demo
mise run start
mise run dashboard
mise run respond
mise run conversion
mise run demo:stop
```

Use `mise run on-time` instead of `start` for the no-claim branch. The configured Temporal CLI is `$HOME/go/bin/temporal`.

## Parity

Temporal retry arithmetic and activity boundaries are not applicable because every IR node is deterministic. Go adds typed parent/child orchestration and typed live queries beyond the single-pipeline Python emitter; HITL uses a durable Temporal signal and no workflow-reachable I/O, wall clock, randomness, native channels, or goroutines.
