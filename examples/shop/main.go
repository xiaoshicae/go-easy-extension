package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

func run(out io.Writer) error {
	registry, err := easyext.New[OrderParam]().
		Point[FreightCalc]().Point[Delivery]().Point[Notify]().
		Default(&CommerceDefaults{}).
		Ability(&FreeShipping{}).Ability(&RapidDelivery{}).
		Business(&Fresh{}).Business(&Retail{}).
		Build()
	if err != nil {
		return err
	}
	service := NewCheckoutService(registry)

	for _, param := range []OrderParam{
		{Biz: "fresh", Amount: 100, Province: "广东省", Items: 3},
		{Biz: "fresh", Amount: 100, MemberLevel: "SVIP", Province: "广东省", Items: 3, Urgent: true},
		{Biz: "retail", Amount: 100, Province: "广东省", Items: 3, Urgent: true},
	} {
		// Request entry: match once. HTTP callers can let httpx.Middleware bind instead.
		ctx, err := registry.Bind(context.Background(), param)
		if err != nil {
			return err
		}
		quote, err := service.Checkout(ctx, param.Province, param.Items)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s: freight=%d delivery=%dd notify=%s\n",
			param.Biz, quote.Freight, quote.DeliveryDays, quote.Notification); err != nil {
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
