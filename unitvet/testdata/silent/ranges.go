// The interval layer of D15, on the silent side. Not one line in this file may
// produce a diagnostic: the half of the contract that keeps the checker
// switched on.
package silent

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/ratio"
	"github.com/timzifer/metrology/units/temperature"
)

// The ordinary case: one dimension, two units, and every operation the kind
// rules of D6 allow.
func rangesThatAgree() {
	metres := uncertainty.Of(length.Metre.Of(1))
	kilometres := uncertainty.Of(length.Kilometre.Of(1))
	_, _ = metres.Add(kilometres)
	_, _ = metres.Sub(kilometres)
	_, _ = metres.Mul(kilometres)
	_, _ = metres.Div(kilometres)
	_, _ = metres.Pow(2)
	_, _ = metres.Pow(-3)
	_, _ = metres.To(length.Kilometre)
	_, _ = metres.Overlaps(kilometres)
	_, _ = metres.Mid()
	_, _ = metres.Width()
	_, _ = uncertainty.Between(length.Metre.Of(1), length.Metre.Of(2))
	_, _ = uncertainty.Symmetric(length.Metre.Of(1), length.Metre.Of(0.1))
}

// Every affine combination D6 allows, over ranges.
func absoluteRangesThatAgree() {
	warm := uncertainty.Of(temperature.Celsius.Of(20))
	step := uncertainty.Of(interval.Kelvin.Of(5))
	_, _ = warm.Add(step)
	_, _ = step.Add(warm)
	_, _ = warm.Sub(step)
	_, _ = warm.Sub(uncertainty.Of(temperature.Celsius.Of(15)))
	_, _ = warm.To(temperature.Fahrenheit)
	_, _ = warm.Overlaps(uncertainty.Of(temperature.Celsius.Of(21)))
	_, _ = uncertainty.Between(temperature.Celsius.Of(19), temperature.Celsius.Of(21))
	_, _ = uncertainty.Symmetric(temperature.Celsius.Of(20), interval.Kelvin.Of(0.5))
}

// The midpoint of an absolute range is a point on its own scale, so it meets a
// span along that scale and nothing is wrong with it.
func theMidpointIsOnTheScale() {
	mid, _ := uncertainty.Of(temperature.Celsius.Of(20)).Mid()
	_, _ = mid.Add(interval.Kelvin.Of(5))
}

// The width of an absolute range is read on the interval unit the scale
// declares — K for °C — and which unit that is, is not in the generated table.
// The dimension is settled, the unit is not, so there is no answer and the pass
// says nothing rather than guessing at one.
func theWidthOfAnAbsoluteRangeIsUnknown() {
	width, _ := uncertainty.Of(temperature.Celsius.Of(20)).Width()
	_, _ = width.Add(length.Metre.Of(1))
}

// A range chosen at run time is a phi node, and which of the two it is is not
// decided here.
func aRuntimeChoiceOfRange(fine bool) {
	r := uncertainty.Of(pressure.Bar.Of(1))
	if fine {
		r = uncertainty.Of(pressure.Pascal.Of(1))
	}
	_, _ = r.Add(uncertainty.Of(length.Metre.Of(1)))
}

// A range arriving as a parameter is on whichever scale the caller chose.
func aRangeFromAParameter(r uncertainty.Range) {
	_, _ = r.Add(uncertainty.Of(length.Metre.Of(1)))
	_, _ = r.To(pressure.Bar)
	_, _ = r.Pow(2)
	_, _ = uncertainty.Between(r.Lo(), r.Hi())
}

// A range read out of a container is not resolved either: the pass does not
// track what was put in.
func aRangeFromAContainer(m map[string]uncertainty.Range) {
	_, _ = m["inlet"].Add(uncertainty.Of(temperature.Celsius.Of(20)))
}

// A unit arriving as a parameter is unknown, so the range built on it is too.
func aRangeOnAnUnknownUnit(u metrology.Unit) {
	_, _ = uncertainty.Of(u.Of(1)).Add(uncertainty.Of(length.Metre.Of(1)))
}

// A power that is not a constant is a power the pass cannot compute.
func aRangePowerThatIsNotConstant(n int) {
	_, _ = uncertainty.Of(length.Metre.Of(1)).Pow(n)
}

// A power beyond the range of a dimension exponent (D5) has no result, so the
// operation that follows has no operand.
func aRangePowerBeyondTheRange() {
	area, _ := uncertainty.Of(length.Metre.Of(1)).Pow(200)
	_, _ = area.Add(uncertainty.Of(length.Metre.Of(1)))
}

// The provenance a product dropped is not a tag: converting the magnitude into
// a curie is legal and stays silent, and only a conflicting tag is reported
// (D16).
func aDroppedTagIsNotATagOverRanges() {
	scaled, _ := uncertainty.Of(activity.Becquerel.Of(5)).Mul(uncertainty.Of(length.Metre.Of(2)))
	_, _ = scaled.Div(uncertainty.Of(length.Metre.Of(1)))
}

// An untagged range meets a tagged one: the wildcard of D6 is what keeps a
// computed magnitude nameable.
func anUntaggedRangeMeetsATaggedOne() {
	perSecond, _ := uncertainty.Of(ratio.One.Of(1)).Div(uncertainty.Of(duration.Second.Of(1)))
	_, _ = perSecond.Add(uncertainty.Of(frequency.Hertz.Of(50)))
}

// A method value binds its receiver out of sight, and the pass proves nothing
// about a receiver it cannot see.
func aBoundRangeMethod() {
	add := uncertainty.Of(pressure.Bar.Of(1)).Add
	_, _ = add(uncertainty.Of(length.Metre.Of(1)))
}
