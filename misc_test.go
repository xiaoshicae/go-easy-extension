package easyext_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v2"
	"github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/shop"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := mustBuild(t, shopBuilder().Logger(logger))
	mustResolve(t, c, shop.Param{Biz: "fresh", Abilities: []string{"rapid"}})
	if _, err := c.Resolve(shop.Param{Biz: "unknown"}); err == nil {
		t.Fatal("expected an error")
	}
	out := buf.String()
	for _, want := range []string{
		`msg="easyext: resolved" business=biz.fresh chain="[ability.rapid biz.fresh]" skipped=[ability.free-shipping]`,
		`msg="easyext: resolution failed" param=shop.Param error="easyext: NO_BUSINESS_MATCHED: no business matched"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
}

func TestStringers(t *testing.T) {
	for k, want := range map[easyext.Kind]string{easyext.KindBusiness: "business", easyext.KindAbility: "ability", easyext.KindDefault: "default", 9: "Kind(9)"} {
		if k.String() != want {
			t.Errorf("Kind(%d) = %q", int(k), k.String())
		}
	}
	for r, want := range map[easyext.Reason]string{
		easyext.NoBinding: "NO_BINDING", easyext.NoBusinessMatched: "NO_BUSINESS_MATCHED",
		easyext.MultipleBusinessesMatched: "MULTIPLE_BUSINESSES_MATCHED", easyext.BusinessNotFound: "BUSINESS_NOT_FOUND",
		easyext.ExtensionNotFound: "EXTENSION_NOT_FOUND", 9: "Reason(9)",
	} {
		if r.String() != want {
			t.Errorf("Reason(%d) = %q", int(r), r.String())
		}
	}
	if got := easyext.ErrNoBinding.Error(); got != "easyext: NO_BINDING" {
		t.Errorf("sentinel message = %q", got)
	}
	if errors.Is(easyext.ErrNoBinding, easyext.ErrNoBusinessMatched) || errors.Is(errors.New("x"), easyext.ErrNoBinding) {
		t.Error("errors.Is matches a different reason")
	}
}

func TestAllAndExplainPanicOnUnregisteredPoint(t *testing.T) {
	r := mustResolve(t, mustBuild(t, shopBuilder()), shop.Param{Biz: "fresh"})
	for name, call := range map[string]func(){
		"All":     func() { r.All[unregistered]() },
		"Explain": func() { r.Explain[unregistered]() },
	} {
		func() {
			defer func() {
				if !errors.Is(recover().(error), easyext.ErrExtensionNotFound) {
					t.Errorf("%s: wrong panic", name)
				}
			}()
			call()
		}()
	}
}

func TestAllUnbound(t *testing.T) {
	if _, err := easyext.All[shop.Freight](context.Background()); !errors.Is(err, easyext.ErrNoBinding) {
		t.Fatalf("err = %v", err)
	}
}

func TestExplainSelectsTheDefault(t *testing.T) {
	r := mustResolve(t, mustBuild(t, shopBuilder()), shop.Param{Biz: "retail"}) // retail implements no Freight
	x := r.Explain[shop.Freight]()
	if want := (easyext.Link{Code: "shop.DefaultFreight", Kind: easyext.KindDefault}); x.Selected != want {
		t.Fatalf("selected = %+v, want %+v", x.Selected, want)
	}
}
