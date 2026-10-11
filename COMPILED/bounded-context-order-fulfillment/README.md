# bounded-context-order-fulfillment

| context | pipeline | version | input event/command | exit events |
|---|---|---|---|---|
| sales | bounded-context-order-fulfillment-sales | 1.0.0 | Pricing.CartPriced · PlaceOrder (gate `place_order`) · Billing.PaymentAuthorized · Billing.PaymentDeclined | OrderPlaced / OrderPlacementRejected · OrderStatus |
| billing | bounded-context-order-fulfillment-billing | 0.3.0 | Sales.OrderPlaced | PaymentAuthorized / PaymentDeclined · PaymentStatus |
| fulfillment | bounded-context-order-fulfillment-fulfillment | 1.0.0 | Sales.OrderPlaced · Billing.PaymentAuthorized · Warehouse.InventoryReplenished | ShipmentQueue · AllocateShipmentCommand (retry feedback) |

Cross-context links:
- pricing (external) produces `CartPriced` — `rf 21` input to sales.
- sales produces `OrderPlaced` — `rf 06` input to billing, `rf 28` input to fulfillment.
- billing produces `PaymentAuthorized` — `rf 11` input to fulfillment, `rf 30` input to sales (confirmation slice).
- billing produces `PaymentDeclined` — `rf 16` input to sales (recovery slice).
- warehouse (external) produces `InventoryReplenished` — `rf 25` input to fulfillment.
