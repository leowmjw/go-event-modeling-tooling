# Compile report: fulfillment

**Context banner:** Fulfillment bounded context

## Frames
- rf 11 evt Billing.PaymentAuthorized
- tf 12 pcr FulfillmentTranslator
- tf 13 cmd AllocateShipment
- tf 14 evt ShipmentAllocated
- tf 15 rmo ShipmentQueue

## Nodes
- `translate_fulfillment`
- `decide_allocate_shipment`
- `project_shipment_queue`
