package easyext

import (
	"fmt"
	"reflect"
	"slices"
)

// Catalog describes the registered components. Every call returns independent slices.
type Catalog struct {
	ParamType  string
	Points     []PointInfo
	Abilities  []AbilityInfo
	Businesses []BusinessInfo
}

// PointInfo describes an extension point and the type of its default implementation.
type PointInfo struct {
	Type    string
	Default string
}

// AbilityInfo describes the snapshotted metadata and inferred points of an ability.
type AbilityInfo struct {
	Code     string
	Type     string
	Points   []string
	Requires []string
	Excludes []string
}

// BusinessInfo describes a business and its declared precedence, including implicit Self.
type BusinessInfo struct {
	Code      string
	Type      string
	Points    []string
	Abilities []string
}

// Catalog returns a copy of the metadata, not the component instances.
// Type labels are for display; the registry identifies points by reflect.Type, never these strings.
func (r *Registry[P]) Catalog() Catalog {
	catalog := Catalog{ParamType: r.paramType}
	for _, point := range r.points {
		catalog.Points = append(catalog.Points, PointInfo{Type: point.String(), Default: fmt.Sprintf("%T", r.defaults[point].impl)})
	}
	for _, ability := range r.abilities {
		catalog.Abilities = append(catalog.Abilities, AbilityInfo{
			Code: ability.code, Type: fmt.Sprintf("%T", ability.matcher), Points: typeNames(ability.points),
			Requires: slices.Clone(ability.requires), Excludes: slices.Clone(ability.excludes),
		})
	}
	for _, business := range r.businesses {
		order := make([]string, len(business.plan.steps))
		for i, st := range business.plan.steps {
			order[i] = st.Code
			if st.ability < 0 {
				order[i] = Self
			}
		}
		catalog.Businesses = append(catalog.Businesses, BusinessInfo{
			Code: business.plan.code, Type: fmt.Sprintf("%T", business.matcher),
			Points: typeNames(business.points), Abilities: order,
		})
	}
	return catalog
}

func typeNames(types []reflect.Type) []string {
	result := make([]string, len(types))
	for i, typ := range types {
		result[i] = typ.String()
	}
	return result
}
