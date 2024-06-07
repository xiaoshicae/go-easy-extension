package ability

import (
	"strings"

	ei "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	"github.com/xiaoshicae/go-easy-extension/test/extension_point"
	"github.com/xiaoshicae/go-easy-extension/test/param"
)

// Ability1 能力1
type Ability1 struct {
}

// Code 能力code
func (a *Ability1) Code() string {
	return "ecom.ability.ability_1"
}

// Match 能力匹配生效的条件
func (a *Ability1) Match(p interface{}) bool {
	return strings.Contains(p.(param.MyParam).Extra, "ability_1")
}

// ImplementExtensions 能力实现了哪些扩展点(必须通过new(扩展点接口方式)指定类型，否则无法正确匹配)
func (a *Ability1) ImplementExtensions() []ei.ExtensionType {
	return []ei.ExtensionType{new(extension_point.Extension1)}
}

// DoSomething1 能力1实现了扩展点1的方法
func (a *Ability1) DoSomething1() string {
	return "ability_1 do something1"
}
