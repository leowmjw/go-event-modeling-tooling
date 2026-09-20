# Compile report: flight-arrival-post-flight-settlement / marketing

Version 1.0.0. Six deterministic functions and one `hitl_gate` cover audience tagging, offer sending, campaign projection, traveler response, and conversion projection. The gate signal is `respond_to_recovery_offer` with a seven-day timeout. Accepted and declined GWTs include deterministic response timestamps. `MarketingCampaignDashboard46` and the conversion report are exact projection tests. Input is `Finance.PayoutIssued`; no cross-context output, LLM, external call, or open question remains.

## Changes

`0.1.0 → 1.0.0`: added the missing offer GWT, completed payloads, replaced six stubs, corrected the durable gate and bindings, and renamed nodes. The incompatible shape requires a major version.
