package easyext

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"reflect"
)

// Context is a validated, immutable assembly built by [Builder.Build]. It is safe for concurrent use;
// share one per application.
type Context[T any] struct {
	points     []pointEntry         // registration order
	defaults   map[reflect.Type]any // extension point -> default implementation
	abilities  []compiledAbility[T] // registration order
	businesses []compiledBusiness[T]
	byCode     map[string]int // business code -> index
	strict     bool
	resolver   func(T) (string, bool)
	logger     *slog.Logger
	paramType  string
}

type compiledAbility[T any] struct {
	code               string
	impl               Matcher[T]
	points             []reflect.Type
	requires, excludes []string
}

type compiledBusiness[T any] struct {
	code   string
	impl   Matcher[T]
	points []reflect.Type
	steps  []step
}

// step is one entry of a business's resolution order: an ability (index into Context.abilities) or the
// business itself (-1).
type step struct {
	ability int
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

// Resolve resolves param without binding it anywhere: it selects the business and evaluates every ability
// the business mounts, now. The result is an immutable snapshot; do not reuse it for another request.
func (c *Context[T]) Resolve(param T) (*Resolution, error) {
	debug := c.logger.Enabled(context.Background(), slog.LevelDebug)
	b, err := c.selectBusiness(param)
	if err != nil {
		if debug {
			c.logger.Debug("easyext: resolution failed", slog.String("param", c.paramType), slog.Any("error", err))
		}
		return nil, err
	}
	r := &Resolution{defaults: c.defaults}
	if b != nil {
		r.business = b.code
		r.chain = make([]link, 0, len(b.steps))
		for _, s := range b.steps {
			if s.ability < 0 {
				r.chain = append(r.chain, link{code: b.code, kind: KindBusiness, impl: b.impl})
				continue
			}
			a := &c.abilities[s.ability]
			if a.impl.Match(param) {
				r.chain = append(r.chain, link{code: a.code, kind: KindAbility, impl: a.impl})
			} else {
				r.skipped = append(r.skipped, a.code)
			}
		}
	}
	if debug {
		c.logger.Debug("easyext: resolved", slog.String("business", r.business),
			slog.Any("chain", r.chainCodes()), slog.Any("skipped", r.skipped))
	}
	return r, nil
}

func (c *Context[T]) selectBusiness(param T) (*compiledBusiness[T], error) {
	if c.resolver != nil {
		code, ok := c.resolver(param)
		if !ok {
			return c.noBusiness()
		}
		if i, known := c.byCode[code]; known {
			return &c.businesses[i], nil
		}
		if c.strict {
			return nil, &ResolutionError{Reason: BusinessNotFound,
				Detail: fmt.Sprintf("business %q returned by the business resolver is not registered", code)}
		}
		return nil, nil
	}

	first := -1
	for i := range c.businesses {
		if !c.businesses[i].impl.Match(param) {
			continue
		}
		if first < 0 {
			first = i
			if !c.strict {
				break // not strict: the first registered match wins
			}
			continue
		}
		return nil, c.multipleMatched(param) // strict: a second match is an error
	}
	if first < 0 {
		return c.noBusiness()
	}
	return &c.businesses[first], nil
}

// multipleMatched builds the error listing every matching business; only on the error path.
func (c *Context[T]) multipleMatched(param T) error {
	var codes []string
	for i := range c.businesses {
		if c.businesses[i].impl.Match(param) {
			codes = append(codes, c.businesses[i].code)
		}
	}
	return &ResolutionError{Reason: MultipleBusinessesMatched, Detail: fmt.Sprintf("matched businesses %q", codes)}
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
// Unlike Resolution.First it reports an unregistered E as an error ([ExtensionNotFound]) instead of panicking.
func First[E any](ctx context.Context) (E, error) {
	r, err := From(ctx)
	if err != nil {
		var zero E
		return zero, err
	}
	return r.lookup[E]()
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
