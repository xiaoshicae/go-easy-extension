// Package easyext selects extension point implementations for requests belonging to different businesses.
//
// A point is a non-empty Go interface. An Ability[P] supplies Code and Match, implements one or more
// points, and can be reused by several businesses. A Business[P] supplies Code, Match and Abilities;
// its Abilities method returns ability codes in precedence order, optionally including Self.
// Defaults are independent components and may implement multiple points.
//
// Assemble at startup:
//
//	registry, err := easyext.New[OrderParam]().
//		Point[Freight]().Point[Delivery]().
//		Default(&CommerceDefaults{}).
//		Ability(&FreeShipping{}).
//		Business(&Fresh{}).
//		Build()
//
// The component metadata belongs to the component, not to the bootstrap code:
//
//	func (*Fresh) Abilities() []string {
//		return []string{FreeShippingCode, easyext.Self}
//	}
//
// Build validates the complete assembly and snapshots metadata. Each registered point must have exactly
// one explicit default. Optional Requires and Excludes methods constrain co-mounting, not activation or execution.
//
// Each Resolve must match exactly one business. Its abilities are matched once, producing an immutable
// Resolution. First returns the first active implementation of a point, otherwise its default; All yields
// active implementations followed by the default, deduplicating shared providers. Queries return errors
// for unregistered points, not lookup panics.
//
// Bind once at the request entry and inject the Registry into services. Services query point interfaces
// using the same bound context, then call the selected implementation's ordinary methods:
//
//	ctx, err := registry.Bind(ctx, param)
//	freight, err := registry.First[Freight](ctx)
//
// Point methods are ordinary Go interface methods with no framework-specific signature requirements.
// Resolve is available when an explicit Resolution is needed instead of a context binding; extensions is
// a useful variable name for this selection snapshot, not the business methods' computed result.
//
// Bindings are isolated by Registry identity and follow context.Context into child scopes and goroutines.
// Registry and Resolution are safe to share, but implementations themselves must be safe for concurrent calls.
// Match should be a side-effect-free decision without external I/O.
//
// See examples/shop for a complete runnable example and doc/design-v3.md for the design.
package easyext
