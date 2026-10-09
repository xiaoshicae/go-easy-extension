package easyext

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"reflect"
	"time"
)

// Context is a validated, immutable assembly built by [Builder.Build]. It is safe for concurrent use;
// share one per application.
type Context[T any] struct {
	points     []pointEntry
	defaults   map[reflect.Type]defaultEntry
	abilities  map[string]compiledAbility[T]
	businesses []compiledBusiness[T] // registration order
	byCode     map[string]int
	strict     bool
	resolver   func(T) (string, bool)
	selector   func(T, []string) (string, bool)
	logger     *slog.Logger
	paramType  string
}

type defaultEntry struct {
	name string
	impl any
}

type compiledAbility[T any] struct {
	code   string
	impl   Matcher[T]
	points []reflect.Type
	opts   abilityOptions
}

type compiledBusiness[T any] struct {
	code    string
	impl    any
	matcher Matcher[T] // nil when a business resolver routes requests
	points  []reflect.Type
	links   []link
}

// Kind tells what a link of a resolution chain is.
type Kind int

const (
	KindBusiness Kind = iota + 1
	KindAbility
	KindDefault
)

func (k Kind) String() string {
	switch k {
	case KindBusiness:
		return "business"
	case KindAbility:
		return "ability"
	case KindDefault:
		return "default"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

type link struct {
	code string
	kind Kind
	impl any // set in a Resolution, nil in a compiled business
}

// Resolve resolves param without binding it anywhere: it selects the business and evaluates every ability
// the business mounts, now. The result is an immutable snapshot; do not reuse it for another request.
func (c *Context[T]) Resolve(param T) (*Resolution, error) {
	start := time.Now()
	b, err := c.selectBusiness(param)
	if err != nil {
		c.logger.Debug("easyext: resolution failed", slog.String("param", c.paramType), slog.Any("error", err))
		return nil, err
	}
	r := &Resolution{defaults: c.defaults}
	if b != nil {
		r.business = b.code
		r.chain = make([]link, 0, len(b.links))
		for _, l := range b.links {
			if l.kind == KindBusiness {
				r.chain = append(r.chain, link{code: b.code, kind: KindBusiness, impl: b.impl})
				continue
			}
			a := c.abilities[l.code]
			if a.impl.Match(param) {
				r.chain = append(r.chain, link{code: a.code, kind: KindAbility, impl: a.impl})
			} else {
				r.skipped = append(r.skipped, a.code)
			}
		}
	}
	r.elapsed = time.Since(start)
	c.logger.Debug("easyext: resolved", slog.String("business", r.business),
		slog.Any("chain", r.chainCodes()), slog.Any("skipped", r.skipped), slog.Duration("elapsed", r.elapsed))
	return r, nil
}

func (c *Context[T]) selectBusiness(param T) (*compiledBusiness[T], error) {
	if c.resolver != nil {
		code, ok := c.resolver(param)
		if !ok {
			return c.noBusiness()
		}
		i, known := c.byCode[code]
		if !known {
			if c.strict {
				return nil, &ResolutionError{Reason: BusinessNotFound,
					Detail: fmt.Sprintf("business %q returned by the business resolver is not registered", code)}
			}
			return nil, nil
		}
		return &c.businesses[i], nil
	}

	var matched []*compiledBusiness[T]
	for i := range c.businesses {
		if c.businesses[i].matcher.Match(param) {
			matched = append(matched, &c.businesses[i])
		}
	}
	switch {
	case len(matched) == 0:
		return c.noBusiness()
	case len(matched) == 1:
		return matched[0], nil
	}
	codes := make([]string, len(matched))
	for i, b := range matched {
		codes[i] = b.code
	}
	if c.strict {
		return nil, &ResolutionError{Reason: MultipleBusinessesMatched, Detail: fmt.Sprintf("matched businesses %q", codes)}
	}
	if c.selector == nil {
		return matched[0], nil // registration order: deterministic
	}
	code, ok := c.selector(param, codes)
	if !ok {
		return nil, nil
	}
	for _, b := range matched {
		if b.code == code {
			return b, nil
		}
	}
	return nil, &ResolutionError{Reason: BusinessNotFound,
		Detail: fmt.Sprintf("business %q chosen by the business selector is not among the matched businesses %q", code, codes)}
}

func (c *Context[T]) noBusiness() (*compiledBusiness[T], error) {
	if c.strict {
		return nil, &ResolutionError{Reason: NoBusinessMatched, Detail: "no business matched"}
	}
	return nil, nil
}

// Bind resolves param and returns a child of ctx carrying the [Resolution]; code below reads it with
// [From], [First] or [All]. Bindings follow context.Context: they nest (a child context may be bound to
// another business) and travel with the context to other goroutines.
func (c *Context[T]) Bind(ctx context.Context, param T) (context.Context, error) {
	r, err := c.Resolve(param)
	if err != nil {
		return ctx, err
	}
	return WithResolution(ctx, r), nil
}

type bindingKey struct{}

// WithResolution returns a child of ctx carrying r.
func WithResolution(ctx context.Context, r *Resolution) context.Context {
	return context.WithValue(ctx, bindingKey{}, r)
}

// From returns the Resolution bound to ctx, or an error with reason [NoBinding].
func From(ctx context.Context) (*Resolution, error) {
	if r, ok := ctx.Value(bindingKey{}).(*Resolution); ok && r != nil {
		return r, nil
	}
	return nil, &ResolutionError{Reason: NoBinding,
		Detail: "no resolution is bound to the context: bind one where the request starts (Context.Bind, httpx.Middleware) and pass that context down"}
}

// First returns the implementation of extension point E for the resolution bound to ctx (see [Resolution.First]).
func First[E any](ctx context.Context) (E, error) {
	r, err := From(ctx)
	if err != nil {
		var zero E
		return zero, err
	}
	return r.Lookup[E]()
}

// All returns every implementation of extension point E for the resolution bound to ctx (see [Resolution.All]).
func All[E any](ctx context.Context) (iter.Seq[E], error) {
	r, err := From(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := r.defaultOf[E](); err != nil {
		return nil, err
	}
	return r.All[E](), nil
}
