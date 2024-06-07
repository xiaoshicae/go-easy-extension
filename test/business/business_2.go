package business

import (
	"github.com/xiaoshicae/go-easy-extension/core/business"
	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	"github.com/xiaoshicae/go-easy-extension/test/ability"
	"github.com/xiaoshicae/go-easy-extension/test/extension_point"
	"github.com/xiaoshicae/go-easy-extension/test/param"
)

// Business2 业务2
type Business2 struct {
}

// Code 业务code
func (a *Business2) Code() string {
	return "ecom.business.business_2"
}

// Match 业务匹配生效的条件
func (a *Business2) Match(p interface{}) bool {
	return p.(param.MyParam).Name == "business_2"
}

// Priority 业务优先级
func (a *Business2) Priority() int64 {
	return 100
}

// UsedAbilities 业务使用了哪些能力
func (a *Business2) UsedAbilities() []business.UsedAbility {
	return []business.UsedAbility{
		{
			AbilityCode: (&ability.Ability1{}).Code(),
			Priority:    50,
		},
		{
			AbilityCode: (&ability.Ability2{}).Code(),
			Priority:    10,
		},
	}
}

// ImplementExtensions 业务实现了哪些扩展点
func (a *Business2) ImplementExtensions() []ei.ExtensionType {
	return []ei.ExtensionType{new(extension_point.Extension1), new(extension_point.Extension2)}
}

// DoSomething1 业务2实现了扩展点1的方法
func (a *Business2) DoSomething1() string {
	return "business_2 do something1"
}

// DoSomething2 业务2实现了扩展点2的方法
func (a *Business2) DoSomething2() string {
	return "business_2 do something2"
}
