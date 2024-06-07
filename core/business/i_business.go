package business

import ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"

type IBusiness interface {
	// Code ability code
	Code() string

	// Match ability is enabled
	Match(param interface{}) bool

	// Priority effective priority of business
	Priority() int64

	// UsedAbilities all abilities used by business
	UsedAbilities() []UsedAbility

	// ImplementExtensions a group of extension interface implemented by the instance
	ImplementExtensions() []ei.ExtensionType
}

type UsedAbility struct {
	AbilityCode string
	Priority    int64
}
