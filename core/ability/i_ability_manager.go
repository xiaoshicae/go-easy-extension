package ability

import "context"

type IAbilityManager interface {
	RegisterAbility(ctx context.Context, ability IAbility) error

	GetAbility(ctx context.Context, code string) (IAbility, error)

	ListAllAbilities(ctx context.Context) []IAbility
}
