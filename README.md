# go-easy-extension

用 Go 接口组织多业务定制逻辑：业务决定能力编排，框架按请求选择实现。领域模型参考 Java [easy-extension](https://github.com/xiaoshicae/easy-extension)。

需要 Go 1.27。模块路径为 `github.com/xiaoshicae/go-easy-extension/v3`，包名 `easyext`：

```sh
go get github.com/xiaoshicae/go-easy-extension/v3
```

v3 不兼容旧 API，迁移见 [升级说明](doc/design-v3.md#11-从-v2-升级)。

## 核心模型

- Point：非空 Go 接口，定义扩展方法。
- Ability：可复用能力，实现 `Code()`、`Match(P)` 及一个或多个 Point。
- Business：实现 `Code()`、`Match(P)`、`Abilities() []string`，声明能力顺序，也可实现零个或多个 Point。
- Default：独立的兜底组件，可实现多个 Point；每个已注册 Point 恰好有一个默认实现。

Business 与 Ability、组件与 Point 都支持多对多关系。框架根据 Go 接口的方法集合识别实现关系，只检查已注册的 Point。

业务自己编排能力，启动代码只注册实例。例如 Fresh：

```go
func (*Fresh) Abilities() []string {
	return []string{FreeShippingCode, RapidDeliveryCode, easyext.Self}
}
```

列表引用已注册能力的 Code，越靠前越优先；`easyext.Self` 表示业务自身的位置，省略时自身排最前。

## 快速开始

在仓库根目录运行商店示例：

```sh
go run ./examples/shop
```

```text
fresh: freight=21 delivery=2d notify=standard
fresh: freight=0 delivery=1d notify=express
retail: freight=6 delivery=1d notify=retail
```

示例按 [扩展点契约](examples/shop/points.go)、[组件](examples/shop/components.go)、[Service](examples/shop/service.go)、[启动入口](examples/shop/main.go) 分层。下面只展示关键片段，完整代码见这些文件。

| 组件 | 实现的 Point | 业务声明的顺序 |
|---|---|---|
| CommerceDefaults | FreightCalc、Delivery、Notify | — |
| FreeShipping | FreightCalc | — |
| RapidDelivery | Delivery、Notify | — |
| Fresh | FreightCalc、Delivery | FreeShipping → RapidDelivery → Self |
| Retail | FreightCalc、Notify | FreeShipping → Self → RapidDelivery |

FreeShipping 根据会员、金额和地区匹配；RapidDelivery 根据加急、地区和件数匹配。调用方提供请求事实，不传“启用哪个能力”的开关。

### 1. 定义扩展点

示例采用常见的 ctx / error 签名，方法只接收自身需要的业务参数；匹配参数 P 用于选择实现。Point 的方法签名由应用决定，框架不额外限制。

```go
type FreightCalc interface {
	CalcFreight(ctx context.Context, province string, items int) (int, error)
}
```

### 2. 启动时装配

分别注册契约、默认实现、能力和业务，`Build` 校验后返回不可变的 Registry：

```go
registry, err := easyext.New[OrderParam]().
	Point[FreightCalc]().Point[Delivery]().Point[Notify]().
	Default(&CommerceDefaults{}).
	Ability(&FreeShipping{}).Ability(&RapidDelivery{}).
	Business(&Fresh{}).Business(&Retail{}).
	Build()
if err != nil {
	return err
}
```

Service 注入 Registry，在每次调用中按当前 ctx 查询 Point，不缓存某次请求选中的实现：

```go
type CheckoutService struct {
	registry *easyext.Registry[OrderParam]
}
```

```go
service := NewCheckoutService(registry)
```

### 3. 请求入口绑定

沿用入口已有的 `ctx`，绑定一次后把返回的 `ctx` 传给所有下游调用：

```go
param := OrderParam{Biz: "fresh", Amount: 100, Province: "广东省", Items: 3}
ctx, err := registry.Bind(ctx, param)
if err != nil {
	return err
}
quote, err := service.Checkout(ctx, param.Province, param.Items)
if err != nil {
	return err
}
fmt.Println(quote.Freight) // 21
```

HTTP 接入由下面的中间件完成绑定，Handler 无需手动 Bind。

### 4. Service 查询扩展点，再调用

`First[E](ctx)` 返回普通 Point 接口。例如 [CheckoutService](examples/shop/service.go) 的运费部分：

```go
freight, err := s.registry.First[FreightCalc](ctx)
if err != nil {
	return Quote{}, err
}
cost, err := freight.CalcFreight(ctx, province, items)
if err != nil {
	return Quote{}, err
}
```

配送和通知同样查询并调用。Service 不需要请求级 `result` 包装；单元测试可以把 fake 实现注册到测试 Registry。

## 匹配与查询

- 每个请求必须恰好匹配一个 Business；否则返回 `ErrNoBusinessMatched` 或 `ErrMultipleBusinessesMatched`。
- Bind 对每个 Business 匹配一次，再匹配选中业务声明的能力。First / All 查询不会重新 Match。
- `registry.First[E](ctx) (E, error)` 选择第一个激活的实现，没有则使用默认实现。
- 方法由调用方执行；框架不因实现返回 error 自动降级，也不恢复组件 panic。
- 绑定按 Registry 隔离；必须使用对应 Registry 的 ctx，缺少绑定返回 `ErrNoBinding`。跨 goroutine 也显式传递这个 ctx。

需要多个实现时，用 `registry.All[E](ctx) (iter.Seq[E], error)` 按业务声明顺序遍历激活实现，最后加入默认实现。共享实现去重，聚合和错误处理由业务流程决定：

```go
notifications, err := registry.All[Notify](ctx)
if err != nil {
	return err
}
for notify := range notifications {
	message, err := notify.Notification(ctx, "order.created")
	if err != nil {
		return err
	}
	fmt.Println(message)
}
```

查询未注册的 Point 返回 `ErrExtensionNotFound`，不产生查询 panic。父子 ctx、去重等详细语义见 [请求解析与绑定](doc/design-v3.md#6-请求解析)。

## HTTP 接入

`httpx.Middleware` 从请求生成匹配参数并绑定 `r.Context()`。Handler 调用使用同一个 Registry 的 Service：

```go
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	quote, err := service.Checkout(r.Context(), "广东省", 3)
	if err != nil {
		http.Error(w, "checkout failed", http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, quote.Freight)
})
mux.Handle("/checkout", httpx.Middleware(registry, deriveParam)(handler))
```

`deriveParam` 的签名为 `func(*http.Request) (OrderParam, error)`，应使用路径或已认证身份等可信输入，不消费 body，也不把客户端能力标记当成权限。只包装需要业务身份的路由。

默认参数错误返回 400，无业务匹配返回 422，匹配歧义等内部错误返回 500；用 `httpx.OnError` 自定义处理。

## 使用约束与排查

- `Code()` 非空、无首尾空白、不等于 `easyext.Self`，在 Registry 内唯一。能力 Code 建议使用共享常量。
- 有指针接收者的组件注册指针。默认绑定有重叠时，用 `DefaultFor[E](impl)` 限定绑定范围，仍须先注册 Point。
- 能力可选实现 `Requires() []string` / `Excludes() []string`，声明同一业务中必须或不能共同使用的能力；Build 校验，不自动补挂、重排或强制运行期激活。
- Registry 可以并发共享，组件须自行保证并发安全。Match 应基于准备好的参数无副作用判断，不执行外部 I/O。
- 下游沿用入口 ctx，不重新创建 Background，也不为每个 Point 再次 Bind。
- `Build` 返回聚合的 `*RegistrationError`，运行期选择错误为 `*ResolutionError`，可用 `errors.Is` / `errors.As` 检查。

排查装配关系用 `registry.Catalog()`；排查当前请求用 `registry.From(ctx)` 取快照，再调用 `Trace()` / `Explain[E]()`，不会重新 Match。Builder 的 `Logger(slog.Default())` 可输出 debug 日志。

## 开发与更多资料

```sh
go test -race -count=1 ./...
go vet ./...
```

更多内容见 [设计与约束](doc/design-v3.md)、[升级说明](doc/design-v3.md#11-从-v2-升级)、[基准与 Fuzz 验证](doc/design-v3.md#12-开发验证)。

## License

[Apache 2.0](LICENSE)
