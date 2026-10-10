package main

import "context"

// OrderParam contains request facts used for matching, not switches selecting abilities.
// Amount uses whole currency units solely to keep this example's arithmetic simple.
type OrderParam struct {
	Biz         string
	Amount      int64
	MemberLevel string
	Province    string
	Items       int
	Urgent      bool
}

// Each point receives only the arguments it needs, separately from the matcher param.
// Context carries request cancellation and deadlines through each point call.
type FreightCalc interface {
	CalcFreight(ctx context.Context, province string, items int) (int, error)
}

type Delivery interface {
	DeliveryDays(ctx context.Context) (int, error)
}

type Notify interface {
	Notification(ctx context.Context, event string) (string, error)
}
