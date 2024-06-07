package business

import "context"

type IBusinessManager interface {
	RegisterAbility(ctx context.Context, business IBusiness) error

	GetBusiness(ctx context.Context, code string) (IBusiness, error)

	ListAllIBusinesses(ctx context.Context) []IBusiness
}
