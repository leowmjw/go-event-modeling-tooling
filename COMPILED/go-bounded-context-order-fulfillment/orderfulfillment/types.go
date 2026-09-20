package orderfulfillment

const (
	TaskQueue                    = "bounded-context-order-fulfillment-v1"
	OrderFulfillmentWorkflowName = "OrderFulfillmentWorkflowV1"
	SalesWorkflowName            = "SalesWorkflowV1"
	BillingWorkflowName          = "BillingWorkflowV1"
	FulfillmentWorkflowName      = "FulfillmentWorkflowV1"
	SalesRecoveryWorkflowName    = "SalesRecoveryWorkflowV1"
	AuthorizePaymentActivityName = "AuthorizePayment"
	CheckInventoryActivityName   = "CheckInventory"
	InventoryReplenishedSignal   = "inventory_replenished"
	ShipmentQueueQuery           = "shipment_queue"
)

type LineItem struct {
	SKU       string  `json:"sku"`
	Quantity  int     `json:"qty"`
	UnitPrice float64 `json:"unitPrice"`
}

type CartPriced struct {
	CartID     string     `json:"cartId"`
	CustomerID string     `json:"customerId"`
	Items      []LineItem `json:"items"`
	Total      float64    `json:"total"`
}

type CartSummaryView struct {
	CartID     string     `json:"cartId"`
	CustomerID string     `json:"customerId"`
	Items      []LineItem `json:"items"`
	Total      float64    `json:"total"`
}

type PlaceOrderCommand struct {
	CartID     string  `json:"cartId"`
	CustomerID string  `json:"customerId"`
	Total      float64 `json:"total"`
}

type OrderPlaced struct {
	OrderID    string  `json:"orderId"`
	CustomerID string  `json:"customerId"`
	Total      float64 `json:"total"`
}

type OrderPlacementRejected struct {
	CartID string `json:"cartId"`
	Reason string `json:"reason"`
}

type OrderStatusView struct {
	OrderID    string `json:"orderId,omitempty"`
	CustomerID string `json:"customerId,omitempty"`
	Status     string `json:"status"`
}

type SalesResult struct {
	Event       string                  `json:"event"`
	Placed      *OrderPlaced            `json:"placed,omitempty"`
	Rejected    *OrderPlacementRejected `json:"rejected,omitempty"`
	CartSummary CartSummaryView         `json:"cartSummary"`
	OrderStatus OrderStatusView         `json:"orderStatus"`
}

type AuthorizePaymentCommand struct {
	OrderID         string  `json:"orderId"`
	Amount          float64 `json:"amount"`
	PaymentMethodID string  `json:"paymentMethodId"`
}

type PaymentAuthorized struct {
	OrderID         string  `json:"orderId"`
	AuthorizationID string  `json:"authorizationId"`
	Amount          float64 `json:"amount"`
}

type PaymentDeclined struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

type PaymentStatusView struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
}

type PaymentResult struct {
	Event      string             `json:"event"`
	Authorized *PaymentAuthorized `json:"authorized,omitempty"`
	Declined   *PaymentDeclined   `json:"declined,omitempty"`
	Status     PaymentStatusView  `json:"status"`
}

type AllocateShipmentCommand struct {
	OrderID     string `json:"orderId"`
	WarehouseID string `json:"warehouseId"`
}

type ShipmentAllocated struct {
	OrderID     string `json:"orderId"`
	ShipmentID  string `json:"shipmentId"`
	WarehouseID string `json:"warehouseId"`
}

type ShipmentAllocationDeferred struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

type ShipmentQueueView struct {
	ShipmentID  string `json:"shipmentId,omitempty"`
	OrderID     string `json:"orderId"`
	WarehouseID string `json:"warehouseId,omitempty"`
	Status      string `json:"status"`
}

type InventoryReplenished struct {
	WarehouseID string `json:"warehouseId"`
	SKU         string `json:"sku"`
}

type ShipmentResult struct {
	Event     string                      `json:"event"`
	Allocated *ShipmentAllocated          `json:"allocated,omitempty"`
	Deferred  *ShipmentAllocationDeferred `json:"deferred,omitempty"`
	Queue     ShipmentQueueView           `json:"queue"`
}

type CancelOrderCommand struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

type OrderCancelled struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

type OrderCancellationIgnored struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

type CancelledOrderStatusView struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
}

type OrderEvent struct {
	Type    string `json:"type"`
	OrderID string `json:"orderId"`
}

type CancellationResult struct {
	Event     string                    `json:"event"`
	Cancelled *OrderCancelled           `json:"cancelled,omitempty"`
	Ignored   *OrderCancellationIgnored `json:"ignored,omitempty"`
	Status    CancelledOrderStatusView  `json:"status"`
}

type SalesWorkflowInput struct {
	Cart    CartPriced        `json:"cart"`
	Command PlaceOrderCommand `json:"command"`
}

type BillingWorkflowInput struct {
	Order           OrderPlaced `json:"order"`
	PaymentMethodID string      `json:"paymentMethodId"`
}

type FulfillmentWorkflowInput struct {
	Payment     PaymentAuthorized `json:"payment"`
	WarehouseID string            `json:"warehouseId"`
}

type SalesRecoveryWorkflowInput struct {
	Declined    PaymentDeclined `json:"declined"`
	PriorEvents []OrderEvent    `json:"priorEvents,omitempty"`
}

type OrderFulfillmentInput struct {
	Cart            CartPriced        `json:"cart"`
	Command         PlaceOrderCommand `json:"command"`
	PaymentMethodID string            `json:"paymentMethodId"`
	WarehouseID     string            `json:"warehouseId"`
}

type OrderFulfillmentResult struct {
	Sales        SalesResult         `json:"sales"`
	Payment      *PaymentResult      `json:"payment,omitempty"`
	Shipment     *ShipmentResult     `json:"shipment,omitempty"`
	Cancellation *CancellationResult `json:"cancellation,omitempty"`
}

type PaymentGatewayRequest struct {
	Command AuthorizePaymentCommand `json:"command"`
}

type PaymentGatewayResponse struct {
	Authorized      bool   `json:"authorized"`
	AuthorizationID string `json:"authorizationId,omitempty"`
	DeclineReason   string `json:"declineReason,omitempty"`
}

type InventoryRequest struct {
	Command AllocateShipmentCommand `json:"command"`
}

type InventoryResponse struct {
	Available  bool   `json:"available"`
	ShipmentID string `json:"shipmentId,omitempty"`
}
