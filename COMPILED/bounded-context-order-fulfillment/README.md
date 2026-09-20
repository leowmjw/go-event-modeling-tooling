# bounded-context-order-fulfillment

| context | pipeline | version | input event/command | exit events |
|---|---|---|---|---|
| sales | bounded-context-order-fulfillment-sales | 0.2.0 | PlaceOrderCommand · Pricing.CartPriced · Billing.PaymentDeclined | OrderPlaced / OrderPlacementRejected · OrderCancelled / OrderCancellationIgnored |
| billing | bounded-context-order-fulfillment-billing | 0.2.0 | Sales.OrderPlaced | PaymentAuthorized / PaymentDeclined |
| fulfillment | bounded-context-order-fulfillment-fulfillment | 0.2.0 | Billing.PaymentAuthorized · Warehouse.InventoryReplenished | ShipmentAllocated / ShipmentAllocationDeferred · AllocateShipmentCommand (retry) |

Cross-context links:
- pricing (external) produces `CartPriced` — `rf 21` input to sales.
- sales produces `OrderPlaced` — `rf 06` input to billing.
- billing produces `PaymentAuthorized` — `rf 11` input to fulfillment.
- billing produces `PaymentDeclined` — `rf 16` input to sales (recovery slice).
- warehouse (external) produces `InventoryReplenished` — `rf 25` input to fulfillment.
