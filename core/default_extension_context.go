package core

import (
	"context"
	"log"
	"strings"

	"github.com/xiaoshicae/go-easy-extension/core/ability"
	"github.com/xiaoshicae/go-easy-extension/core/business"
	"github.com/xiaoshicae/go-easy-extension/core/errors"
	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	"github.com/xiaoshicae/go-easy-extension/core/session"
)

type DefaultExtensionContext struct {
	AbilityManager           ability.IAbilityManager
	BusinessManager          business.IBusinessManager
	ExtensionInstanceManager ei.IExtensionInstanceManager
	Session                  session.ISession
	EnableLogger             bool
	MatchBusinessStrict      bool
}

func (d *DefaultExtensionContext) InitSession(ctx context.Context, param interface{}) (err error) {
	d.RemoveSession(ctx)

	matchedBusinessList := make([]business.IBusiness, 0)
	for _, biz := range d.BusinessManager.ListAllIBusinesses(ctx) {
		if biz.Match(param) {
			matchedBusinessList = append(matchedBusinessList, biz)
		}
	}

	if d.MatchBusinessStrict {
		if len(matchedBusinessList) == 0 {
			return errors.ExtensionFirstMatchedInstanceNotFoundErr
		}
		if len(matchedBusinessList) > 1 {
			return errors.ExtensionFirstMatchedInstanceMultilFoundErr
		}
	}

	if len(matchedBusinessList) > 0 {
		matchedBusiness := matchedBusinessList[0]
		if err := d.Session.AddMatchedCode(ctx, matchedBusiness.Code(), matchedBusiness.Priority()); err != nil {
			return err
		}
		if d.EnableLogger {
			log.Printf("matched business: %s", matchedBusiness.Code())
		}
		for _, ua := range matchedBusiness.UsedAbilities() {
			a, err := d.AbilityManager.GetAbility(ctx, ua.AbilityCode)
			if err != nil {
				return err
			}
			if a.Match(param) {
				if err := d.Session.AddMatchedCode(ctx, ua.AbilityCode, ua.Priority); err != nil {
					return err
				}
				if d.EnableLogger {
					log.Printf("matched ability: %s", a.Code())
				}
			}
		}
	}

	da, err := d.AbilityManager.GetAbility(ctx, BaseDefaultAbilityCode)
	if err != nil {
		return err
	}
	if err := d.Session.AddMatchedCode(ctx, da.Code(), DefaultAbilityPriority); err != nil {
		return err
	}
	if d.EnableLogger {
		log.Printf("matched default ability: %s", da.Code())
	}
	return nil
}

func (d *DefaultExtensionContext) RemoveSession(ctx context.Context) {
	d.Session.Remove(ctx)
}

func (d *DefaultExtensionContext) RegisterAbility(ctx context.Context, ability ability.IAbility) error {
	if d.EnableLogger {
		log.Printf("register ability: %s", ability.Code())
	}
	if err := d.AbilityManager.RegisterAbility(ctx, ability); err != nil {
		return err
	}
	if err := d.ExtensionInstanceManager.RegisterExtension(ctx, ability, ability.ImplementExtensions(), ability.Code()); err != nil {
		return err
	}
	return nil

}

func (d *DefaultExtensionContext) RegisterBusiness(ctx context.Context, business business.IBusiness) error {
	if d.EnableLogger {
		log.Printf("register business: %s", business.Code())
	}
	if err := d.BusinessManager.RegisterAbility(ctx, business); err != nil {
		return err
	}
	if err := d.ExtensionInstanceManager.RegisterExtension(ctx, business, business.ImplementExtensions(), business.Code()); err != nil {
		return err
	}
	return nil
}

func (d *DefaultExtensionContext) GetFirstMatchedExtension(ctx context.Context, extensionType ei.ExtensionType) (extension interface{}, err error) {
	matchedCodes, err := d.Session.GetMatchedCodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, code := range matchedCodes {
		extension, err = d.ExtensionInstanceManager.GetExtension(ctx, extensionType, code)
		if err != nil {
			if err == errors.ExtensionInstanceNotFoundErr {
				continue
			}
			return nil, err
		}
		if extension != nil {
			if d.EnableLogger {
				log.Printf("get first matched extension: %s", extension.(interface{ Code() string }).Code())
			}
			return extension, nil
		}
	}
	return nil, errors.ExtensionFirstMatchedInstanceNotFoundErr
}

func (d *DefaultExtensionContext) GetAllMatchedExtension(ctx context.Context, extensionType ei.ExtensionType) (extensionList []interface{}, err error) {
	matchedCodes, err := d.Session.GetMatchedCodes(ctx)
	if err != nil {
		return nil, err
	}
	extensionList = make([]interface{}, 0)
	for _, code := range matchedCodes {
		extension, err := d.ExtensionInstanceManager.GetExtension(ctx, extensionType, code)
		if err != nil {
			if err == errors.ExtensionInstanceNotFoundErr {
				continue
			}
			return nil, err
		}
		if extension != nil {
			extensionList = append(extensionList, extension)
		}
	}
	if d.EnableLogger {
		extensionCodes := make([]string, 0)
		for _, extension := range extensionList {
			extensionCodes = append(extensionCodes, extension.(interface{ Code() string }).Code())
		}
		log.Printf("get all matched extensions: %s", strings.Join(extensionCodes, " > "))
	}
	return extensionList, nil
}
