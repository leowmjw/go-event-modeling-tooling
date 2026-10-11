package orderfulfillment

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"go.temporal.io/sdk/temporal"
)

// PaymentGateway is the payment_gateway named in note 08.
type PaymentGateway interface {
	Authorize(ctx context.Context, command AuthorizePaymentCommand, idempotencyKey string) (GatewayAuthorization, error)
}

// WarehouseInventory is the warehouse_inventory named in note 13.
type WarehouseInventory interface {
	Allocate(ctx context.Context, command AllocateShipmentCommand, idempotencyKey string) (WarehouseAllocation, error)
}

type Activities struct {
	PaymentGateway PaymentGateway
	Warehouse      WarehouseInventory
}

// AuthorizePaymentWithGateway is IR node billing.authorize_payment_with_gateway.
func (a *Activities) AuthorizePaymentWithGateway(ctx context.Context, request GatewayAuthorizationRequest) (GatewayAuthorization, error) {
	if request.IdempotencyKey == "" {
		return GatewayAuthorization{}, temporal.NewNonRetryableApplicationError("idempotency key is required", "InvalidRequest", nil)
	}
	result, err := a.PaymentGateway.Authorize(ctx, request.Command, request.IdempotencyKey)
	if err != nil {
		return GatewayAuthorization{}, err
	}
	if err := ValidateGatewayAuthorization(result); err != nil {
		return GatewayAuthorization{}, temporal.NewNonRetryableApplicationError(err.Error(), "InvalidGatewayResult", err)
	}
	return result, nil
}

// AllocateShipmentWithWarehouse is IR node fulfillment.allocate_shipment_with_warehouse.
func (a *Activities) AllocateShipmentWithWarehouse(ctx context.Context, request WarehouseAllocationRequest) (WarehouseAllocation, error) {
	if request.IdempotencyKey == "" {
		return WarehouseAllocation{}, temporal.NewNonRetryableApplicationError("idempotency key is required", "InvalidRequest", nil)
	}
	result, err := a.Warehouse.Allocate(ctx, request.Command, request.IdempotencyKey)
	if err != nil {
		return WarehouseAllocation{}, err
	}
	if err := ValidateWarehouseAllocation(result); err != nil {
		return WarehouseAllocation{}, temporal.NewNonRetryableApplicationError(err.Error(), "InvalidWarehouseResult", err)
	}
	return result, nil
}

// DemoPaymentGateway answers every new idempotency key with Result and
// replays the stored answer for a key it has seen.
type DemoPaymentGateway struct {
	Result GatewayAuthorization

	mu   sync.Mutex
	seen map[string]GatewayAuthorization
}

func (g *DemoPaymentGateway) Authorize(_ context.Context, command AuthorizePaymentCommand, idempotencyKey string) (GatewayAuthorization, error) {
	if command.OrderID == "" || command.PaymentMethodID == "" {
		return GatewayAuthorization{}, errors.New("demo gateway: orderId and paymentMethodId are required")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if prior, ok := g.seen[idempotencyKey]; ok {
		return prior, nil
	}
	if g.seen == nil {
		g.seen = map[string]GatewayAuthorization{}
	}
	g.seen[idempotencyKey] = g.Result
	return g.Result, nil
}

// DemoWarehouse reports MissingSKUs as unavailable for the first
// UnavailableAttempts distinct idempotency keys of each order, then
// allocates. A repeated key replays the stored answer.
type DemoWarehouse struct {
	UnavailableAttempts int
	MissingSKUs         []string
	ShipmentID          string
	WarehouseID         string

	mu       sync.Mutex
	seen     map[string]WarehouseAllocation
	attempts map[string]int
}

func (w *DemoWarehouse) Allocate(_ context.Context, command AllocateShipmentCommand, idempotencyKey string) (WarehouseAllocation, error) {
	if command.OrderID == "" || len(command.Items) == 0 {
		return WarehouseAllocation{}, errors.New("demo warehouse: orderId and items are required")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if prior, ok := w.seen[idempotencyKey]; ok {
		return prior, nil
	}
	if w.seen == nil {
		w.seen = map[string]WarehouseAllocation{}
		w.attempts = map[string]int{}
	}
	result := WarehouseAllocation{Allocated: true, ShipmentID: w.ShipmentID, WarehouseID: w.WarehouseID}
	if w.attempts[command.OrderID] < w.UnavailableAttempts {
		if len(w.MissingSKUs) == 0 {
			return WarehouseAllocation{}, fmt.Errorf("demo warehouse: MissingSKUs required when UnavailableAttempts=%d", w.UnavailableAttempts)
		}
		result = WarehouseAllocation{MissingSKUs: slices.Clone(w.MissingSKUs)}
	}
	w.attempts[command.OrderID]++
	w.seen[idempotencyKey] = result
	return result, nil
}
