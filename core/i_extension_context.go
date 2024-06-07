package core

import (
	"context"

	"github.com/xiaoshicae/go-easy-extension/core/ability"
	"github.com/xiaoshicae/go-easy-extension/core/business"
	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
)

type IExtensionContext interface {
	RegisterAbility(ctx context.Context, ability ability.IAbility) error

	RegisterBusiness(ctx context.Context, business business.IBusiness) error

	GetFirstMatchedExtension(ctx context.Context, extensionType ei.ExtensionType) (extension interface{}, err error)

	GetAllMatchedExtension(ctx context.Context, extensionType ei.ExtensionType) (extensionList []interface{}, err error)

	InitSession(ctx context.Context, param interface{}) (err error)

	RemoveSession(ctx context.Context)
}
