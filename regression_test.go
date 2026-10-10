package easyext_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

type countedRapid struct {
	shop.RapidDelivery
	calls int
}

func (a *countedRapid) Match(p shop.Param) bool {
	a.calls++
	return a.RapidDelivery.Match(p)
}

func TestActivationIsOneSnapshotForAllPoints(t *testing.T) {
	rapid := &countedRapid{}
	unused := &testAbility{code: "ability.unused", match: func(shop.Param) bool {
		t.Fatal("unmounted ability was matched")
		return false
	}}
	registry := mustBuild(t, easyext.New[shop.Param]().
		Point[shop.Freight]().Point[shop.Notify]().Point[shop.Delivery]().Default(shop.Defaults{}).
		Ability(shop.FreeShipping{}).Ability(rapid).Ability(unused).Business(shop.Fresh{}))
	flags := []string{"rapid"}
	result := mustResolve(t, registry, shop.Param{Biz: "fresh", Abilities: flags})
	flags[0] = "changed-after-resolve"
	for range 3 {
		if first[shop.Delivery](t, result).DeliveryDays() != 1 {
			t.Fatal("snapshot changed")
		}
		if len(first[shop.Notify](t, result).Channels()) != 3 {
			t.Fatal("ability's other point not activated")
		}
		if len(all[shop.Delivery](t, result)) != 3 {
			t.Fatal("wrong All chain")
		}
		if _, err := result.Explain[shop.Notify](); err != nil {
			t.Fatal(err)
		}
		result.Trace()
		registry.Catalog()
	}
	if rapid.calls != 1 {
		t.Fatalf("Match called %d times", rapid.calls)
	}
}

type businessWithPoints struct {
	testBusiness[shop.Param]
	value int
}

func (b *businessWithPoints) Freight(shop.Order) int { return b.value }
func (b *businessWithPoints) DeliveryDays() int      { return b.value }
func (b *businessWithPoints) Channels() []string     { return []string{"business"} }

func TestBusinessOwnsSharedAbilityPrecedence(t *testing.T) {
	a := &businessWithPoints{testBusiness: testBusiness[shop.Param]{
		code: "biz.a", used: []string{shop.RapidDeliveryCode, easyext.Self},
		match: func(p shop.Param) bool { return p.Biz == "a" },
	}, value: 5}
	b := &businessWithPoints{testBusiness: testBusiness[shop.Param]{
		code: "biz.b", used: []string{easyext.Self, shop.RapidDeliveryCode},
		match: func(p shop.Param) bool { return p.Biz == "b" },
	}, value: 9}
	registry := mustBuild(t, easyext.New[shop.Param]().
		Point[shop.Freight]().Point[shop.Notify]().Point[shop.Delivery]().Default(shop.Defaults{}).
		Ability(shop.RapidDelivery{}).Business(a).Business(b))
	for _, tt := range []struct {
		biz                         string
		delivery, freight, channels int
	}{{"a", 1, 5, 3}, {"b", 9, 9, 1}} {
		result := mustResolve(t, registry, shop.Param{Biz: tt.biz, Abilities: []string{"rapid"}})
		if first[shop.Delivery](t, result).DeliveryDays() != tt.delivery ||
			first[shop.Freight](t, result).Freight(shop.Order{}) != tt.freight ||
			len(first[shop.Notify](t, result).Channels()) != tt.channels {
			t.Fatalf("business %q lost its precedence", tt.biz)
		}
	}
}

func TestMultipleAbilitiesForOnePointHaveBusinessLocalOrder(t *testing.T) {
	registry := mustBuild(t, freightBuilder().
		Ability(&testAbility{code: "a", value: 11}).Ability(&testAbility{code: "b", value: 22}).
		Business(&testBusiness[shop.Param]{code: "biz.a", used: []string{"a", "b"}, match: func(p shop.Param) bool { return p.Biz == "a" }}).
		Business(&testBusiness[shop.Param]{code: "biz.b", used: []string{"b", "a"}, match: func(p shop.Param) bool { return p.Biz == "b" }}))
	for _, tt := range []struct {
		biz  string
		want []int
	}{{"a", []int{11, 22, 8}}, {"b", []int{22, 11, 8}}} {
		result := mustResolve(t, registry, shop.Param{Biz: tt.biz})
		var got []int
		for _, impl := range all[shop.Freight](t, result) {
			got = append(got, impl.Freight(shop.Order{}))
		}
		if !slices.Equal(got, tt.want) {
			t.Fatalf("%s: %v, want %v", tt.biz, got, tt.want)
		}
	}
}

func TestAllDeduplicatesSharedPointerAcrossRoles(t *testing.T) {
	shared := &testAbility{code: "ability.shared", value: 44}
	registry := mustBuild(t, easyext.New[shop.Param]().Point[shop.Freight]().Default(shared).Ability(shared).
		Business(&testBusiness[shop.Param]{code: "biz", used: []string{shared.code}}))
	result := mustResolve(t, registry, shop.Param{})
	impls := all[shop.Freight](t, result)
	if len(impls) != 1 || impls[0] != shared {
		t.Fatalf("All = %v, want shared once", impls)
	}
	x, err := result.Explain[shop.Freight]()
	if err != nil || x.Candidates[len(x.Candidates)-1].Reason != "duplicate" {
		t.Fatalf("explanation = %+v, %v", x, err)
	}
}

func TestSharedDefaultRemainsAvailableWhenAbilityDoesNotMatch(t *testing.T) {
	shared := &testAbility{code: "ability.shared", value: 44, match: func(shop.Param) bool { return false }}
	registry := mustBuild(t, easyext.New[shop.Param]().Point[shop.Freight]().Default(shared).Ability(shared).
		Business(&testBusiness[shop.Param]{code: "biz", used: []string{shared.code}}))
	result := mustResolve(t, registry, shop.Param{})
	if got := all[shop.Freight](t, result); len(got) != 1 || got[0] != shared {
		t.Fatalf("All = %v", got)
	}
	x, err := result.Explain[shop.Freight]()
	if err != nil || x.Selected.Kind != easyext.KindDefault {
		t.Fatalf("explanation = %+v, %v", x, err)
	}
}

func TestAllDoesNotMergeEqualValueRegistrations(t *testing.T) {
	component := shop.FreeShipping{}
	registry := mustBuild(t, easyext.New[shop.Param]().Point[shop.Freight]().Default(component).Ability(component).
		Business(&testBusiness[shop.Param]{code: "biz", used: []string{shop.FreeShippingCode}}))
	result := mustResolve(t, registry, shop.Param{Abilities: []string{"free-shipping"}})
	if len(all[shop.Freight](t, result)) != 2 {
		t.Fatal("equal values were merged as one instance")
	}
}

type mapAbility map[string]int

func (mapAbility) Code() string             { return "ability.map" }
func (mapAbility) Match(shop.Param) bool    { return true }
func (m mapAbility) Freight(shop.Order) int { return m["value"] }

func TestNonComparableProvidersDoNotPanic(t *testing.T) {
	component := mapAbility{"value": 11}
	registry := mustBuild(t, easyext.New[shop.Param]().Point[shop.Freight]().Default(component).Ability(component).
		Business(&testBusiness[shop.Param]{code: "biz", used: []string{"ability.map"}}))
	result := mustResolve(t, registry, shop.Param{})
	if len(all[shop.Freight](t, result)) != 2 {
		t.Fatal("non-pointer registrations should have independent identities")
	}
}

type nilFunctionAbility func()

func (nilFunctionAbility) Code() string           { panic("metadata must not be called on a nil function") }
func (nilFunctionAbility) Match(shop.Param) bool  { panic("nil function matched") }
func (nilFunctionAbility) Freight(shop.Order) int { return 0 }

type nilSliceAbility []int

func (nilSliceAbility) Code() string           { panic("metadata must not be called on a nil slice") }
func (nilSliceAbility) Match(shop.Param) bool  { return true }
func (nilSliceAbility) Freight(shop.Order) int { return 0 }

type nilChannelAbility chan int

func (nilChannelAbility) Code() string           { panic("metadata must not be called on a nil channel") }
func (nilChannelAbility) Match(shop.Param) bool  { return true }
func (nilChannelAbility) Freight(shop.Order) int { return 0 }

func TestNilKindsAreRejectedBeforeMetadataCalls(t *testing.T) {
	for name, component := range map[string]easyext.Ability[shop.Param]{
		"map": mapAbility(nil), "slice": nilSliceAbility(nil), "function": nilFunctionAbility(nil), "channel": nilChannelAbility(nil),
	} {
		t.Run(name, func(t *testing.T) {
			if registry, err := freightBuilder().Ability(component).Build(); registry != nil || err == nil {
				t.Fatalf("registry = %v, err = %v", registry, err)
			}
			if registry, err := easyext.New[shop.Param]().Point[shop.Freight]().Default(component).Build(); registry != nil || err == nil {
				t.Fatalf("nil default: registry = %v, err = %v", registry, err)
			}
		})
	}
}

type metadataAbility struct {
	testAbility
	codeCalls, requiresCalls, excludesCalls int
}

func (a *metadataAbility) Code() string       { a.codeCalls++; return a.code }
func (a *metadataAbility) Requires() []string { a.requiresCalls++; return a.requires }
func (a *metadataAbility) Excludes() []string { a.excludesCalls++; return a.excludes }

type metadataBusiness struct {
	testBusiness[shop.Param]
	codeCalls, abilitiesCalls int
}

func (b *metadataBusiness) Code() string        { b.codeCalls++; return b.code }
func (b *metadataBusiness) Abilities() []string { b.abilitiesCalls++; return b.used }

func TestBuildSnapshotsMetadataAndCopiesCatalog(t *testing.T) {
	ability := &metadataAbility{testAbility: testAbility{
		code: "a.main", value: 44, requires: []string{"a.dep"}, excludes: []string{"a.block"},
	}}
	business := &metadataBusiness{testBusiness: testBusiness[shop.Param]{code: "biz", used: []string{"a.main", "a.dep", easyext.Self}}}
	registry := mustBuild(t, freightBuilder().Ability(ability).
		Ability(&testAbility{code: "a.dep"}).Ability(&testAbility{code: "a.block"}).Business(business))
	ability.code = "changed-ability"
	ability.requires[0] = "changed-required"
	ability.excludes[0] = "changed-excluded"
	business.code = "changed-business"
	business.used[0] = "changed-mount"
	result := mustResolve(t, registry, shop.Param{})
	if result.Business() != "biz" || first[shop.Freight](t, result).Freight(shop.Order{}) != 44 {
		t.Fatal("metadata was not snapshotted")
	}
	catalog := registry.Catalog()
	if catalog.Abilities[0].Code != "a.main" || catalog.Abilities[0].Requires[0] != "a.dep" ||
		catalog.Abilities[0].Excludes[0] != "a.block" || catalog.Businesses[0].Abilities[0] != "a.main" {
		t.Fatalf("catalog = %+v", catalog)
	}
	catalog.Abilities[0].Requires[0] = "mutated"
	catalog.Abilities[0].Excludes[0] = "mutated"
	if registry.Catalog().Abilities[0].Requires[0] != "a.dep" || registry.Catalog().Abilities[0].Excludes[0] != "a.block" {
		t.Fatal("catalog aliases stored constraints")
	}
	result.Trace()
	result.Explain[shop.Freight]()
	all[shop.Freight](t, result)
	if ability.codeCalls != 1 || ability.requiresCalls != 1 || ability.excludesCalls != 1 ||
		business.codeCalls != 1 || business.abilitiesCalls != 1 {
		t.Fatalf("metadata read more than once: %+v / %+v", ability, business)
	}
}

// Each optional constraint method is recognized independently, without requiring its counterpart.
type requiresOnly struct{ shop.FreeShipping }

func (requiresOnly) Requires() []string { return []string{"missing"} }

type excludesOnly struct{ shop.FreeShipping }

func (excludesOnly) Excludes() []string { return []string{"missing"} }

func TestOptionalConstraintsAreIndependent(t *testing.T) {
	for _, component := range []easyext.Ability[shop.Param]{requiresOnly{}, excludesOnly{}} {
		if _, err := freightBuilder().Ability(component).Build(); err == nil {
			t.Fatal("optional constraint was ignored")
		}
	}
}

type changingCodeComponent struct{ calls int }

func (c *changingCodeComponent) Code() string {
	c.calls++
	if c.calls == 1 {
		return "component.first"
	}
	return "component.second"
}
func (*changingCodeComponent) Match(shop.Param) bool  { return true }
func (*changingCodeComponent) Freight(shop.Order) int { return 1 }
func (*changingCodeComponent) Abilities() []string    { return nil }

func TestOnePointerCannotClaimDifferentCodes(t *testing.T) {
	for _, crossRole := range []bool{false, true} {
		component := &changingCodeComponent{}
		builder := freightBuilder().Ability(component)
		if crossRole {
			builder.Business(component)
		} else {
			builder.Ability(component)
		}
		registry, err := builder.Build()
		if registry != nil || err == nil || !strings.Contains(err.Error(), "inconsistent codes") {
			t.Fatalf("crossRole=%v: registry = %v, err = %v", crossRole, registry, err)
		}
	}
}
