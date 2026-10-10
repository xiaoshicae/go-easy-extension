package easyext

import (
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
)

type validator[P any] struct {
	b        *Builder[P]
	problems []string
	codes    map[string]string
	pointers map[any]*provider // only pointer values, which are comparable
	nextID   int
}

func (v *validator[P]) addf(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

func (v *validator[P]) compile() *Registry[P] {
	v.codes = make(map[string]string)
	v.pointers = make(map[any]*provider)
	r := &Registry[P]{
		defaults:  make(map[reflect.Type]*provider),
		scope:     &bindingScope{},
		logger:    v.b.logger,
		paramType: reflect.TypeFor[P]().String(),
	}
	if r.logger == nil {
		r.logger = slog.New(slog.DiscardHandler)
	}
	for _, point := range v.b.points {
		switch _, exists := r.defaults[point]; {
		case point.Kind() != reflect.Interface:
			v.addf("extension point %v is not an interface type", point)
		case point.NumMethod() == 0:
			v.addf("extension point %v is an empty interface", point)
		case exists:
			v.addf("extension point %v is registered more than once", point)
		default:
			r.points = append(r.points, point)
			r.defaults[point] = nil
		}
	}
	v.defaults(r)
	index := v.abilities(r)
	v.constraints(r, index)
	v.businesses(r, index)
	return r
}

func isNil(impl any) bool {
	if impl == nil {
		return true
	}
	value := reflect.ValueOf(impl)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Reused pointer values share identity, including across default/ability/business roles.
// Other values are independent registrations even if they happen to compare equal.
func (v *validator[P]) provider(impl any) *provider {
	pointer := reflect.TypeOf(impl).Kind() == reflect.Pointer
	if pointer {
		if known, ok := v.pointers[impl]; ok {
			return known
		}
	}
	v.nextID++
	result := &provider{id: v.nextID, impl: impl}
	if pointer {
		v.pointers[impl] = result
	}
	return result
}

func (v *validator[P]) component(impl any, code string) *provider {
	result := v.provider(impl)
	if result.code != "" && result.code != code {
		v.addf("component %T is registered with inconsistent codes %q and %q", impl, result.code, code)
		return nil
	}
	result.code = code
	return result
}

func (v *validator[P]) defaults(r *Registry[P]) {
	for i, entry := range v.b.defaults {
		if isNil(entry.impl) {
			v.addf("default implementation at index %d is nil", i)
			continue
		}
		label := fmt.Sprintf("default implementation %T", entry.impl)
		var points []reflect.Type
		if entry.point != nil {
			if _, registered := r.defaults[entry.point]; !registered {
				v.addf("%s targets unregistered extension point %v", label, entry.point)
				continue
			}
			points = []reflect.Type{entry.point}
		} else {
			before := len(v.problems)
			points = v.implemented(label, entry.impl, r.points)
			if len(points) == 0 && before == len(v.problems) {
				v.addf("%s implements no registered extension point", label)
			}
		}
		component := v.provider(entry.impl)
		for _, point := range points {
			if previous := r.defaults[point]; previous != nil {
				if previous == component {
					v.addf("%s is registered more than once as the default of extension point %v", label, point)
				} else {
					v.addf("extension point %v has more than one default implementation: %T and %T",
						point, previous.impl, entry.impl)
				}
				continue
			}
			r.defaults[point] = component
		}
	}
	for _, point := range r.points {
		if r.defaults[point] == nil {
			v.addf("extension point %v has no default implementation", point)
		}
	}
}

func (v *validator[P]) claim(kind, code string) bool {
	switch {
	case strings.TrimSpace(code) == "":
		v.addf("%s with an empty code", kind)
	case strings.TrimSpace(code) != code:
		v.addf("%s code %q has leading or trailing whitespace", kind, code)
	case code == Self:
		v.addf("%s code %q is reserved", kind, Self)
	case v.codes[code] != "":
		article := "an"
		if v.codes[code] == "business" {
			article = "a"
		}
		v.addf("%s code %q is already used by %s %s", kind, code, article, v.codes[code])
	default:
		v.codes[code] = kind
		return true
	}
	return false
}

func (v *validator[P]) implemented(label string, impl any, points []reflect.Type) []reflect.Type {
	t := reflect.TypeOf(impl)
	var result []reflect.Type
	for _, point := range points {
		switch {
		case t.Implements(point):
			result = append(result, point)
		case t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(point):
			v.addf("%s: %v implements %v but %v does not (pointer receiver): register a pointer",
				label, reflect.PointerTo(t), point, t)
		}
	}
	return result
}

func (v *validator[P]) abilities(r *Registry[P]) map[string]int {
	index := make(map[string]int, len(v.b.abilities))
	for i, impl := range v.b.abilities {
		if isNil(impl) {
			v.addf("ability at index %d: implementation is nil", i)
			continue
		}
		code := impl.Code()
		if !v.claim("ability", code) {
			continue
		}
		component := v.component(impl, code)
		if component == nil {
			continue
		}
		before := len(v.problems)
		points := v.implemented(fmt.Sprintf("ability %q", code), impl, r.points)
		if len(points) == 0 && before == len(v.problems) {
			v.addf("ability %q (%T) implements no registered extension point", code, impl)
		}
		entry := compiledAbility[P]{code: code, matcher: impl, provider: component, points: points}
		if requires, ok := impl.(RequiresAbilities); ok {
			entry.requires = slices.Clone(requires.Requires())
		}
		if excludes, ok := impl.(ExcludesAbilities); ok {
			entry.excludes = slices.Clone(excludes.Excludes())
		}
		index[code] = len(r.abilities)
		r.abilities = append(r.abilities, entry)
	}
	return index
}

func (v *validator[P]) references(code, relation string, refs []string, index map[string]int) {
	seen := make(map[string]bool)
	for _, ref := range refs {
		switch {
		case seen[ref]:
			v.addf("ability %q: %s repeats ability %q", code, relation, ref)
		case ref == code:
			verb := "require"
			if relation == "excludes" {
				verb = "exclude"
			}
			v.addf("ability %q: cannot %s itself", code, verb)
		default:
			if _, known := index[ref]; !known {
				v.addf("ability %q: %s unknown ability %q", code, relation, ref)
			}
		}
		seen[ref] = true
	}
}

// Requires denotes co-presence, not execution order: dependency cycles are allowed.
// Every ability's requirement closure must be satisfiable, even if no business uses it.
func (v *validator[P]) constraints(r *Registry[P], index map[string]int) {
	hasExclusions := false
	for _, ability := range r.abilities {
		v.references(ability.code, "requires", ability.requires, index)
		v.references(ability.code, "excludes", ability.excludes, index)
		hasExclusions = hasExclusions || len(ability.excludes) != 0
		for _, excluded := range ability.excludes {
			if excluded != ability.code && slices.Contains(ability.requires, excluded) {
				v.addf("ability %q: both requires and excludes ability %q", ability.code, excluded)
			}
		}
	}
	// Co-presence requirements alone are always satisfiable, including cycles.
	// References still need validation above, and every business's explicit
	// co-mount set is checked in steps regardless of this fast path.
	if !hasExclusions {
		return
	}
	var visited []int
	var included []int
	for root, ability := range r.abilities {
		if len(ability.requires) == 0 {
			continue
		}
		if visited == nil {
			visited = make([]int, len(r.abilities))
		}
		// Root stamps and a reusable work queue avoid per-root maps and recursive
		// stack growth. The same queue becomes the ordered conflict-report list.
		stamp := root + 1
		visited[root] = stamp
		included = append(included[:0], root)
		for position := 0; position < len(included); position++ {
			for _, required := range r.abilities[included[position]].requires {
				if next, known := index[required]; known && visited[next] != stamp {
					visited[next] = stamp
					included = append(included, next)
				}
			}
		}
		slices.Sort(included) // report conflicts in registration order, not traversal order
		for _, i := range included {
			source := r.abilities[i]
			for _, excluded := range source.excludes {
				target, known := index[excluded]
				if !known || visited[target] != stamp || target == i {
					continue
				}
				if i == root && slices.Contains(source.requires, excluded) {
					continue // already reported the direct contradiction
				}
				v.addf("ability %q: requirements cannot be satisfied: ability %q excludes ability %q",
					ability.code, source.code, excluded)
			}
		}
	}
}

func (v *validator[P]) businesses(r *Registry[P], index map[string]int) {
	for i, impl := range v.b.businesses {
		if isNil(impl) {
			v.addf("business at index %d: implementation is nil", i)
			continue
		}
		code := impl.Code()
		if !v.claim("business", code) {
			continue
		}
		component := v.component(impl, code)
		if component == nil {
			continue
		}
		entry := compiledBusiness[P]{
			matcher: impl,
			points:  v.implemented(fmt.Sprintf("business %q", code), impl, r.points),
			plan:    businessPlan{code: code, byPoint: make(map[reflect.Type][]pointCandidate)},
		}
		order := slices.Clone(impl.Abilities())
		entry.plan.steps = v.steps(code, component, order, index, r.abilities)
		for position, st := range entry.plan.steps {
			points := entry.points
			if st.ability >= 0 {
				points = r.abilities[st.ability].points
			}
			for _, point := range points {
				entry.plan.byPoint[point] = append(entry.plan.byPoint[point],
					pointCandidate{position: position, provider: st.provider})
			}
		}
		r.businesses = append(r.businesses, entry)
	}
}

func (v *validator[P]) steps(code string, component *provider, order []string, index map[string]int, abilities []compiledAbility[P]) []step {
	if !slices.Contains(order, Self) {
		order = append([]string{Self}, order...)
	}
	var result []step
	seen := make(map[string]bool)
	for _, ref := range order {
		if seen[ref] {
			if ref == Self {
				v.addf("business %q: Self is listed more than once", code)
			} else {
				v.addf("business %q: ability %q is listed more than once", code, ref)
			}
			continue
		}
		seen[ref] = true
		if ref == Self {
			result = append(result, step{Link: Link{Code: code, Kind: KindBusiness}, ability: -1, provider: component})
			continue
		}
		i, known := index[ref]
		if !known {
			v.addf("business %q: uses unknown ability %q", code, ref)
			continue
		}
		result = append(result, step{Link: Link{Code: ref, Kind: KindAbility}, ability: i, provider: abilities[i].provider})
	}
	for _, st := range result {
		if st.ability < 0 {
			continue
		}
		ability := abilities[st.ability]
		for _, required := range ability.requires {
			if _, known := index[required]; known && !seen[required] {
				v.addf("business %q: ability %q requires ability %q to be listed as well", code, ability.code, required)
			}
		}
		for _, excluded := range ability.excludes {
			if excluded != ability.code && seen[excluded] {
				v.addf("business %q: ability %q excludes ability %q", code, ability.code, excluded)
			}
		}
	}
	return result
}
