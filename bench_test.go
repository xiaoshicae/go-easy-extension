package easyext_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

func BenchmarkResolve(b *testing.B) {
	registry := mustBuild(b, shopBuilder())
	param := shop.Param{Biz: "fresh", Abilities: []string{"free-shipping", "rapid"}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := registry.Resolve(param); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFirst(b *testing.B) {
	result := mustResolve(b, mustBuild(b, shopBuilder()), shop.Param{Biz: "digital"})
	b.ReportAllocs()
	for b.Loop() {
		if _, err := result.First[shop.Notify](); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFirstFromContext(b *testing.B) {
	registry := mustBuild(b, shopBuilder())
	ctx, err := registry.Bind(context.Background(), shop.Param{Biz: "fresh"})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := registry.First[shop.Freight](ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAll(b *testing.B) {
	result := mustResolve(b, mustBuild(b, shopBuilder()), shop.Param{Biz: "fresh", Abilities: []string{"rapid"}})
	b.ReportAllocs()
	for b.Loop() {
		sequence, err := result.All[shop.Delivery]()
		if err != nil {
			b.Fatal(err)
		}
		for impl := range sequence {
			_ = impl.DeliveryDays()
		}
	}
}

func BenchmarkResolveScale(b *testing.B) {
	for _, businesses := range []int{1, 100, 1000} {
		for _, abilities := range []int{1, 16} {
			for _, points := range []int{1, 4} {
				b.Run(fmt.Sprintf("businesses=%d/abilities=%d/points=%d", businesses, abilities, points), func(b *testing.B) {
					builder := easyext.New[shop.Param]().Point[shop.Freight]()
					if points == 4 {
						builder.Point[shop.AfterSale]().Point[shop.Notify]().Point[shop.Delivery]()
					}
					builder.Default(shop.Defaults{})
					var codes []string
					for i := range abilities {
						code := fmt.Sprintf("ability.%d", i)
						codes = append(codes, code)
						builder.Ability(&testAbility{code: code, value: i})
					}
					for i := range businesses {
						code := fmt.Sprintf("biz.%d", i)
						builder.Business(&testBusiness[shop.Param]{code: code, used: codes,
							match: func(p shop.Param) bool { return p.Biz == code }})
					}
					registry := mustBuild(b, builder)
					param := shop.Param{Biz: fmt.Sprintf("biz.%d", businesses-1)}
					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						if _, err := registry.Resolve(param); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

// Build is a startup operation, but long co-mount chains should not incur
// requirement-closure work when no ability declares an exclusion.
func BenchmarkBuildConstraints(b *testing.B) {
	for _, size := range []int{16, 128, 512} {
		for _, mode := range []string{"none", "requires", "requires-with-excludes"} {
			b.Run(fmt.Sprintf("abilities=%d/%s", size, mode), func(b *testing.B) {
				builder := freightBuilder()
				codes := make([]string, size)
				for i := range codes {
					codes[i] = fmt.Sprintf("ability.%d", i)
				}
				for i, code := range codes {
					ability := &testAbility{code: code, value: i}
					if mode != "none" && i+1 < size {
						ability.requires = []string{codes[i+1]}
					}
					if mode == "requires-with-excludes" && i == 0 {
						ability.excludes = []string{"ability.unused"}
					}
					builder.Ability(ability)
				}
				if mode == "requires-with-excludes" {
					builder.Ability(&testAbility{code: "ability.unused"})
				}
				builder.Business(&testBusiness[shop.Param]{code: "biz", used: codes})
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if _, err := builder.Build(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
