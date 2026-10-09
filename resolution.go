package easyext

import (
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strings"
	"time"
)

// Resolution is the immutable outcome of resolving one request param: the matched business and the
// applicable abilities, in precedence order. It is safe for concurrent use.
type Resolution struct {
	business string // "" when no business matched (not strict)
	chain    []link
	skipped  []string
	defaults map[reflect.Type]defaultEntry
	elapsed  time.Duration
}

// Business returns the code of the matched business; ok is false when no business matched (not strict).
func (r *Resolution) Business() (code string, ok bool) {
	return r.business, r.business != ""
}

// First returns the implementation of extension point E: the first link of the chain (business and
// applicable abilities, in precedence order) implementing E, else E's default implementation.
// It panics with a [*ResolutionError] (reason [ExtensionNotFound]) if E was never registered,
// which is a programming error; use [Resolution.Lookup] to get an error instead.
func (r *Resolution) First[E any]() E {
	e, err := r.Lookup[E]()
	if err != nil {
		panic(err)
	}
	return e
}

// Lookup is like [Resolution.First] but returns an error if E was never registered.
func (r *Resolution) Lookup[E any]() (E, error) {
	d, err := r.defaultOf[E]()
	if err != nil {
		var zero E
		return zero, err
	}
	for _, l := range r.chain {
		if e, ok := l.impl.(E); ok {
			return e, nil
		}
	}
	return d.impl.(E), nil
}

// All yields every implementation of extension point E: the links of the chain implementing E in precedence
// order, then E's default implementation. It panics like [Resolution.First] if E was never registered.
func (r *Resolution) All[E any]() iter.Seq[E] {
	d, err := r.defaultOf[E]()
	if err != nil {
		panic(err)
	}
	return func(yield func(E) bool) {
		for _, l := range r.chain {
			if e, ok := l.impl.(E); ok && !yield(e) {
				return
			}
		}
		yield(d.impl.(E))
	}
}

func (r *Resolution) defaultOf[E any]() (defaultEntry, error) {
	t := reflect.TypeFor[E]()
	d, ok := r.defaults[t]
	if !ok {
		return defaultEntry{}, &ResolutionError{Reason: ExtensionNotFound,
			Detail: fmt.Sprintf("extension point %v is not registered; register it with Builder.Point", t)}
	}
	return d, nil
}

// Link is one entry of a resolution chain.
type Link struct {
	Code string
	Kind Kind
}

// Trace describes how a request was resolved.
type Trace struct {
	Business string   // "" when no business matched
	Chain    []Link   // business and applicable abilities, in precedence order
	Skipped  []string // abilities mounted by the business whose Match returned false
	Elapsed  time.Duration
}

// Trace returns how this resolution was made.
func (r *Resolution) Trace() Trace {
	chain := make([]Link, len(r.chain))
	for i, l := range r.chain {
		chain[i] = Link{Code: l.code, Kind: l.kind}
	}
	return Trace{Business: r.business, Chain: chain, Skipped: slices.Clone(r.skipped), Elapsed: r.elapsed}
}

// Candidate is a link of the chain considered for an extension point.
type Candidate struct {
	Link
	Implements bool
}

// Explanation tells which implementation answers an extension point, and why.
type Explanation struct {
	Point      string
	Candidates []Candidate // the chain, then the default implementation
	Selected   Link
}

// Explain tells which link answers extension point E. It panics like [Resolution.First] if E was never registered.
func (r *Resolution) Explain[E any]() Explanation {
	d, err := r.defaultOf[E]()
	if err != nil {
		panic(err)
	}
	x := Explanation{Point: d.name}
	for _, l := range r.chain {
		_, ok := l.impl.(E)
		c := Candidate{Link: Link{Code: l.code, Kind: l.kind}, Implements: ok}
		x.Candidates = append(x.Candidates, c)
		if ok && x.Selected.Kind == 0 {
			x.Selected = c.Link
		}
	}
	def := Link{Code: fmt.Sprintf("%T", d.impl), Kind: KindDefault}
	x.Candidates = append(x.Candidates, Candidate{Link: def, Implements: true})
	if x.Selected.Kind == 0 {
		x.Selected = def
	}
	return x
}

// String renders the resolution for logs, e.g. Resolution[business=biz.fresh, chain=[ability.x biz.fresh], skipped=[ability.y]].
func (r *Resolution) String() string {
	return fmt.Sprintf("Resolution[business=%s, chain=[%s], skipped=[%s]]",
		r.business, strings.Join(r.chainCodes(), " "), strings.Join(r.skipped, " "))
}

func (r *Resolution) chainCodes() []string {
	codes := make([]string, len(r.chain))
	for i, l := range r.chain {
		codes[i] = l.code
	}
	return codes
}
