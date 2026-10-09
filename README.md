# go-easy-extension

Go 扩展点框架:让多业务接入的系统用**扩展点**替代散落各处的 if-else。Java 版 [easy-extension](https://github.com/xiaoshicae/easy-extension) 的 Go 实现,模型与 Java 4.x 一致。

```go
import "github.com/xiaoshicae/go-easy-extension/v2"   // 包名 easyext
```

> **v2 需要 Go 1.27**(方法上的类型参数 `res.First[Freight]()`)。v2 与 v1 不兼容,见 [设计文档](doc/design-v2.md)。

## 解决什么问题

一个系统接入多个业务方,每个业务方的定制逻辑不同:

```go
if biz == "retail" {
    freight = 0                 // 零售包邮
} else if biz == "fresh" {
    freight = coldChain(order)  // 生鲜冷链运费
} else {
    freight = 8
} // 促销、风控、支付……每个流程都要重复一遍
```

用扩展点:通用流程只依赖接口,不同业务提供各自的实现,框架按当前请求的业务身份选择正确的那个。

## 快速开始

### 1. 定义扩展点、默认实现、能力、业务

```go
// 匹配参数:每个请求"是谁"
type OrderParam struct {
    Biz       string
    Abilities []string
}

// 扩展点:任意接口
type Freight interface{ Calc(items int) int }

// 默认实现:没有业务或能力覆盖时调用
type DefaultFreight struct{}
func (DefaultFreight) Calc(int) int { return 8 }

// 能力:可复用的实现,Match 决定这个请求是否启用
type FreeShipping struct{}
func (FreeShipping) Match(p OrderParam) bool { return slices.Contains(p.Abilities, "free-shipping") }
func (FreeShipping) Calc(int) int            { return 0 }

// 业务:接入方,Match 识别属于它的请求,也可以自己实现扩展点
type Fresh struct{}
func (Fresh) Match(p OrderParam) bool { return p.Biz == "fresh" }
func (Fresh) Calc(items int) int      { return 15 + 2*items }
```

实现了哪些扩展点由类型自动推导,不需要额外声明。

> 注意接收者:方法写在指针上(`func (*Fresh) Calc`)时要注册指针 `&Fresh{}`。按值注册时这些方法不属于值类型,`Build()` 会报错提示,而不是悄悄忽略。

### 2. 启动时装配

```go
c, err := easyext.New[OrderParam]().
    Point[Freight](DefaultFreight{}).                    // 扩展点 + 它的默认实现
    Ability("ability.free-shipping", FreeShipping{}).
    Business("biz.fresh", Fresh{},
        easyext.Abilities("ability.free-shipping", easyext.Self)).
    Build()                                              // 一次性校验全部装配
```

`Abilities` 的**顺序就是优先级**,越靠前越优先;`easyext.Self` 代表业务自己,不写时业务自己排最前。上例包邮排在 `Self` 之前,所以会覆盖生鲜自己的运费。

### 3. 每个请求绑定一次,任何地方取用

```go
ctx, err := c.Bind(ctx, OrderParam{Biz: "fresh"})   // 通常在中间件里
// ……调用栈里任何地方:
freight, err := easyext.First[Freight](ctx)
freight.Calc(3)                                      // 21;带上 free-shipping 时是 0
```

HTTP 服务直接用中间件:

```go
import "github.com/xiaoshicae/go-easy-extension/v2/httpx"

mux.Handle("/checkout", httpx.Middleware(c, func(r *http.Request) (OrderParam, error) {
    return OrderParam{Biz: r.Header.Get("X-Biz")}, nil
})(checkoutHandler))
```

只包住需要扩展点的路由,健康检查等路由不需要业务身份。

## 核心概念

| 概念 | 写法 | 说明 |
|---|---|---|
| 扩展点 | 任意接口,`Point[E](默认实现)` | 规定"做什么";注册时必须给默认实现,编译期检查类型 |
| 能力 | `Ability(code, impl, Requires(...), Excludes(...))` | 可被多个业务复用的实现,实现 `Matcher[T]` 决定何时生效 |
| 业务 | `Business(code, impl, Abilities(...))` | 接入方,实现 `Matcher[T]` 识别请求,挂载能力,也可以自己实现扩展点 |
| 默认实现 | `Point` 的参数 | 兜底,每个扩展点恰好一个,保证永远可调用 |

**解析顺序**:`Abilities` 里的能力(只保留 `Match` 为 true 的)与 `Self` 按声明顺序 → 扩展点的默认实现。`First` 取第一个实现了该扩展点的,`All` 按顺序取全部(`iter.Seq`)。

### 严格模式

默认**每个请求必须恰好匹配一个业务**,否则 `Bind` / `Resolve` 返回错误(`ErrNoBusinessMatched` / `ErrMultipleBusinessesMatched`)。`.Strict(false)` 后:无匹配时只走默认实现;多个匹配时取最先注册的那个。

### 按 code 直达业务

```go
easyext.New[OrderParam]().
    BusinessResolver(func(p OrderParam) (string, bool) { return "biz." + p.Biz, p.Biz != "" })
```

配置后不再调用业务的 `Match`(业务类型仍需有 `Match` 方法,保证编译期类型检查)。

### 排查

```go
r, _ := c.Resolve(param)
fmt.Println(r)               // Resolution[business=biz.fresh, chain=[biz.fresh], skipped=[ability.free-shipping]]
r.Explain[Freight]()         // 该扩展点的所有候选,以及最终选了谁
r.Trace()                    // 命中的业务、解析链、被跳过的能力
c.Catalog()                  // 全部扩展点/能力/业务的只读描述
```

`.Logger(slog.Default())` 后,每次解析在 debug 级别输出一行日志。

### 错误

`Build()` 一次性报出全部装配问题(`*easyext.RegistrationError`)。运行期错误是 `*easyext.ResolutionError`,用 `errors.Is(err, easyext.ErrNoBusinessMatched)` 等判断。

## 并发

`Build()` 得到的 `*Context[T]` 与每次解析得到的 `*Resolution` 都不可变,可以在 goroutine 间共享。绑定跟随 `context.Context`:子 context 可以再绑定别的业务而不影响父 context,把 ctx 传给新的 goroutine 即带上绑定。

## 性能

Apple M 系列上的参考值(`go test -bench .`):

| 操作 | 耗时 | 分配 |
|---|---|---|
| `Resolve`(一次请求解析一次) | ~45 ns | 2 次 |
| `First[E]` / `easyext.First[E](ctx)` | ~13 ns | 0 |

## 文档

[设计文档](doc/design-v2.md) · [Java 版 Wiki](https://github.com/xiaoshicae/easy-extension/wiki)(概念相同)

## License

[Apache 2.0](LICENSE)
