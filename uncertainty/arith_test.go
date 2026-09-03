package uncertainty_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
)

func TestAdd(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b uncertainty.Range
		want string
	}{
		{"two spans", span(t, length.Metre, "1", "2"), span(t, length.Metre, "10", "20"), "[11, 22] m"},
		{"across zero", span(t, length.Metre, "-2", "3"), span(t, length.Metre, "-1", "1"), "[-3, 4] m"},
		{"across scales", span(t, length.Metre, "1", "2"), span(t, length.Kilometre, "1", "2"), "[1001, 2002] m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.a.Add(tc.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// The kind rules of D6 reach a range through the bounds and are not restated
// anywhere in this package. This table is what proves it.
func TestKindRules(t *testing.T) {
	point := uncertainty.Of(temperature.Celsius.Of(20))
	warmer := uncertainty.Of(temperature.Celsius.Of(25))
	step := uncertainty.Of(interval.Kelvin.Of(5))

	for _, tc := range []struct {
		name    string
		op      func() (uncertainty.Range, error)
		want    string
		wantErr error
	}{
		{"absolute + interval is absolute", func() (uncertainty.Range, error) { return point.Add(step) }, "[25, 25] °C", nil},
		{"interval + absolute is absolute", func() (uncertainty.Range, error) { return step.Add(point) }, "[25, 25] °C", nil},
		{"interval + interval is interval", func() (uncertainty.Range, error) { return step.Add(step) }, "[10, 10] K", nil},
		{"absolute + absolute is an error", func() (uncertainty.Range, error) { return point.Add(warmer) }, "", metrology.ErrKind},
		{"absolute − absolute is interval", func() (uncertainty.Range, error) { return warmer.Sub(point) }, "[5, 5] K", nil},
		{"absolute − interval is absolute", func() (uncertainty.Range, error) { return point.Sub(step) }, "[15, 15] °C", nil},
		{"interval − absolute is an error", func() (uncertainty.Range, error) { return step.Sub(point) }, "", metrology.ErrKind},
		{"a point has no product", func() (uncertainty.Range, error) { return point.Mul(step) }, "", metrology.ErrKind},
		{"nor a quotient", func() (uncertainty.Range, error) { return point.Div(step) }, "", metrology.ErrKind},
		{"nor a power", func() (uncertainty.Range, error) { return point.Pow(2) }, "", metrology.ErrKind},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.op()
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

// [a, b] − [c, d] is [a−d, b−c]: the smallest difference takes the largest away
// from the smallest. Crossing the bounds is the whole of it, and getting it
// wrong yields an interval that is too narrow rather than an error.
func TestSubCrossesTheBounds(t *testing.T) {
	got, err := span(t, length.Metre, "10", "20").Sub(span(t, length.Metre, "1", "3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[7, 19] m"; got.String() != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// x − x is not zero and x / x is not one. That is the dependency problem, it is
// what makes this interval arithmetic rather than an uncertainty budget, and
// the package says so on its first line.
func TestTheDependencyProblem(t *testing.T) {
	x := span(t, length.Metre, "1", "2")
	diff, err := x.Sub(x)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[-1, 1] m"; diff.String() != want {
		t.Errorf("x − x is %s, want %s", diff, want)
	}
	quotient, err := x.Div(x)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[0.5, 2] m/m"; quotient.String() != want {
		t.Errorf("x / x is %s, want %s", quotient, want)
	}
}

// All four products, because an interval straddling zero does not take its
// extreme at the corner one would guess.
func TestMulCorners(t *testing.T) {
	for _, tc := range []struct {
		name   string
		a, b   [2]string
		want   string
		reason string
	}{
		{"both positive", [2]string{"1", "2"}, [2]string{"3", "4"}, "[3, 8] m²", "lo·lo and hi·hi"},
		{"both negative", [2]string{"-2", "-1"}, [2]string{"-4", "-3"}, "[3, 8] m²", "hi·hi and lo·lo"},
		{"one negative", [2]string{"-2", "-1"}, [2]string{"3", "4"}, "[-8, -3] m²", "lo·hi and hi·lo"},
		{"the left straddles zero", [2]string{"-2", "3"}, [2]string{"2", "4"}, "[-8, 12] m²", "lo·hi and hi·hi"},
		{"both straddle zero", [2]string{"-2", "3"}, [2]string{"-2", "3"}, "[-6, 9] m²", "the minimum is at lo·hi"},
		{"the square of one straddling zero", [2]string{"-4", "3"}, [2]string{"-4", "3"}, "[-12, 16] m²", "the minimum is at lo·hi"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := span(t, length.Metre, tc.a[0], tc.a[1]).Mul(span(t, length.Metre, tc.b[0], tc.b[1]))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s (%s)", got, tc.want, tc.reason)
			}
		})
	}
}

func TestDivCorners(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b [2]string
		want string
	}{
		{"both positive", [2]string{"1", "2"}, [2]string{"4", "8"}, "[0.125, 0.5] m/m"},
		{"a negative divisor", [2]string{"1", "2"}, [2]string{"-8", "-4"}, "[-0.5, -0.125] m/m"},
		{"a dividend straddling zero", [2]string{"-2", "3"}, [2]string{"2", "4"}, "[-1, 1.5] m/m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := span(t, length.Metre, tc.a[0], tc.a[1]).Div(span(t, length.Metre, tc.b[0], tc.b[1]))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A divisor covering zero has no quotient with finite bounds, and reporting one
// would be a lie about the data.
func TestDivByAnIntervalCoveringZero(t *testing.T) {
	for _, divisor := range [][2]string{{"-1", "1"}, {"0", "1"}, {"-1", "0"}, {"0", "0"}} {
		got, err := span(t, length.Metre, "1", "2").Div(span(t, length.Metre, divisor[0], divisor[1]))
		if !errors.Is(err, uncertainty.ErrUnbounded) {
			t.Errorf("[%s, %s]: got %s, %v, want ErrUnbounded", divisor[0], divisor[1], got, err)
		}
	}
	_, err := span(t, length.Metre, "1", "2").Div(span(t, length.Metre, "-1", "1"))
	const want = "uncertainty: Div: the divisor [-1, 1] m covers zero, so the result has no bound"
	if err.Error() != want {
		t.Errorf("message is %q,\n           want %q", err, want)
	}
}

func TestPow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bounds [2]string
		n      int
		want   string
		reason string
	}{
		{"a square of a positive interval", [2]string{"2", "3"}, 2, "[4, 9] m²", "monotone"},
		{"a square of a negative interval", [2]string{"-3", "-2"}, 2, "[4, 9] m²", "the bounds change places"},
		{"a square across zero", [2]string{"-2", "3"}, 2, "[0, 9] m²", "the minimum is zero, at neither bound"},
		{"a square across zero, the other way", [2]string{"-4", "3"}, 2, "[0, 16] m²", "the maximum is at the far bound"},
		{"a cube keeps its sign", [2]string{"-2", "3"}, 3, "[-8, 27] m³", "an odd power is monotone"},
		{"the first power", [2]string{"-2", "3"}, 1, "[-2, 3] m", "monotone"},
		{"the zeroth power", [2]string{"-2", "3"}, 0, "[1, 1] 1", "every range to the zeroth is the dimensionless 1"},
		{"a negative power", [2]string{"2", "4"}, -1, "[0.25, 0.5] m⁻¹", "the reciprocal decreases"},
		{"a negative even power", [2]string{"2", "4"}, -2, "[0.0625, 0.25] m⁻²", "the reciprocal of the square"},
		{"a negative power of a negative interval", [2]string{"-4", "-2"}, -1, "[-0.5, -0.25] m⁻¹", "still decreasing"},
		{"a negative odd power of a negative interval", [2]string{"-4", "-2"}, -3, "[-0.125, -0.015625] m⁻³", "the cube then the reciprocal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := span(t, length.Metre, tc.bounds[0], tc.bounds[1]).Pow(tc.n)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s (%s)", got, tc.want, tc.reason)
			}
		})
	}
}

func TestPowErrors(t *testing.T) {
	if _, err := span(t, length.Metre, "-1", "1").Pow(-2); !errors.Is(err, uncertainty.ErrUnbounded) {
		t.Errorf("a negative power of an interval covering zero: %v, want ErrUnbounded", err)
	}
	if _, err := span(t, length.Metre, "1", "2").Pow(metrology.MaxPower + 1); !errors.Is(err, metrology.ErrRange) {
		t.Errorf("a power beyond the range: %v, want ErrRange", err)
	}
	if _, err := span(t, length.Metre, "1", "2").Pow(-metrology.MaxPower - 1); !errors.Is(err, metrology.ErrRange) {
		t.Errorf("a power beyond the range: %v, want ErrRange", err)
	}
}

func TestMid(t *testing.T) {
	for _, tc := range []struct {
		name   string
		r      uncertainty.Range
		want   string
		reason string
	}{
		{"a span", span(t, length.Metre, "1", "2"), "1.5 m", ""},
		{"across zero", span(t, length.Metre, "-2", "3"), "0.5 m", ""},
		{"a point", span(t, length.Metre, "7", "7"), "7 m", ""},
		{
			name: "a point on an affine scale",
			r:    span(t, temperature.Celsius, "19.5", "20.5"),
			want: "20 °C",
			// (lo + hi) / 2 in the scale's own coordinates. The sum of two
			// absolute magnitudes is not a magnitude (D6), and an affine map
			// preserves midpoints, so the arithmetic happens here and not
			// through Add.
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.r.Mid()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestWidth(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    uncertainty.Range
		want string
	}{
		{"a span", span(t, length.Metre, "1", "2.5"), "1.5 m"},
		{"a point", span(t, length.Metre, "7", "7"), "0 m"},
		// D6 for free: absolute − absolute is an interval, read on the interval
		// unit the scale declares. The width of 19.5 … 20.5 °C is 1 K.
		{"an absolute range", span(t, temperature.Celsius, "19.5", "20.5"), "1 K"},
		{"a Fahrenheit range", span(t, temperature.Fahrenheit, "68", "69.8"), "1.8 °R"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.r.Width()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
			if got.Kind() != metrology.Interval {
				t.Errorf("a width is %s, want interval", got.Kind())
			}
		})
	}
}

func TestOverlaps(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b uncertainty.Range
		want bool
	}{
		{"disjoint", span(t, length.Metre, "1", "2"), span(t, length.Metre, "3", "4"), false},
		{"disjoint the other way", span(t, length.Metre, "3", "4"), span(t, length.Metre, "1", "2"), false},
		{"touching at a bound", span(t, length.Metre, "1", "2"), span(t, length.Metre, "2", "3"), true},
		{"overlapping", span(t, length.Metre, "1", "3"), span(t, length.Metre, "2", "4"), true},
		{"one inside the other", span(t, length.Metre, "1", "4"), span(t, length.Metre, "2", "3"), true},
		{"across scales", span(t, length.Metre, "999", "1001"), span(t, length.Kilometre, "1", "2"), true},
		{"across scales, disjoint", span(t, length.Metre, "1", "2"), span(t, length.Kilometre, "1", "2"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.a.Overlaps(tc.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOverlapsAcrossDimensions(t *testing.T) {
	_, err := span(t, length.Metre, "1", "2").Overlaps(span(t, pressure.Bar, "1", "2"))
	if !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("got %v, want ErrDimension", err)
	}
}

func TestTo(t *testing.T) {
	got, err := span(t, length.Metre, "1000", "2000").To(length.Kilometre)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[1, 2] km"; got.String() != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := span(t, length.Metre, "1", "2").To(pressure.Bar); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("got %v, want ErrDimension", err)
	}
	if _, err := span(t, temperature.Celsius, "1", "2").To(interval.Kelvin); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("got %v, want ErrKind", err)
	}
}

// A negative conversion factor puts the bounds the other way round, and one
// comparison at the end of every operation is what keeps a Range in order
// without each of them having to argue that it cannot happen there.
func TestToAcrossANegativeFactor(t *testing.T) {
	mirrored := metrology.MustUnit(metrology.UnitDef{
		Dimension: length.Metre.Dimension(),
		Symbol:    metrology.MustUnit(metrology.UnitDef{Dimension: length.Metre.Dimension()}).Symbol(),
		Numerator: "-1",
	})
	got, err := span(t, length.Metre, "1", "2").To(mirrored)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lo, hi := got.Lo(), got.Hi(); lo.Decimal().Cmp(hi.Decimal()) > 0 {
		t.Fatalf("the bounds came back reversed: %s", got)
	}
	if want := "[-2, -1] "; got.String() != want {
		t.Errorf("got %q, want %q", got.String(), want)
	}
}

// Every operation lands both bounds on one unit, because the core derives the
// result unit from the operand units alone and both bounds share theirs. The
// type could not hold two, so this is the assertion that the invariant survives
// a change to the core.
func TestBothBoundsStayOnOneUnit(t *testing.T) {
	a := span(t, pressure.Bar, "1", "2")
	b := span(t, pressure.Torr, "3", "4")
	for _, tc := range []struct {
		name string
		op   func() (uncertainty.Range, error)
	}{
		{"Add", func() (uncertainty.Range, error) { return a.Add(b) }},
		{"Sub", func() (uncertainty.Range, error) { return a.Sub(b) }},
		{"Mul", func() (uncertainty.Range, error) { return a.Mul(b) }},
		{"Div", func() (uncertainty.Range, error) { return a.Div(b) }},
		{"Pow", func() (uncertainty.Range, error) { return a.Pow(3) }},
		{"To", func() (uncertainty.Range, error) { return a.To(pressure.Pascal) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.op()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Lo().Unit().Equal(got.Hi().Unit()) {
				t.Errorf("the bounds are on %s and %s", got.Lo().Unit(), got.Hi().Unit())
			}
			if !got.Lo().Unit().Equal(got.Unit()) {
				t.Errorf("the bounds are on %s, the range says %s", got.Lo().Unit(), got.Unit())
			}
		})
	}
}

// An operand at the edge of the exponent range stops the operation rather than
// producing a number that looks fine. It is the one way a chain of magnitude
// steps fails, and every operation that has such a chain reports it.
func TestArithmeticBeyondTheExponentRange(t *testing.T) {
	huge := span(t, length.Metre, "9E+100000", "9E+100000")

	if _, err := huge.Mid(); err == nil {
		t.Error("a midpoint beyond the exponent range must fail")
	}
	if _, err := huge.Pow(2); err == nil {
		t.Error("a square beyond the exponent range must fail")
	}
	if _, err := huge.Pow(-2); err == nil {
		t.Error("a reciprocal square beyond the exponent range must fail")
	}
	if _, err := huge.Mul(huge); err == nil {
		t.Error("a product beyond the exponent range must fail")
	}
	if _, err := huge.Add(huge); err == nil {
		t.Error("a sum beyond the exponent range must fail")
	}
	if _, err := huge.Sub(span(t, length.Metre, "-9E+100000", "-9E+100000")); err == nil {
		t.Error("a difference beyond the exponent range must fail")
	}
	if _, err := huge.To(length.Angstrom); err == nil {
		t.Error("a conversion beyond the exponent range must fail")
	}

	// The ± form has no answer either, and says so the way it says everything
	// else it cannot write.
	if got, ok := huge.PlusMinus(); ok {
		t.Errorf("got %q, want no ± form", got)
	}
}
