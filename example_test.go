package easyext_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

// These types also demonstrate that business metadata belongs to the component itself.
type OrderParam struct {
	Biz          string
	FreeShipping bool
}

type Freight interface{ Calc(items int) int }
type Delivery interface{ DeliveryDays() int }

type CommerceDefaults struct{}

func (CommerceDefaults) Calc(int) int      { return 8 }
func (CommerceDefaults) DeliveryDays() int { return 3 }

type FreeShipping struct{}

func (FreeShipping) Code() string            { return "ability.free-shipping" }
func (FreeShipping) Match(p OrderParam) bool { return p.FreeShipping }
func (FreeShipping) Calc(int) int            { return 0 }

type Fresh struct{}

func (Fresh) Code() string            { return "biz.fresh" }
func (Fresh) Match(p OrderParam) bool { return p.Biz == "fresh" }
func (Fresh) Abilities() []string     { return []string{"ability.free-shipping", easyext.Self} }
func (Fresh) Calc(items int) int      { return 15 + 2*items }
func (Fresh) DeliveryDays() int       { return 2 }

type Retail struct{}

func (Retail) Code() string            { return "biz.retail" }
func (Retail) Match(p OrderParam) bool { return p.Biz == "retail" }
func (Retail) Abilities() []string     { return []string{"ability.free-shipping"} }

func exampleRegistry() (*easyext.Registry[OrderParam], error) {
	return easyext.New[OrderParam]().Point[Freight]().Point[Delivery]().Default(CommerceDefaults{}).
		Ability(FreeShipping{}).Business(Fresh{}).Business(Retail{}).Build()
}

func Example() {
	registry, err := exampleRegistry()
	if err != nil {
		panic(err)
	}
	for _, param := range []OrderParam{{Biz: "fresh"}, {Biz: "fresh", FreeShipping: true}, {Biz: "retail"}} {
		ctx, err := registry.Bind(context.Background(), param)
		if err != nil {
			panic(err)
		}
		freight, err := registry.First[Freight](ctx)
		if err != nil {
			panic(err)
		}
		fmt.Println(param.Biz, param.FreeShipping, "->", freight.Calc(3))
	}
	// Output:
	// fresh false -> 21
	// fresh true -> 0
	// retail false -> 8
}

func ExampleResolution_All() {
	registry, err := exampleRegistry()
	if err != nil {
		panic(err)
	}
	result, err := registry.Resolve(OrderParam{Biz: "fresh", FreeShipping: true})
	if err != nil {
		panic(err)
	}
	sequence, err := result.All[Freight]()
	if err != nil {
		panic(err)
	}
	for impl := range sequence {
		fmt.Println(impl.Calc(3))
	}
	// Output:
	// 0
	// 21
	// 8
}

func ExampleResolution_Explain() {
	registry, err := exampleRegistry()
	if err != nil {
		panic(err)
	}
	result, err := registry.Resolve(OrderParam{Biz: "fresh"})
	if err != nil {
		panic(err)
	}
	x, err := result.Explain[Freight]()
	if err != nil {
		panic(err)
	}
	for _, candidate := range x.Candidates {
		fmt.Println(candidate.Kind, candidate.Code, candidate.Reason)
	}
	fmt.Println("selected:", x.Selected.Code)
	// Output:
	// ability ability.free-shipping match-false
	// business biz.fresh selected
	// default easyext_test.CommerceDefaults lower-priority
	// selected: biz.fresh
}

type missingAbilityBusiness struct{ Fresh }

func (missingAbilityBusiness) Abilities() []string { return []string{"ability.missing"} }

func ExampleBuilder_Build_invalid() {
	_, err := easyext.New[OrderParam]().Point[Freight]().Default(CommerceDefaults{}).
		Business(missingAbilityBusiness{}).Build()
	fmt.Println(err)
	// Output:
	// easyext: invalid assembly: business "biz.fresh": uses unknown ability "ability.missing"
}

func TestREADMEExampleMatchesExecutableSource(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("examples/shop/main.go")
	if err != nil {
		t.Fatal(err)
	}
	block := "```go\n" + strings.TrimSpace(string(source)) + "\n```"
	if !strings.Contains(string(readme), block) {
		t.Fatal("README's complete example differs from examples/shop/main.go")
	}
}
