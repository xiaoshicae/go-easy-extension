package httpx_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	"github.com/xiaoshicae/go-easy-extension/v3/httpx"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

func newRegistry(t *testing.T) *easyext.Registry[shop.Param] {
	t.Helper()
	registry, err := easyext.New[shop.Param]().
		Point[shop.Freight]().Point[shop.Notify]().Point[shop.Delivery]().Default(shop.Defaults{}).
		Ability(shop.FreeShipping{}).Ability(shop.RapidDelivery{}).Business(shop.Fresh{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func paramFromRequest(r *http.Request) (shop.Param, error) {
	biz := r.Header.Get("X-Biz")
	if biz == "" {
		return shop.Param{}, errors.New("missing X-Biz header")
	}
	var abilities []string
	if value := r.URL.Query().Get("abilities"); value != "" {
		abilities = strings.Split(value, ",")
	}
	return shop.Param{Biz: biz, Abilities: abilities}, nil
}

func freightHandler(registry *easyext.Registry[shop.Param]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		impl, err := registry.First[shop.Freight](r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, impl.Freight(shop.Order{Items: 3}))
	})
}

func serve(handler http.Handler, header, target string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if header != "" {
		req.Header.Set("X-Biz", header)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	response := recorder.Result()
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return recorder.Code, strings.TrimSpace(string(body))
}

func TestMiddleware(t *testing.T) {
	registry := newRegistry(t)
	handler := httpx.Middleware(registry, paramFromRequest)(freightHandler(registry))
	for _, tt := range []struct {
		name, header, target string
		status               int
		body                 string
	}{
		{"fresh", "fresh", "/checkout", 200, "21"},
		{"free shipping overrides fresh", "fresh", "/checkout?abilities=free-shipping", 200, "0"},
		{"param error", "", "/checkout", 400, "Bad Request"},
		{"no business", "unknown", "/checkout", 422, "Unprocessable Entity"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if status, body := serve(handler, tt.header, tt.target); status != tt.status || body != tt.body {
				t.Fatalf("got %d %q, want %d %q", status, body, tt.status, tt.body)
			}
		})
	}
}

type otherFresh struct{ shop.Fresh }

func (otherFresh) Code() string { return "biz.fresh-too" }

func TestSeveralBusinessesIsAServerError(t *testing.T) {
	registry, err := easyext.New[shop.Param]().
		Point[shop.Freight]().Point[shop.Notify]().Point[shop.Delivery]().Default(shop.Defaults{}).
		Ability(shop.FreeShipping{}).Ability(shop.RapidDelivery{}).
		Business(shop.Fresh{}).Business(otherFresh{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	handler := httpx.Middleware(registry, paramFromRequest)(freightHandler(registry))
	if status, body := serve(handler, "fresh", "/"); status != 500 || body != "Internal Server Error" {
		t.Fatalf("got %d %q", status, body)
	}
}

func TestOnError(t *testing.T) {
	registry := newRegistry(t)
	var got error
	handler := httpx.Middleware(registry, paramFromRequest, httpx.OnError(func(w http.ResponseWriter, _ *http.Request, err error) {
		got = err
		http.Error(w, "custom", http.StatusTeapot)
	}))(freightHandler(registry))
	if status, body := serve(handler, "unknown", "/"); status != http.StatusTeapot || body != "custom" || !errors.Is(got, easyext.ErrNoBusinessMatched) {
		t.Fatalf("got %d %q, err %v", status, body, got)
	}
	serve(handler, "", "/")
	var paramError *httpx.ParamError
	if !errors.As(got, &paramError) || paramError.Error() != "httpx: deriving the matcher param: missing X-Biz header" ||
		errors.Unwrap(paramError).Error() != "missing X-Biz header" {
		t.Fatalf("param error = %v", got)
	}
}

func TestUnwrappedRoutesAreNotBound(t *testing.T) {
	registry := newRegistry(t)
	mux := http.NewServeMux()
	mux.Handle("/checkout", httpx.Middleware(registry, paramFromRequest)(freightHandler(registry)))
	mux.Handle("/health", freightHandler(registry))
	if status, body := serve(mux, "", "/health"); status != 500 || !strings.Contains(body, "NO_BINDING") {
		t.Fatalf("got %d %q", status, body)
	}
	if status, body := serve(mux, "fresh", "/checkout"); status != 200 || body != "21" {
		t.Fatalf("got %d %q", status, body)
	}
}

func TestNestedMiddlewareKeepsBothRegistryBindings(t *testing.T) {
	a, b := newRegistry(t), newRegistry(t)
	check := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fa, ea := a.First[shop.Freight](r.Context())
		fb, eb := b.First[shop.Freight](r.Context())
		if ea != nil || eb != nil {
			t.Errorf("bindings: %v / %v", ea, eb)
			return
		}
		fmt.Fprintf(w, "%d/%d", fa.Freight(shop.Order{Items: 3}), fb.Freight(shop.Order{Items: 3}))
	})
	handler := httpx.Middleware(a, paramFromRequest)(
		httpx.Middleware(b, func(*http.Request) (shop.Param, error) {
			return shop.Param{Biz: "fresh", Abilities: []string{"free-shipping"}}, nil
		})(check))
	if status, body := serve(handler, "fresh", "/"); status != 200 || body != "21/0" {
		t.Fatalf("got %d %q", status, body)
	}
}
