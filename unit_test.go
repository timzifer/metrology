package metrology_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

func TestNewUnitDefaults(t *testing.T) {
	u, err := metrology.NewUnit(metrology.UnitDef{Dimension: dimension.L, Symbol: symbol.SI("m")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := u.Factor()
	if f.Num.String() != "1" || f.Den.String() != "1" || f.Pi != 0 {
		t.Errorf("factor = %s/%s·π^%d, want 1/1·π^0", f.Num, f.Den, f.Pi)
	}
	if got := u.Offset(); got.String() != "0" {
		t.Errorf("offset = %s, want 0", got)
	}
	if u.Kind() != metrology.Interval {
		t.Errorf("kind = %s, want interval", u.Kind())
	}
	if _, ok := u.IntervalUnit(); ok {
		t.Error("a unit declared without an interval counterpart reports one")
	}
	if u.String() != "m" || u.Dimension() != dimension.L {
		t.Errorf("unit = %s of %s", u, u.Dimension())
	}
	if !u.Symbol().Equal(symbol.SI("m")) {
		t.Error("Symbol() is not the symbol it was built with")
	}
}

func TestNewUnitRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		def  metrology.UnitDef
		want error
	}{
		{"a numerator that is not a number",
			metrology.UnitDef{Numerator: "one"}, metrology.ErrSyntax},
		{"a denominator that is not a number",
			metrology.UnitDef{Denominator: "760ish"}, metrology.ErrSyntax},
		{"an offset that is not a number",
			metrology.UnitDef{Kind: metrology.Absolute, Offset: "freezing"}, metrology.ErrSyntax},
		// A zero numerator collapses every magnitude to zero, a zero
		// denominator converts nothing at all. Neither is a unit.
		{"a zero numerator", metrology.UnitDef{Numerator: "0"}, metrology.ErrZeroFactor},
		{"a zero denominator", metrology.UnitDef{Denominator: "0"}, metrology.ErrZeroFactor},
		// D6: an offset is what makes a scale affine, and an affine scale
		// measures points, not spans.
		{"an offset on an interval unit",
			metrology.UnitDef{Offset: "273.15"}, metrology.ErrOffsetKind},
		{"an absolute unit as the interval counterpart",
			metrology.UnitDef{Dimension: dimension.Θ, Kind: metrology.Absolute, Interval: &Celsius},
			metrology.ErrOffsetKind},
		{"an interval counterpart of another dimension",
			metrology.UnitDef{Dimension: dimension.Θ, Kind: metrology.Absolute, Interval: &Metre},
			metrology.ErrDimension},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := metrology.NewUnit(tc.def); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMustUnitPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustUnit accepted a zero denominator")
		}
		if msg, ok := r.(string); !ok || msg == "" {
			t.Errorf("panic value = %v, want a message", r)
		}
	}()
	metrology.MustUnit(metrology.UnitDef{Denominator: "0"})
}

func TestUnitEqual(t *testing.T) {
	half := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.L, Symbol: symbol.Static("half"), Numerator: "1", Denominator: "2",
	})
	fiveTenths := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.L, Symbol: symbol.Static("half"), Numerator: "5", Denominator: "10",
	})
	for _, tc := range []struct {
		name string
		a, b metrology.Unit
		want bool
	}{
		{"a unit equals itself", Bar, Bar, true},
		// The scale comparison reads the decimals' pointers before it reads
		// their values, so the case that matters is the one where the pointers
		// differ and the scale does not: an independently built bar is the same
		// scale as the catalogue's, and a shortcut that answered from identity
		// alone would say otherwise.
		{"a unit equals an independently built copy", Bar, metrology.MustUnit(metrology.UnitDef{
			Dimension: Bar.Dimension(), Symbol: symbol.Static("bar"), Numerator: "100000",
		}), true},
		// The factor is compared as a number, not digit by digit: 1/2 and 5/10
		// are the same scale, however the catalogue happens to write them.
		{"the same ratio written differently", half, fiveTenths, true},
		{"different factors", Bar, Pascal, false},
		{"different dimensions", Metre, Pascal, false},
		{"different symbols", Bar, metrology.MustUnit(metrology.UnitDef{
			Dimension: Bar.Dimension(), Symbol: symbol.Static("b"), Numerator: "100000",
		}), false},
		{"different kinds", Kelvin, KelvinAbsolute, false},
		{"different offsets", Celsius, metrology.MustUnit(metrology.UnitDef{
			Dimension: Celsius.Dimension(), Kind: metrology.Absolute,
			Symbol: symbol.Static("°C"), Offset: "273.16",
		}), false},
		// Equal reports that two units are the same scale, not that either is
		// a usable one: two zero Units are the same absence of a scale, and
		// saying so is what uncertainty.Between relies on for a zero Range.
		{"the zero unit is the same scale as itself", metrology.Unit{}, metrology.Unit{}, true},
		// Unnamed agrees with the zero Unit on dimension, kind, quantity and
		// symbol, so this is the pair that reaches the factors themselves —
		// where one side has none.
		{"an unspelled unit of factor one is not the absence of a scale", Unnamed, metrology.Unit{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.want {
				t.Errorf("Equal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIntervalUnit(t *testing.T) {
	got, ok := Celsius.IntervalUnit()
	if !ok {
		t.Fatal("Celsius declares an interval counterpart but does not report one")
	}
	if !got.Equal(Kelvin) {
		t.Errorf("interval unit = %s, want K", got)
	}
}

// The zero Unit is not a scale, and every operation that would have to read one
// says so instead of dereferencing what is not there. Before this, all of these
// panicked — a nil *apd.Decimal reaching apd's own arithmetic — which is the
// one thing this library promises not to do outside a Must variant.
//
// The zero Unit is dimensionless, so the additive paths reach the scale check
// only against a dimensionless partner; against anything else sameQuantity
// reports the dimension mismatch first, and rightly.
func TestZeroUnitHasNoScale(t *testing.T) {
	zeroUnit := metrology.Unit{}
	zero := metrology.Measurement{}

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"a product of units", func() error { _, err := zeroUnit.Times(Metre); return err }},
		{"a product of units, the other way round", func() error { _, err := Metre.Times(zeroUnit); return err }},
		{"a quotient of units", func() error { _, err := zeroUnit.Per(Metre); return err }},
		{"a power", func() error { _, err := zeroUnit.Pow(2); return err }},
		// Pow(0) answers for every built unit without reading its factor. It
		// does not answer for a unit that has none: one rule, no exception.
		{"the zeroth power", func() error { _, err := zeroUnit.Pow(0); return err }},
		{"a product", func() error { _, err := zero.Mul(Metre.Of(1)); return err }},
		{"a quotient", func() error { _, err := Metre.Of(1).Div(zero); return err }},
		{"a sum", func() error { _, err := One.Of(1).Add(zero); return err }},
		{"a sum of two of them", func() error { _, err := zero.Add(zero); return err }},
		{"a difference", func() error { _, err := zero.Sub(One.Of(1)); return err }},
		{"a comparison", func() error { _, err := One.Of(1).Cmp(zero); return err }},
		{"a conversion onto a scale", func() error { _, err := zero.To(One); return err }},
		{"a conversion off one", func() error { _, err := One.Of(1).To(zeroUnit); return err }},
		{"a conversion between two of them", func() error { _, err := zero.To(zeroUnit); return err }},
		// Unnamed is what makes the equal-units fast path in convert agree, so
		// this is the case a check placed below that fast path would miss.
		{"a conversion onto a scale that renders the same way", func() error {
			_, err := zero.To(Unnamed)
			return err
		}},
		{"reading a magnitude out", func() error { _, err := zero.In[float64](One); return err }},
		{"reading a magnitude out exactly", func() error { _, err := zero.DecimalIn(One); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, metrology.ErrNoScale) {
				t.Errorf("error = %v, want ErrNoScale", err)
			}
		})
	}
}

// D11: the class is one half of an error, the context the other, and the
// message is what a user gets in place of a compile error.
func TestNoScaleErrorNamesTheOperation(t *testing.T) {
	_, err := One.Of(1).Add(metrology.Measurement{})

	var ne *metrology.NoScaleError
	if !errors.As(err, &ne) {
		t.Fatalf("error = %v, want a *NoScaleError", err)
	}
	if ne.Op != "Add" {
		t.Errorf("Op = %q, want %q", ne.Op, "Add")
	}
	want := "metrology: Add: the zero Unit has no scale; " +
		"build one with NewUnit or take one from a quantity package"
	if got := ne.Error(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// Equal answers a question, not an error (see [metrology.Measurement.Equal]),
// so the refusal arrives here as false. It is the one place the zero value's
// new behaviour is silent, which is why it is asserted on its own.
func TestZeroMeasurementIsEqualToNothing(t *testing.T) {
	zero := metrology.Measurement{}
	if zero.Equal(zero) {
		t.Error("two values that are not measurements compare equal")
	}
	if One.Of(0).Equal(zero) {
		t.Error("a magnitude on a scale equals one on no scale")
	}
}

// An accessor has no error channel, so it reports what NewUnit would have
// defaulted to rather than a nil decimal that moves the dereference one frame
// out. Compare TestNewUnitDefaults: the answers are the same, and only the
// arithmetic tells the two units apart.
func TestZeroUnitReportsTheIdentityFactor(t *testing.T) {
	zeroUnit := metrology.Unit{}

	f := zeroUnit.Factor()
	if f.Num.String() != "1" || f.Den.String() != "1" || f.Pi != 0 {
		t.Errorf("factor = %s/%s·π^%d, want 1/1 with no π", f.Num, f.Den, f.Pi)
	}
	if got := zeroUnit.Offset(); got.String() != "0" {
		t.Errorf("offset = %s, want 0", got)
	}
}

// D20: the exponent is stored in an int8 and bounded by the same MaxPower the
// dimension exponents are, so a definition past it is refused where it is
// written rather than wrapped where it is used.
func TestNewUnitRefusesAPiExponentOutOfRange(t *testing.T) {
	for _, exponent := range []int{metrology.MaxPower + 1, -metrology.MaxPower - 1} {
		_, err := metrology.NewUnit(metrology.UnitDef{
			Dimension: dimension.One, Symbol: symbol.Static("x"), Pi: exponent,
		})
		if !errors.Is(err, metrology.ErrRange) {
			t.Errorf("a π exponent of %d gave %v, want ErrRange", exponent, err)
		}
	}
}

// Two scales with the same fraction and different powers of π are two
// different scales, and Equal has to say so. It may compare the exponents as
// themselves because π is transcendental: πᵈ is rational only for d = 0, so no
// pair of fractions can make up the difference (D20).
func TestUnitsDifferingOnlyInTheirPiExponentAreNotEqual(t *testing.T) {
	plain := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("x"), Denominator: "180",
	})
	withPi := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("x"), Denominator: "180", Pi: 1,
	})
	if plain.Equal(withPi) {
		t.Error("1/180 and π/180 compare equal")
	}
	if !withPi.Equal(withPi) {
		t.Error("a unit does not equal itself")
	}
	if got := withPi.Factor().Pi; got != 1 {
		t.Errorf("Factor().Pi = %d, want 1", got)
	}
}
