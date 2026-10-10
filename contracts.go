package easyext

// Matcher identifies a business or activates an ability for one request.
// Match must be a side-effect-free decision using param, not an external I/O operation.
// Each business and each ability used by the selected business is matched at most once per Resolve.
type Matcher[P any] interface {
	Match(param P) bool
}

// MatcherFunc adapts a function to Matcher. Embed it in a component that supplies its own metadata.
type MatcherFunc[P any] func(param P) bool

func (f MatcherFunc[P]) Match(param P) bool { return f(param) }

// Ability is a reusable implementation of one or more registered extension points.
// Code must be stable and unique across abilities and businesses in a Registry.
type Ability[P any] interface {
	Matcher[P]
	Code() string
}

// Business identifies requests, declares the abilities it uses, and optionally implements extension points.
// Abilities returns registered ability codes in precedence order, optionally including Self.
// If Self is omitted, the business's own implementation comes first.
// Build reads and snapshots this metadata; it is not evaluated for each request.
type Business[P any] interface {
	Matcher[P]
	Code() string
	Abilities() []string
}

// Self places the business's own implementation among its abilities.
// It is a reserved marker, not an ability or business code.
const Self = "<self>"

// RequiresAbilities is an optional contract for an ability with co-mount requirements.
// Every business using it must explicitly list these abilities too.
// It does not imply precedence, invocation, or activation of those abilities for a request.
type RequiresAbilities interface {
	Requires() []string
}

// ExcludesAbilities is an optional contract for abilities that cannot be used by the same business.
type ExcludesAbilities interface {
	Excludes() []string
}
