package easyext

import (
	"fmt"
	"log/slog"
	"reflect"
	"slices"
)

type validator[T any] struct {
	b        *Builder[T]
	problems []string
	codes    map[string]string // claimed code -> "ability" / "business", whether or not its entry is valid
}

func (v *validator[T]) addf(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

// compile validates the builder and turns it into a Context; the result is only valid when no problem was added.
// Everything is visited in registration order, so the problem list is deterministic.
func (v *validator[T]) compile() *Context[T] {
	b := v.b
	v.codes = map[string]string{}
	c := &Context[T]{
		defaults:  make(map[reflect.Type]any, len(b.points)),
		byCode:    make(map[string]int, len(b.businesses)),
		strict:    b.strict,
		resolver:  b.resolver,
		logger:    b.logger,
		paramType: reflect.TypeFor[T]().String(),
	}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}

	for _, p := range b.points {
		switch _, dup := c.defaults[p.typ]; {
		case p.typ.Kind() != reflect.Interface:
			v.addf("extension point %v is not an interface type", p.typ)
		case p.impl == nil:
			v.addf("extension point %v: default implementation is nil", p.typ)
		case dup:
			v.addf("extension point %v is registered more than once", p.typ)
		default:
			c.defaults[p.typ] = p.impl
			c.points = append(c.points, p)
		}
	}

	abilityIdx := map[string]int{}
	for _, a := range b.abilities {
		if !v.claim("ability", a.code) {
			continue
		}
		if a.impl == nil {
			v.addf("ability %q: implementation is nil", a.code)
			continue
		}
		before := len(v.problems)
		points := v.implemented("ability", a.code, a.impl, c.points)
		if len(points) == 0 && len(v.problems) == before {
			v.addf("ability %q (%T) implements no registered extension point", a.code, a.impl)
		}
		abilityIdx[a.code] = len(c.abilities)
		c.abilities = append(c.abilities, compiledAbility[T]{code: a.code, impl: a.impl, points: points,
			requires: a.opts.requires, excludes: a.opts.excludes})
	}
	for _, a := range c.abilities {
		for _, ref := range slices.Concat(a.requires, a.excludes) {
			if v.codes[ref] != "ability" {
				v.addf("ability %q: requires/excludes unknown ability %q", a.code, ref)
			}
		}
	}

	for _, e := range b.businesses {
		if !v.claim("business", e.code) {
			continue
		}
		if e.impl == nil {
			v.addf("business %q: implementation is nil", e.code)
			continue
		}
		c.byCode[e.code] = len(c.businesses)
		c.businesses = append(c.businesses, compiledBusiness[T]{code: e.code, impl: e.impl,
			points: v.implemented("business", e.code, e.impl, c.points),
			steps:  v.steps(e, abilityIdx, c.abilities)})
	}
	return c
}

func (v *validator[T]) claim(kind, code string) bool {
	switch {
	case code == "":
		v.addf("%s with an empty code", kind)
		return false
	case code == Self:
		v.addf("%s code %q is reserved", kind, Self)
		return false
	case v.codes[code] != "":
		v.addf("%s code %q is already used by a %s", kind, code, v.codes[code])
		return false
	}
	v.codes[code] = kind
	return true
}

// implemented lists the registered extension points that impl's type implements. A point implemented only by
// the pointer type is a problem: registered by value, those methods would be silently ignored.
func (v *validator[T]) implemented(kind, code string, impl any, points []pointEntry) []reflect.Type {
	t := reflect.TypeOf(impl)
	var out []reflect.Type
	for _, p := range points {
		switch {
		case t.Implements(p.typ):
			out = append(out, p.typ)
		case t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(p.typ):
			v.addf("%s %q: %v implements %v but %v does not (pointer receiver): register a pointer",
				kind, code, reflect.PointerTo(t), p.typ, t)
		}
	}
	return out
}

// steps turns the Abilities list of a business into its resolution order, checking references, duplicates,
// Self and requires/excludes.
func (v *validator[T]) steps(e businessEntry[T], abilityIdx map[string]int, abilities []compiledAbility[T]) []step {
	mounts := e.opts.mounts
	if !slices.Contains(mounts, Self) {
		mounts = append([]string{Self}, mounts...)
	}
	steps := make([]step, 0, len(mounts))
	mounted := map[string]bool{}
	selfSeen := false
	for _, code := range mounts {
		switch {
		case code == Self && selfSeen:
			v.addf("business %q: Self is listed more than once", e.code)
		case code == Self:
			selfSeen = true
			steps = append(steps, step{ability: -1})
		case v.codes[code] != "ability":
			v.addf("business %q: mounts unknown ability %q", e.code, code)
		case mounted[code]:
			v.addf("business %q: ability %q is mounted more than once", e.code, code)
		default:
			mounted[code] = true
			if i, ok := abilityIdx[code]; ok { // not ok: the ability itself is invalid and already reported
				steps = append(steps, step{ability: i})
			}
		}
	}
	for _, s := range steps {
		if s.ability < 0 {
			continue
		}
		a := abilities[s.ability]
		for _, r := range a.requires {
			if v.codes[r] == "ability" && !mounted[r] {
				v.addf("business %q: ability %q requires ability %q to be mounted as well", e.code, a.code, r)
			}
		}
		for _, x := range a.excludes {
			if mounted[x] {
				v.addf("business %q: ability %q excludes ability %q", e.code, a.code, x)
			}
		}
	}
	return steps
}
