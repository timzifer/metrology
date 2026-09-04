package gum_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/temperature"
)

// of is Unit.OfString for a magnitude the test author wrote, so that a test
// reads as the numbers it is about rather than as error handling.
func of(t *testing.T, u metrology.Unit, magnitude string) metrology.Measurement {
	t.Helper()
	m, err := u.OfString(magnitude)
	if err != nil {
		t.Fatalf("%s %s: %v", magnitude, u, err)
	}
	return m
}

// input builds a value out of two magnitudes on one unit.
func input(t *testing.T, u metrology.Unit, estimate, uncertain string) gum.Value {
	t.Helper()
	v, err := gum.Standard(of(t, u, estimate), of(t, u, uncertain))
	if err != nil {
		t.Fatalf("Standard(%s, %s) %s: %v", estimate, uncertain, u, err)
	}
	return v
}

// combined is Value.Uncertainty for a test that is about the number.
func combined(t *testing.T, v gum.Value) string {
	t.Helper()
	u, err := v.Uncertainty()
	if err != nil {
		t.Fatalf("Uncertainty: %v", err)
	}
	return u.String()
}

// The property this package exists for, and the one the interval layer cannot
// have: a value minus itself is exactly zero, uncertainty and all, because the
// two contributions come from one input and cancel (D21).
//
// The same subtraction in the interval layer gives a width of twice the
// tolerance, and that is not a defect there — a Range does not know where its
// bounds came from. Both are asserted here, side by side, because the contrast
// is the reason there are two packages.
func TestAValueMinusItselfHasNoUncertainty(t *testing.T) {
	x := input(t, length.Metre, "2", "0.01")

	difference, err := x.Sub(x)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if got, want := difference.Estimate().String(), "0 m"; got != want {
		t.Errorf("estimate = %s, want %s", got, want)
	}
	if got, want := combined(t, difference), "0 m"; got != want {
		t.Errorf("uncertainty = %s, want %s", got, want)
	}

	// The interval layer, asked the same question about the same numbers.
	interval, err := uncertainty.Symmetric(of(t, length.Metre, "2"), of(t, length.Metre, "0.01"))
	if err != nil {
		t.Fatalf("Symmetric: %v", err)
	}
	spread, err := interval.Sub(interval)
	if err != nil {
		t.Fatalf("Range.Sub: %v", err)
	}
	width, err := spread.Width()
	if err != nil {
		t.Fatalf("Width: %v", err)
	}
	if got, want := width.String(), "0.04 m"; got != want {
		t.Errorf("the interval layer's width = %s, want %s — the contrast in D21", got, want)
	}
}

// The product rule, on the example every GUM introduction uses: an area from
// two lengths, each with its own uncertainty.
//
//	u(A)² = (W·u(L))² + (L·u(W))² = 5² + 5² = 50
func TestProductCombinesInQuadrature(t *testing.T) {
	l := input(t, length.Metre, "100", "0.1")
	w := input(t, length.Metre, "50", "0.05")

	area, err := l.Mul(w)
	if err != nil {
		t.Fatalf("Mul: %v", err)
	}
	if got, want := area.Estimate().String(), "5000 m²"; got != want {
		t.Errorf("estimate = %s, want %s", got, want)
	}
	if got, want := combined(t, area), "7.0710678118654752441 m²"; got != want {
		t.Errorf("uncertainty = %s, want %s (√50)", got, want)
	}

	rows := area.Contributions()
	if len(rows) != 2 {
		t.Fatalf("got %d contributions, want 2", len(rows))
	}
	for _, row := range rows {
		if got, want := row.Value.String(), "5 m²"; got != want {
			t.Errorf("contribution = %s, want %s", got, want)
		}
	}
}

// A value multiplied by itself is not two independent inputs. The contributions
// merge, and 2·L·u(L) comes out — the chain rule, arrived at without
// differentiating anything.
func TestSquareIsNotTwoIndependentFactors(t *testing.T) {
	l := input(t, length.Metre, "2", "0.01")

	square, err := l.Mul(l)
	if err != nil {
		t.Fatalf("Mul: %v", err)
	}
	if got, want := combined(t, square), "0.04 m²"; got != want {
		t.Errorf("uncertainty = %s, want %s (2·L·u)", got, want)
	}
	if rows := square.Contributions(); len(rows) != 1 {
		t.Errorf("got %d contributions, want the one input that produced them", len(rows))
	}

	power, err := l.Pow(2)
	if err != nil {
		t.Fatalf("Pow: %v", err)
	}
	if got, want := combined(t, power), combined(t, square); got != want {
		t.Errorf("Pow(2) gives %s and Mul gives %s", got, want)
	}
}

// Correlation is shared provenance and nothing else: two values built by
// Correlated draw on one source, and the sum of the two shows it.
//
// u(x + y)² = u(x)² + u(y)² + 2ρ·u(x)·u(y), so with u(x) = u(y) = 0.01 the
// sum runs from 0 at ρ = −1 to 0.02 at ρ = 1.
func TestCorrelatedInputsCombineByTheirCoefficient(t *testing.T) {
	for _, tc := range []struct {
		correlation string
		want        string
	}{
		{"1", "0.02 m"},
		{"0.5", "0.017320508075688772936 m"},
		{"0", "0.014142135623730950489 m"},
		{"-1", "0 m"},
	} {
		t.Run(tc.correlation, func(t *testing.T) {
			a := gum.Input{Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "0.01"), Name: "a"}
			b := gum.Input{Estimate: of(t, length.Metre, "2"), Uncertainty: of(t, length.Metre, "0.01"), Name: "b"}

			x, y, err := gum.Correlated(a, b, tc.correlation)
			if err != nil {
				t.Fatalf("Correlated: %v", err)
			}
			sum, err := x.Add(y)
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if got := combined(t, sum); got != tc.want {
				t.Errorf("u(x + y) = %s, want %s", got, tc.want)
			}
		})
	}
}

// Two values built independently are independent, whatever their names: the
// identity of an input is the source it was minted with and never the label.
func TestTwoInputsWithOneNameAreStillTwoInputs(t *testing.T) {
	first, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "0.01"), Name: "gauge",
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	second, err := gum.Of(gum.Input{
		Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "0.01"), Name: "gauge",
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}

	difference, err := first.Sub(second)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if got, want := combined(t, difference), "0.014142135623730950489 m"; got != want {
		t.Errorf("u = %s, want %s — two inputs that share a name do not share a source", got, want)
	}
	if rows := difference.Contributions(); len(rows) != 2 {
		t.Errorf("got %d contributions, want 2", len(rows))
	}
}

// An uncertainty is a span along a scale and never a point on it (D6), so a
// value on an absolute scale carries its uncertainty on the interval unit the
// scale declares — and a conversion moves the two differently.
func TestAnAbsoluteValueCarriesASpan(t *testing.T) {
	warm, err := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if got, want := warm.String(), "20 °C ± 0.3 K"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	fahrenheit, err := warm.To(temperature.Fahrenheit)
	if err != nil {
		t.Fatalf("To: %v", err)
	}
	if got, want := fahrenheit.String(), "68 °F ± 0.54 °R"; got != want {
		t.Errorf("got %s, want %s — the estimate converts as a point and the uncertainty as a span", got, want)
	}
}

// The zero value of a Source is not a source, and its accessors say so rather
// than panicking: a Contribution handed out by this package always carries a
// real one.
func TestSourceCarriesItsLabel(t *testing.T) {
	v, err := gum.Of(gum.Input{
		Estimate:    of(t, length.Metre, "1"),
		Uncertainty: of(t, length.Metre, "0.01"),
		Name:        "gauge block",
		Freedom:     4,
	})
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	row := v.Contributions()[0]
	if got, want := row.Source.Name(), "gauge block"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := row.Source.Freedom(), 4; got != want {
		t.Errorf("freedom = %d, want %d", got, want)
	}

	var none gum.Source
	if none.Name() != "" || none.Freedom() != gum.Infinite {
		t.Errorf("the zero Source reports %q and %d, want the empty name and Infinite",
			none.Name(), none.Freedom())
	}
}

// An exact value has no contributions at all, so it adds nothing to a budget it
// enters and appears in no row of one.
func TestExactlyHasNoContributions(t *testing.T) {
	constant := gum.Exactly(of(t, length.Metre, "2"))
	if rows := constant.Contributions(); len(rows) != 0 {
		t.Errorf("got %d contributions, want none", len(rows))
	}
	if got, want := combined(t, constant), "0 m"; got != want {
		t.Errorf("uncertainty = %s, want %s", got, want)
	}

	scaled, err := constant.Mul(input(t, duration.Second, "3", "0.1"))
	if err != nil {
		t.Fatalf("Mul: %v", err)
	}
	if got, want := combined(t, scaled), "0.2 m·s"; got != want {
		t.Errorf("uncertainty = %s, want %s", got, want)
	}
}

// D3, the aliasing guard, over this package's own values.
//
// Two hundred digits on purpose: apd/v3 inlines a small coefficient, so a
// shared slice is invisible below thirty-eight of them, and a test that used a
// short magnitude would pass on a library that corrupts every long one.
func TestValueHandsOutNoInteriorPointer(t *testing.T) {
	long := strings.Repeat("9", 200)
	v, err := gum.Standard(of(t, length.Metre, long), of(t, length.Metre, "0."+long))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	before := v.String()

	v.Estimate().Decimal().Coeff.SetInt64(1)
	v.Contributions()[0].Value.Decimal().Coeff.SetInt64(1)
	u, err := v.Uncertainty()
	if err != nil {
		t.Fatalf("Uncertainty: %v", err)
	}
	u.Decimal().Coeff.SetInt64(1)

	if after := v.String(); after != before {
		t.Errorf("the value changed under a write to a handed-out decimal:\n%s\n%s", before, after)
	}
}

// Every error this package reports itself, in one table. Everything else a
// caller can get wrong is a rule of the core, and the core reports it (D11).
func TestInputErrors(t *testing.T) {
	metre := of(t, length.Metre, "1")

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"a negative standard uncertainty", errorOf(gum.Standard(metre, of(t, length.Metre, "-0.1")))},
		{"negative degrees of freedom", errorOf(gum.Of(gum.Input{
			Estimate: metre, Uncertainty: of(t, length.Metre, "0.1"), Freedom: -1,
		}))},
		{"a sample of one observation", errorOf(gum.Sample("once", []metrology.Measurement{metre}))},
		{"a coverage factor below one", errorOfMeasurement(gum.FromExpanded(of(t, length.Metre, "0.2"), 0))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, gum.ErrInput) {
				t.Errorf("got %v, want ErrInput", tc.err)
			}
			var input *gum.InputError
			if !errors.As(tc.err, &input) {
				t.Errorf("got %v, want an *InputError", tc.err)
			}
		})
	}
}

// A correlation coefficient is text, exactly as a catalogue factor is (D4), and
// both ways of getting it wrong are reported apart.
func TestCorrelationIsCheckedBeforeItIsUsed(t *testing.T) {
	in := gum.Input{Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "0.01"), Name: "a"}

	if _, _, err := gum.Correlated(in, in, "warm"); !errors.Is(err, metrology.ErrSyntax) {
		t.Errorf("got %v, want ErrSyntax", err)
	}
	for _, correlation := range []string{"1.5", "-2", "Infinity"} {
		if _, _, err := gum.Correlated(in, in, correlation); !errors.Is(err, gum.ErrInput) {
			t.Errorf("a correlation of %s gave %v, want ErrInput", correlation, err)
		}
	}
	// A bad input is reported before the coefficient is used at all.
	bad := gum.Input{Estimate: of(t, length.Metre, "1"), Uncertainty: of(t, length.Metre, "-1")}
	if _, _, err := gum.Correlated(bad, in, "0.5"); !errors.Is(err, gum.ErrInput) {
		t.Errorf("got %v, want ErrInput for the first input", err)
	}
	if _, _, err := gum.Correlated(in, bad, "0.5"); !errors.Is(err, gum.ErrInput) {
		t.Errorf("got %v, want ErrInput for the second input", err)
	}
}

// errorOf and errorOfMeasurement drop the value of a two-result call, so that a
// table of failures reads as a table.
func errorOf(_ gum.Value, err error) error { return err }

func errorOfMeasurement(_ metrology.Measurement, err error) error { return err }

func errorOfFreedom(_ int, err error) error { return err }
