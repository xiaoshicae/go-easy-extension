package easyext_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v2"
	pa "github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/a"
	pb "github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/b"
	"github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/shop"
)

// shopBuilder assembles the e-commerce fixture; tests add or change options on top of it.
func shopBuilder() *easyext.Builder[shop.Param] {
	return easyext.New[shop.Param]().
		Point[shop.Freight](shop.DefaultFreight{}).
		Point[shop.AfterSale](shop.DefaultAfterSale{}).
		Point[shop.Notify](shop.DefaultNotify{}).
		Ability("ability.free-shipping", shop.FreeShipping{}).
		Ability("ability.return-7d", shop.Return7Days{}).
		Ability("ability.rapid", shop.RapidDelivery{}).
		Business("biz.retail", shop.Retail{}, easyext.Abilities("ability.free-shipping", "ability.return-7d")).
		Business("biz.fresh", shop.Fresh{}, easyext.Abilities("ability.free-shipping", "ability.rapid", easyext.Self)).
		Business("biz.digital", shop.Digital{}, easyext.Abilities("ability.return-7d", easyext.Self))
}

func mustBuild[T any](t testing.TB, b *easyext.Builder[T]) *easyext.Context[T] {
	t.Helper()
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustResolve[T any](t testing.TB, c *easyext.Context[T], p T) *easyext.Resolution {
	t.Helper()
	r, err := c.Resolve(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFirstFollowsAbilitiesOrder(t *testing.T) {
	c := mustBuild(t, shopBuilder())
	order := shop.Order{Items: 3}
	tests := []struct {
		name       string
		param      shop.Param
		freight    int
		returnDays int
		channels   []string
	}{
		{"fresh alone: its own cold-chain freight", shop.Param{Biz: "fresh"}, 21, 0, []string{"PUSH"}},
		{"free shipping is listed before Self: it overrides fresh", shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}}, 0, 0, []string{"PUSH"}},
		{"rapid delivery adds channels", shop.Param{Biz: "fresh", Abilities: []string{"rapid"}}, 21, 0, []string{"SMS", "PUSH", "WECHAT_MSG"}},
		{"digital alone: its own 15 days", shop.Param{Biz: "digital"}, 8, 15, []string{"PUSH"}},
		{"return-7d is listed before Self: it overrides digital", shop.Param{Biz: "digital", Abilities: []string{"return-7d"}}, 8, 7, []string{"PUSH"}},
		{"retail implements nothing: abilities, then defaults", shop.Param{Biz: "retail", Abilities: []string{"free-shipping"}}, 0, 0, []string{"PUSH"}},
		{"ability not mounted by the business is ignored", shop.Param{Biz: "digital", Abilities: []string{"free-shipping"}}, 8, 15, []string{"PUSH"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mustResolve(t, c, tt.param)
			if got := r.First[shop.Freight]().Freight(order); got != tt.freight {
				t.Errorf("freight = %d, want %d", got, tt.freight)
			}
			if got := r.First[shop.AfterSale]().ReturnDays(); got != tt.returnDays {
				t.Errorf("return days = %d, want %d", got, tt.returnDays)
			}
			if got := r.First[shop.Notify]().Channels(); !slices.Equal(got, tt.channels) {
				t.Errorf("channels = %v, want %v", got, tt.channels)
			}
		})
	}
}

func TestAllYieldsChainThenDefault(t *testing.T) {
	c := mustBuild(t, shopBuilder())
	r := mustResolve(t, c, shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}})
	var got []int
	for f := range r.All[shop.Freight]() {
		got = append(got, f.Freight(shop.Order{Items: 3}))
	}
	if want := []int{0, 21, 8}; !slices.Equal(got, want) { // free shipping, fresh, default
		t.Fatalf("All = %v, want %v", got, want)
	}
	n := 0
	for range r.All[shop.Freight]() { // stops early
		n++
		break
	}
	if n != 1 {
		t.Fatalf("break after first: yielded %d", n)
	}
}

// v1 keyed extension points by reflect.TypeOf(new(I)).String(): both of these were "*ext.Point".
func TestSameNamedPointsInDifferentPackagesAreDistinct(t *testing.T) {
	c := mustBuild(t, easyext.New[string]().
		Point[pa.Point](namedA("default-a")).
		Point[pb.Point](namedB("default-b")).
		Business("biz", easyext.MatcherFunc[string](func(string) bool { return true })))
	r := mustResolve(t, c, "x")
	if a, b := r.First[pa.Point]().Name(), r.First[pb.Point]().Name(); a != "default-a" || b != "default-b" {
		t.Fatalf("got %q and %q", a, b)
	}
}

type namedA string

func (n namedA) Name() string { return string(n) }

type namedB string

func (n namedB) Name() string { return string(n) }

type always string

func (always) Match(string) bool        { return true }
func (a always) Freight(shop.Order) int { return len(a) }

// v1 kept businesses in a map: with several matches it picked one at random.
func TestSeveralMatchingBusinesses(t *testing.T) {
	builder := func() *easyext.Builder[string] {
		b := easyext.New[string]().Point[shop.Freight](shop.DefaultFreight{})
		for _, code := range []string{"biz.a", "biz.b", "biz.c"} {
			b.Business(code, always(code))
		}
		return b
	}

	t.Run("strict: error listing the businesses", func(t *testing.T) {
		_, err := mustBuild(t, builder()).Resolve("x")
		if !errors.Is(err, easyext.ErrMultipleBusinessesMatched) {
			t.Fatalf("err = %v", err)
		}
		if want := `easyext: MULTIPLE_BUSINESSES_MATCHED: matched businesses ["biz.a" "biz.b" "biz.c"]`; err.Error() != want {
			t.Fatalf("message = %q, want %q", err.Error(), want)
		}
	})
	t.Run("not strict: registration order, every time", func(t *testing.T) {
		c := mustBuild(t, builder().Strict(false))
		for range 200 {
			if code, _ := mustResolve(t, c, "x").Business(); code != "biz.a" {
				t.Fatalf("business = %q", code)
			}
		}
	})
	t.Run("not strict: selector decides", func(t *testing.T) {
		var seen []string
		c := mustBuild(t, builder().Strict(false).BusinessSelector(func(_ string, matched []string) (string, bool) {
			seen = matched
			return "biz.c", true
		}))
		if code, _ := mustResolve(t, c, "x").Business(); code != "biz.c" {
			t.Fatalf("business = %q", code)
		}
		if !slices.Equal(seen, []string{"biz.a", "biz.b", "biz.c"}) {
			t.Fatalf("selector saw %v", seen)
		}
	})
	t.Run("not strict: selector names an unmatched business", func(t *testing.T) {
		c := mustBuild(t, builder().Strict(false).BusinessSelector(func(string, []string) (string, bool) { return "biz.z", true }))
		if _, err := c.Resolve("x"); !errors.Is(err, easyext.ErrBusinessNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("not strict: selector declines, defaults answer", func(t *testing.T) {
		c := mustBuild(t, builder().Strict(false).BusinessSelector(func(string, []string) (string, bool) { return "", false }))
		r := mustResolve(t, c, "x")
		if _, ok := r.Business(); ok || r.First[shop.Freight]().Freight(shop.Order{}) != 8 {
			t.Fatalf("got %v", r)
		}
	})
}

func TestNoBusinessMatched(t *testing.T) {
	_, err := mustBuild(t, shopBuilder()).Resolve(shop.Param{Biz: "unknown"})
	if !errors.Is(err, easyext.ErrNoBusinessMatched) || err.Error() != "easyext: NO_BUSINESS_MATCHED: no business matched" {
		t.Fatalf("err = %v", err)
	}

	r := mustResolve(t, mustBuild(t, shopBuilder().Strict(false)), shop.Param{Biz: "unknown", Abilities: []string{"free-shipping"}})
	if _, ok := r.Business(); ok {
		t.Fatal("expected no business")
	}
	if got := r.First[shop.Freight]().Freight(shop.Order{}); got != 8 { // abilities need a business to mount them
		t.Fatalf("freight = %d, want the default 8", got)
	}
}

func TestBusinessResolver(t *testing.T) {
	byBiz := func(p shop.Param) (string, bool) { return "biz." + p.Biz, p.Biz != "" }

	c := mustBuild(t, easyext.New[shop.Param]().
		Point[shop.AfterSale](shop.DefaultAfterSale{}).
		Business("biz.digital", digitalNoMatcher{}).
		BusinessResolver(byBiz))
	if got := mustResolve(t, c, shop.Param{Biz: "digital"}).First[shop.AfterSale]().ReturnDays(); got != 15 {
		t.Fatalf("return days = %d", got)
	}
	if _, err := c.Resolve(shop.Param{}); !errors.Is(err, easyext.ErrNoBusinessMatched) {
		t.Fatalf("resolver declined: err = %v", err)
	}
	_, err := c.Resolve(shop.Param{Biz: "nope"})
	if !errors.Is(err, easyext.ErrBusinessNotFound) ||
		err.Error() != `easyext: BUSINESS_NOT_FOUND: business "biz.nope" returned by the business resolver is not registered` {
		t.Fatalf("unknown code: err = %v", err)
	}

	lenient := mustBuild(t, easyext.New[shop.Param]().
		Point[shop.AfterSale](shop.DefaultAfterSale{}).
		Business("biz.digital", digitalNoMatcher{}).
		BusinessResolver(byBiz).Strict(false))
	if got := mustResolve(t, lenient, shop.Param{Biz: "nope"}).First[shop.AfterSale]().ReturnDays(); got != 0 {
		t.Fatalf("not strict, unknown code: return days = %d, want the default", got)
	}
}

// digitalNoMatcher has no Match: allowed because a business resolver routes the requests.
type digitalNoMatcher struct{}

func (digitalNoMatcher) ReturnDays() int { return 15 }

func TestUnregisteredExtensionPoint(t *testing.T) {
	r := mustResolve(t, mustBuild(t, shopBuilder()), shop.Param{Biz: "fresh"})
	_, err := r.Lookup[unregistered]()
	if !errors.Is(err, easyext.ErrExtensionNotFound) {
		t.Fatalf("Lookup: err = %v", err)
	}
	if want := "easyext: EXTENSION_NOT_FOUND: extension point easyext_test.unregistered is not registered; register it with Builder.Point"; err.Error() != want {
		t.Fatalf("message = %q", err.Error())
	}
	defer func() {
		re, ok := errors.AsType[*easyext.ResolutionError](recover().(error))
		if !ok || re.Reason != easyext.ExtensionNotFound {
			t.Fatalf("First should panic with a ResolutionError, got %v", re)
		}
	}()
	r.First[unregistered]()
}

type unregistered interface{ X() }

func TestTraceExplainString(t *testing.T) {
	c := mustBuild(t, shopBuilder())
	r := mustResolve(t, c, shop.Param{Biz: "fresh", Abilities: []string{"rapid"}})

	tr := r.Trace()
	// fresh mounts free-shipping, rapid, Self; free-shipping does not match, so the chain is rapid, then fresh.
	wantChain := []easyext.Link{{Code: "ability.rapid", Kind: easyext.KindAbility}, {Code: "biz.fresh", Kind: easyext.KindBusiness}}
	if tr.Business != "biz.fresh" || !slices.Equal(tr.Chain, wantChain) || !slices.Equal(tr.Skipped, []string{"ability.free-shipping"}) {
		t.Fatalf("trace = %+v", tr)
	}
	if got := r.String(); got != "Resolution[business=biz.fresh, chain=[ability.rapid biz.fresh], skipped=[ability.free-shipping]]" {
		t.Fatalf("String() = %q", got)
	}

	x := r.Explain[shop.Freight]()
	if x.Point != "shop.Freight" || x.Selected != (easyext.Link{Code: "biz.fresh", Kind: easyext.KindBusiness}) {
		t.Fatalf("explain = %+v", x)
	}
	if n := len(x.Candidates); n != 3 || x.Candidates[0].Implements || !x.Candidates[1].Implements ||
		x.Candidates[2].Link != (easyext.Link{Code: "shop.DefaultFreight", Kind: easyext.KindDefault}) {
		t.Fatalf("candidates = %+v", x.Candidates)
	}
}

func TestBindIsPerContext(t *testing.T) {
	c := mustBuild(t, shopBuilder())
	if _, err := easyext.From(context.Background()); !errors.Is(err, easyext.ErrNoBinding) {
		t.Fatalf("unbound: err = %v", err)
	}
	if _, err := easyext.First[shop.Freight](context.Background()); !errors.Is(err, easyext.ErrNoBinding) {
		t.Fatalf("First unbound: err = %v", err)
	}

	parent, err := c.Bind(context.Background(), shop.Param{Biz: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	// v1: a child context shared the parent's mutable session, so work on the child changed the parent.
	child, err := c.Bind(parent, shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}})
	if err != nil {
		t.Fatal(err)
	}
	freight := func(ctx context.Context) int {
		f, err := easyext.First[shop.Freight](ctx)
		if err != nil {
			t.Fatal(err)
		}
		return f.Freight(shop.Order{Items: 3})
	}
	if p, ch := freight(parent), freight(child); p != 21 || ch != 0 {
		t.Fatalf("parent = %d (want 21), child = %d (want 0)", p, ch)
	}

	all, err := easyext.All[shop.Notify](parent)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(slices.Collect(all)); n != 1 {
		t.Fatalf("All[Notify] for fresh alone: %d implementations, want the default only", n)
	}
	if _, err := easyext.All[unregistered](parent); !errors.Is(err, easyext.ErrExtensionNotFound) {
		t.Fatalf("All unregistered: err = %v", err)
	}

	if _, err := c.Bind(context.Background(), shop.Param{Biz: "unknown"}); !errors.Is(err, easyext.ErrNoBusinessMatched) {
		t.Fatalf("Bind error: %v", err)
	}
}

// v1: concurrent InitSession on contexts derived from one request was a data race. Run with -race.
func TestConcurrentUse(t *testing.T) {
	c := mustBuild(t, shopBuilder())
	root, err := c.Bind(context.Background(), shop.Param{Biz: "digital"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			p := shop.Param{Biz: "fresh"}
			if i%2 == 0 {
				p.Abilities = []string{"free-shipping"}
			}
			ctx, err := c.Bind(root, p)
			if err != nil {
				t.Error(err)
				return
			}
			want := 21
			if i%2 == 0 {
				want = 0
			}
			if f, _ := easyext.First[shop.Freight](ctx); f.Freight(shop.Order{Items: 3}) != want {
				t.Errorf("goroutine %d: wrong freight", i)
			}
		})
	}
	wg.Wait()
	if d, _ := easyext.First[shop.AfterSale](root); d.ReturnDays() != 15 {
		t.Fatal("root binding changed")
	}
}

// v1 used one global context: registrations of one test leaked into the next.
func TestContextsAreIndependent(t *testing.T) {
	a := mustBuild(t, shopBuilder())
	b := mustBuild(t, easyext.New[shop.Param]().Point[shop.Freight](shop.DefaultFreight{}).Business("biz.fresh", shop.Fresh{}))
	if len(a.Catalog().Businesses) != 3 || len(b.Catalog().Businesses) != 1 {
		t.Fatal("contexts share state")
	}
}

func TestBusinessImplementingNothing(t *testing.T) {
	c := mustBuild(t, easyext.New[shop.Param]().
		Point[shop.Freight](shop.DefaultFreight{}).
		Ability("ability.free-shipping", shop.FreeShipping{}).
		Business("biz.trial", easyext.MatcherFunc[shop.Param](func(p shop.Param) bool { return p.Biz == "trial" }),
			easyext.Abilities("ability.free-shipping")))
	if got := mustResolve(t, c, shop.Param{Biz: "trial"}).First[shop.Freight]().Freight(shop.Order{}); got != 8 {
		t.Fatalf("freight = %d", got)
	}
	if got := mustResolve(t, c, shop.Param{Biz: "trial", Abilities: []string{"free-shipping"}}).First[shop.Freight]().Freight(shop.Order{}); got != 0 {
		t.Fatalf("freight = %d", got)
	}
}

func TestCatalog(t *testing.T) {
	cat := mustBuild(t, shopBuilder().Strict(false)).Catalog()
	if cat.ParamType != "shop.Param" || cat.Strict || len(cat.Points) != 3 || cat.Points[0] != (easyext.PointInfo{Type: "shop.Freight", Default: "shop.DefaultFreight"}) {
		t.Fatalf("catalog = %+v", cat)
	}
	if a := cat.Abilities[0]; a.Code != "ability.free-shipping" || !slices.Equal(a.Points, []string{"shop.Freight"}) {
		t.Fatalf("abilities = %+v", cat.Abilities)
	}
	fresh := cat.Businesses[1]
	if fresh.Code != "biz.fresh" || !slices.Equal(fresh.Order, []string{"ability.free-shipping", "ability.rapid", easyext.Self}) ||
		!slices.Equal(fresh.Points, []string{"shop.Freight"}) {
		t.Fatalf("fresh = %+v", fresh)
	}
	if retail := cat.Businesses[0]; !slices.Equal(retail.Order, []string{easyext.Self, "ability.free-shipping", "ability.return-7d"}) {
		t.Fatalf("retail order = %v", retail.Order)
	}
}
