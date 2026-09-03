// Package provable holds one function per pattern the checker of D13 says it
// can decide. Every diagnostic below is one the pass must produce.
package provable

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/area"
	"github.com/timzifer/metrology/units/dose"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/force"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/ratio"
	"github.com/timzifer/metrology/units/temperature"
)

// A pressure and a temperature have nothing in common but the syntax.
func addAcrossDimensions() {
	_, _ = pressure.Bar.Of(2.5).Add(temperature.Celsius.Of(20)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹`
}

// The same through local variables: the operands are SSA registers, and the
// walk follows them back to the unit they were built on.
func subThroughLocals() {
	p := pressure.Bar.Of(2.5)
	t := temperature.Celsius.Of(20)
	_, _ = p.Sub(t) // want `Sub on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹`
}

// The affine rules of D6 are decidable wherever the units are.
func addTwoPoints() {
	_, _ = temperature.Celsius.Of(20).Add(temperature.Celsius.Of(5)) // want `Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it`
}

func subPointFromSpan() {
	_, _ = interval.Kelvin.Of(5).Sub(temperature.Celsius.Of(20)) // want `Sub on incompatible kinds: interval and absolute; a point on a scale cannot be subtracted from a span along it`
}

func multiplyAPoint() {
	_, _ = temperature.Celsius.Of(20).Mul(duration.Second.Of(2)) // want `Mul on incompatible kinds: absolute and interval; a point on a scale has no product`
}

func divideByAPoint() {
	_, _ = duration.Second.Of(2).Div(temperature.Celsius.Of(20)) // want `Div on incompatible kinds: interval and absolute; a point on a scale has no product`
}

// The hertz and the becquerel are both T⁻¹ and are not the same thing (D6).
func convertAcrossQuantities() {
	_, _ = frequency.Hertz.Of(50).To(activity.Becquerel) // want `To on incompatible quantities: frequency and radioactivity`
}

func addAcrossQuantities() {
	_, _ = frequency.Hertz.Of(50).Add(activity.Becquerel.Of(1)) // want `Add on incompatible quantities: frequency and radioactivity`
}

// A conversion is checked against its target, whichever of the three ways of
// asking for one is used.
func convertAcrossDimensions() {
	_, _ = pressure.Bar.Of(2.5).To(length.Metre) // want `To on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

func readAcrossDimensions() {
	_, _ = pressure.Bar.Of(2.5).In[float64](length.Metre) // want `In on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

func readDecimalAcrossDimensions() {
	_, _ = pressure.Bar.Of(2.5).DecimalIn(length.Metre) // want `DecimalIn on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

// A point and a span are not the same quantity, whichever way round.
func convertPointToSpan() {
	_, _ = temperature.Celsius.Of(20).To(interval.Kelvin) // want `To on incompatible kinds: absolute and interval; a point on a scale and a span along it are not the same quantity`
}

func compareAcrossDimensions() {
	_, _ = pressure.Bar.Of(2.5).Cmp(length.Metre.Of(1)) // want `Cmp on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

func comparePointWithSpan() {
	_, _ = temperature.Celsius.Of(20).Cmp(interval.Kelvin.Of(5)) // want `Cmp on incompatible kinds: absolute and interval; a point on a scale and a span along it are not comparable`
}

// The same operations on an engine, where both operands are arguments (D9).
func engineArithmetic() {
	e := metrology.NewEngine(50)
	_, _ = e.Add(pressure.Bar.Of(2.5), temperature.Celsius.Of(20))  // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹`
	_, _ = e.To(pressure.Bar.Of(2.5), length.Metre)                 // want `To on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
	_, _ = e.Mul(temperature.Celsius.Of(20), duration.Second.Of(2)) // want `Mul on incompatible kinds: absolute and interval; a point on a scale has no product`
}

// Composing units is checked with the same rules as composing magnitudes.
func composeAPoint() {
	_, _ = temperature.Celsius.Times(duration.Second) // want `Times on incompatible kinds: absolute and interval; a point on a scale has no product`
	_, _ = temperature.Celsius.Per(duration.Second)   // want `Per on incompatible kinds: absolute and interval; a point on a scale has no product`
	_, _ = temperature.Celsius.Pow(2)                 // want `Pow on an absolute unit; a point on a scale has no power`
}

// A computed magnitude carries the dimension its operands gave it, so the walk
// does not stop at the arithmetic.
func throughAQuotient() {
	q, _ := force.Newton.Of(100).Div(area.SquareMetre.Of(2))
	_, _ = q.Add(length.Metre.Of(1)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

func throughAProduct() {
	work, _ := force.Newton.Of(100).Mul(length.Metre.Of(2))
	_, _ = work.To(length.Metre) // want `To on incompatible dimensions: L²M¹T⁻² and L¹`
}

func throughASum() {
	total, _ := length.Metre.Of(1).Add(length.Metre.Of(2))
	_, _ = total.Add(duration.Second.Of(1)) // want `Add on incompatible dimensions: L¹ and T¹`
}

func throughADifference() {
	span, _ := length.Metre.Of(3).Sub(length.Metre.Of(1))
	_, _ = span.Cmp(duration.Second.Of(1)) // want `Cmp on incompatible dimensions: L¹ and T¹`
}

func throughAConversion() {
	p, _ := pressure.Bar.Of(2.5).To(pressure.Pascal)
	_, _ = p.Add(length.Metre.Of(1)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

// A point plus a span is a point, and it stays one for the next operation.
func throughAnAffineSum() {
	t, _ := temperature.Celsius.Of(20).Add(interval.Kelvin.Of(5))
	_, _ = t.Add(temperature.Celsius.Of(1)) // want `Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it`
}

func throughAnAffineDifference() {
	t, _ := temperature.Celsius.Of(25).Sub(interval.Kelvin.Of(5))
	_, _ = t.Mul(duration.Second.Of(2)) // want `Mul on incompatible kinds: absolute and interval; a point on a scale has no product`
}

// A unit built out of other units carries the dimension the composition gave
// it, and a magnitude on it is checked like any other.
func throughAComposedUnit() {
	speed, _ := length.Metre.Per(duration.Second)
	_, _ = speed.Of(3).Add(length.Metre.Of(1)) // want `Add on incompatible dimensions: L¹T⁻¹ and L¹`

	volume, _ := length.Metre.Pow(3)
	_, _ = volume.Of(2).To(area.SquareMetre) // want `To on incompatible dimensions: L³ and L²`

	square, _ := length.Metre.Times(length.Metre)
	_, _ = square.Of(2).Add(duration.Second.Of(1)) // want `Add on incompatible dimensions: L² and T¹`
}

// A magnitude read out of a string is on the unit that read it.
func throughOfString() {
	p, _ := pressure.Bar.OfString("2.5")
	_, _ = p.Add(length.Metre.Of(1)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

// An untagged operand takes the tag of the one that has it, so the conflict
// appears at the second step rather than the first.
func throughAnUntaggedSum() {
	sum, _ := frequency.Hertz.Of(50).Add(frequency.Hertz.Of(10))
	_, _ = sum.To(activity.Becquerel) // want `To on incompatible quantities: frequency and radioactivity`
}

// An operation the rules forbid has no result: it returns the zero
// Measurement. The mistake is therefore reported once, at its source, and does
// not cascade into every operation downstream of it.
func aForbiddenSumDoesNotCascade() {
	sum, _ := pressure.Bar.Of(1).Add(length.Metre.Of(1)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
	_, _ = sum.Add(duration.Second.Of(1))
}

func aForbiddenQuantitySumDoesNotCascade() {
	sum, _ := frequency.Hertz.Of(50).Add(activity.Becquerel.Of(1)) // want `Add on incompatible quantities: frequency and radioactivity`
	_, _ = sum.Add(length.Metre.Of(1))
}

func aForbiddenAffineSumDoesNotCascade() {
	sum, _ := temperature.Celsius.Of(20).Add(temperature.Celsius.Of(5)) // want `Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it`
	_, _ = sum.Add(length.Metre.Of(1))
}

func aForbiddenDifferenceDoesNotCascade() {
	d, _ := interval.Kelvin.Of(5).Sub(temperature.Celsius.Of(20)) // want `Sub on incompatible kinds: interval and absolute; a point on a scale cannot be subtracted from a span along it`
	_, _ = d.Add(length.Metre.Of(1))
}

func aForbiddenProductDoesNotCascade() {
	p, _ := temperature.Celsius.Of(20).Mul(duration.Second.Of(2)) // want `Mul on incompatible kinds: absolute and interval; a point on a scale has no product`
	_, _ = p.Add(length.Metre.Of(1))
}

func aForbiddenPowerDoesNotCascade() {
	u, _ := temperature.Celsius.Pow(2) // want `Pow on an absolute unit; a point on a scale has no power`
	_, _ = u.Of(1).Add(duration.Second.Of(1))
}

// A computed magnitude has no tag, so it takes on the tag of the operand that
// has one — and carries it into the next operation, where it is what makes the
// conflict visible (D6).
func anUntaggedSumTakesTheTag() {
	rate, _ := ratio.One.Of(1).Div(duration.Second.Of(1))
	sum, _ := rate.Add(frequency.Hertz.Of(50))
	_, _ = sum.To(activity.Becquerel) // want `To on incompatible quantities: frequency and radioactivity`
}

// A product drops the quantity tag (D6), so the run time accepts what follows
// and this pass does not: scaling a becquerel by a plain number leaves a T⁻¹
// that is still a radioactivity, whatever the arithmetic has forgotten (D16).
func aDroppedTagStillConflicts() {
	scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
	_, _ = scaled.Add(frequency.Hertz.Of(50)) // want `Add on incompatible quantities: a magnitude computed from radioactivity and frequency; Mul and Div drop the tag \(D6\), so the run time no longer sees the conflict`
}

func aDroppedTagConflictsWithAConversion() {
	scaled, _ := activity.Becquerel.Of(5).Div(ratio.One.Of(2))
	_, _ = scaled.To(frequency.Hertz) // want `To on incompatible quantities: a magnitude computed from radioactivity and frequency; Mul and Div drop the tag \(D6\), so the run time no longer sees the conflict`
}

// The same through the composition of the units themselves, and with the tag
// on the far side of the comparison.
func aDroppedTagSurvivesUnitComposition() {
	per, _ := activity.Becquerel.Per(ratio.One)
	_, _ = frequency.Hertz.Of(50).Cmp(per.Of(5)) // want `Cmp on incompatible quantities: frequency and a magnitude computed from radioactivity; Mul and Div drop the tag \(D6\), so the run time no longer sees the conflict`
}

// Provenance travels through a sum, which neither drops a tag nor restores
// one.
func aDroppedTagSurvivesASum() {
	scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
	sum, _ := scaled.Add(scaled)
	_, _ = sum.To(frequency.Hertz) // want `To on incompatible quantities: a magnitude computed from radioactivity and frequency; Mul and Div drop the tag \(D6\), so the run time no longer sees the conflict`
}

// A catalogue unit is a package-level variable, and Go lets an importer assign
// to it. The resolver trusts it by name, so the write is what makes the table
// untrue — reported where it happens, not at the uses it invalidates.
func assigningACatalogueUnit() {
	dose.Sievert = length.Metre // want `dose.Sievert is assigned; the generated table no longer describes it, and every unit resolved through it is unproven`
}
