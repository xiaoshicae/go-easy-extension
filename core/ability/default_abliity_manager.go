package ability

import (
	"context"

	"github.com/xiaoshicae/go-easy-extension/core/errors"
)

type DefaultAbilityManager struct {
	abilityMap map[string]IAbility
}

func (d *DefaultAbilityManager) RegisterAbility(ctx context.Context, ability IAbility) error {
	if ability == nil {
		return errors.AbilityNilErr
	}
	if d.abilityMap == nil {
		d.abilityMap = make(map[string]IAbility)
	}
	if _, ok := d.abilityMap[ability.Code()]; ok {
		return errors.AbilityRegisterDuplicateErr
	}
	d.abilityMap[ability.Code()] = ability
	return nil
}

func (d *DefaultAbilityManager) GetAbility(ctx context.Context, code string) (IAbility, error) {
	if d.abilityMap == nil {
		return nil, errors.AbilityNotFoundErr
	}
	ability, ok := d.abilityMap[code]
	if !ok {
		return nil, errors.AbilityNotFoundErr
	}
	return ability, nil
}

func (d *DefaultAbilityManager) ListAllAbilities(ctx context.Context) []IAbility {
	if d.abilityMap == nil {
		return nil
	}
	abilities := make([]IAbility, 0)
	for _, ability := range d.abilityMap {
		abilities = append(abilities, ability)
	}
	return abilities
}
