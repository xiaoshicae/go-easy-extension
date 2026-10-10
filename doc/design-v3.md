# go-easy-extension v3 设计

当前 API 使用 Go 1.27 的具体类型泛型方法，不考虑与旧 API 兼容。参考 Java easy-extension 的领域语义，但不复制所有 Java 接入机制。

## 1. 职责与关系

Point 是契约，Ability 是可复用组件，Business 是请求身份及能力编排，Default 是独立的兜底组件。

- Business — Ability：多对多，业务各自声明能力顺序。
- Ability — Point：多对多，能力实现一个或多个 Point。
- Business — Point：多对多，业务可以实现零个或多个 Point。
- Default — Point：一个组件可以覆盖多个 Point，每个 Point 恰好一个默认实现。

Point 是查询入口，不是业务装配的组织单元。业务编排属于 Business，而不是启动代码或 Point。

## 2. 自描述组件

```go
type Matcher[P any] interface {
    Match(P) bool
}

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

Business 必须实现 Matcher，不提供 BusinessResolver。业务始终通过自己定义的 Match 识别请求，不保留两套路由路径或空实现 Match。

Abilities 返回能力 Code 列表，用户无需理解 Mount、ProviderRef 或 Composition 等额外类型和术语。Self 是保留的字符串常量，标记业务自身的位置；缺省放在列表最前。顺序就是优先级，不引入全局数字 Priority。

RequiresAbilities 和 ExcludesAbilities 是独立的小接口，分别识别可选的 Requires / Excludes 方法；无约束组件不需要空方法。

Code 显式、稳定、跨业务及能力唯一，不从类型名生成。共享 SDK 导出能力 Code 常量，业务使用常量引用能力；不在 Abilities 内实例化能力，也不自动注册引用到的组件。

启动负责选用模块、构造组件和注入依赖；业务的可配置编排可通过构造函数传入，在 Build 时冻结为元数据快照。

## 3. 独立注册契约和默认实现

```go
registry, err := easyext.New[OrderParam]().
    Point[Freight]().Point[Delivery]().Point[Notify]().
    Default(&CommerceDefaults{}).
    Ability(&FreeShipping{}).Ability(&RapidDelivery{}).
    Business(&Fresh{}).Business(&Retail{}).
    Build()
```

所有调用只收集声明，注册先后不影响解析结果。Build 先取得完整契约集合，再推导实现关系。DefaultFor[E](impl) 显式限制默认绑定，避免同方法集合接口或多接口组件造成意外重叠。

Point 必须是非空接口。空接口没有领域契约，会使所有组件都成为候选，因此拒绝。默认实现不能缺失或冲突；不隐式合成零值或 no-op。需要 no-op 的通知类 Point 应显式提供实现。

Go 采用结构化接口，无法从一个实现类型中枚举“显式 implements 过的接口”。只对已注册 Point 使用 Type.Implements 推导，不承诺 Java 注解扫描中的“未注册 Point 检测”。接口身份使用 reflect.Type，类型字符串仅用于显示；不同语义的接口应通过方法名称或参数类型区分。

## 4. 装配编译

Build 每次创建独立的校验器和 Registry，不在 Builder 上积累上次的错误。

阶段顺序：

1. 检查并收集 Point：非空接口、无重复。
2. 绑定默认实现：自动推导或 DefaultFor 限制，每个 Point 恰好一个。
3. 读取能力 metadata，检查 code、nil、实现关系和可选约束。
4. 校验全部能力的引用及可满足性，包括未被业务使用的能力。
5. 读取业务 metadata，检查 code 和能力顺序，校验共同挂载集合。
6. 编译每个业务的稀疏 Point 候选索引及 provider 身份。

注册时的接口参数让 Code、Match(P)、Abilities 的缺失或错误签名在编译期失败。Build 处理值级问题：typed nil、空 Code、保留标记、重复引用、默认冲突等，并聚合错误。nil 检查在读取方法前进行，覆盖指针、函数、map、slice 和 channel 等 nil 值。

元数据方法每次合法注册只读取一次；返回的 slice 全部复制。Build 后追加注册或修改原始 metadata 不影响现有 Registry。实际组件不克隆，它们的内部状态和 Match 行为仍由实现者负责。

没有 Excludes 时，只需检查引用及业务共同挂载集合，不遍历依赖闭包：单纯共同挂载依赖始终可满足。有 Excludes 时，闭包检查使用带根节点标记的 visited 数组和可复用工作队列，避免为每个能力分配 map，也不使用递归；冲突按注册顺序报告。

## 5. 约束语义

Requires / Excludes 是静态共同挂载规则：

- Requires 不暗含运行期激活，不保证执行，也不规定先后顺序。
- Excludes 拒绝同业务中的共同挂载，不只是同时 Match。
- 引用必须是注册的 Ability，不能引用 Business 或 Self。
- 自依赖、自排斥、重复引用、直接或传递矛盾在 Build 报错。
- 验证每个能力的依赖闭包是否包含排斥冲突，即使从未挂载。
- 允许 A requires B、B requires A 的无矛盾共同挂载组；不做执行拓扑排序。
- 不自动补挂缺失能力，不改变业务声明的顺序。

这样的模型是实现选择器，不是工作流引擎。真正的执行依赖和错误降级由扩展点协议及业务流程表达。

## 6. 请求解析

每个请求必须恰好匹配一个业务，0 个返回 NoBusinessMatched，多个返回 MultipleBusinessesMatched。没有 Strict(false)、注册顺序择优或备用路由器。多个匹配时直接使用已收集结果报错，不重新 Match。

选中业务后只评价它使用的能力，每个能力一次，将其所有 Point 一起激活或关闭。结果存储业务的静态 plan 与只读 activation 数组，形成 Resolution 快照；不保存用于后续重新匹配的参数。

实现类型扫描、接口关系推导、字符串能力引用解析都在 Build 完成。First / All 使用按 Point 编译的稀疏候选索引，不在每次查询时扫描所有实现类型或所有 Point。

- First：第一个激活的候选，否则默认实现。
- All：按同一顺序返回全部激活候选及默认实现，按 provider ID 去重。
- 每个查询统一返回 error；未注册 Point 不再产生查询 panic。
- All 返回可重复遍历的 iter.Seq，支持提前退出，不触发 Match 或修改快照。

组件指针按 Go 指针值相等语义共用 provider ID；同一指针不能在一次装配中声明不同 Code。非指针注册项独立，不比较任意接口值，也不按 reflect.DeepEqual 合并。零大小类型的不同分配不保证不同指针值，不以分配次数作为实例身份。默认角色始终可用，即使同对象作为能力时未匹配。

组件 Code 唯一、同指针 Code 一致、业务引用不重复，共同保证每个业务候选链中的 provider 不重复。All 只需识别默认实现是否已作为激活候选出现，无需维护逐查询去重集合。

## 7. Registry 隔离绑定

Registry 和 context.Context 是不同概念，使用明确的 Registry 名称。

```go
ctx, err := registry.Bind(ctx, param)
impl, err := registry.First[Freight](ctx)
result, err := registry.From(ctx)
```

每次 Build 分配独立、非零大小的私有绑定 key：

- 同 Registry 的子 context 重绑定只覆盖子作用域。
- 不同 Registry 同时绑定一个 context，互不覆盖。
- From / First / All 只能读取调用者所属 Registry 的绑定。
- 绑定失败返回原 context，不清理或修改已有绑定。
- 没有包级 From / First / All，也没有全局当前业务或 ThreadLocal。
- 跨 goroutine 必须显式传递 context 或 Resolution。

httpx 使用传入的 Registry 绑定，处理器使用相同 Registry 查询。参数生成失败返回 400，无业务匹配返回 422，匹配歧义返回 500，详细信息默认不泄露给客户端。支持 OnError 自定义处理。

## 8. 可观测性与并发

Catalog 返回静态 Point、Ability、Business 关系的副本；Trace 返回激活链与跳过能力的副本；Explain 返回声明顺序中的全部候选，包括未激活者及未实现查询 Point 者。

Explain 原因包括 selected、match-false、not-implemented、lower-priority、duplicate。诊断不会执行 Match；修改返回值不影响 Registry 或 Resolution。

关闭 debug 时不生成日志诊断对象。不记录无条件的逐请求耗时，性能以 benchmark 验证。

Registry 和 Resolution 的结构不可变，但它们共享用户提供的组件。框架不承诺组件的线程安全，不在任意扩展方法周围加锁，不拦截组件 panic。Match 应是对稳定输入的无副作用判断；涉及外部数据时，应先取得数据再生成匹配参数。

## 9. 验收

测试覆盖：

- 多业务复用能力，单个业务及能力实现多个 Point，一个 Point 多能力且业务内顺序不同。
- 注册顺序无关、多 Point 默认、DefaultFor 限制、默认冲突。
- 未挂载能力不匹配，匹配一次，包括多业务错误路径与重复遍历。
- 全部静态约束、无矛盾依赖环、typed nil、指针接收者。
- metadata 快照、诊断副本、共享指针去重、非可比较组件。
- 未注册 Point 的统一错误，父子绑定、跨 Registry 与同 Builder 多次 Build 隔离。
- HTTP 正常、参数失败、匹配失败、歧义和嵌套 Registry 中间件。
- 并发共享 Registry / Resolution，执行 race 检查。
- README 中的完整示例与可执行源码一致，示例输出进入测试。
- Fuzz 将多业务、多 Point、能力顺序、激活组合及共享默认身份，与直接接口扫描对照；静态约束与独立位集合闭包对照。

benchmark 覆盖不同业务数、挂载能力数、Point 数及无约束 / 依赖链 / 带排斥的依赖链的装配开销；不沿用旧版小 fixture 的耗时。

## 10. 代码结构

- contracts.go：Matcher、Ability、Business、Self 和可选能力约束。
- builder.go / validate.go：收集注册、装配校验与编译。
- registry.go：业务匹配、能力激活和隔离绑定。
- resolution.go：First / All、Trace / Explain。
- catalog.go / errors.go：静态诊断和错误分类。
- httpx：net/http 中间件。
- examples/shop：README 的完整可运行示例。

没有第三方依赖；没有兼容旧 API 的入口。
