package easyext_test

import (
	"context"
	"fmt"
	"slices"

	"github.com/xiaoshicae/go-easy-extension/v2"
)

// OrderParam identifies a request: the business line and the abilities it asks for.
type OrderParam struct {
	Biz       string
	Abilities []string
}

// Freight is an extension point: the generic checkout flow depends on it.
type Freight interface{ Calc(items int) int }

// DefaultFreight answers when neither the business nor its abilities implement Freight.
type DefaultFreight struct{}

func (DefaultFreight) Calc(int) int { return 8 }

// FreeShipping is an ability: any business may mount it, and it applies when the request asks for it.
type FreeShipping struct{}

func (FreeShipping) Match(p OrderParam) bool { return slices.Contains(p.Abilities, "free-shipping") }
func (FreeShipping) Calc(int) int            { return 0 }

// Fresh is a business with its own cold-chain freight.
type Fresh struct{}

func (Fresh) Match(p OrderParam) bool { return p.Biz == "fresh" }
func (Fresh) Calc(items int) int      { return 15 + 2*items }

// Retail is a business that only mounts abilities.
type Retail struct{}

func (Retail) Match(p OrderParam) bool { return p.Biz == "retail" }

func Example() {
	c, err := easyext.New[OrderParam]().
		Point[Freight](DefaultFreight{}).
		Ability("ability.free-shipping", FreeShipping{}).
		Business("biz.fresh", Fresh{}, easyext.Abilities("ability.free-shipping", easyext.Self)).
		Business("biz.retail", Retail{}, easyext.Abilities("ability.free-shipping")).
		Build()
	if err != nil {
		panic(err)
	}

	for _, p := range []OrderParam{
		{Biz: "fresh"},
		{Biz: "fresh", Abilities: []string{"free-shipping"}},
		{Biz: "retail"},
	} {
		ctx, err := c.Bind(context.Background(), p) // once per request, e.g. in a middleware
		if err != nil {
			panic(err)
		}
		freight, _ := easyext.First[Freight](ctx) // anywhere below
		fmt.Println(p, "->", freight.Calc(3))
	}
	// Output:
	// {fresh []} -> 21
	// {fresh [free-shipping]} -> 0
	// {retail []} -> 8
}

func ExampleResolution_All() {
	c, _ := easyext.New[OrderParam]().
		Point[Freight](DefaultFreight{}).
		Ability("ability.free-shipping", FreeShipping{}).
		Business("biz.fresh", Fresh{}, easyext.Abilities("ability.free-shipping", easyext.Self)).
		Build()
	r, _ := c.Resolve(OrderParam{Biz: "fresh", Abilities: []string{"free-shipping"}})
	for f := range r.All[Freight]() {
		fmt.Println(f.Calc(3))
	}
	// Output:
	// 0
	// 21
	// 8
}

func ExampleResolution_Explain() {
	c, _ := easyext.New[OrderParam]().
		Point[Freight](DefaultFreight{}).
		Ability("ability.free-shipping", FreeShipping{}).
		Business("biz.fresh", Fresh{}, easyext.Abilities("ability.free-shipping", easyext.Self)).
		Build()
	r, _ := c.Resolve(OrderParam{Biz: "fresh"})
	fmt.Println(r)
	x := r.Explain[Freight]()
	for _, cand := range x.Candidates {
		fmt.Println(cand.Kind, cand.Code, cand.Implements)
	}
	fmt.Println("selected:", x.Selected.Code)
	// Output:
	// Resolution[business=biz.fresh, chain=[biz.fresh], skipped=[ability.free-shipping]]
	// business biz.fresh true
	// default easyext_test.DefaultFreight true
	// selected: biz.fresh
}

func ExampleBuilder_Build_invalid() {
	_, err := easyext.New[OrderParam]().
		Point[Freight](DefaultFreight{}).
		Business("biz.fresh", Fresh{}, easyext.Abilities("ability.missing")).
		Build()
	fmt.Println(err)
	// Output:
	// easyext: invalid assembly: business "biz.fresh": mounts unknown ability "ability.missing"
}
