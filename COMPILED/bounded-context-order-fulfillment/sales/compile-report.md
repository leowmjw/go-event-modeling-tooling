# Compile report: sales

**Context banner:** Sales bounded context

## Frames
- tf 01 ui CheckoutScreen
- tf 02 rmo CartSummary
- tf 03 cmd PlaceOrder
- tf 04 evt OrderPlaced
- tf 05 rmo OrderStatus

## Nodes
- `decide_place_order`
- `project_order_status`

## Open questions / judgment calls
- rmo CartSummary (tf 02) has no producing event in the model. Add a CartPriced event or remove the read model.
- UI CheckoutScreen (tf 01) becomes the pipeline input PlaceOrderCommand.
