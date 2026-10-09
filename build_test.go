package easyext_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v2"
	"github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/shop"
)

type notAPoint struct{}

type requiresFree struct{ shop.RapidDelivery }

type excludesRapid struct{ shop.Return7Days }

func TestBuildValidation(t *testing.T) {
	matchAll := easyext.MatcherFunc[shop.Param](func(shop.Param) bool { return true })
	tests := []struct {
		name  string
		build func() *easyext.Builder[shop.Param]
		want  []string
	}{
		{"extension point must be an interface", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[notAPoint](notAPoint{})
		}, []string{"extension point easyext_test.notAPoint is not an interface type"}},
		{"default implementation must not be nil", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[shop.Freight](nil)
		}, []string{"extension point shop.Freight: default implementation is nil"}},
		{"extension point registered twice", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[shop.Freight](shop.DefaultFreight{}).Point[shop.Freight](shop.DefaultFreight{})
		}, []string{"extension point shop.Freight is registered more than once"}},
		{"codes: empty, reserved, duplicated", func() *easyext.Builder[shop.Param] {
			return shopBuilder().
				Ability("", shop.FreeShipping{}).
				Ability(easyext.Self, shop.FreeShipping{}).
				Business("ability.rapid", matchAll)
		}, []string{
			"ability with an empty code",
			`ability code "<self>" is reserved`,
			`business code "ability.rapid" is already used by a ability`,
		}},
		{"implementations", func() *easyext.Builder[shop.Param] {
			return easyext.New[shop.Param]().Point[shop.Freight](shop.DefaultFreight{}).
				Ability("ability.nil", nil).
				Ability("ability.useless", matchAll).
				Business("biz.nil", nil).
				Business("biz.no-matcher", shop.DefaultFreight{})
		}, []string{
			`ability "ability.nil": implementation is nil`,
			`ability "ability.useless" (easyext.MatcherFunc[...]) implements no registered extension point`,
			`business "biz.nil": implementation is nil`,
			`business "biz.no-matcher" (shop.DefaultFreight) does not implement Matcher[shop.Param] and no business resolver is configured`,
		}},
		{"abilities list", func() *easyext.Builder[shop.Param] {
			return shopBuilder().Business("biz.x", matchAll,
				easyext.Abilities("ability.unknown", "ability.rapid", "ability.rapid", easyext.Self, easyext.Self))
		}, []string{
			`business "biz.x": mounts unknown ability "ability.unknown"`,
			`business "biz.x": ability "ability.rapid" is mounted more than once`,
			`business "biz.x": Self is listed more than once`,
		}},
		{"requires and excludes", func() *easyext.Builder[shop.Param] {
			return shopBuilder().
				Ability("ability.needs-free", requiresFree{}, easyext.Requires("ability.free-shipping")).
				Ability("ability.no-rapid", excludesRapid{}, easyext.Excludes("ability.rapid")).
				Ability("ability.dangling", requiresFree{}, easyext.Requires("ability.ghost")).
				Business("biz.x", matchAll, easyext.Abilities("ability.needs-free", "ability.no-rapid", "ability.rapid"))
		}, []string{
			`ability "ability.dangling": requires/excludes unknown ability "ability.ghost"`,
			`business "biz.x": ability "ability.needs-free" requires ability "ability.free-shipping" to be mounted as well`,
			`business "biz.x": ability "ability.no-rapid" excludes ability "ability.rapid"`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.build().Build()
			re, ok := errors.AsType[*easyext.RegistrationError](err)
			if !ok {
				t.Fatalf("err = %v, want a *RegistrationError", err)
			}
			got := slices.Clone(re.Problems)
			for i, p := range got { // the generic type name of MatcherFunc is long; compare its stable prefix
				if j := len(`ability "ability.useless" (easyext.MatcherFunc[`); len(p) > j && p[:j] == `ability "ability.useless" (easyext.MatcherFunc[` {
					got[i] = `ability "ability.useless" (easyext.MatcherFunc[...]) implements no registered extension point`
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("problems:\n  got  %q\n  want %q", got, tt.want)
			}
		})
	}
}

func TestRegistrationErrorMessage(t *testing.T) {
	_, err := easyext.New[shop.Param]().Point[notAPoint](notAPoint{}).Build()
	if want := "easyext: invalid assembly: extension point easyext_test.notAPoint is not an interface type"; err.Error() != want {
		t.Fatalf("got %q", err.Error())
	}
	_, err = easyext.New[shop.Param]().Point[notAPoint](notAPoint{}).Point[shop.Freight](nil).Build()
	want := "easyext: invalid assembly:\n  - extension point easyext_test.notAPoint is not an interface type\n  - extension point shop.Freight: default implementation is nil"
	if err.Error() != want {
		t.Fatalf("got %q", err.Error())
	}
}

func TestBuildDoesNotLeakBetweenCalls(t *testing.T) {
	b := easyext.New[shop.Param]().Point[notAPoint](notAPoint{})
	_, err1 := b.Build()
	_, err2 := b.Build()
	p1, _ := errors.AsType[*easyext.RegistrationError](err1)
	p2, _ := errors.AsType[*easyext.RegistrationError](err2)
	if len(p1.Problems) != 1 || len(p2.Problems) != 1 {
		t.Fatalf("problems accumulate across Build calls: %v / %v", p1.Problems, p2.Problems)
	}
}
