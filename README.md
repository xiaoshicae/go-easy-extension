# Easy-Extension
Easy-Extension框架目标是提高`复杂系统的扩展性`，适用于系统有多个接入方，且不同接入方有定制化的扩展诉求。例如电商交易，履约等中台系统。

# 框架特点
* 轻量易用
* 可以实现业务逻辑和平台逻辑分离，提高提供扩展性和稳定性

# 框架解决的业务场景
![](/doc/target.png)

# 框架使用Demo
```go
package main

import (
	ee "github.com/xiaoshicae/go-easy-extension"
)

type Extension interface {
	DoSomething()
}

func main() {
	// 注册默认能力，为匹配到任何扩展点实现时，默认兜底实现
	_ = ee.RegisterDefaultAbility(ctx, &ability.DefaultAbility{})
	// 注册能力
	_ = ee.RegisterAbility(ctx, &ability.Ability1{})
	// 注册业务
	_ = ee.RegisterBusiness(ctx, &business.Business1{})

	// 向你的ctx注入session，用于保存一起请求匹配的上下文信息
	// http rpc框架，逻辑可以放在middleware
	ctx = ee.NewCtxWithSession(ctx)
    
	// 每次请求都需要初始化session，MyParam用于业务和能力匹配
	// http rpc框架，逻辑可以放在middleware
	_ = ee.InitSession(ctx, param.MyParam{
		Name:  "business_1", 
		Extra: "ability_1 & ability_2",
	})

	// 根据匹配到的业务，动态获取Extension的实现
	ext, err := ee.GetFirstMatchedExtension[Extension](ctx)
	if err != nil {
		panic(err)
	}
	res := ext.DoSomething()
	fmt.Println(res)
}
```

# 代码样例
* 业务及能力扩展点实现情况
<img src="/doc/go-sample-1.png"  width="600" />
* 请求命中业务1时，扩展点实现情况
<img src="/doc/go-sample-2.png"  width="600" />
* 请求命中业务1时，扩展点运行分析
<img src="/doc/go-sample-3.png"  width="600" />

* 样例源码请参考test文件: [easy-extension-test](/test)

# 文档
请参考java版本的实现: [wiki](https://github.com/xiaoshicae/easy-extension/wiki)

# License
Easy-Extension遵循Apache开源协议，具体内容请参考LICENSE文件。
