package main

import (
	"context"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

const (
	FreeShippingCode  = "ability.free-shipping"
	RapidDeliveryCode = "ability.rapid-delivery"
)

// One default component implements all three points.
type CommerceDefaults struct{}

func (*CommerceDefaults) CalcFreight(context.Context, string, int) (int, error) { return 8, nil }
func (*CommerceDefaults) DeliveryDays(context.Context) (int, error)             { return 3, nil }
func (*CommerceDefaults) Notification(context.Context, string) (string, error) {
	return "standard", nil
}

type FreeShipping struct{}

func (*FreeShipping) Code() string { return FreeShippingCode }
func (*FreeShipping) Match(p OrderParam) bool {
	if p.MemberLevel == "SVIP" {
		return true
	}
	switch p.Province {
	case "新疆", "西藏", "内蒙古", "青海", "宁夏":
		return false
	default:
		return p.Amount >= 500
	}
}
func (*FreeShipping) CalcFreight(context.Context, string, int) (int, error) { return 0, nil }

// One ability implements two points and is reused by both businesses.
type RapidDelivery struct{}

func (*RapidDelivery) Code() string { return RapidDeliveryCode }
func (*RapidDelivery) Match(p OrderParam) bool {
	return p.Urgent && (p.Province == "广东省" || p.Province == "上海市") && p.Items > 0 && p.Items <= 10
}
func (*RapidDelivery) DeliveryDays(context.Context) (int, error) { return 1, nil }
func (*RapidDelivery) Notification(context.Context, string) (string, error) {
	return "express", nil
}

type Fresh struct{}

func (*Fresh) Code() string            { return "biz.fresh" }
func (*Fresh) Match(p OrderParam) bool { return p.Biz == "fresh" }
func (*Fresh) CalcFreight(_ context.Context, province string, items int) (int, error) {
	cost := 15 + 2*items
	if province != "广东省" {
		cost += 10
	}
	return cost, nil
}
func (*Fresh) DeliveryDays(context.Context) (int, error) { return 2, nil }
func (*Fresh) Abilities() []string {
	return []string{FreeShippingCode, RapidDeliveryCode, easyext.Self}
}

type Retail struct{}

func (*Retail) Code() string                                          { return "biz.retail" }
func (*Retail) Match(p OrderParam) bool                               { return p.Biz == "retail" }
func (*Retail) CalcFreight(context.Context, string, int) (int, error) { return 6, nil }
func (*Retail) Notification(context.Context, string) (string, error)  { return "retail", nil }
func (*Retail) Abilities() []string {
	return []string{FreeShippingCode, easyext.Self, RapidDeliveryCode}
}
