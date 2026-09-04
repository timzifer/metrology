package metrology_test

import (
	"errors"
	"math"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
)

// mustOf is the exact constructor for table entries, where an error in the
// literal itself is a defect in the test rather than a case under test.
func mustOf(u metrology.Unit, magnitude string) metrology.Measurement {
	m, err := u.OfString(magnitude)
	if err != nil {
		panic(err)
	}
	return m
}

func TestConversion(t *testing.T) {
	for _, tc := range []struct {
		name string
		from metrology.Measurement
		to   metrology.Unit
		want string
	}{
		{"bar to pascal", Bar.Of(2.5), Pascal, "250000 Pa"},
		{"pascal to bar", Pascal.Of(250000), Bar, "2.5 bar"},
		{"metre to kilometre", Metre.Of(1500), Kilometre, "1.5 km"},
		{"a unit to itself", Bar.Of(2.5), Bar, "2.5 bar"},

		// D4: 760 torr is exactly one atmosphere. With a pre-rounded factor of
		// 133.32236842105263 it is 101324.99999999999 Pa.
		{"the torr is exact", Torr.Of(760), Pascal, "101325 Pa"},
		{"and exact in reverse", Pascal.Of(101325), Torr, "760 Torr"},

		{"celsius to kelvin", Celsius.Of(20), KelvinAbsolute, "293.15 K"},
		{"kelvin to celsius", KelvinAbsolute.Of(293.15), Celsius, "20 °C"},
		{"absolute zero", mustOf(Celsius, "-273.15"), KelvinAbsolute, "0 K"},

		// The 5/9 of the Fahrenheit scale is the other factor D4 exists for.
		{"freezing point", Fahrenheit.Of(32), Celsius, "0 °C"},
		{"boiling point", Fahrenheit.Of(212), Celsius, "100 °C"},
		{"celsius to fahrenheit", Celsius.Of(100), Fahrenheit, "212 °F"},
		{"a fahrenheit interval is rankine", Kelvin.Of(1), Rankine, "1.8 °R"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.from.To(tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A round trip across every pair of the mini catalogue reproduces the input
// exactly. Exactness is the claim of D4 and this is what tests it: a factor
// stored pre-rounded fails here in the third decimal.
func TestRoundTripAcrossEveryPair(t *testing.T) {
	groups := map[string][]metrology.Unit{
		"length":      {Metre, Kilometre},
		"pressure":    {Pascal, Bar, Torr},
		"temperature": {Celsius, KelvinAbsolute, Fahrenheit},
		"interval":    {Kelvin, Rankine},
	}
	magnitudes := []string{"1", "2.5", "760", "0.0001", "-40", "123456789.123456789"}

	for group, units := range groups {
		for _, from := range units {
			for _, to := range units {
				for _, magnitude := range magnitudes {
					name := group + "/" + from.String() + "→" + to.String() + "/" + magnitude
					t.Run(name, func(t *testing.T) {
						start, err := from.OfString(magnitude)
						if err != nil {
							t.Fatalf("OfString: %v", err)
						}
						there, err := start.To(to)
						if err != nil {
							t.Fatalf("to %s: %v", to, err)
						}
						back, err := there.To(from)
						if err != nil {
							t.Fatalf("back to %s: %v", from, err)
						}
						if cmp, err := start.Cmp(back); err != nil || cmp != 0 {
							t.Errorf("%s → %s → %s, want %s (cmp=%d, err=%v)",
								start, there, back, start, cmp, err)
						}
					})
				}
			}
		}
	}
}

func TestConversionErrors(t *testing.T) {
	t.Run("across dimensions", func(t *testing.T) {
		_, err := Bar.Of(1).To(Metre)
		var de *metrology.DimensionError
		if !errors.As(err, &de) {
			t.Fatalf("error = %v, want a *DimensionError", err)
		}
		if !errors.Is(err, metrology.ErrDimension) {
			t.Error("a DimensionError must match ErrDimension")
		}
		// D11: the message names both dimensions, because at runtime it is
		// what replaces a compile error.
		if got, want := de.Error(), "metrology: To: expected L¹, got L⁻¹M¹T⁻²"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("across kinds", func(t *testing.T) {
		_, err := Kelvin.Of(5).To(Celsius)
		var ke *metrology.KindError
		if !errors.As(err, &ke) {
			t.Fatalf("error = %v, want a *KindError", err)
		}
		if !errors.Is(err, metrology.ErrKind) {
			t.Error("a KindError must match ErrKind")
		}
		if ke.Left != metrology.Interval || ke.Right != metrology.Absolute {
			t.Errorf("kinds = %s/%s", ke.Left, ke.Right)
		}
	})

	// A non-finite magnitude is carried rather than rejected at the boundary,
	// and it stays visible: it propagates as an infinity and refuses to become
	// an integer.
	t.Run("a non-finite magnitude propagates", func(t *testing.T) {
		got, err := Bar.Of(math.Inf(-1)).To(Pascal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "-Infinity Pa"; got.String() != want {
			t.Errorf("got %s, want %s", got, want)
		}
		if _, err := got.In[int](Pascal); !errors.Is(err, metrology.ErrRange) {
			t.Errorf("In[int] error = %v, want ErrRange", err)
		}
	})
}

// An inexact conversion rounds once, to the precision of the engine, and the
// result carries no padding zeros (D9).
func TestConversionRoundsOnceAndReduces(t *testing.T) {
	third, err := Torr.OfString("1")
	if err != nil {
		t.Fatalf("OfString: %v", err)
	}
	got, err := third.To(Pascal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 101325/760 = 133.32236842105263157894736842105…, rounded to the default
	// twenty significant digits, with no trailing zeros bolted on.
	if want := "133.32236842105263158 Pa"; got.String() != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// D20: the π exponents of two units subtract, so a conversion that stays
// inside them cancels π and computes exactly what D4 always computed. This is
// the half of D20 that costs nothing, and it is the half that carries the
// catalogue: a degree is exactly 3600 arcseconds, in every precision.
func TestPiExponentsCancel(t *testing.T) {
	for _, tc := range []struct {
		name string
		from metrology.Measurement
		to   metrology.Unit
		want string
	}{
		{"a degree is 3600 arcseconds exactly", Degree.Of(1), Arcsecond, "3600 ″"},
		{"and an arcsecond a 3600th of a degree", Arcsecond.Of(3600), Degree, "1 °"},
		{"a third of a degree keeps its digits", mustOf(Degree, "0.5"), Arcsecond, "1800 ″"},
		{"an oersted is an oersted", Oersted.Of(2.5), Oersted, "2.5 Oe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.from.To(tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// The crossing conversion is the one D20 pays for: π is a number here and the
// result rounds twice, once at π and once at the division. This is the test
// that says the second rounding does not reach the digits reported — the same
// conversion at sixty digits, rounded back to twenty, has to be the same
// answer.
//
// It is a comparison against more precision rather than against a literal
// because a literal would be this library computing its own expectation. The
// digits of π themselves are checked in internal/pi, against Machin's formula.
func TestCrossingPiConversionAgreesWithMorePrecision(t *testing.T) {
	reference := metrology.NewEngine(60)

	for _, pair := range []struct {
		from metrology.Unit
		to   metrology.Unit
	}{
		{Degree, Radian}, // the exponent multiplies the numerator
		{Radian, Degree}, // and here the denominator
		{Oersted, AmperePerMetre},
		{AmperePerMetre, Oersted},
	} {
		for _, magnitude := range []string{"1", "2.5", "-7", "180", "0.000001", "123456789.123456789"} {
			t.Run(pair.from.String()+"→"+pair.to.String()+"/"+magnitude, func(t *testing.T) {
				start := mustOf(pair.from, magnitude)

				got, err := start.To(pair.to)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want, err := reference.To(start, pair.to)
				if err != nil {
					t.Fatalf("unexpected error at sixty digits: %v", err)
				}

				ctx := apd.BaseContext
				ctx.Precision = metrology.DefaultPrecision
				var rounded apd.Decimal
				if _, err := ctx.Round(&rounded, want.Decimal()); err != nil {
					t.Fatalf("rounding the reference: %v", err)
				}
				if got.Decimal().Cmp(&rounded) != 0 {
					t.Errorf("got %s, want %s (from %s at sixty digits)", got, &rounded, want)
				}
			})
		}
	}
}

// A converted bound has to move outward whatever the conversion does with π
// (D15, D20). Which direction π itself has to be rounded in depends on two
// signs — the exponent's and the magnitude's — so all four combinations are
// here, and each one is checked against a reference computed with far more
// digits than either bound reports.
func TestPiBoundsRoundOutward(t *testing.T) {
	lower := metrology.Engine{}.Rounding(apd.RoundFloor)
	upper := metrology.Engine{}.Rounding(apd.RoundCeiling)
	reference := metrology.NewEngine(60)

	for _, tc := range []struct {
		name string
		from metrology.Measurement
		to   metrology.Unit
	}{
		{"a positive magnitude out of the π units", Degree.Of(180), Radian},
		{"a negative one", Degree.Of(-180), Radian},
		{"a positive magnitude into them", Radian.Of(1), Degree},
		{"a negative one into them", Radian.Of(-1), Degree},
		{"a positive magnitude where the exponent is negative", Oersted.Of(1), AmperePerMetre},
		{"and a negative one", Oersted.Of(-1), AmperePerMetre},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lo, err := lower.To(tc.from, tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			hi, err := upper.To(tc.from, tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want, err := reference.To(tc.from, tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if lo.Decimal().Cmp(want.Decimal()) > 0 {
				t.Errorf("the lower bound %s is above the value %s", lo, want)
			}
			if hi.Decimal().Cmp(want.Decimal()) < 0 {
				t.Errorf("the upper bound %s is below the value %s", hi, want)
			}
			if lo.Decimal().Cmp(hi.Decimal()) >= 0 {
				t.Errorf("the bounds %s and %s do not enclose anything", lo, hi)
			}
		})
	}
}

// The digits of π are a constant, so there is a precision past which a crossing
// conversion cannot be served. It fails there rather than returning fewer
// correct digits than the engine promises (D20).
func TestPiConversionRefusesPrecisionBeyondTheConstant(t *testing.T) {
	_, err := metrology.NewEngine(1000).To(Degree.Of(1), Radian)
	if !errors.Is(err, metrology.ErrPrecision) {
		t.Fatalf("got %v, want ErrPrecision", err)
	}

	var precision *metrology.PrecisionError
	if !errors.As(err, &precision) {
		t.Fatalf("got %v, want a *PrecisionError", err)
	}
	if precision.Requested != 1000 {
		t.Errorf("Requested = %d, want 1000", precision.Requested)
	}
	if precision.Max == 0 || precision.Max >= precision.Requested {
		t.Errorf("Max = %d, want a limit below the request", precision.Max)
	}

	// The same engine converts anything without a π in it, because nothing
	// there needs a digit of π at all.
	if _, err := metrology.NewEngine(1000).To(Degree.Of(1), Arcsecond); err != nil {
		t.Errorf("a conversion that cancels π should not need it: %v", err)
	}
}
