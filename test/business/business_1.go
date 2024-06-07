package business

import (
	"github.com/xiaoshicae/go-easy-extension/core/business"
	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	"github.com/xiaoshicae/go-easy-extension/test/extension_point"
	"github.com/xiaoshicae/go-easy-extension/test/param"
)

// Business1 业务1
type Business1 struct {
}

// Code 业务code(业务身份)
func (a *Business1) Code() string {
	return "ecom.business.business_1"
}

// Match 业务匹配生效的条件
func (a *Business1) Match(p interface{}) bool {
	return p.(param.MyParam).Name == "business_1"
}

// Priority 业务优先级(业务实现了一组扩展点，能力也实现了一组扩展点，因此扩展点实现存在冲突，需要通过优先级的策略来解决冲突)
func (a *Business1) Priority() int64 {
	return 100
}

// UsedAbilities 业务使用了哪些能力(同时还需要指定能力的优先级)
func (a *Business1) UsedAbilities() []business.UsedAbility {
	return []business.UsedAbility{
		{
			AbilityCode: "ecom.ability.ability_1",
			Priority:    200,
		},
	}
}

// ImplementExtensions 业务实现了哪些扩展点(必须通过new(扩展点接口方式)指定类型，否则无法正确匹配)
func (a *Business1) ImplementExtensions() []ei.ExtensionType {
	return []ei.ExtensionType{new(extension_point.Extension1)}
}

// DoSomething1 业务1实现了扩展点1的方法
func (a *Business1) DoSomething1() string {
	return "business_1 do something1"
}
