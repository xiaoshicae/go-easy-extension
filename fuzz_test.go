package easyext_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/xiaoshicae/go-easy-extension/v3"
	"github.com/xiaoshicae/go-easy-extension/v3/internal/fixtures/shop"
)

type fuzzInput struct {
	data []byte
	next int
}

func (in *fuzzInput) take() byte {
	if len(in.data) == 0 {
		return 0
	}
	value := in.data[in.next%len(in.data)]
	in.next++
	return value
}

func (in *fuzzInput) shuffle(items []int) {
	for i := len(items) - 1; i > 0; i-- {
		j := int(in.take()) % (i + 1)
		items[i], items[j] = items[j], items[i]
	}
}

type fuzzParam struct {
	business int
	active   byte
}

type fuzzComponent struct {
	code    string
	used    []string
	ability bool
	index   int
	calls   int
}

func (c *fuzzComponent) Code() string        { return c.code }
func (c *fuzzComponent) Abilities() []string { return c.used }
func (c *fuzzComponent) Match(p fuzzParam) bool {
	c.calls++
	if c.ability {
		return p.active&(1<<c.index) != 0
	}
	return p.business == c.index
}

type fuzzFreight int

func (v fuzzFreight) Freight(shop.Order) int { return int(v) }

type fuzzDelivery int

func (v fuzzDelivery) DeliveryDays() int { return int(v) }

// All providers are non-zero-sized pointers with distinct IDs. Varying their
// method sets makes the oracle independent of Build's compiled point indexes.
func fuzzProvider(component *fuzzComponent, id int, points byte) any {
	switch points {
	case 1:
		return &struct {
			*fuzzComponent
			fuzzFreight
		}{component, fuzzFreight(id)}
	case 2:
		return &struct {
			*fuzzComponent
			fuzzDelivery
		}{component, fuzzDelivery(id)}
	case 3:
		return &struct {
			*fuzzComponent
			fuzzFreight
			fuzzDelivery
		}{component, fuzzFreight(id), fuzzDelivery(id)}
	default:
		return component
	}
}

type fuzzStep struct {
	provider any
	link     easyext.Link
	active   bool
}

func fuzzDefault[E any](pick byte, providers []any) E {
	for i := range len(providers) {
		if impl, ok := providers[(int(pick)+i)%len(providers)].(E); ok {
			return impl
		}
	}
	panic("the oracle always supplies a provider implementing both points")
}

// Compare the sparse-index implementation against a full, direct interface scan.
// The scan deduplicates only pointer identities, including the trailing default.
func checkFuzzPoint[E any](t *testing.T, result *easyext.Resolution, steps []fuzzStep, fallback E) {
	t.Helper()
	var want []any
	var selected easyext.Link
	expected := make([]easyext.Candidate, 0, len(steps)+1)
	seen := make(map[any]bool)
	for i, step := range steps {
		impl, implements := step.provider.(E)
		candidate := easyext.Candidate{Link: step.link, Position: i, Implements: implements, Active: step.active}
		switch {
		case !step.active:
			candidate.Reason = "match-false"
		case !implements:
			candidate.Reason = "not-implemented"
		case selected.Kind == 0:
			candidate.Reason = "selected"
			selected = step.link
		default:
			candidate.Reason = "lower-priority"
		}
		expected = append(expected, candidate)
		if step.active && implements && !seen[any(impl)] {
			seen[any(impl)] = true
			want = append(want, impl)
		}
	}
	last := easyext.Candidate{
		Link:     easyext.Link{Code: fmt.Sprintf("%T", fallback), Kind: easyext.KindDefault},
		Position: len(steps), Implements: true, Active: true, Reason: "lower-priority",
	}
	if selected.Kind == 0 {
		selected = last.Link
		last.Reason = "selected"
	} else if seen[any(fallback)] {
		last.Reason = "duplicate"
	}
	expected = append(expected, last)
	if !seen[any(fallback)] {
		want = append(want, fallback)
	}
	gotFirst, err := result.First[E]()
	if err != nil || any(gotFirst) != want[0] {
		t.Fatalf("First[%T] = %v, %v; want %v", fallback, gotFirst, err, want[0])
	}
	sequence, err := result.All[E]()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		var got []any
		for impl := range sequence {
			got = append(got, impl)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("All[%T] = %v; want %v", fallback, got, want)
		}
	}
	for impl := range sequence {
		if any(impl) != want[0] {
			t.Fatal("early termination changed the first provider")
		}
		break
	}
	explanation, err := result.Explain[E]()
	if err != nil || explanation.Selected != selected || !slices.Equal(explanation.Candidates, expected) {
		t.Fatalf("Explain[%T] = %+v, %v; want selected=%+v, candidates=%+v", fallback, explanation, err, selected, expected)
	}
}

func FuzzResolutionMatchesReference(f *testing.F) {
	for _, data := range [][]byte{nil, {0}, {255}, {8, 1, 2, 3, 4, 5, 6, 7}, {5, 17, 0, 251, 3, 8, 2, 9, 64}} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		in := fuzzInput{data: data}
		count := int(in.take() % 9)
		providers := []any{fuzzProvider(&fuzzComponent{code: "default"}, 0, 3)}
		components := make([]*fuzzComponent, 0, count+2)
		for i := range count {
			component := &fuzzComponent{code: fmt.Sprintf("a.%d", i), ability: true, index: i}
			components = append(components, component)
			providers = append(providers, fuzzProvider(component, i+1, 1+in.take()%3))
		}
		orders := make([][]int, 2)
		for b := range 2 {
			self := count + b + 1
			component := &fuzzComponent{code: fmt.Sprintf("b.%d", b), index: b}
			components = append(components, component)
			providers = append(providers, fuzzProvider(component, self, in.take()%4))
			for i := range count {
				if in.take()&1 != 0 {
					orders[b] = append(orders[b], i+1)
				}
			}
			explicitSelf := in.take()&1 != 0
			if explicitSelf {
				orders[b] = append(orders[b], self)
			}
			in.shuffle(orders[b])
			for _, id := range orders[b] {
				code := easyext.Self
				if id != self {
					code = components[id-1].code
				}
				component.used = append(component.used, code)
			}
			if !explicitSelf {
				orders[b] = append([]int{self}, orders[b]...)
			}
		}
		freight := fuzzDefault[shop.Freight](in.take(), providers)
		delivery := fuzzDefault[shop.Delivery](in.take(), providers)
		builder := easyext.New[fuzzParam]().DefaultFor[shop.Freight](freight).DefaultFor[shop.Delivery](delivery)
		if in.take()&1 != 0 {
			builder.Point[shop.Freight]().Point[shop.Delivery]()
		} else {
			builder.Point[shop.Delivery]().Point[shop.Freight]()
		}
		registration := make([]int, count+2)
		for i := range registration {
			registration[i] = i
		}
		in.shuffle(registration)
		for _, i := range registration {
			if i < count {
				builder.Ability(providers[i+1].(easyext.Ability[fuzzParam]))
			} else {
				builder.Business(providers[i+1].(easyext.Business[fuzzParam]))
			}
		}
		registry := mustBuild(t, builder)
		param := fuzzParam{business: int(in.take() % 2), active: in.take()}
		result := mustResolve(t, registry, param)
		var steps []fuzzStep
		trace := easyext.Trace{Business: components[count+param.business].code}
		for _, id := range orders[param.business] {
			component := components[id-1]
			step := fuzzStep{provider: providers[id], link: easyext.Link{Code: component.code, Kind: easyext.KindBusiness}, active: true}
			if component.ability {
				step.link.Kind = easyext.KindAbility
				step.active = param.active&(1<<component.index) != 0
			}
			steps = append(steps, step)
			if step.active {
				trace.Chain = append(trace.Chain, step.link)
			} else {
				trace.Skipped = append(trace.Skipped, step.link.Code)
			}
		}
		checkFuzzPoint[shop.Freight](t, result, steps, freight)
		checkFuzzPoint[shop.Delivery](t, result, steps, delivery)
		gotTrace := result.Trace()
		if result.Business() != trace.Business || gotTrace.Business != trace.Business ||
			!slices.Equal(gotTrace.Chain, trace.Chain) || !slices.Equal(gotTrace.Skipped, trace.Skipped) {
			t.Fatalf("Trace = %+v; want %+v", gotTrace, trace)
		}
		for i, component := range components {
			wantCalls := 1
			if component.ability && !slices.Contains(orders[param.business], i+1) {
				wantCalls = 0
			}
			if component.calls != wantCalls {
				t.Fatalf("%s matched %d times; want %d", component.code, component.calls, wantCalls)
			}
		}
	})
}

func FuzzConstraintsMatchReference(f *testing.F) {
	for _, data := range [][]byte{nil, {0}, {255}, {1, 2, 4, 8, 16, 32, 64, 128}, {3, 2, 0, 1, 0, 8, 0, 4, 0, 15}} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		in := fuzzInput{data: data}
		count := 1 + int(in.take()%8)
		mask := uint16(1<<count) - 1
		requires, excludes := make([]uint16, count), make([]uint16, count)
		codes := make([]string, count)
		for i := range codes {
			codes[i] = fmt.Sprintf("a.%d", i)
			requires[i], excludes[i] = uint16(in.take())&mask, uint16(in.take())&mask
		}
		mounted := uint16(in.take()) & mask
		valid := true
		builder := freightBuilder()
		var used []string
		for i, code := range codes {
			ability := &testAbility{code: code}
			for j, ref := range codes {
				bit := uint16(1 << j)
				if requires[i]&bit != 0 {
					ability.requires = append(ability.requires, ref)
				}
				if excludes[i]&bit != 0 {
					ability.excludes = append(ability.excludes, ref)
				}
			}
			builder.Ability(ability)
			bit := uint16(1 << i)
			if requires[i]&bit != 0 || excludes[i]&bit != 0 {
				valid = false
			}
			if mounted&bit != 0 {
				used = append(used, code)
				if requires[i]&mounted != requires[i] || excludes[i]&mounted != 0 {
					valid = false
				}
			}
			// A fixed-point bitset is independent of the validator's graph traversal.
			closure := bit
			for previous := uint16(0); previous != closure; {
				previous = closure
				for j := range count {
					if closure&(1<<j) != 0 {
						closure |= requires[j]
					}
				}
			}
			for j := range count {
				if closure&(1<<j) != 0 && excludes[j]&closure != 0 {
					valid = false
				}
			}
		}
		registry, err := builder.Business(&testBusiness[shop.Param]{code: "biz", used: used}).Build()
		if (err == nil) != valid || (registry != nil) != valid {
			t.Fatalf("Build = %v; want valid=%v; requires=%08b excludes=%08b mounted=%08b", err, valid, requires, excludes, mounted)
		}
	})
}
