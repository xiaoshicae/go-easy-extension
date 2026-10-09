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
}

func (v *validator[T]) addf(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

// compile validates the builder and turns it into a Context. The result is only valid when no problem was added.
func (v *validator[T]) compile() *Context[T] {
	b := v.b
	c := &Context[T]{
		defaults:  make(map[reflect.Type]defaultEntry, len(b.points)),
		abilities: make(map[string]compiledAbility[T], len(b.abilities)),
		byCode:    make(map[string]int, len(b.businesses)),
		strict:    b.strict,
		resolver:  b.resolver,
		selector:  b.selector,
		logger:    b.logger,
		paramType: reflect.TypeFor[T]().String(),
	}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}
	for _, p := range b.points {
		c.defaults[p.typ] = defaultEntry{name: p.typ.String(), impl: p.impl}
	}

	seen := map[string]string{} // code -> "ability" / "business"
	claim := func(kind, code string) bool {
		switch {
		case code == "":
			v.addf("%s with an empty code", kind)
			return false
		case code == Self:
			v.addf("%s code %q is reserved", kind, Self)
			return false
		case seen[code] != "":
			v.addf("%s code %q is already used by a %s", kind, code, seen[code])
			return false
		}
		seen[code] = kind
		return true
	}

	for _, a := range b.abilities {
		if !claim("ability", a.code) {
			continue
		}
		if a.impl == nil {
			v.addf("ability %q: implementation is nil", a.code)
			continue
		}
		points := implemented(a.impl, b.points)
		if len(points) == 0 {
			v.addf("ability %q (%T) implements no registered extension point", a.code, a.impl)
		}
		c.abilities[a.code] = compiledAbility[T]{code: a.code, impl: a.impl, points: points, opts: a.opts}
	}
	for _, a := range b.abilities { // registration order keeps the problem list deterministic
		if _, ok := c.abilities[a.code]; !ok {
			continue
		}
		for _, ref := range slices.Concat(a.opts.requires, a.opts.excludes) {
			if _, ok := c.abilities[ref]; !ok {
				v.addf("ability %q: requires/excludes unknown ability %q", a.code, ref)
			}
		}
	}

	for _, e := range b.businesses {
		if !claim("business", e.code) {
			continue
		}
		if e.impl == nil {
			v.addf("business %q: implementation is nil", e.code)
			continue
		}
		cb := compiledBusiness[T]{code: e.code, impl: e.impl, points: implemented(e.impl, b.points)}
		if m, ok := e.impl.(Matcher[T]); ok {
			cb.matcher = m
		} else if b.resolver == nil {
			v.addf("business %q (%T) does not implement Matcher[%s] and no business resolver is configured",
				e.code, e.impl, c.paramType)
		}
		cb.links = v.links(e, c.abilities)
		c.byCode[e.code] = len(c.businesses)
		c.businesses = append(c.businesses, cb)
	}
	c.points = b.points
	return c
}

// links turns the Abilities list of a business into its resolution order, checking references,
// duplicates, Self and requires/excludes.
func (v *validator[T]) links(e businessEntry, abilities map[string]compiledAbility[T]) []link {
	mounts := e.opts.mounts
	if !slices.Contains(mounts, Self) {
		mounts = append([]string{Self}, mounts...)
	}
	links := make([]link, 0, len(mounts))
	mounted := map[string]bool{}
	selfSeen := false
	for _, code := range mounts {
		if code == Self {
			if selfSeen {
				v.addf("business %q: Self is listed more than once", e.code)
				continue
			}
			selfSeen = true
			links = append(links, link{code: e.code, kind: KindBusiness})
			continue
		}
		if _, ok := abilities[code]; !ok {
			v.addf("business %q: mounts unknown ability %q", e.code, code)
			continue
		}
		if mounted[code] {
			v.addf("business %q: ability %q is mounted more than once", e.code, code)
			continue
		}
		mounted[code] = true
		links = append(links, link{code: code, kind: KindAbility})
	}
	for _, l := range links {
		if l.kind != KindAbility {
			continue
		}
		code := l.code
		a := abilities[code]
		for _, r := range a.opts.requires {
			if _, known := abilities[r]; known && !mounted[r] {
				v.addf("business %q: ability %q requires ability %q to be mounted as well", e.code, code, r)
			}
		}
		for _, x := range a.opts.excludes {
			if mounted[x] {
				v.addf("business %q: ability %q excludes ability %q", e.code, code, x)
			}
		}
	}
	return links
}

// implemented lists the registered extension points that impl's type implements, in registration order.
func implemented(impl any, points []pointEntry) []reflect.Type {
	t := reflect.TypeOf(impl)
	var out []reflect.Type
	for _, p := range points {
		if t.Implements(p.typ) {
			out = append(out, p.typ)
		}
	}
	return out
}
