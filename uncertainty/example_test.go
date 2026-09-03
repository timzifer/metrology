package uncertainty_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
)

func ExampleBetween() {
	lo, _ := length.Metre.OfString("3.65")
	hi, _ := length.Metre.OfString("3.75")

	r, err := uncertainty.Between(lo, hi)
	fmt.Println(r, err)

	// A range holds one scale, so two bounds of one dimension read on two
	// different units are refused rather than silently converted.
	_, err = uncertainty.Between(pressure.Bar.Of(1), pressure.Pascal.Of(200000))
	fmt.Println(errors.Is(err, uncertainty.ErrScale))
	// Output:
	// [3.65, 3.75] m <nil>
	// true
}

func ExampleSymmetric() {
	// A tolerance is a span along a scale and never a point on it, so 20 °C
	// takes its tolerance in kelvin — which is what D6 says about every other
	// addition too.
	r, err := uncertainty.Symmetric(temperature.Celsius.Of(20), interval.Kelvin.Of(0.5))
	fmt.Println(r, err)

	width, _ := r.Width()
	fmt.Println("width:", width)
	// Output:
	// [19.5, 20.5] °C <nil>
	// width: 1 K
}

func ExampleOf() {
	// A magnitude that is known exactly is a range of zero width, and every
	// operation of this package is reachable from it.
	r := uncertainty.Of(length.Metre.Of(2))
	squared, err := r.Pow(2)
	fmt.Println(squared, err)
	// Output: [4, 4] m² <nil>
}

// A conversion widens a range rather than narrowing it, which is the finding
// D15 rests on: rounding a bound inward can turn an overlap into a disjoint
// pair, and that is a disagreement the conversion invented.
func ExampleRange_To() {
	r, _ := uncertainty.Parse("[2.5, 2.6] bar")

	inTorr, _ := r.To(pressure.Torr)
	fmt.Println(inTorr)

	back, _ := inTorr.To(pressure.Bar)
	fmt.Println(back)
	// Output:
	// [1875.154206760424377, 1950.1603750308413521] Torr
	// [2.4999999999999999999, 2.6000000000000000001] bar
}

// All four products, because an interval that straddles zero does not take its
// extreme at the corner one would guess.
func ExampleRange_Mul() {
	a, _ := uncertainty.Parse("[-2, 3] m")
	product, err := a.Mul(a)
	fmt.Println(product, err)

	// And the square is not the product: an even power of an interval across
	// zero has its minimum at zero, which is at neither bound.
	square, err := a.Pow(2)
	fmt.Println(square, err)
	// Output:
	// [-6, 9] m² <nil>
	// [0, 9] m² <nil>
}

// x − x is not zero and x / x is not one. That is the dependency problem, and
// it is why this package is called what it is called and warns on its first
// line: interval arithmetic gives worst-case bounds, not an uncertainty budget.
func ExampleRange_Sub_dependency() {
	x, _ := uncertainty.Parse("[1, 2] m")
	difference, _ := x.Sub(x)
	fmt.Println(difference)
	// Output: [-1, 1] m
}

// A divisor covering zero has no quotient with finite bounds, and reporting one
// would be a lie about the data.
func ExampleRange_Div() {
	a, _ := uncertainty.Parse("[1, 2] m")
	b, _ := uncertainty.Parse("[4, 8] m")
	quotient, err := a.Div(b)
	fmt.Println(quotient, err)

	_, err = a.Div(uncertainty.Of(length.Metre.Of(0)))
	fmt.Println(errors.Is(err, uncertainty.ErrUnbounded))
	// Output:
	// [0.125, 0.5] m/m <nil>
	// true
}

func ExampleRange_Overlaps() {
	measured, _ := uncertainty.Parse("2.55 ± 0.05 bar")
	specified, _ := uncertainty.Parse("[254, 258] kPa")

	agree, err := measured.Overlaps(specified)
	fmt.Println(agree, err)

	tighter, _ := uncertainty.Parse("[261, 263] kPa")
	disagree, err := measured.Overlaps(tighter)
	fmt.Println(disagree, err)
	// Output:
	// true <nil>
	// false <nil>
}

// The bracket form is canonical because it states the two magnitudes the range
// holds; the ± form is a rendering, and it says so when it cannot be written
// exactly (D12).
func ExampleRange_PlusMinus() {
	r, _ := uncertainty.Parse("[3.5, 3.9] m")
	fmt.Println(r)

	text, ok := r.PlusMinus()
	fmt.Println(text, ok)
	// Output:
	// [3.5, 3.9] m
	// 3.7 ± 0.2 m true
}

// Three spellings go in, one comes out: reading is a parser and writing is a
// method (D12).
func ExampleParse() {
	for _, text := range []string{"[3.65, 3.75] mm", "3.7 ± 0.05 mm", "3.70(5) mm"} {
		r, err := uncertainty.Parse(text)
		fmt.Printf("%-16s → %v %v\n", text, r, err)
	}
	// Output:
	// [3.65, 3.75] mm  → [3.65, 3.75] mm <nil>
	// 3.7 ± 0.05 mm    → [3.65, 3.75] mm <nil>
	// 3.70(5) mm       → [3.65, 3.75] mm <nil>
}

// The standard decoding interfaces are handed no catalogue, so the parser
// travels in the destination (D7).
func ExampleText() {
	var row struct {
		P uncertainty.Text `json:"p"`
	}
	if err := json.Unmarshal([]byte(`{"p":"[2.5, 2.6] bar"}`), &row); err != nil {
		panic(err)
	}
	fmt.Println(row.P.Range)

	out, _ := json.Marshal(row)
	fmt.Println(string(out))
	// Output:
	// [2.5, 2.6] bar
	// {"p":"[2.5, 2.6] bar"}
}

// Precision belongs to the computation here as it does in the core (D9). What
// does not belong to the caller is the rounding mode: a lower bound rounds
// toward −∞ and an upper bound toward +∞, or the interval is not one.
func ExampleNewEngine() {
	r, _ := uncertainty.Parse("[1, 1] bar")

	fmt.Println(r.To(pressure.Torr))
	fmt.Println(uncertainty.NewEngine(25).To(r, pressure.Torr))
	// Output:
	// [750.0616827041697508, 750.06168270416975081] Torr <nil>
	// [750.0616827041697508018751, 750.0616827041697508018752] Torr <nil>
}

// The kind rules of D6 reach a range through its bounds, and this package adds
// no clause of its own.
func ExampleRange_Add() {
	warm, _ := uncertainty.Parse("20 ± 0.5 °C")
	step := uncertainty.Of(interval.Kelvin.Of(5))

	fmt.Println(warm.Add(step))

	// Two points on a scale have no sum, whether or not they are ranges.
	_, err := warm.Add(warm)
	fmt.Println(errors.Is(err, metrology.ErrKind))
	// Output:
	// [24.5, 25.5] °C <nil>
	// true
}
