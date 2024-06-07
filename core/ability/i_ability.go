package ability

import ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"

type IAbility interface {
	// Code ability code
	Code() string

	// Match ability is enabled
	Match(param interface{}) bool

	// ImplementExtensions a group of extension interface implemented by the instance
	ImplementExtensions() []ei.ExtensionType
}
