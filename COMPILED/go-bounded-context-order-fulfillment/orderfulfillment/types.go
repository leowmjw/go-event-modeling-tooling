package orderfulfillment

const (
	TaskQueue = "bounded-context-order-fulfillment-v2"

	OrderFulfillmentWorkflowName = "OrderFulfillmentWorkflowV2"
	SalesWorkflowName            = "SalesWorkflowV2"
	BillingWorkflowName          = "BillingWorkflowV2"
	FulfillmentWorkflowName      = "FulfillmentWorkflowV2"

	AuthorizePaymentWithGatewayActivityName   = "AuthorizePaymentWithGateway"
	AllocateShipmentWithWarehouseActivityName = "AllocateShipmentWithWarehouse"

	// PlaceOrderSignal carries the CheckoutScreen command into gate_place_order.
	PlaceOrderSignal = "place_order"
	// OrderPlacementSignal reports the Sales placement decision to the parent.
	OrderPlacementSignal = "order_placement"
	// BillingOutcomeSignal delivers rf 30 / rf 16 from the parent to Sales.
	BillingOutcomeSignal = "billing_outcome"
	// InventoryReplenishedSignal delivers rf 25 to a deferred Fulfillment child.
	InventoryReplenishedSignal = "inventory_replenished"

	CartSummaryQuery      = "cart_summary"
	OrderStatusQuery      = "order_status"
	PaymentStatusQuery    = "payment_status"
	PaidOrdersToShipQuery = "paid_orders_to_ship"
	ShipmentQueueQuery    = "shipment_queue"

	SalesChildSuffix       = "/sales"
	BillingChildSuffix     = "/billing"
	FulfillmentChildSuffix = "/fulfillment"
)

// Sales

type LineItem struct {
	SKU       string  `json:"sku"`
	Qty       int     `json:"qty"`
	UnitPrice float64 `json:"unitPrice"`
}

// CartPriced is rf 21 Pricing.CartPriced.
type CartPriced struct {
	CartID     string     `json:"cartId"`
	CustomerID string     `json:"customerId"`
	Items      []LineItem `json:"items"`
	Total      float64    `json:"total"`
}

// CartSummaryView is tf 02 rmo CartSummary.
type CartSummaryView struct {
	CartID     string     `json:"cartId"`
	CustomerID string     `json:"customerId"`
	Items      []LineItem `json:"items"`
	Total      float64    `json:"total"`
}

// PlaceOrderCommand is tf 03 cmd PlaceOrder, issued from CheckoutScreen.
type PlaceOrderCommand struct {
	OrderID         string `json:"orderId"`
	CartID          string `json:"cartId"`
	CustomerID      string `json:"customerId"`
	PaymentMethodID string `json:"paymentMethodId"`
}

// OrderPlaced is tf 04 evt OrderPlaced.
type OrderPlaced struct {
	OrderID         string     `json:"orderId"`
	CartID          string     `json:"cartId"`
	CustomerID      string     `json:"customerId"`
	PaymentMethodID string     `json:"paymentMethodId"`
	Items           []LineItem `json:"items"`
	Total           float64    `json:"total"`
}

// OrderPlacementRejected is tf 27 evt OrderPlacementRejected.
type OrderPlacementRejected struct {
	OrderID string `json:"orderId"`
	CartID  string `json:"cartId"`
	Reason  string `json:"reason"`
}

// PlaceOrderResult is the output of decide_place_order; exactly one of
// Placed or Rejected is set.
type PlaceOrderResult struct {
	Event    string                  `json:"event"`
	Placed   *OrderPlaced            `json:"placed,omitempty"`
	Rejected *OrderPlacementRejected `json:"rejected,omitempty"`
}

// OrderStatusView is tf 05 rmo OrderStatus.
type OrderStatusView struct {
	OrderID    string `json:"orderId"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
}

// SalesPaymentAuthorized is rf 30 Billing.PaymentAuthorized as Sales sees it.
type SalesPaymentAuthorized struct {
	OrderID         string `json:"orderId"`
	AuthorizationID string `json:"authorizationId"`
}

// SalesPaymentDeclined is rf 16 Billing.PaymentDeclined as Sales sees it.
type SalesPaymentDeclined struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// BillingOutcome carries exactly one of the two billing facts to Sales.
type BillingOutcome struct {
	Authorized *SalesPaymentAuthorized `json:"authorized,omitempty"`
	Declined   *SalesPaymentDeclined   `json:"declined,omitempty"`
}

// ConfirmOrderCommand is tf 32 cmd ConfirmOrder.
type ConfirmOrderCommand struct {
	OrderID         string `json:"orderId"`
	AuthorizationID string `json:"authorizationId"`
}

// OrderConfirmationResult is the output of decide_confirm_order:
// tf 33 OrderConfirmed (AuthorizationID set) or tf 34 OrderConfirmationIgnored (Reason set).
type OrderConfirmationResult struct {
	Event           string `json:"event"`
	OrderID         string `json:"orderId"`
	AuthorizationID string `json:"authorizationId,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// CancelOrderCommand is tf 18 cmd CancelOrder.
type CancelOrderCommand struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// OrderCancellationResult is the output of decide_cancel_order:
// tf 19 OrderCancelled or tf 24 OrderCancellationIgnored.
type OrderCancellationResult struct {
	Event   string `json:"event"`
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// RecordedEvent is one entry of the Sales order_history.
type RecordedEvent struct {
	Event   string `json:"event"`
	OrderID string `json:"orderId"`
}

type SalesWorkflowInput struct {
	CartPriced CartPriced `json:"cartPriced"`
}

type SalesResult struct {
	CartSummary   CartSummaryView           `json:"cartSummary"`
	Placement     PlaceOrderResult          `json:"placement"`
	Confirmations []OrderConfirmationResult `json:"confirmations,omitempty"`
	Cancellations []OrderCancellationResult `json:"cancellations,omitempty"`
	OrderStatus   *OrderStatusView          `json:"orderStatus,omitempty"`
	History       []RecordedEvent           `json:"history"`
}

// Billing

// SalesOrderPlaced is rf 06 Sales.OrderPlaced.
type SalesOrderPlaced struct {
	OrderID         string  `json:"orderId"`
	PaymentMethodID string  `json:"paymentMethodId"`
	Total           float64 `json:"total"`
}

// AuthorizePaymentCommand is tf 08 cmd AuthorizePayment.
type AuthorizePaymentCommand struct {
	OrderID         string  `json:"orderId"`
	Amount          float64 `json:"amount"`
	PaymentMethodID string  `json:"paymentMethodId"`
}

type GatewayAuthorizationRequest struct {
	Command        AuthorizePaymentCommand `json:"command"`
	IdempotencyKey string                  `json:"idempotencyKey"`
}

// GatewayAuthorization is the payment_gateway result named in note 08.
type GatewayAuthorization struct {
	Approved        bool   `json:"approved"`
	AuthorizationID string `json:"authorizationId,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// PaymentAuthorized is tf 09 evt PaymentAuthorized.
type PaymentAuthorized struct {
	OrderID         string  `json:"orderId"`
	AuthorizationID string  `json:"authorizationId"`
	Amount          float64 `json:"amount"`
}

// PaymentDeclined is tf 22 evt PaymentDeclined.
type PaymentDeclined struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// PaymentAuthorizationResult is the output of decide_authorize_payment.
type PaymentAuthorizationResult struct {
	Event      string             `json:"event"`
	Authorized *PaymentAuthorized `json:"authorized,omitempty"`
	Declined   *PaymentDeclined   `json:"declined,omitempty"`
}

// PaymentStatusView is tf 10 rmo PaymentStatus.
type PaymentStatusView struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

type BillingWorkflowInput struct {
	OrderPlaced SalesOrderPlaced `json:"orderPlaced"`
}

type BillingResult struct {
	Payment       PaymentAuthorizationResult `json:"payment"`
	PaymentStatus PaymentStatusView          `json:"paymentStatus"`
}

// Fulfillment

type ShipmentItem struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

// FulfillmentOrderPlaced is rf 28 Sales.OrderPlaced.
type FulfillmentOrderPlaced struct {
	OrderID string         `json:"orderId"`
	Items   []ShipmentItem `json:"items"`
}

// FulfillmentPaymentAuthorized is rf 11 Billing.PaymentAuthorized.
type FulfillmentPaymentAuthorized struct {
	OrderID string  `json:"orderId"`
	Amount  float64 `json:"amount"`
}

// PaidOrdersToShipView is tf 29 rmo PaidOrdersToShip.
type PaidOrdersToShipView struct {
	OrderID string         `json:"orderId"`
	Items   []ShipmentItem `json:"items"`
	Status  string         `json:"status"`
}

// AllocateShipmentCommand is tf 13 cmd AllocateShipment.
type AllocateShipmentCommand struct {
	OrderID string         `json:"orderId"`
	Items   []ShipmentItem `json:"items"`
}

type WarehouseAllocationRequest struct {
	Command        AllocateShipmentCommand `json:"command"`
	IdempotencyKey string                  `json:"idempotencyKey"`
}

// WarehouseAllocation is the warehouse_inventory result named in note 13.
type WarehouseAllocation struct {
	Allocated   bool     `json:"allocated"`
	ShipmentID  string   `json:"shipmentId,omitempty"`
	WarehouseID string   `json:"warehouseId,omitempty"`
	MissingSKUs []string `json:"missingSkus,omitempty"`
}

// ShipmentAllocated is tf 14 evt ShipmentAllocated.
type ShipmentAllocated struct {
	OrderID     string `json:"orderId"`
	ShipmentID  string `json:"shipmentId"`
	WarehouseID string `json:"warehouseId"`
}

// ShipmentAllocationDeferred is tf 23 evt ShipmentAllocationDeferred.
type ShipmentAllocationDeferred struct {
	OrderID     string   `json:"orderId"`
	Reason      string   `json:"reason"`
	MissingSKUs []string `json:"missingSkus"`
}

// ShipmentAllocationResult is the output of decide_allocate_shipment.
type ShipmentAllocationResult struct {
	Event     string                      `json:"event"`
	Allocated *ShipmentAllocated          `json:"allocated,omitempty"`
	Deferred  *ShipmentAllocationDeferred `json:"deferred,omitempty"`
}

// ShipmentQueueView is tf 15 rmo ShipmentQueue.
type ShipmentQueueView struct {
	ShipmentID  string   `json:"shipmentId,omitempty"`
	OrderID     string   `json:"orderId"`
	WarehouseID string   `json:"warehouseId,omitempty"`
	Status      string   `json:"status"`
	MissingSKUs []string `json:"missingSkus,omitempty"`
}

// InventoryReplenished is rf 25 Warehouse.InventoryReplenished.
type InventoryReplenished struct {
	WarehouseID string `json:"warehouseId"`
	SKU         string `json:"sku"`
}

type FulfillmentWorkflowInput struct {
	OrderPlaced       FulfillmentOrderPlaced       `json:"orderPlaced"`
	PaymentAuthorized FulfillmentPaymentAuthorized `json:"paymentAuthorized"`
}

type FulfillmentResult struct {
	PaidOrdersToShip PaidOrdersToShipView       `json:"paidOrdersToShip"`
	Allocations      []ShipmentAllocationResult `json:"allocations"`
	ShipmentQueue    ShipmentQueueView          `json:"shipmentQueue"`
	Attempts         int                        `json:"attempts"`
}

// Parent

type OrderFulfillmentInput struct {
	CartPriced CartPriced `json:"cartPriced"`
}

type OrderFulfillmentResult struct {
	Sales       SalesResult        `json:"sales"`
	Billing     *BillingResult     `json:"billing,omitempty"`
	Fulfillment *FulfillmentResult `json:"fulfillment,omitempty"`
}
