// Package shop is a small e-commerce assembly used by the tests, after the Java easy-extension sample.
package shop

import "slices"

// Param identifies a request: the business line and the abilities it asks for.
type Param struct {
	Biz       string
	Abilities []string
}

func (p Param) has(ability string) bool { return slices.Contains(p.Abilities, ability) }

// Order is what the extension points work on.
type Order struct{ Items int }

// Extension points.
type (
	Freight   interface{ Freight(o Order) int }
	AfterSale interface{ ReturnDays() int }
	Notify    interface{ Channels() []string }
)

// Default implementations.
type (
	DefaultFreight   struct{}
	DefaultAfterSale struct{}
	DefaultNotify    struct{}
)

func (DefaultFreight) Freight(Order) int { return 8 }
func (DefaultAfterSale) ReturnDays() int { return 0 }
func (DefaultNotify) Channels() []string { return []string{"PUSH"} }

// Abilities.
type (
	FreeShipping  struct{}
	Return7Days   struct{}
	RapidDelivery struct{}
)

func (FreeShipping) Match(p Param) bool  { return p.has("free-shipping") }
func (FreeShipping) Freight(Order) int   { return 0 }
func (Return7Days) Match(p Param) bool   { return p.has("return-7d") }
func (Return7Days) ReturnDays() int      { return 7 }
func (RapidDelivery) Match(p Param) bool { return p.has("rapid") }
func (RapidDelivery) Channels() []string { return []string{"SMS", "PUSH", "WECHAT_MSG"} }

// Businesses.
type (
	Retail  struct{}
	Fresh   struct{}
	Digital struct{}
)

func (Retail) Match(p Param) bool  { return p.Biz == "retail" }
func (Fresh) Match(p Param) bool   { return p.Biz == "fresh" }
func (Digital) Match(p Param) bool { return p.Biz == "digital" }

// Fresh: cold-chain freight, 15 + 2 per item.
func (Fresh) Freight(o Order) int { return 15 + 2*o.Items }

// Digital: 15 days return window.
func (Digital) ReturnDays() int { return 15 }
