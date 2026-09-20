package orderfulfillment

import (
	"context"
	"fmt"
	"sync"
)

type PaymentGateway interface {
	Authorize(context.Context, AuthorizePaymentCommand) (PaymentGatewayResponse, error)
}

type InventoryService interface {
	Allocate(context.Context, AllocateShipmentCommand) (InventoryResponse, error)
}

type Activities struct {
	PaymentGateway PaymentGateway
	Inventory      InventoryService
}

func (a *Activities) AuthorizePayment(ctx context.Context, request PaymentGatewayRequest) (PaymentGatewayResponse, error) {
	if a.PaymentGateway == nil {
		return PaymentGatewayResponse{}, fmt.Errorf("payment gateway is not configured")
	}
	return a.PaymentGateway.Authorize(ctx, request.Command)
}

func (a *Activities) CheckInventory(ctx context.Context, request InventoryRequest) (InventoryResponse, error) {
	if a.Inventory == nil {
		return InventoryResponse{}, fmt.Errorf("inventory service is not configured")
	}
	return a.Inventory.Allocate(ctx, request.Command)
}

type StaticPaymentGateway struct {
	Response PaymentGatewayResponse
}

func (g StaticPaymentGateway) Authorize(context.Context, AuthorizePaymentCommand) (PaymentGatewayResponse, error) {
	return g.Response, nil
}

type StaticInventoryService struct {
	Response InventoryResponse
}

func (s StaticInventoryService) Allocate(context.Context, AllocateShipmentCommand) (InventoryResponse, error) {
	return s.Response, nil
}

type SequencedInventoryService struct {
	mu        sync.Mutex
	Responses []InventoryResponse
	next      int
}

func (s *SequencedInventoryService) Allocate(context.Context, AllocateShipmentCommand) (InventoryResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Responses) == 0 {
		return InventoryResponse{}, fmt.Errorf("inventory response sequence is empty")
	}
	index := min(s.next, len(s.Responses)-1)
	s.next++
	return s.Responses[index], nil
}
