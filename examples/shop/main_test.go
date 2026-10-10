package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
)

func TestExampleOutput(t *testing.T) {
	var output bytes.Buffer
	if err := run(&output); err != nil {
		t.Fatal(err)
	}
	want := "fresh: freight=21 delivery=2d notify=standard\n" +
		"fresh: freight=0 delivery=1d notify=express\n" +
		"retail: freight=6 delivery=1d notify=retail\n"
	if output.String() != want {
		t.Fatalf("got %q, want %q", output.String(), want)
	}
}

type failedWriter struct{}

var errWrite = errors.New("write failed")

func (failedWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestExamplePropagatesWriterError(t *testing.T) {
	if err := run(failedWriter{}); !errors.Is(err, errWrite) {
		t.Fatalf("error = %v", err)
	}
	if err := run(io.Discard); err != nil {
		t.Fatal(err)
	}
}

type selectionBusiness struct {
	code    string
	matches bool
	calls   int
}

func (b *selectionBusiness) Code() string      { return b.code }
func (*selectionBusiness) Abilities() []string { return nil }
func (b *selectionBusiness) Match(OrderParam) bool {
	b.calls++
	return b.matches
}

func TestServiceRequiresRegisteredPoints(t *testing.T) {
	for _, tt := range []struct {
		name   string
		points byte // FreightCalc=1, Delivery=2, Notify=4
	}{
		{"all points", 7}, {"missing freight", 6}, {"missing delivery", 5}, {"missing notify", 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			business := &selectionBusiness{code: "biz", matches: true}
			builder := easyext.New[OrderParam]().Default(&CommerceDefaults{}).Business(business)
			if tt.points&1 != 0 {
				builder.Point[FreightCalc]()
			}
			if tt.points&2 != 0 {
				builder.Point[Delivery]()
			}
			if tt.points&4 != 0 {
				builder.Point[Notify]()
			}
			registry, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			service := NewCheckoutService(registry)
			if business.calls != 0 {
				t.Fatalf("constructing the service evaluated Match %d times", business.calls)
			}
			ctx, err := registry.Bind(context.Background(), OrderParam{})
			if err != nil {
				t.Fatal(err)
			}
			quote, err := service.Checkout(ctx, "广东省", 3)
			if tt.points != 7 {
				if !errors.Is(err, easyext.ErrExtensionNotFound) || quote != (Quote{}) {
					t.Fatalf("missing point: quote=%+v error=%v", quote, err)
				}
			} else if err != nil || quote != (Quote{8, 3, "standard"}) {
				t.Fatalf("registered points: quote=%+v error=%v", quote, err)
			}
			if business.calls != 1 {
				t.Fatalf("queries evaluated Match %d times; want one Bind", business.calls)
			}
		})
	}
}

type countedFresh struct {
	Fresh
	calls int
}

func (b *countedFresh) Match(param OrderParam) bool {
	b.calls++
	return b.Fresh.Match(param)
}

type countedShipping struct {
	FreeShipping
	matches, invoked int
}

func (a *countedShipping) Match(param OrderParam) bool {
	a.matches++
	return a.FreeShipping.Match(param)
}

func (a *countedShipping) CalcFreight(ctx context.Context, province string, items int) (int, error) {
	a.invoked++
	return a.FreeShipping.CalcFreight(ctx, province, items)
}

type countedRapid struct {
	RapidDelivery
	matches, invoked int
}

func (a *countedRapid) Match(param OrderParam) bool {
	a.matches++
	return a.RapidDelivery.Match(param)
}

func (a *countedRapid) DeliveryDays(ctx context.Context) (int, error) {
	a.invoked++
	return a.RapidDelivery.DeliveryDays(ctx)
}

func (a *countedRapid) Notification(ctx context.Context, event string) (string, error) {
	a.invoked++
	return a.RapidDelivery.Notification(ctx, event)
}

func TestServiceUsesOneSnapshotAndFollowsChildBindings(t *testing.T) {
	fresh, shipping, rapid := &countedFresh{}, &countedShipping{}, &countedRapid{}
	registry, err := easyext.New[OrderParam]().
		Point[FreightCalc]().Point[Delivery]().Point[Notify]().Default(&CommerceDefaults{}).
		Ability(shipping).Ability(rapid).Business(fresh).Build()
	if err != nil {
		t.Fatal(err)
	}
	service := NewCheckoutService(registry)
	if fresh.calls != 0 || shipping.matches != 0 || rapid.matches != 0 {
		t.Fatal("startup evaluated Match")
	}
	if got, err := service.Checkout(context.Background(), "广东省", 3); !errors.Is(err, easyext.ErrNoBinding) || got != (Quote{}) {
		t.Fatalf("unbound call: %+v, %v", got, err)
	}
	parent, err := registry.Bind(context.Background(), OrderParam{Biz: "fresh", Province: "广东省", Items: 3})
	if err != nil {
		t.Fatal(err)
	}
	child, err := registry.Bind(parent, OrderParam{Biz: "fresh", MemberLevel: "SVIP", Province: "广东省", Items: 3, Urgent: true})
	if err != nil {
		t.Fatal(err)
	}
	if shipping.invoked != 0 || rapid.invoked != 0 {
		t.Fatal("binding invoked point methods")
	}
	for _, test := range []struct {
		ctx  context.Context
		want Quote
	}{
		{child, Quote{0, 1, "express"}},
		{parent, Quote{21, 2, "standard"}},
		{child, Quote{0, 1, "express"}},
	} {
		got, err := service.Checkout(test.ctx, "广东省", 3)
		if err != nil || got != test.want {
			t.Fatalf("quote=%+v, error=%v; want %+v", got, err, test.want)
		}
	}
	if fresh.calls != 2 || shipping.matches != 2 || rapid.matches != 2 {
		t.Fatalf("queries rematched: counts=%d/%d/%d", fresh.calls, shipping.matches, rapid.matches)
	}
	if shipping.invoked != 2 || rapid.invoked != 4 {
		t.Fatalf("invocations=%d/%d", shipping.invoked, rapid.invoked)
	}
}

func TestServiceRegistryIsolation(t *testing.T) {
	builder := easyext.New[OrderParam]().Point[FreightCalc]().Point[Delivery]().Point[Notify]().
		Default(&CommerceDefaults{}).Ability(&FreeShipping{}).Ability(&RapidDelivery{}).Business(&Fresh{})
	a, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	b, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	sa, sb := NewCheckoutService(a), NewCheckoutService(b)
	ctx, err := a.Bind(context.Background(), OrderParam{Biz: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sb.Checkout(ctx, "广东省", 3); !errors.Is(err, easyext.ErrNoBinding) {
		t.Fatalf("foreign binding: %v", err)
	}
	ctx, err = b.Bind(ctx, OrderParam{Biz: "fresh", MemberLevel: "SVIP"})
	if err != nil {
		t.Fatal(err)
	}
	qa, err := sa.Checkout(ctx, "广东省", 3)
	if err != nil {
		t.Fatal(err)
	}
	qb, err := sb.Checkout(ctx, "广东省", 3)
	if err != nil || qa.Freight != 21 || qb.Freight != 0 {
		t.Fatalf("bindings mixed: a=%+v b=%+v error=%v", qa, qb, err)
	}
}

type failingPoints struct {
	at       string
	calls    []string
	contexts []context.Context
	province string
	items    int
	event    string
	err      error
}

func (p *failingPoints) call(ctx context.Context, at string) error {
	p.calls = append(p.calls, at)
	p.contexts = append(p.contexts, ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.at == at {
		return p.err
	}
	return nil
}
func (p *failingPoints) CalcFreight(ctx context.Context, province string, items int) (int, error) {
	p.province, p.items = province, items
	return 99, p.call(ctx, "freight")
}
func (p *failingPoints) DeliveryDays(ctx context.Context) (int, error) {
	return 7, p.call(ctx, "delivery")
}
func (p *failingPoints) Notification(ctx context.Context, event string) (string, error) {
	p.event = event
	return "fake", p.call(ctx, "notify")
}

func TestServicePreservesBusinessErrorsAndStops(t *testing.T) {
	for _, at := range []string{"freight", "delivery", "notify"} {
		t.Run(at, func(t *testing.T) {
			want := errors.New("point failed")
			points := &failingPoints{at: at, err: want}
			registry, err := easyext.New[OrderParam]().Point[FreightCalc]().Point[Delivery]().Point[Notify]().
				Default(points).Business(&selectionBusiness{code: "biz", matches: true}).Build()
			if err != nil {
				t.Fatal(err)
			}
			service := NewCheckoutService(registry)
			ctx, err := registry.Bind(context.Background(), OrderParam{})
			if err != nil {
				t.Fatal(err)
			}
			if got, err := service.Checkout(ctx, "广东省", 3); err != want || got != (Quote{}) {
				t.Fatalf("quote=%+v, error=%v; want original error", got, err)
			}
			wantCalls := map[string]int{"freight": 1, "delivery": 2, "notify": 3}[at]
			if len(points.calls) != wantCalls {
				t.Fatalf("continued after error: %v", points.calls)
			}
			if at == "freight" {
				freight, err := registry.First[FreightCalc](ctx)
				if err != nil {
					t.Fatal(err)
				}
				value, err := freight.CalcFreight(ctx, "广东省", 3)
				if value != 99 || err != want {
					t.Fatalf("implementation returns: %d, %v", value, err)
				}
			}
		})
	}
}

func TestServiceUsesRegisteredFakesAndPassesContext(t *testing.T) {
	points := &failingPoints{}
	business := &selectionBusiness{code: "biz", matches: true}
	registry, err := easyext.New[OrderParam]().Point[FreightCalc]().Point[Delivery]().Point[Notify]().
		Default(points).Business(business).Build()
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, err := registry.Bind(parent, OrderParam{})
	if err != nil {
		t.Fatal(err)
	}
	service := NewCheckoutService(registry)
	quote, err := service.Checkout(ctx, "上海市", 5)
	if err != nil || quote != (Quote{99, 7, "fake"}) {
		t.Fatalf("quote=%+v, error=%v", quote, err)
	}
	if !slices.Equal(points.calls, []string{"freight", "delivery", "notify"}) {
		t.Fatalf("calls=%v", points.calls)
	}
	for _, received := range points.contexts {
		if received != ctx {
			t.Fatal("point did not receive the bound request context")
		}
	}
	if points.province != "上海市" || points.items != 5 || points.event != "order.created" {
		t.Fatalf("arguments changed: %+v", points)
	}
	cancel()
	if quote, err := service.Checkout(ctx, "上海市", 5); err != context.Canceled || quote != (Quote{}) {
		t.Fatalf("cancellation lost: quote=%+v, error=%v", quote, err)
	}
	if !slices.Equal(points.calls, []string{"freight", "delivery", "notify", "freight"}) || business.calls != 1 {
		t.Fatalf("canceled call continued or rematched: calls=%v matches=%d", points.calls, business.calls)
	}
}

func TestAllNotificationsFollowsBusinessOrder(t *testing.T) {
	registry, err := easyext.New[OrderParam]().Point[FreightCalc]().Point[Delivery]().Point[Notify]().
		Default(&CommerceDefaults{}).Ability(&FreeShipping{}).Ability(&RapidDelivery{}).
		Business(&Fresh{}).Business(&Retail{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		biz  string
		want []string
	}{
		{"fresh", []string{"express", "standard"}},
		{"retail", []string{"retail", "express", "standard"}},
	} {
		t.Run(tc.biz, func(t *testing.T) {
			ctx, err := registry.Bind(context.Background(), OrderParam{Biz: tc.biz, Province: "广东省", Items: 3, Urgent: true})
			if err != nil {
				t.Fatal(err)
			}
			notifications, err := registry.All[Notify](ctx)
			if err != nil {
				t.Fatal(err)
			}
			var messages []string
			for notify := range notifications {
				message, err := notify.Notification(ctx, "order.created")
				if err != nil {
					t.Fatal(err)
				}
				messages = append(messages, message)
			}
			if !slices.Equal(messages, tc.want) {
				t.Fatalf("messages=%v, want %v", messages, tc.want)
			}
		})
	}
}

func TestAbilityMatchesRequestFacts(t *testing.T) {
	for _, test := range []struct {
		p           OrderParam
		free, rapid bool
	}{
		{OrderParam{Amount: 499, Province: "广东省", Items: 3}, false, false},
		{OrderParam{Amount: 500, Province: "广东省", Items: 3}, true, false},
		{OrderParam{Amount: 600, Province: "新疆", Items: 3}, false, false},
		{OrderParam{MemberLevel: "SVIP", Province: "新疆", Items: 3}, true, false},
		{OrderParam{Province: "广东省", Items: 10, Urgent: true}, false, true},
		{OrderParam{Province: "广东省", Items: 11, Urgent: true}, false, false},
		{OrderParam{Province: "广东省", Items: 0, Urgent: true}, false, false},
		{OrderParam{Province: "新疆", Items: 3, Urgent: true}, false, false},
	} {
		if (&FreeShipping{}).Match(test.p) != test.free || (&RapidDelivery{}).Match(test.p) != test.rapid {
			t.Errorf("unexpected match for %+v", test.p)
		}
	}
}

func TestSharedServiceConcurrentRequests(t *testing.T) {
	registry, err := easyext.New[OrderParam]().Point[FreightCalc]().Point[Delivery]().Point[Notify]().
		Default(&CommerceDefaults{}).Ability(&FreeShipping{}).Ability(&RapidDelivery{}).Business(&Fresh{}).Build()
	if err != nil {
		t.Fatal(err)
	}
	service := NewCheckoutService(registry)
	var workers sync.WaitGroup
	failures := make(chan error, 100)
	for i := range 100 {
		workers.Go(func() {
			param := OrderParam{Biz: "fresh", Province: "广东省", Items: 3}
			want := Quote{21, 2, "standard"}
			if i%2 == 0 {
				param.MemberLevel, param.Urgent = "SVIP", true
				want = Quote{0, 1, "express"}
			}
			ctx, err := registry.Bind(context.Background(), param)
			if err != nil {
				failures <- err
				return
			}
			got, err := service.Checkout(ctx, param.Province, param.Items)
			if err != nil || got != want {
				failures <- fmt.Errorf("quote=%+v, error=%v; want %+v", got, err, want)
			}
		})
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
