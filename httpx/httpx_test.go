package httpx_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v2"
	"github.com/xiaoshicae/go-easy-extension/v2/httpx"
	"github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/shop"
)

func newContext(t *testing.T) *easyext.Context[shop.Param] {
	t.Helper()
	c, err := easyext.New[shop.Param]().
		Point[shop.Freight](shop.DefaultFreight{}).
		Ability("ability.free-shipping", shop.FreeShipping{}).
		Business("biz.fresh", shop.Fresh{}, easyext.Abilities("ability.free-shipping", easyext.Self)).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func paramFromRequest(r *http.Request) (shop.Param, error) {
	biz := r.Header.Get("X-Biz")
	if biz == "" {
		return shop.Param{}, errors.New("missing X-Biz header")
	}
	var abilities []string
	if a := r.URL.Query().Get("abilities"); a != "" {
		abilities = strings.Split(a, ",")
	}
	return shop.Param{Biz: biz, Abilities: abilities}, nil
}

var freightHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	f, err := easyext.First[shop.Freight](r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, f.Freight(shop.Order{Items: 3}))
})

func serve(h http.Handler, header, target string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if header != "" {
		req.Header.Set("X-Biz", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, strings.TrimSpace(string(body))
}

func TestMiddleware(t *testing.T) {
	h := httpx.Middleware(newContext(t), paramFromRequest)(freightHandler)
	tests := []struct {
		name, header, target string
		code                 int
		body                 string
	}{
		{"bound to fresh", "fresh", "/checkout", 200, "21"},
		{"free shipping overrides fresh", "fresh", "/checkout?abilities=free-shipping", 200, "0"},
		{"param error", "", "/checkout", 400, "Bad Request"},
		{"no business (strict)", "unknown", "/checkout", 422, "Unprocessable Entity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, body := serve(h, tt.header, tt.target); code != tt.code || body != tt.body {
				t.Fatalf("got %d %q, want %d %q", code, body, tt.code, tt.body)
			}
		})
	}
}

// Two matching businesses is a problem of the assembly, not of the request: 500, not 422.
func TestSeveralBusinessesIsAServerError(t *testing.T) {
	c, err := easyext.New[shop.Param]().
		Point[shop.Freight](shop.DefaultFreight{}).
		Business("biz.fresh", shop.Fresh{}).
		Business("biz.fresh-too", shop.Fresh{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	h := httpx.Middleware(c, paramFromRequest)(freightHandler)
	if code, body := serve(h, "fresh", "/"); code != 500 || body != "Internal Server Error" {
		t.Fatalf("got %d %q", code, body)
	}
}

func TestOnError(t *testing.T) {
	var got error
	h := httpx.Middleware(newContext(t), paramFromRequest, httpx.OnError(func(w http.ResponseWriter, _ *http.Request, err error) {
		got = err
		http.Error(w, "custom", http.StatusTeapot)
	}))(freightHandler)

	if code, body := serve(h, "unknown", "/"); code != http.StatusTeapot || body != "custom" || !errors.Is(got, easyext.ErrNoBusinessMatched) {
		t.Fatalf("got %d %q, err %v", code, body, got)
	}
	serve(h, "", "/")
	pe, ok := errors.AsType[*httpx.ParamError](got)
	if !ok || pe.Error() != "httpx: deriving the matcher param: missing X-Biz header" || errors.Unwrap(pe).Error() != "missing X-Biz header" {
		t.Fatalf("param error = %v", got)
	}
}

func TestUnwrappedRoutesAreNotBound(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/checkout", httpx.Middleware(newContext(t), paramFromRequest)(freightHandler))
	mux.Handle("/health", freightHandler) // not wrapped: no business identity required, nothing bound
	if code, body := serve(mux, "", "/health"); code != 500 || !strings.Contains(body, "NO_BINDING") {
		t.Fatalf("got %d %q", code, body)
	}
	if code, body := serve(mux, "fresh", "/checkout"); code != 200 || body != "21" {
		t.Fatalf("got %d %q", code, body)
	}
}
