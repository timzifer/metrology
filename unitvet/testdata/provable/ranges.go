// The interval layer of D15. A range added to a range of another dimension is
// exactly the provable class this checker exists for, and the rules that decide
// it are the ones already here: the dimension, the quantity and the kind of a
// range are those of its bounds.
package provable

import (
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

// The same mistake as addAcrossDimensions, one layer up.
func addRangesAcrossDimensions() {
	p := uncertainty.Of(pressure.Bar.Of(2.5))
	t := uncertainty.Of(temperature.Celsius.Of(20))
	_, _ = p.Add(t) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹`
}

func subRangesAcrossDimensions() {
	_, _ = uncertainty.Of(length.Metre.Of(1)).Sub(uncertainty.Of(duration.Second.Of(1))) // want `Sub on incompatible dimensions: L¹ and T¹`
}

// Two points on a scale have no sum, and a range of them is still two points.
func addTwoAbsoluteRanges() {
	warm := uncertainty.Of(temperature.Celsius.Of(20))
	_, _ = warm.Add(warm) // want `Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it`
}

func subAnAbsoluteRangeFromASpan() {
	_, _ = uncertainty.Of(interval.Kelvin.Of(5)).Sub(uncertainty.Of(temperature.Celsius.Of(20))) // want `Sub on incompatible kinds: interval and absolute; a point on a scale cannot be subtracted from a span along it`
}

// A point on a scale has no product, whether it arrives alone or as a range.
func multiplyAnAbsoluteRange() {
	_, _ = uncertainty.Of(temperature.Celsius.Of(20)).Mul(uncertainty.Of(length.Metre.Of(2))) // want `Mul on incompatible kinds: absolute and interval; a point on a scale has no product`
}

func divideByAnAbsoluteRange() {
	_, _ = uncertainty.Of(length.Metre.Of(2)).Div(uncertainty.Of(temperature.Celsius.Of(20))) // want `Div on incompatible kinds: interval and absolute; a point on a scale has no product`
}

func powerOfAnAbsoluteRange() {
	_, _ = uncertainty.Of(temperature.Celsius.Of(20)).Pow(2) // want `Pow on an absolute unit; a point on a scale has no power`
}

// A conversion of a range is a conversion of both its bounds, and it lands
// where a conversion of a measurement lands.
func convertARangeAcrossDimensions() {
	_, _ = uncertainty.Of(pressure.Bar.Of(2.5)).To(length.Metre) // want `To on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

// A hertz and a becquerel are both T⁻¹ and are not the same measurement (D16).
func convertARangeAcrossQuantities() {
	_, _ = uncertainty.Of(frequency.Hertz.Of(50)).To(activity.Becquerel) // want `To on incompatible quantities: frequency and radioactivity`
}

// Overlaps compares two ranges, so it is refused where a comparison is.
func overlapAcrossDimensions() {
	_, _ = uncertainty.Of(length.Metre.Of(1)).Overlaps(uncertainty.Of(pressure.Bar.Of(1))) // want `Overlaps on incompatible dimensions: L¹ and L⁻¹M¹T⁻²`
}

func overlapAcrossKinds() {
	_, _ = uncertainty.Of(temperature.Celsius.Of(20)).Overlaps(uncertainty.Of(interval.Kelvin.Of(5))) // want `Overlaps on incompatible kinds: absolute and interval; a point on a scale and a span along it are not comparable`
}

// A range holds one scale, so two bounds of different dimensions never make
// one — and this is the constructor, not an operation on a range.
func betweenAcrossDimensions() {
	_, _ = uncertainty.Between(length.Metre.Of(1), pressure.Bar.Of(2)) // want `Between on incompatible dimensions: L¹ and L⁻¹M¹T⁻²`
}

func betweenAcrossKinds() {
	_, _ = uncertainty.Between(temperature.Celsius.Of(19), interval.Kelvin.Of(21)) // want `Between on incompatible kinds: absolute and interval; a point on a scale and a span along it are not comparable`
}

// A tolerance is a distance along a scale and never a place on it.
func symmetricWithAnAbsoluteTolerance() {
	_, _ = uncertainty.Symmetric(temperature.Celsius.Of(20), temperature.Celsius.Of(0.5)) // want `Symmetric on incompatible kinds: absolute and absolute; a tolerance is a span along a scale, not a point on it`
}

func symmetricAcrossDimensions() {
	_, _ = uncertainty.Symmetric(length.Metre.Of(1), duration.Second.Of(1)) // want `Symmetric on incompatible dimensions: L¹ and T¹`
}

// The one rule that outlives the run time (D16), now over ranges: a product
// drops the quantity tag, and the checker keeps the provenance the run time no
// longer has.
func aDroppedTagStillConflictsForRanges() {
	scaled, _ := uncertainty.Of(activity.Becquerel.Of(5)).Mul(uncertainty.Of(ratio.One.Of(2)))
	_, _ = scaled.Add(uncertainty.Of(frequency.Hertz.Of(50))) // want `Add on incompatible quantities: a magnitude computed from radioactivity and frequency; Mul and Div drop the tag \(D6\), so the run time no longer sees the conflict`
}

// The bounds of a range are measurements on its scale, so a mistake made with
// one is the mistake the pass already reports.
func aBoundIsAMeasurement() {
	_, _ = uncertainty.Of(pressure.Bar.Of(2.5)).Lo().Add(length.Metre.Of(1)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

// The width of a span is a span on the same scale.
func aWidthIsASpan() {
	w, _ := uncertainty.Of(length.Metre.Of(1)).Width()
	_, _ = w.Add(duration.Second.Of(1)) // want `Add on incompatible dimensions: L¹ and T¹`
}

// The engine is the same operation at another precision, and its operands sit
// at the same two positions.
func throughAnEngine() {
	e := uncertainty.NewEngine(34)
	_, _ = e.Add(uncertainty.Of(pressure.Bar.Of(1)), uncertainty.Of(length.Metre.Of(1))) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
	_, _ = e.Pow(uncertainty.Of(temperature.Celsius.Of(20)), 2)                          // want `Pow on an absolute unit; a point on a scale has no power`
}

// A power of a range composes exactly as a power of a unit does, so a mistake
// made with the result is provable too.
func aPowerOfARange() {
	area, _ := uncertainty.Of(length.Metre.Of(2)).Pow(2)
	_, _ = area.Add(uncertainty.Of(length.Metre.Of(1))) // want `Add on incompatible dimensions: L² and L¹`
}

// A range whose bounds disagree is no range at all. The constructor is where
// that is reported, and what follows has no operand to be wrong about — one
// mistake, one diagnostic, rather than the same mistake reported at every
// operation downstream of it.
func aRangeThatWasNeverBuilt() {
	r, _ := uncertainty.Between(length.Metre.Of(1), pressure.Bar.Of(2)) // want `Between on incompatible dimensions: L¹ and L⁻¹M¹T⁻²`
	_, _ = r.Add(uncertainty.Of(duration.Second.Of(1)))

	s, _ := uncertainty.Symmetric(temperature.Celsius.Of(20), temperature.Celsius.Of(1)) // want `Symmetric on incompatible kinds: absolute and absolute; a tolerance is a span along a scale, not a point on it`
	_, _ = s.Add(uncertainty.Of(duration.Second.Of(1)))
}
