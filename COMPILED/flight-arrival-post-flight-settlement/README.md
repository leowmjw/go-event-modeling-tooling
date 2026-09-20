# flight-arrival-post-flight-settlement compiled IR

| context | pipeline | version | input | exits |
|---|---|---|---|---|
| operations | [`operations/pipeline.yaml`](operations/pipeline.yaml) | 1.0.0 | flight operations envelope | arrival and closure facts/views |
| compensation | [`compensation/pipeline.yaml`](compensation/pipeline.yaml) | 1.0.0 | Operations/Finance facts | calculated, closed, or escalated claim views |
| finance | [`finance/pipeline.yaml`](finance/pipeline.yaml) | 1.0.0 | compensation/payout facts | disbursement and payout views |
| marketing | [`marketing/pipeline.yaml`](marketing/pipeline.yaml) | 1.0.0 | payout and offer input | campaign, response, and conversion views |
