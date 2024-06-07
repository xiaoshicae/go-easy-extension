package test

import (
	"context"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/core"
	"github.com/xiaoshicae/go-easy-extension/core/session"
	"github.com/xiaoshicae/go-easy-extension/test/ability"
	"github.com/xiaoshicae/go-easy-extension/test/business"
	et "github.com/xiaoshicae/go-easy-extension/test/extension_point"
	"github.com/xiaoshicae/go-easy-extension/test/param"
)

func TestApp(t *testing.T) {
	ctx := session.NewCtxWithSession(context.Background())
	ec := core.GetExtensionContext()
	ec.RegisterAbility(ctx, &ability.DefaultAbility{})
	ec.RegisterAbility(ctx, &ability.Ability1{})
	ec.RegisterAbility(ctx, &ability.Ability2{})
	ec.RegisterBusiness(ctx, &business.Business1{})
	ec.RegisterBusiness(ctx, &business.Business2{})

	ec.InitSession(ctx, param.MyParam{Name: "business_2", Extra: "ability_1 & ability_2"})

	ext1, err := ec.GetFirstMatchedExtension(ctx, new(et.Extension1))
	if err != nil {
		t.Fatal(err)
	}
	res := ext1.(et.Extension1).DoSomething1()
	t.Log(res)
}
