package test

import (
	"context"
	"testing"

	ee "github.com/xiaoshicae/go-easy-extension"
	"github.com/xiaoshicae/go-easy-extension/test/ability"
	"github.com/xiaoshicae/go-easy-extension/test/business"
	ep "github.com/xiaoshicae/go-easy-extension/test/extension_point"
	"github.com/xiaoshicae/go-easy-extension/test/param"
)

func TestContainer(t *testing.T) {
	ctx := context.Background()

	// 向你的ctx注入session，用于保存一起请求匹配的上下文信息
	ctx = ee.NewCtxWithSession(ctx)

	// 注册默认能力，为匹配到任何扩展点实现时，默认兜底实现
	if err := ee.RegisterDefaultAbility(ctx, &ability.DefaultAbility{}); err != nil {
		t.Fatal(err)
	}

	// 注册能力
	if err := ee.RegisterAbility(ctx, &ability.Ability1{}); err != nil {
		t.Fatal(err)
	}
	if err := ee.RegisterAbility(ctx, &ability.Ability2{}); err != nil {
		t.Fatal(err)
	}

	// 注册业务
	if err := ee.RegisterBusiness(ctx, &business.Business1{}); err != nil {
		t.Fatal(err)
	}
	if err := ee.RegisterBusiness(ctx, &business.Business2{}); err != nil {
		t.Fatal(err)
	}

	// 初始化session，MyParam用于业务和能力匹配
	// Name == "business_1" 因此会匹配到 Business1
	// Extra == "ability_1 & ability_2" 因此会匹配到 Ability1 和 Ability2
	if err := ee.InitSession(ctx, param.MyParam{
		Name:  "business_1",
		Extra: "ability_1 & ability_2",
	}); err != nil {
		t.Fatal(err)
	}

	// 获取Extension1优先级最高的实现
	// Business1实现了Extension1且优先级为100
	// Business1挂载了Ability1且实现了Extension1且优先级为200
	// 默认兜底能力DefaultAbility实现了Extension1且优先级为math.MaxInt64
	ext1, err := ee.GetFirstMatchedExtension[ep.Extension1](ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Business1实现优先级最高，因此会返回Business1实现 res == "business_1 do something1"
	res := ext1.DoSomething1()
	t.Logf(res)

	// 获取Extension3优先级最高的实现
	// Business1实现没有实现Extension3
	// Business1挂载了Ability1也没有实现Extension3
	// 默认兜底能力DefaultAbility实现了Extension3
	ext3, err := ee.GetFirstMatchedExtension[ep.Extension3](ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Business1和挂载的能力均为实现Extension3，因此返回默认兜底实现 res == "default ability do something3"
	res3 := ext3.DoSomething3()
	t.Logf(res3)

	// 获取Extension1所有匹配到的实现
	// Business1实现了Extension1且优先级为100
	// Business1挂载了Ability1且实现了Extension1且优先级为200
	// 默认兜底能力DefaultAbility实现了Extension1且优先级为math.MaxInt64
	// 按优先级从高到低排序，因此返回 [Business1的实现, Ability1的实现, DefaultAbility的实现]
	ext1List, err := ee.GetAllMatchedExtension[ep.Extension1](ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 按优先级从高到低排序，依次返回
	// res == "business_1 do something1"
	// res == "ability_1 do something1"
	// res == "default ability do something1"
	for _, ext := range ext1List {
		t.Logf(ext.DoSomething1())
	}

	// 如果一个请求结束后，ctx不会自动销毁，则需要手动调用RemoveSession(ctx)，清理session
	ee.RemoveSession(ctx)
}
