package core

import (
	"github.com/xiaoshicae/go-easy-extension/core/ability"
	"github.com/xiaoshicae/go-easy-extension/core/business"
	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	"github.com/xiaoshicae/go-easy-extension/core/session"
)

var globalExtensionContext IExtensionContext = &DefaultExtensionContext{
	AbilityManager:           &ability.DefaultAbilityManager{},
	BusinessManager:          &business.DefaultBusinessManager{},
	ExtensionInstanceManager: &ei.DefaultExtensionInstanceManager{},
	Session:                  &session.DefaultSession{},
	EnableLogger:             true,
	MatchBusinessStrict:      false,
}

func GetExtensionContext() IExtensionContext {
	return globalExtensionContext
}

func SetEnableLogger(enable bool) {
	globalExtensionContext.(*DefaultExtensionContext).EnableLogger = enable
}

func SetMatchBusinessStrict(strict bool) {
	globalExtensionContext.(*DefaultExtensionContext).MatchBusinessStrict = strict
}
