package uncertainty_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
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

// span is Between for two magnitudes on one unit.
func span(t *testing.T, u metrology.Unit, lo, hi string) uncertainty.Range {
	t.Helper()
	r, err := uncertainty.Between(of(t, u, lo), of(t, u, hi))
	if err != nil {
		t.Fatalf("Between(%s, %s) %s: %v", lo, hi, u, err)
	}
	return r
}

func TestOf(t *testing.T) {
	r := uncertainty.Of(pressure.Bar.Of(2.5))
	if got, want := r.String(), "[2.5, 2.5] bar"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if !r.Lo().Equal(r.Hi()) {
		t.Error("a point has two different bounds")
	}
	if got := r.Unit(); !got.Equal(pressure.Bar) {
		t.Errorf("unit is %s, want bar", got)
	}
	if got := r.Dimension(); got != pressure.Bar.Dimension() {
		t.Errorf("dimension is %s, want %s", got, pressure.Bar.Dimension())
	}
	if got := r.Kind(); got != metrology.Interval {
		t.Errorf("kind is %s, want interval", got)
	}
}

// The quantity tag travels with the unit, so a range of hertz is a frequency
// and refuses to meet a radioactivity (D16). Nothing here restates the rule; it
// is inherited whole from the unit the bounds are on.
func TestQuantityIsInherited(t *testing.T) {
	r := uncertainty.Of(of(t, frequency.Hertz, "50"))
	if got := r.Quantity(); got != frequency.Quantity {
		t.Errorf("quantity is %q, want %q", got, frequency.Quantity)
	}
}

func TestBetween(t *testing.T) {
	r := span(t, length.Metre, "1", "2")
	if got, want := r.String(), "[1, 2] m"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got, want := r.Lo().String(), "1 m"; got != want {
		t.Errorf("Lo is %s, want %s", got, want)
	}
	if got, want := r.Hi().String(), "2 m"; got != want {
		t.Errorf("Hi is %s, want %s", got, want)
	}
}

// A zero-width range is a range: the bounds may meet, they may not cross.
func TestBetweenEqualBounds(t *testing.T) {
	if got := span(t, length.Metre, "1", "1").String(); got != "[1, 1] m" {
		t.Errorf("got %s, want [1, 1] m", got)
	}
}

func TestBetweenErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lo, hi  metrology.Measurement
		want    error
		message string
	}{
		{
			name: "across dimensions",
			lo:   length.Metre.Of(1), hi: pressure.Bar.Of(1),
			want:    metrology.ErrDimension,
			message: "metrology: Between: expected L¹, got L⁻¹M¹T⁻²",
		},
		{
			name: "one dimension, two scales",
			lo:   pressure.Bar.Of(1), hi: pressure.Pascal.Of(200000),
			want:    uncertainty.ErrScale,
			message: "uncertainty: Between: a range holds one scale, got bar and Pa; convert one of them first",
		},
		{
			name: "the bounds the wrong way round",
			lo:   length.Metre.Of(2), hi: length.Metre.Of(1),
			want:    uncertainty.ErrReversed,
			message: "uncertainty: Between: the lower bound 2 m is above the upper bound 1 m",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uncertainty.Between(tc.lo, tc.hi)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if err.Error() != tc.message {
				t.Errorf("message is %q,\n           want %q", err, tc.message)
			}
		})
	}
}

// Two units of one dimension that are the same scale written differently are
// one scale: Unit.Equal compares the fraction as a value, not digit by digit.
func TestBetweenAcceptsTheSameScaleWrittenTwice(t *testing.T) {
	half := metrology.MustUnit(metrology.UnitDef{
		Dimension: length.Metre.Dimension(),
		Symbol:    length.Metre.Symbol(),
		Numerator: "5", Denominator: "10",
	})
	twice := metrology.MustUnit(metrology.UnitDef{
		Dimension: length.Metre.Dimension(),
		Symbol:    length.Metre.Symbol(),
		Numerator: "1", Denominator: "2",
	})
	if _, err := uncertainty.Between(half.Of(1), twice.Of(2)); err != nil {
		t.Errorf("5/10 and 1/2 are one scale: %v", err)
	}
}

func TestSymmetric(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    func(*testing.T) metrology.Measurement
		tol      func(*testing.T) metrology.Measurement
		want     string
		wantErr  error
		errorMsg string
	}{
		{
			name:  "a span with a span",
			value: func(t *testing.T) metrology.Measurement { return of(t, length.Metre, "3.7") },
			tol:   func(t *testing.T) metrology.Measurement { return of(t, length.Metre, "0.2") },
			want:  "[3.5, 3.9] m",
		},
		{
			name:  "a point with a span along its scale",
			value: func(t *testing.T) metrology.Measurement { return of(t, temperature.Celsius, "20") },
			tol:   func(t *testing.T) metrology.Measurement { return of(t, interval.Kelvin, "0.5") },
			want:  "[19.5, 20.5] °C",
		},
		{
			name:  "a tolerance on another scale of the same dimension",
			value: func(t *testing.T) metrology.Measurement { return of(t, length.Metre, "1") },
			tol:   func(t *testing.T) metrology.Measurement { return of(t, length.Kilometre, "0.001") },
			want:  "[0, 2] m",
		},
		{
			// D6: the sum of two points on a scale is not a point on it, so a
			// point is not a tolerance. No clause of this package says so.
			name:    "a point as the tolerance",
			value:   func(t *testing.T) metrology.Measurement { return of(t, temperature.Celsius, "20") },
			tol:     func(t *testing.T) metrology.Measurement { return of(t, temperature.Celsius, "0.5") },
			wantErr: metrology.ErrKind,
		},
		{
			name:    "a tolerance of another dimension",
			value:   func(t *testing.T) metrology.Measurement { return of(t, length.Metre, "1") },
			tol:     func(t *testing.T) metrology.Measurement { return of(t, pressure.Bar, "1") },
			wantErr: metrology.ErrDimension,
		},
		{
			name:    "a negative tolerance turns the bounds around",
			value:   func(t *testing.T) metrology.Measurement { return of(t, length.Metre, "3.7") },
			tol:     func(t *testing.T) metrology.Measurement { return of(t, length.Metre, "-0.2") },
			wantErr: uncertainty.ErrReversed,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := uncertainty.Symmetric(tc.value(t), tc.tol(t))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A point subtracted from a span is refused before the sum is reached, so the
// error naming the subtraction is the one that comes back.
func TestSymmetricRefusesAPointTolerance(t *testing.T) {
	_, err := uncertainty.Symmetric(of(t, interval.Kelvin, "5"), of(t, temperature.Celsius, "1"))
	if !errors.Is(err, metrology.ErrKind) {
		t.Fatalf("got %v, want ErrKind", err)
	}
	const want = "metrology: Sub: interval and absolute: a point on a scale cannot be subtracted from a span along it"
	if err.Error() != want {
		t.Errorf("message is %q,\n           want %q", err, want)
	}
}
