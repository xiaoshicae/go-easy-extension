package easyext

import (
	"fmt"
	"iter"
	"reflect"
	"strings"
)

// Resolution is an immutable snapshot of one request's matched business and active abilities.
// It is safe to share, but does not freeze or synchronize the state of the implementations.
type Resolution struct {
	plan     *businessPlan
	active   []bool
	defaults map[reflect.Type]*provider
}

// Business returns the selected business's code.
func (r *Resolution) Business() string { return r.plan.code }

// First returns the first active implementation of E, otherwise E's default.
// An unregistered E is returned as ErrExtensionNotFound, never a lookup panic.
func (r *Resolution) First[E any]() (E, error) {
	d, candidates, err := r.candidates[E]()
	if err != nil {
		var zero E
		return zero, err
	}
	for _, candidate := range candidates {
		if r.active[candidate.position] {
			return candidate.provider.impl.(E), nil
		}
	}
	return d.impl.(E), nil
}

// All yields active implementations of E in precedence order, then its default.
// A shared provider is yielded once at its first active position. Only pointers share identity:
// a non-pointer value registered in two roles (e.g. as a default and as an ability) is yielded twice.
// Iterating the sequence again
// uses the same snapshot and never calls Match again; breaking early is supported.
func (r *Resolution) All[E any]() (iter.Seq[E], error) {
	d, candidates, err := r.candidates[E]()
	if err != nil {
		return nil, err
	}
	return func(yield func(E) bool) {
		defaultSeen := false
		for _, candidate := range candidates {
			if !r.active[candidate.position] {
				continue
			}
			// Unique component codes and references make the active candidates distinct.
			// Only the default may reuse an active component's provider identity.
			defaultSeen = defaultSeen || candidate.provider.id == d.id
			if !yield(candidate.provider.impl.(E)) {
				return
			}
		}
		if !defaultSeen {
			yield(d.impl.(E))
		}
	}, nil
}

func (r *Resolution) candidates[E any]() (*provider, []pointCandidate, error) {
	point := reflect.TypeFor[E]()
	d, registered := r.defaults[point]
	if !registered {
		return nil, nil, &ResolutionError{Reason: ExtensionNotFound,
			Detail: fmt.Sprintf("extension point %v is not registered; register it with Builder.Point", point)}
	}
	return d, r.plan.byPoint[point], nil
}

// Link identifies the role of a candidate implementation.
type Link struct {
	Code string
	Kind Kind
}

// Trace describes the matched business, active chain and abilities skipped by Match.
type Trace struct {
	Business string
	Chain    []Link
	Skipped  []string
}

// Trace returns an independent copy. It does not reevaluate any component.
func (r *Resolution) Trace() Trace {
	trace := Trace{Business: r.plan.code}
	for i, st := range r.plan.steps {
		if r.active[i] {
			trace.Chain = append(trace.Chain, st.Link)
		} else {
			trace.Skipped = append(trace.Skipped, st.Code)
		}
	}
	return trace
}

// Candidate explains a declared candidate, including inactive abilities.
// Reason is selected, match-false, not-implemented, lower-priority, or duplicate.
type Candidate struct {
	Link
	Position   int
	Implements bool
	Active     bool
	Reason     string
}

// Explanation describes why First selects one candidate for a point.
// The default is the last candidate. All uses the same activation and precedence.
type Explanation struct {
	Point      string
	Candidates []Candidate
	Selected   Link
}

// Explain describes every declared step and the default without calling Match again.
func (r *Resolution) Explain[E any]() (Explanation, error) {
	d, candidates, err := r.candidates[E]()
	if err != nil {
		return Explanation{}, err
	}
	x := Explanation{Point: reflect.TypeFor[E]().String(), Candidates: make([]Candidate, 0, len(r.plan.steps)+1)}
	next := 0
	defaultSeen := false
	for i, st := range r.plan.steps {
		candidate := Candidate{Link: st.Link, Position: i, Active: r.active[i]}
		if next < len(candidates) && candidates[next].position == i {
			candidate.Implements = true
			if candidate.Active && st.provider.id == d.id {
				defaultSeen = true
			}
			next++
		}
		switch {
		case !candidate.Active:
			candidate.Reason = "match-false"
		case !candidate.Implements:
			candidate.Reason = "not-implemented"
		case x.Selected.Kind == 0:
			candidate.Reason = "selected"
			x.Selected = st.Link
		default:
			candidate.Reason = "lower-priority"
		}
		x.Candidates = append(x.Candidates, candidate)
	}
	fallback := Candidate{
		Link:     Link{Code: fmt.Sprintf("%T", d.impl), Kind: KindDefault},
		Position: len(r.plan.steps), Implements: true, Active: true, Reason: "lower-priority",
	}
	if x.Selected.Kind == 0 {
		fallback.Reason = "selected"
		x.Selected = fallback.Link
	} else if defaultSeen {
		fallback.Reason = "duplicate"
	}
	x.Candidates = append(x.Candidates, fallback)
	return x, nil
}

// String summarizes active and skipped components.
func (r *Resolution) String() string {
	trace := r.Trace()
	return fmt.Sprintf("Resolution[business=%s, chain=[%s], skipped=[%s]]",
		trace.Business, strings.Join(r.chainCodes(), " "), strings.Join(trace.Skipped, " "))
}

func (r *Resolution) chainCodes() []string {
	var codes []string
	for i, st := range r.plan.steps {
		if r.active[i] {
			codes = append(codes, st.Code)
		}
	}
	return codes
}
