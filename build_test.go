package easyext_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

type notAPoint struct{}

type testAbility struct {
	code               string
	value              int
	requires, excludes []string
	match              func(shop.Param) bool
}

func (a *testAbility) Code() string            { return a.code }
func (a *testAbility) Match(p shop.Param) bool { return a.match == nil || a.match(p) }
func (a *testAbility) Freight(shop.Order) int  { return a.value }
func (a *testAbility) Requires() []string      { return a.requires }
func (a *testAbility) Excludes() []string      { return a.excludes }

type uselessAbility struct{}

func (uselessAbility) Code() string          { return "ability.useless" }
func (uselessAbility) Match(shop.Param) bool { return true }

// Metadata and Match are on the value; the extension method is on the pointer.
type ptrAbility struct{}

func (ptrAbility) Code() string            { return "ability.ptr" }
func (ptrAbility) Match(shop.Param) bool   { return true }
func (*ptrAbility) Freight(shop.Order) int { return 1 }

type mixedBusiness struct{ code string }

func (b mixedBusiness) Code() string          { return b.code }
func (mixedBusiness) Match(shop.Param) bool   { return true }
func (mixedBusiness) Abilities() []string     { return nil }
func (*mixedBusiness) Freight(shop.Order) int { return 99 }

func freightBuilder() *easyext.Builder[shop.Param] {
	return easyext.New[shop.Param]().Point[shop.Freight]().Default(shop.DefaultFreight{})
}

func TestBuildValidation(t *testing.T) {
	tests := []struct {
		name  string
		build func() *easyext.Builder[shop.Param]
		want  []string
	}{
		{"non-interface point", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[notAPoint]()
		}, []string{"is not an interface type"}},
		{"empty interface", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[any]()
		}, []string{"is an empty interface"}},
		{"duplicate point", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Point[shop.Freight]()
		}, []string{"is registered more than once"}},
		{"missing default", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[shop.Freight]()
		}, []string{"has no default implementation"}},
		{"nil defaults", func() *easyext.Builder[shop.Param] {
			var typed *shop.DefaultFreight
			return easyext.New[shop.Param]().Point[shop.Freight]().Default(nil).Default(typed)
		}, []string{"default implementation at index 0 is nil", "default implementation at index 1 is nil"}},
		{"unused default", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Default(notAPoint{})
		}, []string{"implements no registered extension point"}},
		{"conflicting defaults", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Default(shop.Defaults{})
		}, []string{"has more than one default implementation"}},
		{"same default registered twice", func() *easyext.Builder[shop.Param] {
			d := &shop.DefaultFreight{}
			return easyext.New[shop.Param]().Point[shop.Freight]().Default(d).DefaultFor[shop.Freight](d)
		}, []string{"default implementation *shop.DefaultFreight is registered more than once as the default of extension point shop.Freight"}},
		{"DefaultFor requires registered point", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().DefaultFor[shop.Freight](shop.DefaultFreight{})
		}, []string{"targets unregistered extension point shop.Freight"}},
		{"nil DefaultFor", func() *easyext.Builder[shop.Param] {
			var typed *shop.DefaultFreight
			return easyext.New[shop.Param]().Point[shop.Freight]().DefaultFor[shop.Freight](typed)
		}, []string{"default implementation at index 0 is nil"}},
		{"empty and reserved codes", func() *easyext.Builder[shop.Param] {
			return freightBuilder().
				Ability(&testAbility{code: ""}).Ability(&testAbility{code: "  "}).
				Ability(&testAbility{code: " a "}).Ability(&testAbility{code: easyext.Self}).
				Business(&testBusiness[shop.Param]{code: ""}).Business(&testBusiness[shop.Param]{code: easyext.Self})
		}, []string{"ability with an empty code", "has leading or trailing whitespace", "is reserved", "business with an empty code"}},
		{"duplicate ability code", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(shop.FreeShipping{}).Ability(shop.FreeShipping{})
		}, []string{"already used by an ability"}},
		{"duplicate business code", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Business(&testBusiness[shop.Param]{code: "biz"}).Business(&testBusiness[shop.Param]{code: "biz"})
		}, []string{"already used by"}},
		{"cross-role code collision", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(shop.FreeShipping{}).Business(&testBusiness[shop.Param]{code: shop.FreeShippingCode})
		}, []string{"business code", "already used by an ability"}},
		{"nil and typed nil components", func() *easyext.Builder[shop.Param] {
			var ability *testAbility
			var business *testBusiness[shop.Param]
			return freightBuilder().Ability(nil).Ability(ability).Business(nil).Business(business)
		}, []string{"ability at index 0: implementation is nil", "ability at index 1: implementation is nil",
			"business at index 0: implementation is nil", "business at index 1: implementation is nil"}},
		{"ability without a point", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(uselessAbility{})
		}, []string{"implements no registered extension point"}},
		{"pointer receivers", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(ptrAbility{}).Business(mixedBusiness{code: "biz.mixed"})
		}, []string{"pointer receiver", "ability \"ability.ptr\"", "business \"biz.mixed\""}},
		{"default pointer receiver", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[shop.Freight]().Default(ptrAbility{})
		}, []string{"pointer receiver"}},
		{"invalid abilities declaration", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(shop.FreeShipping{}).
				Business(&testBusiness[shop.Param]{code: "biz.x", used: []string{"unknown", shop.FreeShippingCode, shop.FreeShippingCode, easyext.Self, easyext.Self}})
		}, []string{"uses unknown ability", "is listed more than once", "Self is listed more than once"}},
		{"business code is not an ability reference", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Business(&testBusiness[shop.Param]{code: "biz.x", used: []string{"biz.x"}})
		}, []string{"uses unknown ability \"biz.x\""}},
		{"unknown constraint refs", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"missing"}, excludes: []string{"other"}})
		}, []string{"requires unknown ability", "excludes unknown ability"}},
		{"reserved constraint ref", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{easyext.Self}})
		}, []string{"requires unknown ability"}},
		{"self requirements unmounted", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"a"}, excludes: []string{"a"}})
		}, []string{"cannot require itself", "cannot exclude itself"}},
		{"contradictory unmounted ability", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"b"}, excludes: []string{"b"}}).
				Ability(&testAbility{code: "b"})
		}, []string{"both requires and excludes"}},
		{"reverse exclusion unmounted", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"b"}}).
				Ability(&testAbility{code: "b", excludes: []string{"a"}})
		}, []string{"requirements cannot be satisfied", "ability \"b\" excludes ability \"a\""}},
		{"transitive exclusion unmounted", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"b"}, excludes: []string{"c"}}).
				Ability(&testAbility{code: "b", requires: []string{"c"}}).Ability(&testAbility{code: "c"})
		}, []string{"requirements cannot be satisfied", "excludes ability \"c\""}},
		{"duplicate constraint refs", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"b", "b"}, excludes: []string{"c", "c"}}).
				Ability(&testAbility{code: "b"}).Ability(&testAbility{code: "c"})
		}, []string{"requires repeats ability", "excludes repeats ability"}},
		{"missing co-mounted requirement", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", requires: []string{"b"}}).Ability(&testAbility{code: "b"}).
				Business(&testBusiness[shop.Param]{code: "biz", used: []string{"a"}})
		}, []string{"requires ability \"b\" to be listed as well"}},
		{"excluded co-mount", func() *easyext.Builder[shop.Param] {
			return freightBuilder().Ability(&testAbility{code: "a", excludes: []string{"b"}}).Ability(&testAbility{code: "b"}).
				Business(&testBusiness[shop.Param]{code: "biz", used: []string{"a", "b"}})
		}, []string{"business \"biz\": ability \"a\" excludes ability \"b\""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := tt.build().Build()
			var registration *easyext.RegistrationError
			if registry != nil || !errors.As(err, &registration) {
				t.Fatalf("registry = %v, err = %v", registry, err)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error lacks %q:\n%s", want, err)
				}
			}
		})
	}
}

func TestRegistrationErrorsAggregateDeterministically(t *testing.T) {
	builder := easyext.New[shop.Param]().Point[notAPoint]().Point[shop.Freight]().Default(nil)
	_, one := builder.Build()
	_, two := builder.Build()
	var a, b *easyext.RegistrationError
	if !errors.As(one, &a) || !errors.As(two, &b) {
		t.Fatalf("errors = %v / %v", one, two)
	}
	want := []string{
		"extension point easyext_test.notAPoint is not an interface type",
		"default implementation at index 0 is nil",
		"extension point shop.Freight has no default implementation",
	}
	if !slices.Equal(a.Problems, want) || !slices.Equal(a.Problems, b.Problems) {
		t.Fatalf("problems = %q / %q, want %q", a.Problems, b.Problems, want)
	}
	if !strings.HasPrefix(one.Error(), "easyext: invalid assembly:\n  - ") {
		t.Fatalf("aggregate message = %q", one)
	}
	_, err := easyext.New[shop.Param]().Point[notAPoint]().Build()
	if err.Error() != "easyext: invalid assembly: "+want[0] {
		t.Fatalf("single message = %q", err)
	}
}

func TestRequirementConflictsFollowRegistrationOrder(t *testing.T) {
	builder := freightBuilder().
		Ability(&testAbility{code: "a", requires: []string{"c", "b"}}).
		Ability(&testAbility{code: "b", excludes: []string{"c"}}).
		Ability(&testAbility{code: "c", requires: []string{"d"}}).
		Ability(&testAbility{code: "d", excludes: []string{"b"}}).
		Ability(&testAbility{code: "e", requires: []string{"b", "c"}})
	want := []string{
		"ability \"a\": requirements cannot be satisfied: ability \"b\" excludes ability \"c\"",
		"ability \"a\": requirements cannot be satisfied: ability \"d\" excludes ability \"b\"",
		"ability \"e\": requirements cannot be satisfied: ability \"b\" excludes ability \"c\"",
		"ability \"e\": requirements cannot be satisfied: ability \"d\" excludes ability \"b\"",
	}
	for range 2 {
		_, err := builder.Build()
		var registration *easyext.RegistrationError
		if !errors.As(err, &registration) || !slices.Equal(registration.Problems, want) {
			t.Fatalf("Build = %v; want problems %q", err, want)
		}
	}
}

func TestDefaultForDoesNotClaimOtherPoints(t *testing.T) {
	registry := mustBuild(t, easyext.New[shop.Param]().Point[shop.Freight]().Point[shop.Notify]().
		DefaultFor[shop.Freight](shop.Defaults{}).Default(shop.DefaultNotify{}).
		Business(&testBusiness[shop.Param]{code: "biz"}))
	result := mustResolve(t, registry, shop.Param{})
	if first[shop.Freight](t, result).Freight(shop.Order{}) != 8 ||
		len(first[shop.Notify](t, result).Channels()) != 1 {
		t.Fatal("DefaultFor did not restrict binding")
	}
}

func TestPointerRegistrationWorks(t *testing.T) {
	mustBuild(t, freightBuilder().Ability(&ptrAbility{}).Business(&mixedBusiness{code: "biz"}))
}

func TestCoMountCyclesAreNotExecutionCycles(t *testing.T) {
	registry := mustBuild(t, freightBuilder().
		Ability(&testAbility{code: "a", value: 11, requires: []string{"b"}}).
		Ability(&testAbility{code: "b", value: 22, requires: []string{"a"}, match: func(shop.Param) bool { return false }}).
		Business(&testBusiness[shop.Param]{code: "biz", used: []string{"a", "b"}}))
	result := mustResolve(t, registry, shop.Param{})
	if got := first[shop.Freight](t, result).Freight(shop.Order{}); got != 11 {
		t.Fatalf("freight = %d", got)
	}
	if !slices.Equal(result.Trace().Skipped, []string{"b"}) {
		t.Fatal("Requires incorrectly forced activation")
	}
}
