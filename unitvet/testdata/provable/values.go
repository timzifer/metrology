// The propagation layer of D21. A value added to a value of another dimension
// is the same provable class as a range added to a range, and it is decided by
// the same rules: the dimension, the quantity and the kind of a gum.Value are
// its estimate's.
package provable

import (
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
)

// The same mistake as addRangesAcrossDimensions, one layer further up.
func addValuesAcrossDimensions() {
	p := gum.Exactly(pressure.Bar.Of(2.5))
	t := gum.Exactly(temperature.Celsius.Of(20))
	_, _ = p.Add(t) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹`
}

// A difference of two values of different dimensions.
func subtractValuesAcrossDimensions() {
	_, _ = gum.Exactly(length.Metre.Of(1)).Sub(gum.Exactly(duration.Second.Of(1))) // want `Sub on incompatible dimensions: L¹ and T¹`
}

// The affine rules reach the values too: two points do not add.
func addTwoAbsoluteValues() {
	warm := gum.Exactly(temperature.Celsius.Of(20))
	_, _ = warm.Add(warm) // want `Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it`
}

// A point on a scale has no product, whichever layer asks for one.
func multiplyAnAbsoluteValue() {
	_, _ = gum.Exactly(temperature.Celsius.Of(20)).Mul(gum.Exactly(length.Metre.Of(2))) // want `Mul on incompatible kinds: absolute and interval; a point on a scale has no product`
}

// And no power.
func raiseAnAbsoluteValue() {
	_, _ = gum.Exactly(temperature.Celsius.Of(20)).Pow(2) // want `Pow on an absolute unit; a point on a scale has no power`
}

// A conversion across dimensions.
func convertAValueAcrossDimensions() {
	_, _ = gum.Exactly(pressure.Bar.Of(2.5)).To(length.Metre) // want `To on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
}

// And across two quantities that share a dimension (D6).
func convertAValueAcrossQuantities() {
	_, _ = gum.Exactly(frequency.Hertz.Of(50)).To(activity.Becquerel) // want `To on incompatible quantities: frequency and radioactivity`
}

// A standard uncertainty is a span along a scale, never a point on it — the
// rule Symmetric already gets, for the constructor that has the same shape.
func standardWithAnAbsoluteUncertainty() {
	_, _ = gum.Standard(temperature.Celsius.Of(20), temperature.Celsius.Of(0.3)) // want `Standard on incompatible kinds: absolute and absolute; a tolerance is a span along a scale, not a point on it`
}

// And it has to be an uncertainty of the same quantity.
func standardAcrossDimensions() {
	_, _ = gum.Standard(length.Metre.Of(1), duration.Second.Of(0.1)) // want `Standard on incompatible dimensions: L¹ and T¹`
}

// The engine form is the same call with the precision written down.
func standardOnAnEngine() {
	_, _ = gum.NewEngine(34).Standard(length.Metre.Of(1), interval.Kelvin.Of(1)) // want `Standard on incompatible dimensions: L¹ and Θ¹`
}
