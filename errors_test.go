package metrology_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
)

// D11: at runtime these messages are what a user gets instead of a compile
// error, so what they say is part of the API and is asserted, not sampled.
func TestErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"a dimension error names both dimensions",
			&metrology.DimensionError{Op: "Add", Want: dimension.L.Pow(2), Got: dimension.L},
			"metrology: Add: expected L², got L¹"},
		{"a kind error names both kinds and the rule",
			&metrology.KindError{
				Op: "Add", Left: metrology.Absolute, Right: metrology.Absolute,
				Why: "the sum of two points on a scale is not a point on it",
			},
			"metrology: Add: absolute and absolute: the sum of two points on a scale is not a point on it"},
		{"a syntax error quotes what it could not read",
			&metrology.SyntaxError{Op: "OfString", Input: "warm", Err: errors.New("parse exponent: strconv.ParseInt")},
			`metrology: OfString: "warm": parse exponent: strconv.ParseInt`},
		{"a range error names the value and the type",
			&metrology.RangeError{Op: "In", Value: "300", Type: "int8"},
			"metrology: In: 300 does not fit int8"},
		{"a precision error names the request and the limit",
			&metrology.PrecisionError{Op: "convert", Requested: 1200, Max: 990},
			"metrology: convert: 1200 significant digits needs more of π than this library holds; the limit is 990"},
		// The zero Unit renders as the empty string, so this is the one message
		// that names no operand: quoting it would read "expected , got ".
		{"a no-scale error names the operation and the remedy",
			&metrology.NoScaleError{Op: "Add"},
			"metrology: Add: the zero Unit has no scale; " +
				"build one with NewUnit or take one from a quantity package"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every error carries its class as a sentinel and its context as a struct, and
// the two are reachable from the same value (D11).
func TestErrorClasses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		class    error
		notClass error
	}{
		{"dimension", &metrology.DimensionError{}, metrology.ErrDimension, metrology.ErrKind},
		{"kind", &metrology.KindError{}, metrology.ErrKind, metrology.ErrDimension},
		{"syntax", &metrology.SyntaxError{}, metrology.ErrSyntax, metrology.ErrRange},
		{"range", &metrology.RangeError{}, metrology.ErrRange, metrology.ErrSyntax},
		{"precision", &metrology.PrecisionError{}, metrology.ErrPrecision, metrology.ErrRange},
		{"no scale", &metrology.NoScaleError{}, metrology.ErrNoScale, metrology.ErrDimension},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.class) {
				t.Errorf("%T does not match its own class", tc.err)
			}
			if errors.Is(tc.err, tc.notClass) {
				t.Errorf("%T matches a class it is not", tc.err)
			}
		})
	}
}

// Everything below drives the arithmetic past the exponent range of the decimal
// engine. Overflow is an error, not a saturated value (D9) — and these are the
// paths on which an error travels back out of the arithmetic.
func TestOverflowIsReported(t *testing.T) {
	huge := mustOf(Metre, "9E+99999")
	hugeKilometres := mustOf(Kilometre, "9E+99999")
	hugeFahrenheit := mustOf(Fahrenheit, "9E+99999")

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"conversion", func() error {
			// Kilometres into metres multiplies by a thousand, which is what
			// pushes the exponent past what a decimal can hold.
			_, err := hugeKilometres.To(Metre)
			return err
		}},
		{"converting an operand into the receiver's unit", func() error {
			_, err := Metre.Of(1).Add(hugeKilometres)
			return err
		}},
		{"the sum itself", func() error {
			// A signalling NaN is the one operand ordinary addition refuses;
			// a quiet one would propagate silently.
			_, err := mustOf(Metre, "sNaN").Add(Metre.Of(1))
			return err
		}},
		{"the difference of two points", func() error {
			// Fahrenheit into Celsius multiplies by five before it divides,
			// which is enough to leave the exponent range on the way in.
			_, err := Celsius.Of(1).Sub(hugeFahrenheit)
			return err
		}},
		{"moving a difference onto its interval scale", func() error {
			_, err := hugeFahrenheit.Sub(Fahrenheit.Of(1))
			return err
		}},
		{"a product", func() error {
			_, err := huge.Mul(huge)
			return err
		}},
		{"a comparison", func() error {
			_, err := Metre.Of(1).Cmp(hugeKilometres)
			return err
		}},
		{"reading a magnitude out as a float", func() error {
			_, err := mustOf(Metre, "1E+400").In[float64](Metre)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Error("expected an error, got none")
			}
		})
	}
}
