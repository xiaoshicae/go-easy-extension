package easyext_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	registry := mustBuild(t, shopBuilder().Logger(logger))
	mustResolve(t, registry, shop.Param{Biz: "fresh", Abilities: []string{"rapid"}})
	if _, err := registry.Resolve(shop.Param{Biz: "unknown"}); err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{
		"msg=\"easyext: resolved\" business=biz.fresh chain=\"[ability.rapid biz.fresh]\" skipped=[ability.free-shipping]",
		"msg=\"easyext: resolution failed\" paramType=shop.Param error=\"easyext: NO_BUSINESS_MATCHED: none of the 3 registered businesses matched\"",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, buf.String())
		}
	}
}

func TestStringersAndErrorMatching(t *testing.T) {
	for kind, want := range map[easyext.Kind]string{
		easyext.KindBusiness: "business", easyext.KindAbility: "ability", easyext.KindDefault: "default", 9: "Kind(9)",
	} {
		if kind.String() != want {
			t.Errorf("Kind(%d) = %q", kind, kind.String())
		}
	}
	for reason, want := range map[easyext.Reason]string{
		easyext.NoBinding: "NO_BINDING", easyext.NoBusinessMatched: "NO_BUSINESS_MATCHED",
		easyext.MultipleBusinessesMatched: "MULTIPLE_BUSINESSES_MATCHED", easyext.ExtensionNotFound: "EXTENSION_NOT_FOUND", 9: "Reason(9)",
	} {
		if reason.String() != want {
			t.Errorf("Reason(%d) = %q", reason, reason.String())
		}
	}
	if got := easyext.ErrNoBinding.Error(); got != "easyext: NO_BINDING" {
		t.Errorf("sentinel = %q", got)
	}
	if errors.Is(easyext.ErrNoBinding, easyext.ErrNoBusinessMatched) || errors.Is(easyext.ErrNoBinding, errors.New("x")) ||
		errors.Is(easyext.ErrNoBinding, (*easyext.ResolutionError)(nil)) {
		t.Error("errors.Is matched a different or nil target")
	}
}

func TestExplainSelectsDefault(t *testing.T) {
	result := mustResolve(t, mustBuild(t, shopBuilder()), shop.Param{Biz: "retail"})
	x, err := result.Explain[shop.Freight]()
	if err != nil {
		t.Fatal(err)
	}
	if x.Selected != (easyext.Link{Code: "shop.Defaults", Kind: easyext.KindDefault}) ||
		x.Candidates[len(x.Candidates)-1].Reason != "selected" {
		t.Fatalf("explanation = %+v", x)
	}
}

func TestMatcherFuncCanBeEmbedded(t *testing.T) {
	business := &functionBusiness{MatcherFunc: func(p shop.Param) bool { return p.Biz == "fn" }}
	registry := mustBuild(t, freightBuilder().Business(business))
	if mustResolve(t, registry, shop.Param{Biz: "fn"}).Business() != "biz.fn" {
		t.Fatal("function matcher not used")
	}
}

type functionBusiness struct {
	easyext.MatcherFunc[shop.Param]
}

func (*functionBusiness) Code() string        { return "biz.fn" }
func (*functionBusiness) Abilities() []string { return nil }
