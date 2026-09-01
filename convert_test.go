package metrology_test

import (
	"errors"
	"math"
	"testing"

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
