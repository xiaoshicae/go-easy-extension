package easyext

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"reflect"
)

// Registry is an immutable assembly built by Builder.Build, safe for concurrent use.
// Its implementations are shared instances and must separately be safe for concurrent calls.
type Registry[P any] struct {
	points     []reflect.Type
	defaults   map[reflect.Type]*provider
	abilities  []compiledAbility[P]
	businesses []compiledBusiness[P]
	scope      *bindingScope
	logger     *slog.Logger
	paramType  string
}

// A non-zero-sized, private key gives every Build its own binding identity.
type bindingScope struct{ marker byte }

type provider struct {
	id   int
	impl any
	code string // snapshotted component code, empty for a default-only provider
}

type compiledAbility[P any] struct {
	code               string
	matcher            Ability[P]
	provider           *provider
	points             []reflect.Type
	requires, excludes []string
}

type compiledBusiness[P any] struct {
	matcher Business[P]
	points  []reflect.Type
	plan    businessPlan
}

type businessPlan struct {
	code    string
	steps   []step
	byPoint map[reflect.Type][]pointCandidate
}

type step struct {
	Link
	ability  int // -1 for Self; otherwise an index into Registry.abilities
	provider *provider
}

type pointCandidate struct {
	position int
	provider *provider
}

// Kind identifies the role of an implementation in a resolution.
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

// Resolve matches every business once, then matches only the selected business's abilities once each.
// The result snapshots activation, not the mutable state of the implementations or param.
// Do not mutate param during resolution or reuse a result for another request.
func (r *Registry[P]) Resolve(param P) (*Resolution, error) {
	debug := r.logger.Enabled(context.Background(), slog.LevelDebug)
	b, err := r.selectBusiness(param)
	if err != nil {
		if debug {
			r.logger.Debug("easyext: resolution failed", slog.String("paramType", r.paramType), slog.Any("error", err))
		}
		return nil, err
	}
	result := &Resolution{plan: &b.plan, defaults: r.defaults, active: make([]bool, len(b.plan.steps))}
	for i, st := range b.plan.steps {
		result.active[i] = st.ability < 0 || r.abilities[st.ability].matcher.Match(param)
	}
	if debug {
		trace := result.Trace()
		r.logger.Debug("easyext: resolved", slog.String("business", trace.Business),
			slog.Any("chain", result.chainCodes()), slog.Any("skipped", trace.Skipped))
	}
	return result, nil
}

func (r *Registry[P]) selectBusiness(param P) (*compiledBusiness[P], error) {
	first := -1
	var matched []string // allocate only for an ambiguous request
	for i := range r.businesses {
		if !r.businesses[i].matcher.Match(param) {
			continue
		}
		if first < 0 {
			first = i
			continue
		}
		if matched == nil {
			matched = append(matched, r.businesses[first].plan.code)
		}
		matched = append(matched, r.businesses[i].plan.code)
	}
	if len(matched) != 0 {
		return nil, &ResolutionError{Reason: MultipleBusinessesMatched,
			Detail: fmt.Sprintf("matched businesses %q", matched)}
	}
	if first < 0 {
		return nil, &ResolutionError{Reason: NoBusinessMatched,
			Detail: fmt.Sprintf("none of the %d registered businesses matched", len(r.businesses))}
	}
	return &r.businesses[first], nil
}

// Bind resolves param and returns a child context carrying this Registry's result.
// Rebinding this Registry shadows only its parent binding. Other registries remain accessible.
// On failure the original context is returned unchanged.
func (r *Registry[P]) Bind(ctx context.Context, param P) (context.Context, error) {
	result, err := r.Resolve(param)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, r.scope, result), nil
}

// From retrieves this Registry's resolution, never another Registry's binding.
func (r *Registry[P]) From(ctx context.Context) (*Resolution, error) {
	if result, ok := ctx.Value(r.scope).(*Resolution); ok && result != nil {
		return result, nil
	}
	return nil, &ResolutionError{Reason: NoBinding,
		Detail: "no resolution from this registry is bound to the context: use Registry.Bind or httpx.Middleware and pass that context down"}
}

// First looks up E using this Registry's binding in ctx.
func (r *Registry[P]) First[E any](ctx context.Context) (E, error) {
	result, err := r.From(ctx)
	if err != nil {
		var zero E
		return zero, err
	}
	return result.First[E]()
}

// All looks up all implementations of E using this Registry's binding in ctx.
func (r *Registry[P]) All[E any](ctx context.Context) (iter.Seq[E], error) {
	result, err := r.From(ctx)
	if err != nil {
		return nil, err
	}
	return result.All[E]()
}
