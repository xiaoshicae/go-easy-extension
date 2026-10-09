# go-easy-extension v2 设计

> v2 是一次不兼容的重写:沿用 Java 版 [easy-extension 4.x](https://github.com/xiaoshicae/easy-extension) 的领域模型
> (扩展点 / 能力 / 业务 / 默认实现),用 Go 的方式表达。模块路径 `github.com/xiaoshicae/go-easy-extension/v2`,包名 `easyext`,要求 **Go 1.27**。

## 1. 为什么重写

v1(`v1.0.0`)的问题,均已实测复现,并在 v2 中作为回归测试保留:

| # | v1 的问题 | v2 的解法 |
|---|---|---|
| 1 | 扩展点用 `reflect.TypeOf(new(I)).String()` 作 key,只含包短名:不同包里的同名接口(都是 `*ext.Point`)被当成同一个扩展点 | 用 `reflect.TypeFor[E]()`(含完整导入路径)标识扩展点 |
| 2 | 业务存在 map 里,非严格模式下多个业务同时匹配时随机选一个 | 业务按注册顺序保存,非严格模式下取最先注册的那个 |
| 3 | session 是放在 `context.Context` 里的**可变指针**,子 context 上 `RemoveSession` 会清掉父请求的结果 | 解析结果 `Resolution` 不可变,绑定就是 `context.WithValue` 派生出的新 context,天然隔离;不再有 Remove |
| 4 | 并发 `InitSession` 有数据竞争(`go test -race` 可复现) | 构建后的 `Context` 只读,`Resolution` 不可变,无共享可变状态 |
| 5 | 全局单例 + `SetEnableLogger` 等全局开关,测试之间互相污染 | 没有全局状态,`New[T]()...Build()` 得到独立的 `*Context[T]` |
| 6 | `Match(param interface{})` 需要类型断言,传错 panic;`BaseDefaultAbility.ImplementExtensions()` 是 `panic("implement me")` | `Matcher[T]` 泛型,编译期检查;默认实现按扩展点注册,编译期检查类型 |
| 7 | 每个实现手写 `ImplementExtensions()` 并用 `new(接口)` 的技巧声明 | 从实现类型自动推导它实现了哪些已注册的扩展点 |
| 8 | 业务挂载了不存在的能力,要到请求时才报错 | `Build()` 一次性校验全部装配 |
| 9 | 数字优先级、单一"默认能力"必须实现所有扩展点 | `Abilities(...)` 的顺序即优先级、`Self` 标记业务自身;每个扩展点各自一个默认实现 |
| 10 | `go 1.18`、`go.sum` 残留无用依赖、无 CI | Go 1.27、零第三方依赖、CI(vet / race / govulncheck) |

## 2. 为什么是 Go 1.27

泛型从 1.18 就有了(v1 已经在用),选 1.27 是因为 **1.27 允许方法声明类型参数**(语法 `MethodDecl = "func" Receiver MethodName [ TypeParameters ] Signature`,1.26 编译报 `method must have no type parameters`)。这让 API 能写成和 Java 版一一对应的形式:

```go
res.First[Freight]()          // Java: resolution.first(Freight.class)
b.Point[Freight](defaultImpl)  // Java: @DefaultImplementation
```

接口方法仍然不能带类型参数,所以这些都是具体类型上的方法。其他用到的版本特性:`reflect.TypeFor`(1.22)、`iter.Seq`(1.23)、`slog.DiscardHandler`(1.24)、`sync.WaitGroup.Go` / `testing/synctest`(1.25)。

## 3. 模型对照

| Java 4.x | go-easy-extension v2 |
|---|---|
| `@ExtensionPoint` 接口 | 任意 Go 接口类型 |
| `@DefaultImplementation`,每个扩展点恰好一个 | `b.Point[E](defaultImpl)`:注册扩展点就必须给默认实现。Go 无法在运行时合成接口实现,所以没有"只有 void 方法时框架提供空实现" |
| `@Ability(code)` + `Matcher<T>` | `b.Ability(code, impl)`,`impl` 实现 `Matcher[T]`;`Requires(...)` / `Excludes(...)` |
| `@Business(code, abilities = {A, Self})` | `b.Business(code, impl, Abilities("a", Self))`:顺序即优先级,不写 `Self` 时业务自身排最前 |
| 扩展点从类型层次推导 | 对每个已注册扩展点 `E`,检查实现类型是否实现 `E` |
| 启动期校验 | `Build()` 校验全部装配,返回 `*RegistrationError`(列出所有问题) |
| 不可变 `ExtensionContext` / `Resolution` | `*Context[T]` / `*Resolution`,并发安全 |
| ThreadLocal 绑定、`runWith` | `context.Context`:`ctx, err := c.Bind(ctx, param)`,`easyext.From(ctx)` 取出 |
| `@ExtensionInject` 注入代理 | Go 没有动态代理:在调用处 `easyext.First[E](ctx)` / `easyext.All[E](ctx)` |
| `MatcherParamResolver` | 子包 `httpx`:`net/http` 中间件 |
| `BusinessResolver` / `BusinessSelector` / 严格模式 | `b.BusinessResolver(f)` / 不提供(非严格模式取最先注册的)/ `b.Strict(false)`,默认严格 |
| `trace()` / `explain()` / `catalog()` | `res.Trace()` / `res.Explain[E]()` / `c.Catalog()` |
| `ResolutionException(Reason)` | `*ResolutionError{Reason}`,配合 `errors.Is(err, easyext.ErrNoBusinessMatched)` 等 |
| 日志:`Resolver` DEBUG | `b.Logger(*slog.Logger)`,debug 级别,默认不输出 |

## 4. 语义

- **解析链**:`[业务挂载的能力与业务自身,按 Abilities 顺序;只保留 Match 为 true 的能力] → 扩展点的默认实现`。
- **First[E]**:链中第一个实现了 `E` 的,否则默认实现。`E` 未注册是编程错误,`Resolution.First` panic(`ResolutionError{Reason: ExtensionNotFound}`);包级 `easyext.First[E](ctx)` 以 error 返回。
- **All[E]**:链中所有实现了 `E` 的,按顺序,最后是默认实现;返回 `iter.Seq[E]`。
- **业务选择**:配置了 `BusinessResolver` 时按 code 直达,不调用 `Match`;否则按注册顺序逐个 `Match`。业务注册时的类型就是 `Matcher[T]`,编译期检查。
  - 严格模式(缺省):0 个匹配 `NoBusinessMatched`,多个匹配 `MultipleBusinessesMatched`,resolver 给出未注册的 code `BusinessNotFound`。
  - 非严格模式:没有业务时只走默认实现;多个匹配时取最先注册的(遇到第一个匹配即停止)。
- **能力**是否生效在 `Resolve` 时立即求值,`Resolution` 是快照,不要跨请求复用。

## 5. 设计取舍(v2 首轮 review 后的收敛)

- **不提供 `BusinessSelector`**:只在"非严格 + 多业务匹配"时有用,默认取最先注册的;需要时再加,属于兼容的扩展。
- **`Resolution` 不提供返回 error 的 `Lookup`**:取实现只有两种写法,`res.First[E]()`(未注册 panic)与 `easyext.First[E](ctx)`(返回 error)。
- **`Business` 的参数类型是 `Matcher[T]`**:漏写 `Match` 在编译期发现;配置 `BusinessResolver` 时 `Match` 不会被调用。
- **校验全部在 `Build()`**,包括"方法写在指针接收者上、却按值注册"(Go 特有的坑:这些方法会被悄悄忽略)。
- **`Trace` 不记录耗时**:一次解析约 45 ns,单独计时(两次 `time.Now`,约 28 ns)得不偿失。
- **日志惰性构造**:先判断 `Logger.Enabled(debug)`,关闭时不构造任何日志参数。`Resolve` 从约 148 ns / 8 次分配降到约 45 ns / 2 次分配。
- **httpx 状态码**:无业务 / 未知业务是请求的问题(422);多个业务同时匹配是装配的问题(500)。

## 6. 包结构

```
easyext (根包)   Builder / Context / Resolution / First / All / 错误 / Trace / Catalog
httpx            net/http 中间件
internal/...     测试用的夹具
```

零第三方依赖。
