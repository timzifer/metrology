package gum_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/units/area"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/temperature"
	"github.com/timzifer/metrology/units/velocity"
)

// A quotient of a value by itself is exactly one, with no uncertainty at all —
// the other half of the dependency problem the interval layer cannot solve
// (D21). A quotient of two independent values combines their relative
// uncertainties instead.
func TestQuotient(t *testing.T) {
	x := input(t, length.Metre, "10", "0.1")

	unity, err := x.Div(x)
	if err != nil {
		t.Fatalf("Div: %v", err)
	}
	if got, want := unity.Estimate().String(), "1 m/m"; got != want {
		t.Errorf("estimate = %s, want %s", got, want)
	}
	if got, want := combined(t, unity), "0 m/m"; got != want {
		t.Errorf("uncertainty = %s, want %s", got, want)
	}

	// Two independent values: 1 % against 1 %, so √2 % of the quotient.
	y := input(t, duration.Second, "2", "0.02")
	speed, err := x.Div(y)
	if err != nil {
		t.Fatalf("Div: %v", err)
	}
	if got, want := speed.Estimate().String(), "5 m/s"; got != want {
		t.Errorf("estimate = %s, want %s", got, want)
	}
	if got, want := combined(t, speed), "0.070710678118654752441 m/s"; got != want {
		t.Errorf("uncertainty = %s, want %s (5·√2 %%)", got, want)
	}
}

// Every power, including the two that are not a repeated multiplication.
func TestPow(t *testing.T) {
	x := input(t, length.Metre, "2", "0.01")

	for _, tc := range []struct {
		n        int
		estimate string
		want     string
	}{
		{1, "2 m", "0.01 m"},
		{2, "4 m²", "0.04 m²"},
		{3, "8 m³", "0.12 m³"},
		{-1, "0.5 m⁻¹", "0.0025 m⁻¹"},
		{-2, "0.25 m⁻²", "0.0025 m⁻²"},
	} {
		t.Run(tc.estimate, func(t *testing.T) {
			p, err := x.Pow(tc.n)
			if err != nil {
				t.Fatalf("Pow(%d): %v", tc.n, err)
			}
			if got := p.Estimate().String(); got != tc.estimate {
				t.Errorf("estimate = %s, want %s", got, tc.estimate)
			}
			if got := combined(t, p); got != tc.want {
				t.Errorf("uncertainty = %s, want %s", got, tc.want)
			}
		})
	}

	// The zeroth power is one, and one is exact however uncertain x is.
	none, err := x.Pow(0)
	if err != nil {
		t.Fatalf("Pow(0): %v", err)
	}
	if got, want := none.Estimate().String(), "1 1"; got != want {
		t.Errorf("estimate = %s, want %s", got, want)
	}
	if got, want := combined(t, none), "0 1"; got != want {
		t.Errorf("uncertainty = %s, want %s", got, want)
	}
}

// A model this package cannot differentiate, with the derivatives supplied.
// Two lengths into an area, which the arithmetic could have done itself — so
// the answer is checkable against it.
func TestApply(t *testing.T) {
	l := input(t, length.Metre, "100", "0.1")
	w := input(t, length.Metre, "50", "0.05")

	applied, err := gum.Apply(
		area.SquareMetre.Of(5000),
		gum.Partial{Of: l, Derivative: length.Metre.Of(50)},
		gum.Partial{Of: w, Derivative: length.Metre.Of(100)},
	)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	multiplied, err := l.Mul(w)
	if err != nil {
		t.Fatalf("Mul: %v", err)
	}
	if got, want := combined(t, applied), combined(t, multiplied); got != want {
		t.Errorf("Apply gives %s where Mul gives %s", got, want)
	}

	// A model that names one input twice: the contributions merge rather than
	// counting the input twice over.
	twice, err := gum.Apply(
		length.Metre.Of(4),
		gum.Partial{Of: l, Derivative: metrology.MustUnit(metrology.UnitDef{
			Dimension: dimension.One, Symbol: symbol.Static("1"),
		}).Of(1)},
		gum.Partial{Of: l, Derivative: metrology.MustUnit(metrology.UnitDef{
			Dimension: dimension.One, Symbol: symbol.Static("1"),
		}).Of(1)},
	)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rows := twice.Contributions(); len(rows) != 1 {
		t.Errorf("got %d contributions, want the one input named twice", len(rows))
	}
	if got, want := combined(t, twice), "0.2 m"; got != want {
		t.Errorf("uncertainty = %s, want %s (two sensitivities of 1, not two inputs)", got, want)
	}
}

// A derivative whose unit does not carry the model's own is a dimension error,
// and the core is what says so — which is why a Partial holds a measurement and
// not a number.
func TestApplyRefusesADerivativeOfTheWrongDimension(t *testing.T) {
	x := input(t, length.Metre, "2", "0.01")

	_, err := gum.Apply(velocity.MetrePerSecond.Of(1), gum.Partial{Of: x, Derivative: length.Metre.Of(1)})
	if !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("got %v, want ErrDimension", err)
	}
	// And a point on a scale is not a sensitivity.
	_, err = gum.Apply(length.Metre.Of(1), gum.Partial{Of: x, Derivative: temperature.Celsius.Of(1)})
	if !errors.Is(err, metrology.ErrKind) {
		t.Errorf("got %v, want ErrKind", err)
	}
}

// The rules of the core are the rules of this layer, unchanged: it does not
// restate one, it delegates and reports what comes back (D21).
func TestTheCoreRefusesWhatItAlreadyRefused(t *testing.T) {
	metres := input(t, length.Metre, "1", "0.01")
	seconds := input(t, duration.Second, "1", "0.01")
	warm, err := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}

	if _, err := metres.Add(seconds); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("Add across dimensions gave %v, want ErrDimension", err)
	}
	if _, err := metres.Sub(seconds); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("Sub across dimensions gave %v, want ErrDimension", err)
	}
	//unitvet:ignore the assertion is that this addition fails
	if _, err := warm.Add(warm); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("two points added gave %v, want ErrKind", err)
	}
	if _, err := warm.Mul(metres); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("a point multiplied gave %v, want ErrKind", err)
	}
	//unitvet:ignore the assertion is that this power fails
	if _, err := warm.Pow(2); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("a point raised gave %v, want ErrKind", err)
	}
	//unitvet:ignore the assertion is that this power fails
	if _, err := warm.Pow(-1); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("a point inverted gave %v, want ErrKind", err)
	}
	if _, err := metres.To(duration.Second); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("a conversion across dimensions gave %v, want ErrDimension", err)
	}
	if _, err := metres.Div(gum.Exactly(length.Metre.Of(0))); err == nil {
		t.Error("a division by zero was accepted")
	}
}

// An arithmetic that cannot be carried out is reported, and every chain in this
// package reports the first failure rather than the last.
//
// The magnitudes are absurd on purpose: the exponent range of a decimal is what
// an ordinary budget never reaches, so a test that wants the failure path has to
// go and find it. An infinity is the other way in — one that a sensor with a
// disconnected input really does produce.
func TestAnImpossibleArithmeticIsReported(t *testing.T) {
	huge := input(t, length.Metre, "1E99999", "1E99999")
	wide := input(t, length.Metre, "1", "1E99999")
	tiny := input(t, length.Metre, "1E-99999", "0")
	unbounded, err := gum.Standard(length.Metre.Of(math.Inf(1)), length.Metre.Of(math.Inf(1)))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	dimensionless := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("1"),
	})

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"a difference of one input", errorOf(unbounded.Sub(unbounded))},
		{"a product", errorOf(huge.Mul(huge))},
		{"a quotient", errorOf(wide.Div(tiny))},
		{"a power", errorOf(huge.Pow(3))},
		{"a reciprocal", errorOf(tiny.Pow(-2))},
		{"a model whose sensitivity cannot be applied", errorOf(gum.Apply(
			length.Metre.Of(1),
			gum.Partial{Of: unbounded, Derivative: dimensionless.Of(0)},
		))},
		{"a model naming one input twice", errorOf(gum.Apply(
			length.Metre.Of(1),
			gum.Partial{Of: unbounded, Derivative: dimensionless.Of(1)},
			gum.Partial{Of: unbounded, Derivative: dimensionless.Of(-1)},
		))},
		{"a combination of two", errorOfMeasurement(combinationOf(t, huge, wide))},
		{"the degrees of freedom behind one", errorOfFreedom(freedomOf(t, "1E30000").EffectiveFreedom())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Error("an impossible arithmetic went through unreported")
			}
		})
	}
}

// combinationOf sums two values and asks for the uncertainty of the sum, so
// that the combination has two contributions to square rather than the one it
// reports back unchanged.
func combinationOf(t *testing.T, left, right gum.Value) (metrology.Measurement, error) {
	t.Helper()
	sum, err := left.Add(right)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return sum.Uncertainty()
}

// freedomOf builds a value whose fourth power is past the exponent range, for
// the one branch of the Welch-Satterthwaite formula that can fail.
func freedomOf(t *testing.T, magnitude string) gum.Value {
	t.Helper()
	v, err := gum.Of(gum.Input{
		Estimate:    of(t, length.Metre, "1"),
		Uncertainty: of(t, length.Metre, magnitude),
		Freedom:     4,
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	return v
}

// A conversion moves the contributions with the estimate.
func TestConversionMovesTheContributions(t *testing.T) {
	metres := input(t, length.Metre, "1500", "1.5")

	kilometres, err := metres.To(length.Kilometre)
	if err != nil {
		t.Fatalf("To: %v", err)
	}
	if got, want := kilometres.String(), "1.5 km ± 0.0015 km"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got, want := kilometres.Contributions()[0].Value.String(), "0.0015 km"; got != want {
		t.Errorf("contribution = %s, want %s", got, want)
	}

	// Two values on two units of one dimension add without the caller
	// converting first: the core picks the scale and the contributions follow.
	sum, err := metres.Add(kilometres)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got, want := sum.Estimate().String(), "3000 m"; got != want {
		t.Errorf("estimate = %s, want %s", got, want)
	}
	if got, want := combined(t, sum), "3 m"; got != want {
		t.Errorf("uncertainty = %s, want %s — one input, twice, so 2·1.5", got, want)
	}
}

// A caller's own catalogue may declare an interval unit this one cannot convert
// onto: two absolute scales that agree, whose spans carry conflicting quantity
// tags (D6). The estimate converts and the uncertainty does not, and the error
// is the core's.
func TestAConversionThatMovesThePointAndNotTheSpan(t *testing.T) {
	spanA := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Symbol: symbol.Static("K_a"), Quantity: "alpha",
	})
	spanB := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Symbol: symbol.Static("K_b"), Quantity: "beta",
	})
	scaleA := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.Static("°A"),
		Interval: &spanA,
	})
	scaleB := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.Static("°B"),
		Offset: "10", Interval: &spanB,
	})

	v, err := gum.Standard(scaleA.Of(20), spanA.Of(0.5))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if _, err := v.To(scaleB); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("got %v, want ErrQuantity from the span that cannot follow", err)
	}

	// The same disagreement reached through a sum, where the result's scale is
	// the right operand's.
	onB, err := gum.Standard(scaleB.Of(20), spanB.Of(0.5))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	spanOfA, err := gum.Standard(spanA.Of(1), spanA.Of(0.1))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if _, err := spanOfA.Add(onB); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("got %v, want ErrQuantity", err)
	}
	if _, err := onB.Add(spanOfA); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("got %v, want ErrQuantity", err)
	}
}
