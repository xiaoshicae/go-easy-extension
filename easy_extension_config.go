package easy_extension

import "github.com/xiaoshicae/go-easy-extension/core"

// SetEnableLogger 设置是否开启日志
func SetEnableLogger(enable bool) {
	core.SetEnableLogger(enable)
}

// SetMatchBusinessStrict 设置是否严格匹配业务
// true: 严格匹配业务，必须匹配到唯一一个业务，否则报错
// false: 非严格匹配业务，未匹配到任何业务时，走默认能力兜底； 匹配到多个业务时，返回第一个匹配的业务
func SetMatchBusinessStrict(strict bool) {
	core.SetMatchBusinessStrict(strict)
}
