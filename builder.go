package easyext

import (
	"fmt"
	"log/slog"
	"reflect"
	"slices"
)

// Matcher decides, per request, whether a business or an ability applies to the param.
type Matcher[T any] interface {
	Match(param T) bool
}

// MatcherFunc adapts a function to a [Matcher], e.g. for a business that only identifies requests and
// implements no extension point itself.
type MatcherFunc[T any] func(param T) bool

// Match calls f(param).
func (f MatcherFunc[T]) Match(param T) bool { return f(param) }

// Self marks the position of the business's own implementation in [Abilities].
// If Self is not listed, the business comes before all of its abilities.
const Self = "<self>"

// AbilityOption configures an ability registered with [Builder.Ability].
type AbilityOption func(*abilityOptions)

type abilityOptions struct {
	requires, excludes []string
}

// Requires lists abilities that every business mounting this ability must mount as well (order is not checked).
func Requires(codes ...string) AbilityOption {
	return func(o *abilityOptions) { o.requires = append(o.requires, codes...) }
}

// Excludes lists abilities that must not be mounted on the same business as this ability.
func Excludes(codes ...string) AbilityOption {
	return func(o *abilityOptions) { o.excludes = append(o.excludes, codes...) }
}

// BusinessOption configures a business registered with [Builder.Business].
type BusinessOption func(*businessOptions)

type businessOptions struct {
	mounts []string
}

// Abilities mounts abilities on a business. The order is the precedence, earlier entries win; [Self] marks
// where the business's own implementation ranks:
//
//	Abilities("ability.free-shipping")        // business, then free shipping
//	Abilities("ability.free-shipping", Self)  // free shipping overrides the business
func Abilities(codes ...string) BusinessOption {
	return func(o *businessOptions) { o.mounts = append(o.mounts, codes...) }
}

// Builder collects extension points, abilities and businesses, then validates everything at once in [Builder.Build].
// T is the request param that businesses and abilities match on. A Builder is not safe for concurrent use.
type Builder[T any] struct {
	points     []pointEntry
	abilities  []abilityEntry[T]
	businesses []businessEntry
	resolver   func(T) (string, bool)
	selector   func(T, []string) (string, bool)
	strict     bool
	logger     *slog.Logger
	problems   []string
}

type pointEntry struct {
	typ  reflect.Type
	impl any
}

type abilityEntry[T any] struct {
	code string
	impl Matcher[T]
	opts abilityOptions
}

type businessEntry struct {
	code string
	impl any
	opts businessOptions
}

// New starts an assembly. Strict mode is on: every request must match exactly one business.
func New[T any]() *Builder[T] {
	return &Builder[T]{strict: true}
}

// Point registers extension point E, which must be an interface type, together with its default
// implementation: the one that answers when neither the business nor any applicable ability implements E.
func (b *Builder[T]) Point[E any](defaultImpl E) *Builder[T] {
	t := reflect.TypeFor[E]()
	switch {
	case t.Kind() != reflect.Interface:
		b.problems = append(b.problems, fmt.Sprintf("extension point %v is not an interface type", t))
	case any(defaultImpl) == nil:
		b.problems = append(b.problems, fmt.Sprintf("extension point %v: default implementation is nil", t))
	case slices.ContainsFunc(b.points, func(p pointEntry) bool { return p.typ == t }):
		b.problems = append(b.problems, fmt.Sprintf("extension point %v is registered more than once", t))
	default:
		b.points = append(b.points, pointEntry{typ: t, impl: defaultImpl})
	}
	return b
}

// Ability registers a reusable implementation. impl implements one or more registered extension points;
// they are derived from its type.
func (b *Builder[T]) Ability(code string, impl Matcher[T], opts ...AbilityOption) *Builder[T] {
	e := abilityEntry[T]{code: code, impl: impl}
	for _, o := range opts {
		o(&e.opts)
	}
	b.abilities = append(b.abilities, e)
	return b
}

// Business registers an integration party. impl must implement [Matcher] of T unless a business resolver is
// configured, and may implement extension points itself (or none: then only its abilities and the defaults answer).
func (b *Builder[T]) Business(code string, impl any, opts ...BusinessOption) *Builder[T] {
	e := businessEntry{code: code, impl: impl}
	for _, o := range opts {
		o(&e.opts)
	}
	b.businesses = append(b.businesses, e)
	return b
}

// Strict sets strict mode (the default). Strict: no matching business, or several, is an error.
// Not strict: no business means only default implementations answer; several are settled by the business
// selector, or else by registration order.
func (b *Builder[T]) Strict(strict bool) *Builder[T] {
	b.strict = strict
	return b
}

// BusinessResolver routes requests to a business by code instead of asking every business to match.
// Businesses then need not implement [Matcher]; their Match is ignored. ok == false means no business.
func (b *Builder[T]) BusinessResolver(resolve func(param T) (code string, ok bool)) *Builder[T] {
	b.resolver = resolve
	return b
}

// BusinessSelector chooses among several matching businesses (codes in registration order) when not strict.
// ok == false means no business: only default implementations answer.
func (b *Builder[T]) BusinessSelector(selectFn func(param T, matched []string) (code string, ok bool)) *Builder[T] {
	b.selector = selectFn
	return b
}

// Logger sets the logger for resolution details (at debug level). Nothing is logged by default.
func (b *Builder[T]) Logger(logger *slog.Logger) *Builder[T] {
	b.logger = logger
	return b
}

// Build validates the whole assembly and returns an immutable [Context], safe for concurrent use.
// All problems are reported together in a [*RegistrationError].
func (b *Builder[T]) Build() (*Context[T], error) {
	v := validator[T]{b: b, problems: slices.Clone(b.problems)}
	c := v.compile()
	if len(v.problems) > 0 {
		return nil, &RegistrationError{Problems: v.problems}
	}
	return c, nil
}
