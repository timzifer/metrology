// The propagation layer of D21, on the silent side. Not one line in this file
// may produce a diagnostic.
package silent

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/units/area"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/temperature"
)

// The ordinary case: one dimension, two units, and every operation D6 allows.
func valuesThatAgree() {
	metres := gum.Exactly(length.Metre.Of(1))
	kilometres := gum.Exactly(length.Kilometre.Of(1))
	_, _ = metres.Add(kilometres)
	_, _ = metres.Sub(kilometres)
	_, _ = metres.Mul(kilometres)
	_, _ = metres.Div(kilometres)
	_, _ = metres.Pow(2)
	_, _ = metres.Pow(-3)
	_, _ = metres.To(length.Kilometre)
	_, _ = metres.Uncertainty()
	_, _ = metres.Expanded(2)
	_, _ = metres.EffectiveFreedom()
	_ = metres.Estimate()
	_ = metres.Contributions()
	_ = metres.String()
}

// An input and its uncertainty, in every spelling the layer accepts.
func inputsThatAgree() {
	_, _ = gum.Standard(length.Metre.Of(1), length.Metre.Of(0.001))
	_, _ = gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	_, _ = gum.NewEngine(34).Standard(length.Metre.Of(1), length.Metre.Of(0.001))
	_, _ = gum.Rectangular(length.Metre.Of(0.0005))
	_, _ = gum.Triangular(length.Metre.Of(0.0005))
	_, _ = gum.UShaped(length.Metre.Of(0.0005))
	_, _ = gum.FromExpanded(length.Metre.Of(0.002), 2)
}

// A temperature and a span along its scale: the affine combination D6 allows,
// with the uncertainty on the interval unit the scale declares.
func absoluteValuesThatAgree() {
	warm, _ := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	span, _ := gum.Standard(interval.Kelvin.Of(5), interval.Kelvin.Of(0.1))
	_, _ = warm.Add(span)
	_, _ = warm.Sub(span)
	_, _ = warm.Sub(warm)
	_, _ = warm.To(temperature.Fahrenheit)
	_, _ = warm.Uncertainty()
}

// What the pass cannot resolve, it says nothing about (D13). A value built out
// of a struct literal, a slice or a correlated pair reads a container, and this
// checker does not follow one — so none of these lines is proven, and none is
// reported, however the dimensions inside them line up.
func valuesTheCheckerCannotResolve() {
	unnameable, _ := gum.Of(gum.Input{
		Estimate:    length.Metre.Of(1),
		Uncertainty: length.Metre.Of(0.001),
		Name:        "gauge",
		Freedom:     4,
	})
	_, _ = unnameable.Add(gum.Exactly(temperature.Celsius.Of(20)))

	sampled, _ := gum.Sample("repeat", []metrology.Measurement{
		length.Metre.Of(1), length.Metre.Of(1.1),
	})
	_, _ = sampled.Add(gum.Exactly(temperature.Celsius.Of(20)))

	first, second, _ := gum.Correlated(
		gum.Input{Estimate: length.Metre.Of(1), Uncertainty: length.Metre.Of(0.01)},
		gum.Input{Estimate: length.Metre.Of(2), Uncertainty: length.Metre.Of(0.02)},
		"0.5",
	)
	_, _ = first.Add(second)
}

// A model this package cannot differentiate: the estimate is resolvable, the
// partial derivatives are struct literals and are not.
func appliedModel() {
	x, _ := gum.Standard(length.Metre.Of(2), length.Metre.Of(0.01))
	square, _ := gum.Apply(
		area.SquareMetre.Of(4),
		gum.Partial{Of: x, Derivative: length.Metre.Of(4)},
	)
	_, _ = square.Uncertainty()
}
