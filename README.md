# go-easy-extension

用扩展点组织多业务定制逻辑，参考 Java [easy-extension](https://github.com/xiaoshicae/easy-extension) 的业务 / 能力 / 默认实现模型，以 Go 接口和显式注册表达。

需要 Go 1.27。模块路径为 `github.com/xiaoshicae/go-easy-extension/v3`，包名 `easyext`：

```sh
go get github.com/xiaoshicae/go-easy-extension/v3
```

v3 与 v2.0.0 不兼容，不提供兼容层，迁移见 [从 v2 升级](#从-v2-升级)。

## 核心模型

- Point 是非空 Go 接口，规定“做什么”，不拥有业务或能力。
- Ability 自带 `Code()` 和 `Match(P)`，实现一个或多个 Point，可以被多个 Business 复用。
- Business 自带 `Code()`、`Match(P)`、`Abilities() []string`，声明使用哪些能力及优先顺序，也可以自己实现零个或多个 Point。
- Default 是独立组件，可以同时实现多个 Point。每个已注册 Point 恰好一个默认实现。

```go
type Ability[P any] interface {
    Matcher[P]
    Code() string
}

type Business[P any] interface {
    Matcher[P]
    Code() string
    Abilities() []string
}
```

`Abilities()` 返回已注册能力的 Code；顺序就是优先级，越靠前越优先。`easyext.Self` 表示业务自身的位置，省略时业务自身排最前。**启动代码只注册实例，业务自己声明能力编排。**

## 完整、可运行的示例

源文件：[examples/shop/main.go](examples/shop/main.go)。下面代码与源文件保持一致，并由测试检查；直接运行：

```sh
go run ./examples/shop
```

| 组件 | 实现的 Point | 业务声明的顺序 |
|---|---|---|
| CommerceDefaults | Freight、Delivery、Notify | — |
| FreeShipping | Freight | — |
| RapidDelivery | Delivery、Notify | — |
| Fresh | Freight、Delivery | FreeShipping → RapidDelivery → Self |
| Retail | Freight、Notify | FreeShipping → Self → RapidDelivery |

RapidDelivery 被两个业务复用，同时覆盖两个 Point；两个业务对自身和能力的优先顺序不同。

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

const (
	FreeShippingCode  = "ability.free-shipping"
	RapidDeliveryCode = "ability.rapid-delivery"
)

type OrderParam struct {
	Biz          string
	FreeShipping bool
	Rapid        bool
}

type Freight interface{ CalcFreight(items int) int }
type Delivery interface{ DeliveryDays() int }
type Notify interface{ Notification() string }

// One default component implements all three points.
type CommerceDefaults struct{}

func (*CommerceDefaults) CalcFreight(int) int  { return 8 }
func (*CommerceDefaults) DeliveryDays() int    { return 3 }
func (*CommerceDefaults) Notification() string { return "standard" }

type FreeShipping struct{}

func (*FreeShipping) Code() string            { return FreeShippingCode }
func (*FreeShipping) Match(p OrderParam) bool { return p.FreeShipping }
func (*FreeShipping) CalcFreight(int) int     { return 0 }

// One ability implements two points and is reused by both businesses.
type RapidDelivery struct{}

func (*RapidDelivery) Code() string            { return RapidDeliveryCode }
func (*RapidDelivery) Match(p OrderParam) bool { return p.Rapid }
func (*RapidDelivery) DeliveryDays() int       { return 1 }
func (*RapidDelivery) Notification() string    { return "express" }

type Fresh struct{}

func (*Fresh) Code() string              { return "biz.fresh" }
func (*Fresh) Match(p OrderParam) bool   { return p.Biz == "fresh" }
func (*Fresh) CalcFreight(items int) int { return 15 + 2*items }
func (*Fresh) DeliveryDays() int         { return 2 }
func (*Fresh) Abilities() []string {
	return []string{FreeShippingCode, RapidDeliveryCode, easyext.Self}
}

type Retail struct{}

func (*Retail) Code() string            { return "biz.retail" }
func (*Retail) Match(p OrderParam) bool { return p.Biz == "retail" }
func (*Retail) CalcFreight(int) int     { return 6 }
func (*Retail) Notification() string    { return "retail" }
func (*Retail) Abilities() []string {
	return []string{FreeShippingCode, easyext.Self, RapidDeliveryCode}
}

func run(out io.Writer) error {
	registry, err := easyext.New[OrderParam]().
		Point[Freight]().Point[Delivery]().Point[Notify]().
		Default(&CommerceDefaults{}).
		Ability(&FreeShipping{}).Ability(&RapidDelivery{}).
		Business(&Fresh{}).Business(&Retail{}).
		Build()
	if err != nil {
		return err
	}
	for _, param := range []OrderParam{
		{Biz: "fresh"},
		{Biz: "fresh", FreeShipping: true, Rapid: true},
		{Biz: "retail", Rapid: true},
	} {
		result, err := registry.Resolve(param)
		if err != nil {
			return err
		}
		freight, err := result.First[Freight]()
		if err != nil {
			return err
		}
		delivery, err := result.First[Delivery]()
		if err != nil {
			return err
		}
		notify, err := result.First[Notify]()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s: freight=%d delivery=%dd notify=%s\n",
			param.Biz, freight.CalcFreight(3), delivery.DeliveryDays(), notify.Notification()); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

输出：

```text
fresh: freight=21 delivery=2d notify=standard
fresh: freight=0 delivery=1d notify=express
retail: freight=6 delivery=1d notify=retail
```

Fresh 的急速达能力优先于业务自身；Retail 的通知实现优先于急速达能力。注册调用的先后顺序不影响这些规则。

## 装配

`New[P]().Point[E]().Default(impl).Ability(impl).Business(impl).Build()` 统一校验并生成不可变的 `*Registry[P]`。

- `Point[E]()` 只注册契约；E 必须是非空接口。
- `Default(impl)` 推导 impl 实现的所有已注册 Point，并分别绑定默认实现。
- `DefaultFor[E](impl)` 只为 E 绑定默认实现，有编译期参数类型检查；仍需要 `Point[E]()`。
- `Ability(impl)` 和 `Business(impl)` 在编译期检查组件契约，不另传 code 或编排参数。
- `Code()` 必须非空、无首尾空白，不等于 `Self`，并在整个 Registry 内唯一；同一指针不能声明不同 Code。能力 Code 建议定义为共享 SDK 中的常量。
- 使用指针接收者的组件应注册指针；仅指针实现了 Point 却按值注册时，Build 会报错。

实现关系按实际注册类型的 Go 方法集合推导，不需要手写 `ImplementExtensions()`。只推导已注册的接口；同方法集合的不同接口可能被同一类型同时满足。希望只为其中一个接口提供默认实现时使用 `DefaultFor`，不要用类型名字符串推断身份。

## 请求解析与查询

每个请求必须恰好匹配一个 Business。没有匹配返回 `ErrNoBusinessMatched`，多个匹配返回 `ErrMultipleBusinessesMatched`；不会根据业务注册顺序选择赢家，也没有绕过 Match 的业务路由器。

与 Java 版的差异：Java 5.0 的 `allow-unknown-business=true` 允许没有业务匹配的请求全部走默认实现；Go 版没有这个开关，只有严格模式。需要兜底时，在调用方处理 `ErrNoBusinessMatched`(例如 httpx 默认返回 422)。

`Resolve(param)` 调用每个业务的 Match 一次，然后只调用选中业务声明的能力的 Match。每个能力一次性激活它实现的所有 Point，未声明的能力不会参与请求。查询、诊断和重复遍历都不会重新匹配。

- `result.First[E]() (E, error)`：按业务声明顺序取第一个激活的实现，没有则取默认实现。
- `result.All[E]() (iter.Seq[E], error)`：按顺序遍历激活实现，再加入默认实现；支持提前停止。
- E 未注册时，查询统一返回 `ErrExtensionNotFound`，不会因查询未注册 Point 而 panic。
- All 对共享 provider 去重，保留首次激活的位置。跨角色复用同一指针时视为同一 provider（遵循 Go 指针相等语义）；值注册是独立项，不按值相等合并。默认角色不会受同对象的能力 Match 限制。
- 框架只选择实现，不代替调用；实现方法返回 error 不会触发自动降级，All 的调用或聚合语义由业务流程决定。

## context 绑定与 HTTP

不想显式传递 Resolution 时，每个请求绑定一次，下游使用同一个 Registry 查询：

```go
ctx, err := registry.Bind(ctx, param)
if err != nil {
    return err
}
freight, err := registry.First[Freight](ctx)
if err != nil {
    return err
}
cost := freight.CalcFreight(3)
```

`registry.From(ctx)` 取得解析快照，`registry.All[E](ctx)` 取得全部实现。没有本 Registry 的绑定时返回 `ErrNoBinding`。

子 context 可以重新绑定同一个 Registry，不影响父 context；不同 Registry 在同一个 context 中独立共存，不会互相覆盖。没有全局当前业务或包级的隐式查询入口。

HTTP 服务使用子包 `httpx`：

```go
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    freight, err := registry.First[Freight](r.Context())
    if err != nil {
        http.Error(w, "extension lookup failed", http.StatusInternalServerError)
        return
    }
    fmt.Fprint(w, freight.CalcFreight(3))
})
mux.Handle("/checkout", httpx.Middleware(registry, deriveParam)(handler))
```

`deriveParam` 从路径或已认证的请求身份生成 P，不应消费 body，也不应把未经授权的客户端能力标记直接当成可信权限。中间件默认参数错误返回 400，无业务匹配返回 422，多业务匹配返回 500；`httpx.OnError` 可自定义处理。只包装需要业务身份的路由。

## 能力约束

能力可独立实现以下可选方法：

```go
func (*Installment) Requires() []string {
    return []string{RiskControlCode}
}
func (*Installment) Excludes() []string {
    return []string{CashOnlyCode}
}
```

Requires 表示同一个 Business 必须显式声明这些能力，Excludes 表示不能共同声明。它们不规定先后顺序，不强制能力同时 Match，不保证某个扩展方法被调用，也不自动补挂或重排。

Build 会检查未知引用、自依赖、自排斥、直接或传递矛盾，以及业务的共同挂载集合；即使能力没有被任何业务使用，自身矛盾也会报错。共同挂载依赖允许无矛盾的环，因为它不是执行 DAG。

## 错误、诊断与并发

`Build()` 聚合所有装配问题，返回 `*RegistrationError`；运行期返回 `*ResolutionError`，可用 `errors.Is` 和 `errors.As` 检查。

`result.Trace()` 展示激活链和被跳过的能力；`result.Explain[E]()` 返回候选及其原因：`selected`、`match-false`、`not-implemented`、`lower-priority`、`duplicate`。`registry.Catalog()` 展示静态关系。返回的诊断切片是副本，不会暴露内部可变状态。

`Logger(slog.Default())` 可启用 debug 日志；关闭 debug 时不构造诊断日志内容。

Build 对 code、能力列表和约束做快照。Registry 和 Resolution 可以并发共享，**但不会冻结组件对象或为组件加锁**。共享组件必须自行保证并发安全；Match 应只使用已准备好的匹配参数进行无副作用判断，不在其中执行外部 I/O。解析期间不要并发修改参数，也不要跨请求复用 Resolution。

## 验证

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
go test -run '^$' -bench . -benchmem
go test -run '^$' -fuzz '^FuzzResolutionMatchesReference$' -fuzztime=10s
go test -run '^$' -fuzz '^FuzzConstraintsMatchReference$' -fuzztime=10s
```

随机测试使用独立的直接接口扫描和位集合闭包，对照查询顺序、共享默认去重、匹配次数及约束可满足性。基准测试覆盖业务数、能力数、Point 数及不同约束下的装配开销；性能请以目标机器的实测为准，不沿用旧 API 的性能数字。

使用 Go 1.27 配套的工具。注意：`go` 自动切换工具链时，PATH 中独立的旧 `gofmt` 不一定随之切换。

[设计文档](doc/design-v3.md) · [Java 实现](https://github.com/xiaoshicae/easy-extension)

## 从 v2 升级

v3 把组件的元数据交还组件自己声明，并取消非严格模式。import 路径改为 `github.com/xiaoshicae/go-easy-extension/v3`，然后按下表修改：

| v2.0.0 | v3 |
|---|---|
| `Build()` 返回 `*easyext.Context[T]` | 返回 `*easyext.Registry[P]` |
| `Point[E](defaultImpl)` | `Point[E]()` 加 `Default(impl)`(覆盖它实现的所有已注册 Point)或 `DefaultFor[E](impl)` |
| `Ability(code, impl, easyext.Requires(...), easyext.Excludes(...))` | 组件实现 `Code() string`，可选实现 `Requires() []string`、`Excludes() []string`，然后 `Ability(impl)` |
| `Business(code, impl, easyext.Abilities(...))` | 组件实现 `Code() string`、`Abilities() []string`，然后 `Business(impl)` |
| `Strict(false)`：没有业务时走默认实现，多个业务时取先注册的 | 删除；每个请求必须恰好匹配一个业务 |
| `BusinessResolver(func(T) (string, bool))` | 删除；每个业务用自己的 `Match` 识别请求 |
| `Reason` 常量 `BusinessNotFound` / `ErrBusinessNotFound` | 删除；`ExtensionNotFound` 的数值从 5 变为 4，请按常量比较 |
| `easyext.WithResolution(ctx, r)` | 删除；用 `registry.Bind(ctx, param)` |
| 包级 `easyext.From` / `First[E]` / `All[E]`(ctx) | `registry.From` / `registry.First[E]` / `registry.All[E]`(ctx)，绑定按 Registry 隔离 |
| `result.First[E]() E`(未注册时 panic) | `result.First[E]() (E, error)`，未注册返回 `ErrExtensionNotFound` |
| `result.All[E]() iter.Seq[E]` | `result.All[E]() (iter.Seq[E], error)` |
| `result.Explain[E]() Explanation` | `result.Explain[E]() (Explanation, error)` |
| `result.Business() (code string, ok bool)` | `result.Business() string`(总有业务) |
| `httpx.Middleware(*easyext.Context[T], ...)` | `httpx.Middleware(*easyext.Registry[P], ...)` |

## License

[Apache 2.0](LICENSE)
