package core

import (
	"math"

	ie "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
)

const (
	BaseDefaultAbilityCode = "ability.application.default"
	DefaultAbilityPriority = math.MaxInt64
)

type BaseDefaultAbility struct {
}

func (b BaseDefaultAbility) Code() string {
	return BaseDefaultAbilityCode
}

func (b BaseDefaultAbility) Match(param interface{}) bool {
	return true
}

func (b BaseDefaultAbility) ImplementExtensions() []ie.ExtensionType {
	panic("implement me")
}
