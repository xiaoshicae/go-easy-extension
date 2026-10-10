package easyext_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
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
	extensions, err := registry.Resolve(OrderParam{Biz: "fresh", FreeShipping: true})
	if err != nil {
		panic(err)
	}
	sequence, err := extensions.All[Freight]()
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
	extensions, err := registry.Resolve(OrderParam{Biz: "fresh"})
	if err != nil {
		panic(err)
	}
	x, err := extensions.Explain[Freight]()
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

func TestREADMEExcerptsMatchExecutableSource(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	var blocks []string
	for _, part := range strings.Split(string(readme), "```go\n")[1:] {
		code, _, closed := strings.Cut(part, "\n```")
		if !closed {
			t.Fatal("README has an unclosed Go code block")
		}
		formatted, err := format.Source([]byte(code))
		if err != nil {
			t.Fatalf("README has an invalid Go code block: %v\n%s", err, code)
		}
		blocks = append(blocks, strings.TrimSpace(string(formatted)))
	}
	for _, tc := range []struct {
		path, declaration string
		first, count      int // count == 0 selects the whole declaration
	}{
		{"examples/shop/points.go", "FreightCalc", 0, 0},
		{"examples/shop/components.go", "Fresh.Abilities", 0, 0},
		{"examples/shop/main.go", "run", 0, 2}, // assembly and error check
		{"examples/shop/main.go", "run", 2, 1}, // inject the Registry into the service
		{"examples/shop/service.go", "CheckoutService", 0, 0},
		{"examples/shop/service.go", "CheckoutService.Checkout", 0, 4}, // point lookup and method call
	} {
		t.Run(tc.declaration+fmt.Sprint(tc.first), func(t *testing.T) {
			source, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tc.path, source, 0)
			if err != nil {
				t.Fatal(err)
			}
			var excerpt ast.Node
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						if spec, ok := spec.(*ast.TypeSpec); ok && spec.Name.Name == tc.declaration {
							excerpt = decl
						}
					}
				case *ast.FuncDecl:
					name := decl.Name.Name
					if decl.Recv != nil {
						receiver := decl.Recv.List[0].Type
						if pointer, ok := receiver.(*ast.StarExpr); ok {
							receiver = pointer.X
						}
						if receiver, ok := receiver.(*ast.Ident); ok {
							name = receiver.Name + "." + name
						}
					}
					if name == tc.declaration {
						excerpt = decl
					}
				}
			}
			if excerpt == nil {
				t.Fatalf("declaration %s not found in %s", tc.declaration, tc.path)
			}
			start, end := excerpt.Pos(), excerpt.End()
			if tc.count != 0 {
				function := excerpt.(*ast.FuncDecl)
				if len(function.Body.List) < tc.first+tc.count {
					t.Fatal("example no longer has the expected startup statements")
				}
				start = function.Body.List[tc.first].Pos()
				end = function.Body.List[tc.first+tc.count-1].End()
			}
			formatted, err := format.Source(source[fset.Position(start).Offset:fset.Position(end).Offset])
			if err != nil {
				t.Fatal(err)
			}
			for _, block := range blocks {
				if block == strings.TrimSpace(string(formatted)) {
					return
				}
			}
			t.Fatalf("README's %s excerpt differs from %s", tc.declaration, tc.path)
		})
	}
}
