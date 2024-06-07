package easy_extension

import (
	"context"

	"github.com/xiaoshicae/go-easy-extension/core"
	"github.com/xiaoshicae/go-easy-extension/core/ability"
	"github.com/xiaoshicae/go-easy-extension/core/business"
	"github.com/xiaoshicae/go-easy-extension/core/errors"
	"github.com/xiaoshicae/go-easy-extension/core/session"
)

func NewCtxWithSession(ctx context.Context) context.Context {
	return session.NewCtxWithSession(ctx)
}

func InitSession(ctx context.Context, pram interface{}) error {
	return core.GetExtensionContext().InitSession(ctx, pram)
}

func RemoveSession(ctx context.Context) {
	core.GetExtensionContext().RemoveSession(ctx)
}

func GetFirstMatchedExtension[T any](ctx context.Context) (T, error) {
	extension, err := core.GetExtensionContext().GetFirstMatchedExtension(ctx, new(T))
	if err != nil {
		return *new(T), err
	}
	return extension.(T), nil
}

func GetAllMatchedExtension[T any](ctx context.Context) ([]T, error) {
	extensions, err := core.GetExtensionContext().GetAllMatchedExtension(ctx, new(T))
	if err != nil {
		return nil, err
	}
	allMatchedExtension := make([]T, 0)
	for _, e := range extensions {
		allMatchedExtension = append(allMatchedExtension, e.(T))
	}
	return allMatchedExtension, nil
}

func RegisterAbility(ctx context.Context, a ability.IAbility) error {
	return core.GetExtensionContext().RegisterAbility(ctx, a)
}

func RegisterDefaultAbility(ctx context.Context, a ability.IAbility) error {
	if a == nil {
		return errors.AbilityNilErr
	}
	if a.Code() != core.BaseDefaultAbilityCode {
		return errors.AbilityRegisterNotDefaultAbilityErr
	}
	return core.GetExtensionContext().RegisterAbility(ctx, a)
}

func RegisterBusiness(ctx context.Context, b business.IBusiness) error {
	return core.GetExtensionContext().RegisterBusiness(ctx, b)
}
