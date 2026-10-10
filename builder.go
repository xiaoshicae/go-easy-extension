package easyext

import (
	"log/slog"
	"reflect"
)

// Builder collects components and compiles a Registry in Build.
// Registration order does not change matching or precedence. A Builder is not safe for concurrent use.
type Builder[P any] struct {
	points     []reflect.Type
	defaults   []defaultEntry
	abilities  []Ability[P]
	businesses []Business[P]
	logger     *slog.Logger
}

type defaultEntry struct {
	impl  any
	point reflect.Type // nil derives all implemented, registered points
}

// New starts an assembly. Each request must match exactly one business.
func New[P any]() *Builder[P] { return &Builder[P]{} }

// Point registers the non-empty interface E as an extension point.
// Defaults and components are registered separately; their methods determine the implementation relations.
func (b *Builder[P]) Point[E any]() *Builder[P] {
	b.points = append(b.points, reflect.TypeFor[E]())
	return b
}

// Default registers an implementation as the default of every registered point it implements.
// Each point must have exactly one default; overlapping defaults are errors, not overrides.
func (b *Builder[P]) Default(impl any) *Builder[P] {
	b.defaults = append(b.defaults, defaultEntry{impl: impl})
	return b
}

// DefaultFor registers impl as the default of E only, leaving its other interfaces unaffected.
// E must also be registered with Point. The argument's type is checked by the Go compiler.
func (b *Builder[P]) DefaultFor[E any](impl E) *Builder[P] {
	b.defaults = append(b.defaults, defaultEntry{impl: any(impl), point: reflect.TypeFor[E]()})
	return b
}

// Ability registers a component with its own Code, Match and optional Requires/Excludes methods.
func (b *Builder[P]) Ability(impl Ability[P]) *Builder[P] {
	b.abilities = append(b.abilities, impl)
	return b
}

// Business registers a component with its own Code, Match and Abilities methods.
func (b *Builder[P]) Business(impl Business[P]) *Builder[P] {
	b.businesses = append(b.businesses, impl)
	return b
}

// Logger enables resolution details at debug level. Nothing is logged by default.
func (b *Builder[P]) Logger(logger *slog.Logger) *Builder[P] {
	b.logger = logger
	return b
}

// Build validates all registrations and snapshots their metadata into a Registry.
// Errors are aggregated in RegistrationError. Implementations themselves are shared, not cloned;
// they must be safe for concurrent use.
func (b *Builder[P]) Build() (*Registry[P], error) {
	v := validator[P]{b: b}
	r := v.compile()
	if len(v.problems) != 0 {
		return nil, &RegistrationError{Problems: v.problems}
	}
	return r, nil
}
