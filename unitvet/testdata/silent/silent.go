// Package silent holds one function per pattern the checker of D13 says it
// cannot decide, and the correct code it must not complain about either.
//
// Not one line in this file may produce a diagnostic. That is the half of the
// contract that matters most: a dimension checker with false positives is a
// dimension checker that gets switched off, and then it catches nothing at all.
package silent

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/angle"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/ratio"
	"github.com/timzifer/metrology/units/solidangle"
	"github.com/timzifer/metrology/units/temperature"
)

// The same dimension in two units is not a mistake — it is the ordinary case,
// and the conversion is exact.
func sameDimensionDifferentUnits() {
	_, _ = pressure.Bar.Of(2.5).Add(pressure.Pascal.Of(1000))
	_, _ = pressure.Bar.Of(2.5).To(pressure.Torr)
	_, _ = temperature.Celsius.Of(20).Add(interval.Kelvin.Of(5))
	_, _ = temperature.Celsius.Of(25).Sub(temperature.Celsius.Of(20))
	_, _ = frequency.Hertz.Of(50).Cmp(frequency.Hertz.Of(60))
}

// A unit chosen at run time is a phi node, and which of the two it is is not
// decided here.
func aRuntimeChoice(fine bool) {
	u := pressure.Bar
	if fine {
		u = pressure.Pascal
	}
	_, _ = u.Of(1).Add(length.Metre.Of(1))
}

// An operand arriving as a parameter has no origin in this function.
func fromAParameter(u metrology.Unit, m metrology.Measurement) {
	_, _ = u.Of(1).Add(length.Metre.Of(1))
	_, _ = m.Add(length.Metre.Of(1))
	_, _ = m.To(length.Metre)
	_, _ = m.Cmp(length.Metre.Of(1))
}

// Units held in a map, a slice or a struct field are equally out of reach.
var table = map[string]metrology.Unit{"m": length.Metre}

var list = []metrology.Unit{length.Metre}

type reading struct {
	unit metrology.Unit
}

func fromAContainer(r reading) {
	_, _ = table["m"].Of(1).Add(duration.Second.Of(1))
	_, _ = list[0].Of(1).Add(duration.Second.Of(1))
	_, _ = r.unit.Of(1).Add(duration.Second.Of(1))
}

// A unit of somebody else's own is not in the generated table, and nothing
// here assumes a variable it does not know is never written to. This is the
// case the design is built for: a program with a catalogue of its own.
var hand = metrology.MustUnit(metrology.UnitDef{
	Dimension: dimension.New(dimension.Exponents{Length: 1}),
	Symbol:    symbol.Static("hand"),
	Numerator: "1016",
	// Ten thousand hands to the metre, near enough for a horse.
	Denominator: "10000",
})

func fromACatalogueOfOnesOwn() {
	_, _ = hand.Of(1).Add(duration.Second.Of(1))
}

// A call through a func value, through an interface or into a closure does not
// say which function runs.
var source func() metrology.Measurement

type gauge interface {
	Read() metrology.Measurement
}

func throughAnIndirectCall(g gauge) {
	_, _ = source().Add(length.Metre.Of(1))
	_, _ = g.Read().Add(length.Metre.Of(1))
	_, _ = func() metrology.Measurement { return pressure.Bar.Of(1) }().Add(length.Metre.Of(1))
}

// A method value binds its receiver into a closure and calls the wrapper
// without it. Reading the operands off such a call by position would read the
// wrong ones, so the pass does not read them at all.
func throughAMethodValue() {
	add := pressure.Bar.Of(2.5).Add
	_, _ = add(length.Metre.Of(1))
}

// An unexported function carries no fact — a fact exists to cross a package
// boundary, and this one cannot.
func inlet() metrology.Measurement { return pressure.Bar.Of(2.5) }

func throughAnUnexportedFunction() {
	_, _ = inlet().Add(length.Metre.Of(1))
}

// A type of one's own, with methods the pass has no rules for.
type meter struct {
	last metrology.Measurement
}

func (m *meter) record(v metrology.Measurement) { m.last = v }

func (m meter) latest() metrology.Measurement { return m.last }

func throughAnotherTypesMethods() {
	var m meter
	m.record(pressure.Bar.Of(2.5))
	_, _ = m.latest().Add(length.Metre.Of(1))
}

// A unit read back off a measurement is a unit like any other, and the pass
// has no rule that says which one it is.
func throughTheUnitOfAMeasurement(m metrology.Measurement) {
	_, _ = length.Metre.Of(1).To(m.Unit())
}

// A power whose exponent is not a constant, and one outside the range a
// dimension exponent can hold: neither has a result to reason about.
func aPowerThatIsNotConstant(n int) {
	u, _ := length.Metre.Pow(n)
	_, _ = u.Of(1).Add(duration.Second.Of(1))
}

func aPowerOfAnUnknownUnit(u metrology.Unit) {
	_, _ = u.Pow(2)
}

func aPowerBeyondTheRange() {
	u, _ := length.Metre.Pow(200)
	_, _ = u.Of(1).Add(duration.Second.Of(1))
}

// A composition with an operand the pass cannot resolve is itself unresolved,
// on either side.
func composedWithAnUnknownUnit(other metrology.Unit) {
	per, _ := length.Metre.Per(other)
	_, _ = per.Of(1).Add(duration.Second.Of(1))

	times, _ := other.Times(length.Metre)
	_, _ = times.Of(1).Add(duration.Second.Of(1))
}

// Arithmetic on an operand the pass cannot resolve is equally unresolved.
func computedFromAnUnknown(m metrology.Measurement) {
	product, _ := m.Mul(duration.Second.Of(2))
	_, _ = product.Add(length.Metre.Of(1))

	sum, _ := m.Add(duration.Second.Of(2))
	_, _ = sum.Add(length.Metre.Of(1))
}

// The difference of two points is read on the interval unit the scale
// declares — K for °C — and which unit that is, is not in the table. The
// dimension is settled and the tag is not, so the pass says nothing about the
// result rather than guessing at it.
func theDifferenceOfTwoPointsIsNotResolved() {
	span, _ := temperature.Celsius.Of(25).Sub(temperature.Celsius.Of(20))
	_, _ = span.Add(length.Metre.Of(1))
}

// An untagged magnitude joins a tagged one, which is what makes a computed
// result nameable at all (D6).
func anUntaggedOperandGoesEitherWay() {
	rate, _ := ratio.One.Of(1).Div(duration.Second.Of(1))
	_, _ = frequency.Hertz.Of(50).Add(rate)
	_, _ = rate.To(frequency.Hertz)
}

// A plain function call is not a method call, and a call to something with no
// receiver is where the walk stops without a word.
func plainFunctionCalls() {
	_ = metrology.NewEngine(50)
	_ = dimension.Product(dimension.L, dimension.T)
}

// A tag a product dropped is followed only as far as it means something. It
// conflicts with a different tag on the same dimension (D16) and with nothing
// else: the becquerel converted onto the other unit of its own quantity is the
// ordinary case.
func aDroppedTagAgreesWithItsOwnQuantity() {
	scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
	_, _ = scaled.To(activity.Curie)
	_, _ = scaled.Add(activity.Becquerel.Of(1))
}

// Where the dimension changed, the tag is gone for good. A becquerel times a
// metre is a T⁻¹L¹ that names no quantity, and dividing the metre back out
// does not recover one.
func aDroppedTagDoesNotSurviveTheDimension() {
	spread, _ := activity.Becquerel.Of(5).Mul(length.Metre.Of(2))
	back, _ := spread.Div(length.Metre.Of(2))
	_, _ = back.Add(frequency.Hertz.Of(50))
}

// Two tagged operands that both survive the product with tags of their own and
// disagree leave no single answer, and no answer is the same as none: a plane
// angle times a solid angle is dimensionless and is neither.
func aProductOfTwoTagsHasNoProvenance() {
	product, _ := angle.Radian.Of(1).Mul(solidangle.Steradian.Of(1))
	_, _ = product.To(angle.Radian)
	_, _ = product.To(solidangle.Steradian)
}

// A variable of the program's own is its own business: the resolver never
// trusted it, so writing to it takes nothing away.
func assigningAVariableOfOnesOwn() {
	hand = length.Metre
	table["m"] = pressure.Bar
}
