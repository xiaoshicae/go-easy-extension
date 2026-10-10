// Package shop is a many-to-many e-commerce fixture modeled after the Java sample.
package shop

import (
	"slices"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

const (
	FreeShippingCode  = "ability.free-shipping"
	Return7DaysCode   = "ability.return-7d"
	RapidDeliveryCode = "ability.rapid"
	RetailCode        = "biz.retail"
	FreshCode         = "biz.fresh"
	DigitalCode       = "biz.digital"
)

// Param contains request identity and server-derived feature flags.
// These flags activate only abilities declared by the selected business.
type Param struct {
	Biz       string
	Abilities []string
}

func (p Param) has(ability string) bool { return slices.Contains(p.Abilities, ability) }

type Order struct{ Items int }

type (
	Freight   interface{ Freight(Order) int }
	AfterSale interface{ ReturnDays() int }
	Notify    interface{ Channels() []string }
	Delivery  interface{ DeliveryDays() int }
)

// Defaults implements several points; the individual defaults are useful for focused tests.
type (
	Defaults         struct{}
	DefaultFreight   struct{}
	DefaultAfterSale struct{}
	DefaultNotify    struct{}
	DefaultDelivery  struct{}
)

func (Defaults) Freight(Order) int        { return 8 }
func (Defaults) ReturnDays() int          { return 0 }
func (Defaults) Channels() []string       { return []string{"PUSH"} }
func (Defaults) DeliveryDays() int        { return 3 }
func (DefaultFreight) Freight(Order) int  { return 8 }
func (DefaultAfterSale) ReturnDays() int  { return 0 }
func (DefaultNotify) Channels() []string  { return []string{"PUSH"} }
func (DefaultDelivery) DeliveryDays() int { return 3 }

type (
	FreeShipping  struct{}
	Return7Days   struct{}
	RapidDelivery struct{}
)

func (FreeShipping) Code() string        { return FreeShippingCode }
func (FreeShipping) Match(p Param) bool  { return p.has("free-shipping") }
func (FreeShipping) Freight(Order) int   { return 0 }
func (Return7Days) Code() string         { return Return7DaysCode }
func (Return7Days) Match(p Param) bool   { return p.has("return-7d") }
func (Return7Days) ReturnDays() int      { return 7 }
func (RapidDelivery) Code() string       { return RapidDeliveryCode }
func (RapidDelivery) Match(p Param) bool { return p.has("rapid") }
func (RapidDelivery) Channels() []string { return []string{"SMS", "PUSH", "WECHAT_MSG"} }
func (RapidDelivery) DeliveryDays() int  { return 1 }

type (
	Retail  struct{}
	Fresh   struct{}
	Digital struct{}
)

func (Retail) Code() string       { return RetailCode }
func (Retail) Match(p Param) bool { return p.Biz == "retail" }
func (Retail) Abilities() []string {
	return []string{FreeShippingCode, Return7DaysCode, RapidDeliveryCode}
}
func (Fresh) Code() string          { return FreshCode }
func (Fresh) Match(p Param) bool    { return p.Biz == "fresh" }
func (Fresh) Abilities() []string   { return []string{FreeShippingCode, RapidDeliveryCode, easyext.Self} }
func (Fresh) Freight(o Order) int   { return 15 + 2*o.Items }
func (Fresh) DeliveryDays() int     { return 2 }
func (Digital) Code() string        { return DigitalCode }
func (Digital) Match(p Param) bool  { return p.Biz == "digital" }
func (Digital) Abilities() []string { return []string{Return7DaysCode, easyext.Self} }
func (Digital) ReturnDays() int     { return 15 }

var (
	_ easyext.Ability[Param]  = FreeShipping{}
	_ easyext.Ability[Param]  = RapidDelivery{}
	_ easyext.Business[Param] = Fresh{}
)
