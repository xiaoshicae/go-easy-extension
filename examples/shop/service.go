package main

import (
	"context"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

// CheckoutService selects point implementations from the current request's binding.
// The Registry is injected once; selected implementations are never cached across requests.
type CheckoutService struct {
	registry *easyext.Registry[OrderParam]
}

func NewCheckoutService(registry *easyext.Registry[OrderParam]) *CheckoutService {
	return &CheckoutService{registry: registry}
}

type Quote struct {
	Freight      int
	DeliveryDays int
	Notification string
}

func (s *CheckoutService) Checkout(ctx context.Context, province string, items int) (Quote, error) {
	freight, err := s.registry.First[FreightCalc](ctx)
	if err != nil {
		return Quote{}, err
	}
	cost, err := freight.CalcFreight(ctx, province, items)
	if err != nil {
		return Quote{}, err
	}
	delivery, err := s.registry.First[Delivery](ctx)
	if err != nil {
		return Quote{}, err
	}
	days, err := delivery.DeliveryDays(ctx)
	if err != nil {
		return Quote{}, err
	}
	notify, err := s.registry.First[Notify](ctx)
	if err != nil {
		return Quote{}, err
	}
	message, err := notify.Notification(ctx, "order.created")
	if err != nil {
		return Quote{}, err
	}
	return Quote{Freight: cost, DeliveryDays: days, Notification: message}, nil
}
