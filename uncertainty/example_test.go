package uncertainty_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/symbol"
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

// A midpoint is a point and not a bound, so it rounds the way D9 rounds every
// other point. It is taken on the two magnitudes inside the one scale the
// bounds share, because the sum of two absolute magnitudes is not a magnitude
// (D6) and an affine map preserves midpoints anyway.
func ExampleRange_Mid() {
	warm, _ := uncertainty.Parse("[19.5, 20.5] °C")
	fmt.Println(warm.Mid())

	// Rounding to nearest can put the midpoint outside a range narrower than
	// the engine's precision. Where the answer has to be an enclosure, it comes
	// from Lo and Hi, which round outward.
	narrow, _ := uncertainty.Parse("[0.999, 0.9999] m")
	fmt.Println(uncertainty.NewEngine(1).Mid(narrow))
	// Output:
	// 20 °C <nil>
	// 1 m <nil>
}

// A width is a difference of two points, which D6 already makes a span — so it
// is read on the interval unit the scale declares, and the width of 20 … 21 °C
// is 1 K rather than 1 °C. It rounds toward +∞, because a width reported too
// small is a claim the data does not support.
func ExampleRange_Width() {
	warm, _ := uncertainty.Parse("[20, 21] °C")
	fmt.Println(warm.Width())

	measured, _ := uncertainty.Parse("[2.5, 2.6] bar")
	fmt.Println(measured.Width())
	// Output:
	// 1 K <nil>
	// 0.1 bar <nil>
}

// The even power of an interval that straddles zero is where an interval stops
// being a pair of numbers one can compute with separately: its minimum is at
// zero, which is at neither bound.
func ExampleRange_Pow() {
	r, _ := uncertainty.Parse("[-2, 3] m")
	fmt.Println(r.Pow(2))

	// A negative power is the reciprocal of the positive one, so an interval
	// covering zero has none: the quotient runs to infinity in both directions
	// and no pair of finite bounds encloses it.
	_, err := r.Pow(-1)
	fmt.Println(errors.Is(err, uncertainty.ErrUnbounded))
	// Output:
	// [0, 9] m² <nil>
	// true
}

// Reading a symbol needs a catalogue and a catalogue is context (D7), so a
// Parser is a value rather than a registry: a program with units of its own
// builds one over its own [parse.Parser], and the shipped units are what the
// zero Parser reads.
func ExampleNew() {
	widget := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One,
		Symbol:    symbol.Static("widget"),
	})
	mine := uncertainty.New(parse.New([]metrology.Unit{widget}))

	fmt.Println(mine.Range("[1, 2] widget"))

	// A catalogue of one unit does not read the metre, and the shipped
	// catalogue does not read a widget.
	_, err := mine.Range("[1, 2] m")
	fmt.Println(errors.Is(err, parse.ErrUnknownUnit))
	_, err = uncertainty.Parse("[1, 2] widget")
	fmt.Println(errors.Is(err, parse.ErrUnknownUnit))
	// Output:
	// [1, 2] widget <nil>
	// true
	// true
}

// A NUMERIC column carries a magnitude and the schema carries the unit. In
// says which unit a bare number is on, and a number is a range of zero width —
// which is what an exact value in such a column means.
func ExampleText_In() {
	column := uncertainty.Text{}.In(pressure.Bar)
	if err := column.Scan("2.5"); err != nil {
		panic(err)
	}
	fmt.Println(column.Range)

	// A text that names its own unit is still read as one, so a column that
	// grew a unit does not have to be read differently.
	if err := column.Scan("[2.5, 2.6] kPa"); err != nil {
		panic(err)
	}
	fmt.Println(column.Range)
	// Output:
	// [2.5, 2.5] bar
	// [2.5, 2.6] kPa
}
