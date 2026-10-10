package easyext_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	pa "github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/a"
	pb "github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/b"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

func shopBuilder() *easyext.Builder[shop.Param] {
	return easyext.New[shop.Param]().
		Point[shop.Freight]().Point[shop.AfterSale]().Point[shop.Notify]().Point[shop.Delivery]().
		Default(shop.Defaults{}).
		Ability(shop.FreeShipping{}).Ability(shop.Return7Days{}).Ability(shop.RapidDelivery{}).
		Business(shop.Retail{}).Business(shop.Fresh{}).Business(shop.Digital{})
}

func mustBuild[P any](t testing.TB, builder *easyext.Builder[P]) *easyext.Registry[P] {
	t.Helper()
	registry, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func mustResolve[P any](t testing.TB, registry *easyext.Registry[P], param P) *easyext.Resolution {
	t.Helper()
	result, err := registry.Resolve(param)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func first[E any](t testing.TB, result *easyext.Resolution) E {
	t.Helper()
	impl, err := result.First[E]()
	if err != nil {
		t.Fatal(err)
	}
	return impl
}

func all[E any](t testing.TB, result *easyext.Resolution) []E {
	t.Helper()
	sequence, err := result.All[E]()
	if err != nil {
		t.Fatal(err)
	}
	return slices.Collect(sequence)
}

type testBusiness[P any] struct {
	code  string
	used  []string
	match func(P) bool
}

func (b *testBusiness[P]) Code() string        { return b.code }
func (b *testBusiness[P]) Abilities() []string { return b.used }
func (b *testBusiness[P]) Match(p P) bool      { return b.match == nil || b.match(p) }

func TestFirstFollowsAbilitiesOrder(t *testing.T) {
	registry := mustBuild(t, shopBuilder())
	tests := []struct {
		name       string
		param      shop.Param
		freight    int
		returnDays int
		delivery   int
		channels   []string
	}{
		{"fresh overrides freight and delivery", shop.Param{Biz: "fresh"}, 21, 0, 2, []string{"PUSH"}},
		{"free shipping overrides fresh", shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}}, 0, 0, 2, []string{"PUSH"}},
		{"one rapid ability supplies two points", shop.Param{Biz: "fresh", Abilities: []string{"rapid"}}, 21, 0, 1, []string{"SMS", "PUSH", "WECHAT_MSG"}},
		{"digital supplies after sale", shop.Param{Biz: "digital"}, 8, 15, 3, []string{"PUSH"}},
		{"return ability precedes digital", shop.Param{Biz: "digital", Abilities: []string{"return-7d"}}, 8, 7, 3, []string{"PUSH"}},
		{"retail is only a composition", shop.Param{Biz: "retail", Abilities: []string{"free-shipping"}}, 0, 0, 3, []string{"PUSH"}},
		{"shared rapid ability in another business", shop.Param{Biz: "retail", Abilities: []string{"rapid"}}, 8, 0, 1, []string{"SMS", "PUSH", "WECHAT_MSG"}},
		{"unmounted abilities ignored", shop.Param{Biz: "digital", Abilities: []string{"free-shipping", "rapid"}}, 8, 15, 3, []string{"PUSH"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustResolve(t, registry, tt.param)
			if got := first[shop.Freight](t, result).Freight(shop.Order{Items: 3}); got != tt.freight {
				t.Errorf("freight = %d, want %d", got, tt.freight)
			}
			if got := first[shop.AfterSale](t, result).ReturnDays(); got != tt.returnDays {
				t.Errorf("return days = %d, want %d", got, tt.returnDays)
			}
			if got := first[shop.Delivery](t, result).DeliveryDays(); got != tt.delivery {
				t.Errorf("delivery = %d, want %d", got, tt.delivery)
			}
			if got := first[shop.Notify](t, result).Channels(); !slices.Equal(got, tt.channels) {
				t.Errorf("channels = %v, want %v", got, tt.channels)
			}
		})
	}
}

func TestAllYieldsChainThenDefault(t *testing.T) {
	result := mustResolve(t, mustBuild(t, shopBuilder()), shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}})
	sequence, err := result.All[shop.Freight]()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		var got []int
		for impl := range sequence {
			got = append(got, impl.Freight(shop.Order{Items: 3}))
		}
		if !slices.Equal(got, []int{0, 21, 8}) {
			t.Fatalf("All = %v", got)
		}
	}
	n := 0
	for range sequence {
		n++
		break
	}
	if n != 1 {
		t.Fatalf("early break: yielded %d", n)
	}
}

type namedA string

func (n namedA) Name() string { return string(n) }

type namedB string

func (n namedB) Name() string { return string(n) }

func TestSameNamedPointsInDifferentPackagesAreDistinct(t *testing.T) {
	registry := mustBuild(t, easyext.New[string]().
		Point[pa.Point]().Point[pb.Point]().
		DefaultFor[pa.Point](namedA("default-a")).
		DefaultFor[pb.Point](namedB("default-b")).
		Business(&testBusiness[string]{code: "biz"}))
	result := mustResolve(t, registry, "x")
	if a, b := first[pa.Point](t, result).Name(), first[pb.Point](t, result).Name(); a != "default-a" || b != "default-b" {
		t.Fatalf("got %q and %q", a, b)
	}
}

func TestSeveralMatchingBusinesses(t *testing.T) {
	builder := easyext.New[string]().Point[shop.Freight]().Default(shop.DefaultFreight{})
	counts := make([]int, 4)
	for i, code := range []string{"biz.a", "biz.b", "biz.c", "biz.no"} {
		builder.Business(&testBusiness[string]{code: code, match: func(string) bool {
			counts[i]++
			return i != 3
		}})
	}
	_, err := mustBuild(t, builder).Resolve("x")
	if !errors.Is(err, easyext.ErrMultipleBusinessesMatched) {
		t.Fatalf("err = %v", err)
	}
	if want := "easyext: MULTIPLE_BUSINESSES_MATCHED: matched businesses [\"biz.a\" \"biz.b\" \"biz.c\"]"; err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
	if !slices.Equal(counts, []int{1, 1, 1, 1}) {
		t.Fatalf("businesses rematched: %v", counts)
	}
}

func TestNoBusinessMatched(t *testing.T) {
	registry := mustBuild(t, shopBuilder())
	_, err := registry.Resolve(shop.Param{Biz: "unknown"})
	if !errors.Is(err, easyext.ErrNoBusinessMatched) || err.Error() != "easyext: NO_BUSINESS_MATCHED: none of the 3 registered businesses matched" {
		t.Fatalf("err = %v", err)
	}
	ctx := context.Background()
	got, err := registry.Bind(ctx, shop.Param{Biz: "unknown"})
	if got != ctx || !errors.Is(err, easyext.ErrNoBusinessMatched) {
		t.Fatalf("failed Bind changed context: %v, %v", got, err)
	}
}

type unregistered interface{ X() }

func TestUnregisteredExtensionPointReturnsErrors(t *testing.T) {
	registry := mustBuild(t, shopBuilder())
	result := mustResolve(t, registry, shop.Param{Biz: "fresh"})
	ctx, err := registry.Bind(context.Background(), shop.Param{Biz: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]func() error{
		"Resolution.First":   func() error { _, err := result.First[unregistered](); return err },
		"Resolution.All":     func() error { _, err := result.All[unregistered](); return err },
		"Resolution.Explain": func() error { _, err := result.Explain[unregistered](); return err },
		"Registry.First":     func() error { _, err := registry.First[unregistered](ctx); return err },
		"Registry.All":       func() error { _, err := registry.All[unregistered](ctx); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, easyext.ErrExtensionNotFound) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestTraceExplainString(t *testing.T) {
	result := mustResolve(t, mustBuild(t, shopBuilder()), shop.Param{Biz: "fresh", Abilities: []string{"rapid"}})
	trace := result.Trace()
	want := []easyext.Link{{Code: shop.RapidDeliveryCode, Kind: easyext.KindAbility}, {Code: shop.FreshCode, Kind: easyext.KindBusiness}}
	if trace.Business != shop.FreshCode || !slices.Equal(trace.Chain, want) || !slices.Equal(trace.Skipped, []string{shop.FreeShippingCode}) {
		t.Fatalf("trace = %+v", trace)
	}
	if got := result.String(); got != "Resolution[business=biz.fresh, chain=[ability.rapid biz.fresh], skipped=[ability.free-shipping]]" {
		t.Fatalf("String = %q", got)
	}
	x, err := result.Explain[shop.Freight]()
	if err != nil {
		t.Fatal(err)
	}
	if x.Point != "shop.Freight" || x.Selected != (easyext.Link{Code: shop.FreshCode, Kind: easyext.KindBusiness}) {
		t.Fatalf("explanation = %+v", x)
	}
	reasons := make([]string, len(x.Candidates))
	for i, candidate := range x.Candidates {
		reasons[i] = candidate.Reason
		if candidate.Position != i {
			t.Errorf("candidate %d position = %d", i, candidate.Position)
		}
	}
	if !slices.Equal(reasons, []string{"match-false", "not-implemented", "selected", "lower-priority"}) ||
		!x.Candidates[0].Implements || x.Candidates[0].Active {
		t.Fatalf("candidates = %+v", x.Candidates)
	}
	trace.Chain[0].Code = "changed"
	trace.Skipped[0] = "changed"
	x.Candidates[0].Code = "changed"
	if result.Trace().Chain[0].Code != shop.RapidDeliveryCode || result.Trace().Skipped[0] != shop.FreeShippingCode {
		t.Fatal("trace shares mutable state")
	}
}

func TestBindIsPerContextAndRegistry(t *testing.T) {
	registry := mustBuild(t, shopBuilder())
	for name, call := range map[string]func() error{
		"From":  func() error { _, err := registry.From(context.Background()); return err },
		"First": func() error { _, err := registry.First[shop.Freight](context.Background()); return err },
		"All":   func() error { _, err := registry.All[shop.Freight](context.Background()); return err },
	} {
		if err := call(); !errors.Is(err, easyext.ErrNoBinding) {
			t.Fatalf("%s unbound: %v", name, err)
		}
	}
	parent, err := registry.Bind(context.Background(), shop.Param{Biz: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := registry.Bind(parent, shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}})
	if err != nil {
		t.Fatal(err)
	}
	freight := func(r *easyext.Registry[shop.Param], ctx context.Context) int {
		impl, err := r.First[shop.Freight](ctx)
		if err != nil {
			t.Fatal(err)
		}
		return impl.Freight(shop.Order{Items: 3})
	}
	if a, b := freight(registry, parent), freight(registry, child); a != 21 || b != 0 {
		t.Fatalf("parent = %d, child = %d", a, b)
	}
	other := mustBuild(t, shopBuilder())
	if _, err := other.From(child); !errors.Is(err, easyext.ErrNoBinding) {
		t.Fatalf("unbound registry saw foreign binding: %v", err)
	}
	both, err := other.Bind(child, shop.Param{Biz: "digital"})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := freight(registry, both), freight(other, both); a != 0 || b != 8 {
		t.Fatalf("registry bindings overwrite each other: %d, %d", a, b)
	}
	sequence, err := registry.All[shop.Notify](parent)
	if err != nil || len(slices.Collect(sequence)) != 1 {
		t.Fatalf("bound All: %v", err)
	}
}

func TestConcurrentUse(t *testing.T) {
	registry := mustBuild(t, shopBuilder())
	root, err := registry.Bind(context.Background(), shop.Param{Biz: "digital"})
	if err != nil {
		t.Fatal(err)
	}
	result := mustResolve(t, registry, shop.Param{Biz: "fresh", Abilities: []string{"rapid"}})
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			param := shop.Param{Biz: "fresh"}
			if i%2 == 0 {
				param.Abilities = []string{"free-shipping"}
			}
			ctx, err := registry.Bind(root, param)
			if err != nil {
				t.Error(err)
				return
			}
			want := 21
			if i%2 == 0 {
				want = 0
			}
			impl, err := registry.First[shop.Freight](ctx)
			if err != nil {
				t.Error(err)
				return
			}
			if impl.Freight(shop.Order{Items: 3}) != want {
				t.Errorf("goroutine %d: wrong freight", i)
			}
			if _, err := result.Explain[shop.Notify](); err != nil {
				t.Error(err)
			}
			sequence, err := result.All[shop.Delivery]()
			if err != nil {
				t.Error(err)
				return
			}
			if len(slices.Collect(sequence)) != 3 {
				t.Error("shared resolution changed")
			}
			registry.Catalog()
			result.Trace()
		})
	}
	wg.Wait()
	impl, err := registry.First[shop.AfterSale](root)
	if err != nil || impl.ReturnDays() != 15 {
		t.Fatal("root binding changed")
	}
}

func TestBuildOrderIsNotCompositionOrder(t *testing.T) {
	registry := mustBuild(t, easyext.New[shop.Param]().
		Business(shop.Fresh{}).
		Ability(shop.RapidDelivery{}).Ability(shop.FreeShipping{}).
		Default(shop.Defaults{}).
		Point[shop.Freight]().Point[shop.Notify]().Point[shop.Delivery]())
	result := mustResolve(t, registry, shop.Param{Biz: "fresh", Abilities: []string{"free-shipping", "rapid"}})
	if first[shop.Freight](t, result).Freight(shop.Order{}) != 0 || first[shop.Delivery](t, result).DeliveryDays() != 1 {
		t.Fatal("registration order changed precedence")
	}
	if registry.Catalog().Businesses[0].Abilities[0] != shop.FreeShippingCode {
		t.Fatal("business declaration ignored")
	}
}

func TestRegistriesAndBuildCallsAreIndependent(t *testing.T) {
	builder := shopBuilder()
	a, b := mustBuild(t, builder), mustBuild(t, builder)
	ctx, err := a.Bind(context.Background(), shop.Param{Biz: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.From(ctx); !errors.Is(err, easyext.ErrNoBinding) {
		t.Fatalf("Build calls share binding identity: %v", err)
	}
	builder.Business(&testBusiness[shop.Param]{code: "extra", match: func(shop.Param) bool { return false }})
	if len(a.Catalog().Businesses) != 3 || len(mustBuild(t, builder).Catalog().Businesses) != 4 {
		t.Fatal("builder mutation affected a built registry")
	}
}

func TestCatalog(t *testing.T) {
	registry := mustBuild(t, shopBuilder())
	catalog := registry.Catalog()
	if catalog.ParamType != "shop.Param" || len(catalog.Points) != 4 ||
		catalog.Points[0] != (easyext.PointInfo{Type: "shop.Freight", Default: "shop.Defaults"}) {
		t.Fatalf("catalog = %+v", catalog)
	}
	if rapid := catalog.Abilities[2]; !slices.Equal(rapid.Points, []string{"shop.Notify", "shop.Delivery"}) {
		t.Fatalf("rapid = %+v", rapid)
	}
	if fresh := catalog.Businesses[1]; !slices.Equal(fresh.Points, []string{"shop.Freight", "shop.Delivery"}) ||
		!slices.Equal(fresh.Abilities, []string{shop.FreeShippingCode, shop.RapidDeliveryCode, easyext.Self}) {
		t.Fatalf("fresh = %+v", fresh)
	}
	if retail := catalog.Businesses[0]; retail.Abilities[0] != easyext.Self {
		t.Fatalf("implicit Self = %+v", retail)
	}
	catalog.Points[0].Default = "changed"
	catalog.Abilities[2].Points[0] = "changed"
	catalog.Businesses[1].Abilities[0] = "changed"
	if got := registry.Catalog(); got.Points[0].Default != "shop.Defaults" || got.Abilities[2].Points[0] != "shop.Notify" ||
		got.Businesses[1].Abilities[0] != shop.FreeShippingCode {
		t.Fatal("catalog shares mutable slices")
	}
}
