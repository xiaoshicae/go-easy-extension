package main

import (
	"fmt"
	"io"
	"os"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

const (
	FreeShippingCode  = "ability.free-shipping"
	RapidDeliveryCode = "ability.rapid-delivery"
)

type OrderParam struct {
	Biz          string
	FreeShipping bool
	Rapid        bool
}

type Freight interface{ CalcFreight(items int) int }
type Delivery interface{ DeliveryDays() int }
type Notify interface{ Notification() string }

// One default component implements all three points.
type CommerceDefaults struct{}

func (*CommerceDefaults) CalcFreight(int) int  { return 8 }
func (*CommerceDefaults) DeliveryDays() int    { return 3 }
func (*CommerceDefaults) Notification() string { return "standard" }

type FreeShipping struct{}

func (*FreeShipping) Code() string            { return FreeShippingCode }
func (*FreeShipping) Match(p OrderParam) bool { return p.FreeShipping }
func (*FreeShipping) CalcFreight(int) int     { return 0 }

// One ability implements two points and is reused by both businesses.
type RapidDelivery struct{}

func (*RapidDelivery) Code() string            { return RapidDeliveryCode }
func (*RapidDelivery) Match(p OrderParam) bool { return p.Rapid }
func (*RapidDelivery) DeliveryDays() int       { return 1 }
func (*RapidDelivery) Notification() string    { return "express" }

type Fresh struct{}

func (*Fresh) Code() string              { return "biz.fresh" }
func (*Fresh) Match(p OrderParam) bool   { return p.Biz == "fresh" }
func (*Fresh) CalcFreight(items int) int { return 15 + 2*items }
func (*Fresh) DeliveryDays() int         { return 2 }
func (*Fresh) Abilities() []string {
	return []string{FreeShippingCode, RapidDeliveryCode, easyext.Self}
}

type Retail struct{}

func (*Retail) Code() string            { return "biz.retail" }
func (*Retail) Match(p OrderParam) bool { return p.Biz == "retail" }
func (*Retail) CalcFreight(int) int     { return 6 }
func (*Retail) Notification() string    { return "retail" }
func (*Retail) Abilities() []string {
	return []string{FreeShippingCode, easyext.Self, RapidDeliveryCode}
}

func run(out io.Writer) error {
	registry, err := easyext.New[OrderParam]().
		Point[Freight]().Point[Delivery]().Point[Notify]().
		Default(&CommerceDefaults{}).
		Ability(&FreeShipping{}).Ability(&RapidDelivery{}).
		Business(&Fresh{}).Business(&Retail{}).
		Build()
	if err != nil {
		return err
	}
	for _, param := range []OrderParam{
		{Biz: "fresh"},
		{Biz: "fresh", FreeShipping: true, Rapid: true},
		{Biz: "retail", Rapid: true},
	} {
		result, err := registry.Resolve(param)
		if err != nil {
			return err
		}
		freight, err := result.First[Freight]()
		if err != nil {
			return err
		}
		delivery, err := result.First[Delivery]()
		if err != nil {
			return err
		}
		notify, err := result.First[Notify]()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s: freight=%d delivery=%dd notify=%s\n",
			param.Biz, freight.CalcFreight(3), delivery.DeliveryDays(), notify.Notification()); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
