package metrology_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
)

// D9: precision belongs to the computation. The measurement carries none, so
// two engines can compute with the same values and disagree only in how far
// they carry the result.
func TestEnginePrecision(t *testing.T) {
	one := mustOf(Torr, "1")

	for _, tc := range []struct {
		name   string
		engine metrology.Engine
		want   string
	}{
		{"the default", metrology.Engine{}, "133.32236842105263158"},
		{"explicitly the default", metrology.NewEngine(metrology.DefaultPrecision), "133.32236842105263158"},
		{"five digits", metrology.NewEngine(5), "133.32"},
		{"forty digits", metrology.NewEngine(40), "133.3223684210526315789473684210526315789"},
		{"zero means the default", metrology.NewEngine(0), "133.32236842105263158"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.engine.To(one, Pascal)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := tc.want + " Pa"; got.String() != want {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}

func TestEngineReportsItsPrecision(t *testing.T) {
	if got := (metrology.Engine{}).Precision(); got != metrology.DefaultPrecision {
		t.Errorf("the zero engine computes with %d digits, want %d", got, metrology.DefaultPrecision)
	}
	if got := metrology.NewEngine(9).Precision(); got != 9 {
		t.Errorf("Precision() = %d, want 9", got)
	}
}

// Every operation is available on an engine, and the method on Measurement is
// the same operation at the default precision.
func TestEngineOperationsMatchTheMethods(t *testing.T) {
	e := metrology.Engine{}
	a, b := Metre.Of(3), Metre.Of(2)

	for _, tc := range []struct {
		name string
		viaE func() (metrology.Measurement, error)
		viaM func() (metrology.Measurement, error)
	}{
		{"Add", func() (metrology.Measurement, error) { return e.Add(a, b) },
			func() (metrology.Measurement, error) { return a.Add(b) }},
		{"Sub", func() (metrology.Measurement, error) { return e.Sub(a, b) },
			func() (metrology.Measurement, error) { return a.Sub(b) }},
		{"Mul", func() (metrology.Measurement, error) { return e.Mul(a, b) },
			func() (metrology.Measurement, error) { return a.Mul(b) }},
		{"Div", func() (metrology.Measurement, error) { return e.Div(a, b) },
			func() (metrology.Measurement, error) { return a.Div(b) }},
		{"To", func() (metrology.Measurement, error) { return e.To(a, Kilometre) },
			func() (metrology.Measurement, error) { return a.To(Kilometre) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fromEngine, errEngine := tc.viaE()
			fromMethod, errMethod := tc.viaM()
			if errEngine != nil || errMethod != nil {
				t.Fatalf("errors: %v, %v", errEngine, errMethod)
			}
			if fromEngine.String() != fromMethod.String() {
				t.Errorf("engine gave %s, method gave %s", fromEngine, fromMethod)
			}
		})
	}

	cmpEngine, err := e.Cmp(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmpMethod, err := a.Cmp(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmpEngine != cmpMethod {
		t.Errorf("engine gave %d, method gave %d", cmpEngine, cmpMethod)
	}
}

// A higher precision is reachable where a conversion chain needs it, and it
// costs nothing anywhere else because it is not carried in the values.
func TestEngineWithManyDigits(t *testing.T) {
	e := metrology.NewEngine(200)
	got, err := e.To(mustOf(Torr, "1"), Pascal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	digits := strings.TrimSuffix(got.String(), " Pa")
	if len(strings.Replace(digits, ".", "", 1)) != 200 {
		t.Errorf("got %d significant digits, want 200", len(digits)-1)
	}
}

// An engine reports the same errors as the methods do; precision does not
// change which operations are meaningful.
func TestEngineErrors(t *testing.T) {
	e := metrology.NewEngine(5)
	if _, err := e.Add(Bar.Of(1), Metre.Of(1)); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("Add: %v, want ErrDimension", err)
	}
	if _, err := e.Sub(Kelvin.Of(1), Celsius.Of(1)); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("Sub: %v, want ErrKind", err)
	}
	if _, err := e.Mul(Celsius.Of(1), Kelvin.Of(1)); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("Mul: %v, want ErrKind", err)
	}
	if _, err := e.Cmp(Bar.Of(1), Metre.Of(1)); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("Cmp: %v, want ErrDimension", err)
	}
	if _, err := e.To(Bar.Of(1), Metre); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("To: %v, want ErrDimension", err)
	}
}

// D15: an interval bound rounds outward, and this is the method that makes it
// possible. 1 torr is 133.32236842105263157894… Pa, so the twentieth digit is
// where the three modes part company — and the directed pair brackets the
// default rather than agreeing with it on one side and losing on the other.
func TestEngineRounding(t *testing.T) {
	one := mustOf(Torr, "1")

	for _, tc := range []struct {
		name   string
		engine metrology.Engine
		want   string
	}{
		{"the zero engine is unchanged", metrology.Engine{}, "133.32236842105263158"},
		{"toward -inf", metrology.Engine{}.Rounding(apd.RoundFloor), "133.32236842105263157"},
		{"toward +inf", metrology.Engine{}.Rounding(apd.RoundCeiling), "133.32236842105263158"},
		{"half even", metrology.Engine{}.Rounding(apd.RoundHalfEven), "133.32236842105263158"},
		{"the empty mode is the default", metrology.Engine{}.Rounding(""), "133.32236842105263158"},
		{"precision is kept", metrology.NewEngine(5).Rounding(apd.RoundFloor), "133.32"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.engine.To(one, Pascal)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := tc.want + " Pa"; got.String() != want {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}

// The directed modes bracket the exact value from both sides: the floor is never
// above it and the ceiling never below, whichever operation rounds. That is the
// whole property D15 rests on, asserted here on the core rather than on the
// layer that needs it.
func TestEngineRoundingBrackets(t *testing.T) {
	exact := metrology.NewEngine(60)
	floor := metrology.Engine{}.Rounding(apd.RoundFloor)
	ceiling := metrology.Engine{}.Rounding(apd.RoundCeiling)

	for _, tc := range []struct {
		name string
		with func(metrology.Engine) (metrology.Measurement, error)
	}{
		{"a conversion", func(e metrology.Engine) (metrology.Measurement, error) {
			return e.To(mustOf(Torr, "1"), Pascal)
		}},
		{"a quotient", func(e metrology.Engine) (metrology.Measurement, error) {
			return e.Div(mustOf(Metre, "1"), mustOf(Metre, "3"))
		}},
		{"a negative quotient", func(e metrology.Engine) (metrology.Measurement, error) {
			return e.Div(mustOf(Metre, "-1"), mustOf(Metre, "3"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reference, err := tc.with(exact)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			low, err := tc.with(floor)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			high, err := tc.with(ceiling)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmp, err := low.Cmp(reference); err != nil || cmp > 0 {
				t.Errorf("the floor %s is above the exact %s (%v)", low, reference, err)
			}
			if cmp, err := high.Cmp(reference); err != nil || cmp < 0 {
				t.Errorf("the ceiling %s is below the exact %s (%v)", high, reference, err)
			}
		})
	}
}
