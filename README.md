# Easy-Extension
Easy-Extension框架目标是提高`复杂系统的扩展性`，适用于系统有多个接入方，且不同接入方有定制化的扩展诉求。例如电商交易，履约等中台系统。

# 框架解决的业务场景
![](/doc/target.png)

# 框架使用Demo
```go
package main

type Extension interface {
	DoSomething()
}

func main() {
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
![](/doc/go-sample-1.png)
* 请求命中业务1时，扩展点实现情况
![](/doc/go-sample-2.png)
* 请求命中业务1时，扩展点运行分析
![](/doc/go-sample-3.png)
* 样例源码请参考test文件: [easy-extension-test](/test)

# 文档
请参考java版本的实现: [wiki](https://github.com/xiaoshicae/easy-extension/wiki)

# License
Easy-Extension遵循Apache开源协议，具体内容请参考LICENSE文件。
