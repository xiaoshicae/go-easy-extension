// Package easyext is an extension point framework: generic flows depend on interfaces (extension points),
// and each request is answered by the implementation of its business, of the abilities that business mounts,
// or of the extension point's default implementation. It replaces per-business if-else chains in systems
// that serve many integration parties, such as order or fulfilment platforms.
//
// Assemble once, at startup:
//
//	c, err := easyext.New[OrderParam]().
//		Point[Freight](DefaultFreight{}).                       // extension point + its default implementation
//		Ability("ability.free-shipping", FreeShipping{}).       // reusable implementation, Match decides per request
//		Business("biz.fresh", Fresh{},                          // integration party, Match identifies its requests
//			easyext.Abilities("ability.free-shipping", easyext.Self)). // order = precedence; Self = the business itself
//		Build()                                                 // validates everything at once
//
// Then, per request, bind the param to the request context (or use the httpx middleware) and look
// extension points up anywhere below:
//
//	ctx, err := c.Bind(ctx, param)
//	freight, err := easyext.First[Freight](ctx)
//
// Resolution order: the abilities and the business itself in the order of [Abilities] (only abilities whose
// Match returns true), then the default implementation. [Resolution.First] returns the first that implements
// the extension point, [Resolution.All] all of them.
//
// Every value built here is immutable and safe for concurrent use. The design, and how it maps to the
// Java easy-extension 4.x, is described in doc/design-v2.md.
package easyext
