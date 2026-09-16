# Compile report: billing

**Context banner:** Billing bounded context

## Frames
- rf 06 evt Sales.OrderPlaced
- tf 07 pcr BillingTranslator
- tf 08 cmd AuthorizePayment
- tf 09 evt PaymentAuthorized
- tf 10 rmo PaymentStatus

## Nodes
- `translate_billing`
- `decide_authorize_payment`
- `project_payment_status`
