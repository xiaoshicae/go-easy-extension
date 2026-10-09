package easyext

import (
	"fmt"
	"reflect"
	"slices"
)

// Catalog is a read-only description of an assembly, e.g. for an admin page or a startup log.
type Catalog struct {
	ParamType  string
	Strict     bool
	Points     []PointInfo    // registration order
	Abilities  []AbilityInfo  // registration order
	Businesses []BusinessInfo // registration order
}

// PointInfo describes a registered extension point.
type PointInfo struct {
	Type    string // e.g. "shop.FreightCalc"
	Default string // type of the default implementation
}

// AbilityInfo describes a registered ability.
type AbilityInfo struct {
	Code     string
	Type     string
	Points   []string
	Requires []string
	Excludes []string
}

// BusinessInfo describes a registered business.
type BusinessInfo struct {
	Code   string
	Type   string
	Points []string
	Order  []string // resolution order: ability codes and Self
}

// Catalog returns a description of everything registered. The result is a copy.
func (c *Context[T]) Catalog() Catalog {
	cat := Catalog{ParamType: c.paramType, Strict: c.strict}
	for _, p := range c.points {
		cat.Points = append(cat.Points, PointInfo{Type: p.typ.String(), Default: fmt.Sprintf("%T", p.impl)})
	}
	for _, a := range c.abilities {
		cat.Abilities = append(cat.Abilities, AbilityInfo{Code: a.code, Type: fmt.Sprintf("%T", a.impl), Points: typeNames(a.points),
			Requires: slices.Clone(a.requires), Excludes: slices.Clone(a.excludes)})
	}
	for _, b := range c.businesses {
		order := make([]string, len(b.steps))
		for i, st := range b.steps {
			if st.ability < 0 {
				order[i] = Self
			} else {
				order[i] = c.abilities[st.ability].code
			}
		}
		cat.Businesses = append(cat.Businesses, BusinessInfo{Code: b.code, Type: fmt.Sprintf("%T", b.impl),
			Points: typeNames(b.points), Order: order})
	}
	return cat
}

func typeNames(ts []reflect.Type) []string {
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.String()
	}
	return names
}
