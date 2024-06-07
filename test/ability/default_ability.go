package ability

import (
	"github.com/xiaoshicae/go-easy-extension/core"
	ie "github.com/xiaoshicae/go-easy-extension/core/extension_instance"
	et "github.com/xiaoshicae/go-easy-extension/test/extension_point"
)

// DefaultAbility 默认能力，需要实现所有扩展点，作为未获取到任何扩展点实现的兜底逻辑
type DefaultAbility struct {
	// 默认能力必须继承BaseDefaultAbility
	core.BaseDefaultAbility
}

// ImplementExtensions 默认能力实现了哪些扩展点(必须通过new(扩展点接口方式)指定类型，否则无法正确匹配)
// 必须实现所有扩展点，作为未获取到任何扩展点实现的兜底逻辑
func (d *DefaultAbility) ImplementExtensions() []ie.ExtensionType {
	return []ie.ExtensionType{new(et.Extension1), new(et.Extension2), new(et.Extension3)}
}

// DoSomething1 默认能力实现扩展点1的方法
func (d *DefaultAbility) DoSomething1() string {
	return "default ability do something1"
}

// DoSomething2 默认能力实现扩展点2的方法
func (d *DefaultAbility) DoSomething2() string {
	return "default ability do something2"
}

// DoSomething3 默认能力实现扩展点3的方法
func (d *DefaultAbility) DoSomething3() string {
	return "default ability do something3"
}
