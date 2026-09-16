# bounded-context-order-fulfillment

| context | pipeline | version | input event/command | exit events |
|---|---|---|---|---|
| sales | bounded-context-order-fulfillment-sales | 0.1.0 | PlaceOrderCommand | OrderPlaced / OrderPlacementRejected |
| billing | bounded-context-order-fulfillment-billing | 0.1.0 | Sales.OrderPlaced | PaymentAuthorized / PaymentDeclined |
| fulfillment | bounded-context-order-fulfillment-fulfillment | 0.1.0 | Billing.PaymentAuthorized | ShipmentAllocated / ShipmentAllocationDeferred |
| sales-recovery | bounded-context-order-fulfillment-sales-recovery | 0.1.0 | Billing.PaymentDeclined | OrderCancelled / OrderCancellationIgnored |

Cross-context links:
- sales produces `OrderPlaced` which is the `rf 06` input to billing.
- billing produces `PaymentAuthorized` which is the `rf 11` input to fulfillment.
- billing produces `PaymentDeclined` which is the `rf 16` input to sales-recovery.
