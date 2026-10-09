package easyext_test

import (
	"context"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v2"
	"github.com/xiaoshicae/go-easy-extension/v2/internal/fixtures/shop"
)

func BenchmarkResolve(b *testing.B) {
	c := mustBuild(b, shopBuilder())
	p := shop.Param{Biz: "fresh", Abilities: []string{"free-shipping", "rapid"}}
	for b.Loop() {
		if _, err := c.Resolve(p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFirst(b *testing.B) {
	r := mustResolve(b, mustBuild(b, shopBuilder()), shop.Param{Biz: "digital"})
	for b.Loop() {
		_ = r.First[shop.Notify]() // walks the chain, then the default
	}
}

func BenchmarkFirstFromContext(b *testing.B) {
	c := mustBuild(b, shopBuilder())
	ctx, err := c.Bind(context.Background(), shop.Param{Biz: "fresh"})
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := easyext.First[shop.Freight](ctx); err != nil {
			b.Fatal(err)
		}
	}
}
