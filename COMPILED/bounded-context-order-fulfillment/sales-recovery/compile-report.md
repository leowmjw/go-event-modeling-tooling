# Compile report: sales-recovery

**Context banner:** Billing failure translated back into Sales

## Frames
- rf 16 evt Billing.PaymentDeclined
- tf 17 pcr SalesRecoveryTranslator
- tf 18 cmd CancelOrder
- tf 19 evt OrderCancelled
- tf 20 rmo CancelledOrderStatus

## Nodes
- `translate_sales_recovery`
- `decide_cancel_order`
- `project_cancelled_order_status`

## Open questions / judgment calls
- Banner does not match a declared bounded context and starts with an rf. Assigned context slug 'sales-recovery' based on pcr SalesRecoveryTranslator; the expert should rename the banner to 'Sales: billing failure recovery' if the frames belong to Sales.
