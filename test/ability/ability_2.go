package ability

import (
	"strings"

	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	"github.com/xiaoshicae/go-easy-extension/test/extension_point"
	"github.com/xiaoshicae/go-easy-extension/test/param"
)

// Ability2 能力2
type Ability2 struct {
}

// Code 能力code
func (a *Ability2) Code() string {
	return "ecom.ability.ability_2"
}

// Match 能力匹配生效的条件
func (a *Ability2) Match(p interface{}) bool {
	return strings.Contains(p.(param.MyParam).Extra, "ability_2")
}

// ImplementExtensions 能力实现了哪些扩展点(必须通过new(扩展点接口方式)指定类型，否则无法正确匹配)
func (a *Ability2) ImplementExtensions() []ei.ExtensionType {
	return []ei.ExtensionType{new(extension_point.Extension1), new(extension_point.Extension2)}
}

// DoSomething1 能力2实现了扩展点1的方法
func (a *Ability2) DoSomething1() string {
	return "ability_2 do something1"
}

// DoSomething2 能力2实现了扩展点2的方法
func (a *Ability2) DoSomething2() string {
	return "ability_2 do something2"
}
