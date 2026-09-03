package gum_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/temperature"
)

// The combined standard uncertainty rounds up, and the point of that is the
// last digit: a budget computed at twenty digits never reports less than the
// same budget computed at sixty.
func TestTheCombinationRoundsUp(t *testing.T) {
	narrow, err := gum.Engine{}.Add(input(t, length.Metre, "1", "1"), input(t, length.Metre, "1", "1"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	wide, err := gum.NewEngine(60).Add(input(t, length.Metre, "1", "1"), input(t, length.Metre, "1", "1"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	reported, err := narrow.Uncertainty()
	if err != nil {
		t.Fatalf("Uncertainty: %v", err)
	}
	reference, err := gum.NewEngine(60).Uncertainty(wide)
	if err != nil {
		t.Fatalf("Uncertainty: %v", err)
	}
	if reported.Decimal().Cmp(reference.Decimal()) < 0 {
		t.Errorf("the reported uncertainty %s is below the reference %s", reported, reference)
	}
	if got, want := reported.String(), "1.4142135623730950489 m"; got != want {
		t.Errorf("got %s, want %s (√2, rounded up in the last place)", got, want)
	}
}

// An expanded uncertainty leaves this package as an interval of the layer that
// already knows how to write one — and there is no way back, on purpose.
func TestExpanded(t *testing.T) {
	v := input(t, length.Metre, "10", "0.1")

	doubled, err := v.Expanded(2)
	if err != nil {
		t.Fatalf("Expanded: %v", err)
	}
	if got, want := doubled.String(), "[9.8, 10.2] m"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// A coverage factor from a table of the t-distribution is not an integer,
	// and the boundary of D10 takes it as it is written.
	student, err := v.Expanded(2.78)
	if err != nil {
		t.Fatalf("Expanded: %v", err)
	}
	if got, want := student.String(), "[9.722, 10.278] m"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// On an absolute scale the interval is a range of points and the coverage
	// a span, which is D6 and needs no clause of this package's own.
	warm, err := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	band, err := warm.Expanded(2)
	if err != nil {
		t.Fatalf("Expanded: %v", err)
	}
	if got, want := band.String(), "[19.4, 20.6] °C"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExpandedReportsWhatItCannotCompute(t *testing.T) {
	huge := input(t, length.Metre, "1E99999", "1E99999")
	pair, err := huge.Add(input(t, length.Metre, "1", "1E99999"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := pair.Expanded(2); err == nil {
		t.Error("a combination that overflows was expanded anyway")
	}

	// A coverage factor that is not a number reaches the core's arithmetic and
	// is refused there.
	if _, err := gum.Exactly(length.Metre.Of(1)).Expanded(math.Inf(1)); err == nil {
		t.Error("an infinite coverage factor was accepted")
	}
}

// The Welch-Satterthwaite formula, on numbers chosen so the answer can be
// checked by hand: u_c = 5, and 5⁴ / (3⁴/4 + 4⁴/9) is 12.83…, which truncates
// to twelve (JCGM 100 §G.4.1).
func TestEffectiveFreedom(t *testing.T) {
	a, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "10"), Uncertainty: of(t, length.Metre, "3"),
		Name: "a", Freedom: 4,
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	b, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "20"), Uncertainty: of(t, length.Metre, "4"),
		Name: "b", Freedom: 9,
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}

	sum, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got, want := combined(t, sum), "5 m"; got != want {
		t.Errorf("u = %s, want %s", got, want)
	}
	freedom, err := sum.EffectiveFreedom()
	if err != nil {
		t.Fatalf("EffectiveFreedom: %v", err)
	}
	if want := 12; freedom != want {
		t.Errorf("ν_eff = %d, want %d", freedom, want)
	}

	// A single Type A input keeps its own degrees of freedom.
	alone, err := a.EffectiveFreedom()
	if err != nil {
		t.Fatalf("EffectiveFreedom: %v", err)
	}
	if want := 4; alone != want {
		t.Errorf("ν_eff = %d, want %d", alone, want)
	}
}

// A budget of nothing but Type B inputs has no estimate of an estimate in it,
// so its effective degrees of freedom are infinite — and so is a budget whose
// one finite contribution is negligible beside the rest.
func TestEffectiveFreedomIsInfiniteWhereNothingWasEstimated(t *testing.T) {
	typeB := input(t, length.Metre, "1", "0.01")
	freedom, err := typeB.EffectiveFreedom()
	if err != nil {
		t.Fatalf("EffectiveFreedom: %v", err)
	}
	if freedom != gum.Infinite {
		t.Errorf("ν_eff = %d, want Infinite", freedom)
	}

	negligible, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "1E-30"),
		Name: "negligible", Freedom: 1,
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	mixed, err := typeB.Add(negligible)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	freedom, err = mixed.EffectiveFreedom()
	if err != nil {
		t.Fatalf("EffectiveFreedom: %v", err)
	}
	if freedom != gum.Infinite {
		t.Errorf("ν_eff = %d, want Infinite — past what an int holds is past what the table has", freedom)
	}
}

func TestEffectiveFreedomReportsWhatItCannotCompute(t *testing.T) {
	huge := input(t, length.Metre, "1E99999", "1E99999")
	pair, err := huge.Add(input(t, length.Metre, "1", "1E99999"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := pair.EffectiveFreedom(); err == nil {
		t.Error("a combination that overflows was given degrees of freedom anyway")
	}
}

// A value prints as its estimate and its combined uncertainty, with the unit on
// both sides — which is what tells it apart from the ± form of the interval
// layer, where one unit covers a range of two bounds.
func TestString(t *testing.T) {
	if got, want := input(t, length.Metre, "3.7", "0.05").String(), "3.7 m ± 0.05 m"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// A value whose uncertainty cannot be computed prints as its estimate
	// rather than as an arithmetic accident.
	huge := input(t, length.Metre, "1E99999", "1E99999")
	pair, err := huge.Add(input(t, length.Metre, "1", "1E99999"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got, want := pair.String(), pair.Estimate().String(); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// A Value answers the same questions about its scale that a Measurement does,
// because they are the estimate's answers (D21).
func TestValueDescribesItsScale(t *testing.T) {
	v := input(t, length.Metre, "1", "0.01")

	if !v.Unit().Equal(length.Metre) {
		t.Errorf("unit = %s, want m", v.Unit())
	}
	if got, want := v.Dimension(), dimension.L; got != want {
		t.Errorf("dimension = %s, want %s", got, want)
	}
	if got, want := v.Kind(), metrology.Interval; got != want {
		t.Errorf("kind = %s, want %s", got, want)
	}
	warm, err := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if got, want := warm.Kind(), metrology.Absolute; got != want {
		t.Errorf("kind = %s, want %s", got, want)
	}
	if got, want := warm.Quantity(), metrology.Quantity(""); got != want {
		t.Errorf("quantity = %q, want %q", got, want)
	}
}

// A scale whose declared interval unit it cannot convert onto has no span this
// library can name, and every entry point says so rather than producing a
// magnitude on a scale nobody chose.
//
// It takes a catalogue of the caller's own to build one: no unit in this
// library's catalogue tags a scale differently from the unit its differences
// are read on.
func TestABrokenScaleIsRefusedWhereItEnters(t *testing.T) {
	span := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Symbol: symbol.Static("K_b"), Quantity: "beta",
	})
	broken := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.Static("°A"),
		Quantity: "alpha", Interval: &span,
	})
	sound, err := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}

	if _, err := gum.Standard(broken.Of(20), span.Of(1)); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("Standard: got %v, want ErrQuantity", err)
	}
	if _, err := gum.Sample("broken", []metrology.Measurement{broken.Of(20), broken.Of(21)}); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("Sample: got %v, want ErrQuantity", err)
	}
	if _, err := gum.Apply(broken.Of(20)); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("Apply: got %v, want ErrQuantity", err)
	}
	if _, err := sound.To(broken); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("To: got %v, want ErrQuantity", err)
	}
	if _, err := gum.Exactly(interval.Kelvin.Of(1)).Add(gum.Exactly(broken.Of(20))); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("Add: got %v, want ErrQuantity", err)
	}

	// Exactly is the one constructor that cannot fail: it has no contribution
	// to express, so the estimate's own scale stands in for the span it cannot
	// name.
	constant := gum.Exactly(broken.Of(20))
	if got, want := constant.String(), "20 °A ± 0 °A"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// The remaining arithmetic that can fail on the way to a contribution rather
// than on the way to an estimate.
func TestAContributionThatCannotBeComputed(t *testing.T) {
	huge := input(t, length.Metre, "1E99999", "1E99999")
	unbounded, err := gum.Standard(length.Metre.Of(1), length.Metre.Of(math.Inf(1)))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}

	// The left operand contributes nothing and the right one overflows.
	if _, err := gum.Exactly(of(t, length.Metre, "1E99999")).Mul(huge); err == nil {
		t.Error("a product whose second operand overflows was accepted")
	}
	// A quotient of one input by itself, where the two contributions cancel
	// and cannot: ∞ − ∞ is not a number.
	if _, err := unbounded.Div(unbounded); err == nil {
		t.Error("a quotient of an unbounded value by itself was accepted")
	}
	// A power whose sensitivity is fine and whose contribution is not.
	wide, err := gum.Standard(of(t, length.Metre, "50"), of(t, length.Metre, "1E99999"))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if _, err := wide.Pow(2); err == nil {
		t.Error("a power whose contribution overflows was accepted")
	}
	// And a Type B divisor that underflows the exponent range.
	if _, err := gum.Rectangular(of(t, length.Metre, "1E-100000")); err == nil {
		t.Error("a half-width that underflows was accepted")
	}
	if _, err := gum.FromExpanded(of(t, length.Metre, "1E-100000"), 3); err == nil {
		t.Error("an expanded uncertainty that underflows was accepted")
	}
	// A correlation that cannot be applied: nothing times an infinity.
	unboundedInput := gum.Input{Estimate: of(t, length.Metre, "1"), Uncertainty: length.Metre.Of(math.Inf(1))}
	if _, _, err := gum.Correlated(unboundedInput, unboundedInput, "0"); err == nil {
		t.Error("a correlation of an unbounded input was accepted")
	}

	// A difference of one input whose contributions cannot cancel.
	if _, err := unbounded.Sub(unbounded); err == nil {
		t.Error("a difference of an unbounded value by itself was accepted")
	}

	// A product whose estimate is computable and whose contributions are not,
	// from either side.
	exact := gum.Exactly(of(t, length.Metre, "1E99999"))
	small, err := gum.Standard(of(t, length.Metre, "1"), of(t, length.Metre, "1E99999"))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if _, err := small.Mul(exact); err == nil {
		t.Error("a product whose first operand's contribution overflows was accepted")
	}
	if _, err := exact.Mul(small); err == nil {
		t.Error("a product whose second operand's contribution overflows was accepted")
	}

	// A product of two values that share an input, whose contributions have
	// opposite signs and cannot be added.
	mirrored, err := unbounded.Sub(gum.Exactly(of(t, length.Metre, "2")))
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if _, err := unbounded.Mul(mirrored); err == nil {
		t.Error("a product of two values with one unbounded input was accepted")
	}

	// An uncertainty of the wrong dimension is refused where it enters.
	if _, err := gum.Standard(of(t, length.Metre, "1"), of(t, duration.Second, "1")); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("got %v, want ErrDimension", err)
	}

	// And degrees of freedom whose ratio is past the exponent range.
	dominant, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "1E20000"),
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	negligible, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "1E-20000"), Freedom: 1,
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	lopsided, err := dominant.Add(negligible)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := lopsided.EffectiveFreedom(); err == nil {
		t.Error("degrees of freedom past the exponent range were reported as a number")
	}
}

// Two absolute scales whose interval units disagree about the quantity they
// measure: the difference of two points is nameable on one of them and the
// other's contributions cannot follow it there. Which operand is refused
// depends on which scale the difference is read on, so both orders are here.
func TestContributionsThatCannotFollowTheDifference(t *testing.T) {
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

	onA, err := gum.Standard(scaleA.Of(20), spanA.Of(0.5))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	onB, err := gum.Standard(scaleB.Of(20), spanB.Of(0.5))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}

	if _, err := onA.Sub(onB); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("got %v, want ErrQuantity from the right operand", err)
	}
	if _, err := onB.Sub(onA); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("got %v, want ErrQuantity from the left operand", err)
	}
}
