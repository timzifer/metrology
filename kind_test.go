package metrology_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
)

// The five rules of D6, one case each. This table is the reason the kind
// exists; if it ever disagrees with CONCEPT.md, one of the two is wrong and it
// is not automatically this one.
func TestKindRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		op      func(a, b metrology.Measurement) (metrology.Measurement, error)
		left    metrology.Measurement
		right   metrology.Measurement
		want    string // canonical text of the result
		wantErr error
	}{
		{
			name: "absolute + interval is absolute",
			op:   metrology.Measurement.Add,
			left: Celsius.Of(20), right: Kelvin.Of(5),
			want: "25 °C",
		},
		{
			name: "interval + absolute is absolute, addition commutes",
			op:   metrology.Measurement.Add,
			left: Kelvin.Of(5), right: Celsius.Of(20),
			want: "25 °C",
		},
		{
			name: "interval + interval is interval",
			op:   metrology.Measurement.Add,
			left: Kelvin.Of(5), right: Kelvin.Of(3),
			want: "8 K",
		},
		{
			name: "absolute + absolute is an error",
			op:   metrology.Measurement.Add,
			left: Celsius.Of(20), right: Celsius.Of(5),
			wantErr: metrology.ErrKind,
		},
		{
			name: "absolute - absolute is an interval",
			op:   metrology.Measurement.Sub,
			left: Celsius.Of(25), right: Celsius.Of(20),
			want: "5 K",
		},
		{
			name: "absolute - interval is absolute",
			op:   metrology.Measurement.Sub,
			left: Celsius.Of(25), right: Kelvin.Of(5),
			want: "20 °C",
		},
		{
			name: "interval - interval is interval",
			op:   metrology.Measurement.Sub,
			left: Kelvin.Of(5), right: Kelvin.Of(3),
			want: "2 K",
		},
		{
			name: "interval - absolute is an error",
			op:   metrology.Measurement.Sub,
			left: Kelvin.Of(5), right: Celsius.Of(20),
			wantErr: metrology.ErrKind,
		},
		{
			name: "an absolute magnitude has no product",
			op:   metrology.Measurement.Mul,
			left: Celsius.Of(20), right: Kelvin.Of(2),
			wantErr: metrology.ErrKind,
		},
		{
			name: "an absolute magnitude has no quotient either",
			op:   metrology.Measurement.Div,
			left: Kelvin.Of(20), right: Celsius.Of(2),
			wantErr: metrology.ErrKind,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.op(tc.left, tc.right)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
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

// The kind of a result is stated by the rules, never inferred from the
// operands afterwards.
func TestKindOfResult(t *testing.T) {
	for _, tc := range []struct {
		name string
		get  func() (metrology.Measurement, error)
		want metrology.Kind
	}{
		{"sum with an absolute operand", func() (metrology.Measurement, error) {
			return Celsius.Of(20).Add(Kelvin.Of(5))
		}, metrology.Absolute},
		{"sum of two intervals", func() (metrology.Measurement, error) {
			return Kelvin.Of(20).Add(Kelvin.Of(5))
		}, metrology.Interval},
		{"difference of two points", func() (metrology.Measurement, error) {
			return Celsius.Of(25).Sub(Celsius.Of(20))
		}, metrology.Interval},
		// D6: multiplication and division drop the kind entirely rather than
		// guessing one for the result.
		{"product", func() (metrology.Measurement, error) {
			return Newton.Of(100).Mul(Metre.Of(2))
		}, metrology.Interval},
		{"quotient", func() (metrology.Measurement, error) {
			return Newton.Of(100).Div(SquareMetre.Of(2))
		}, metrology.Interval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.get()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind() != tc.want {
				t.Errorf("kind = %s, want %s", got.Kind(), tc.want)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	if got := metrology.Absolute.String(); got != "absolute" {
		t.Errorf("Absolute = %q", got)
	}
	if got := metrology.Interval.String(); got != "interval" {
		t.Errorf("Interval = %q", got)
	}
}

// The difference of two points is read on the interval scale the unit declares,
// which is what makes 25 °C − 20 °C come out as 5 K and not 5 °C.
func TestDifferenceUsesTheDeclaredIntervalUnit(t *testing.T) {
	got, err := Celsius.Of(25).Sub(Celsius.Of(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Unit().String() != "K" {
		t.Errorf("unit = %s, want K", got.Unit())
	}

	// Fahrenheit differences are Rankine, so 5/9 of a kelvin each: the factor
	// travels with the interval unit rather than being assumed to be one.
	diff, err := Fahrenheit.Of(212).Sub(Fahrenheit.Of(32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.String() != "180 °R" {
		t.Errorf("got %s, want 180 °R", diff)
	}
	inKelvin, err := diff.In[float64](Kelvin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inKelvin != 100 {
		t.Errorf("180 °R = %v K, want 100", inKelvin)
	}
}

// Where no interval unit is declared, the difference stays on the scale it was
// computed on — without the offset, which is what makes it a span.
func TestDifferenceWithoutADeclaredIntervalUnit(t *testing.T) {
	celsius := metrology.MustUnit(metrology.UnitDef{
		Dimension: Celsius.Dimension(), Kind: metrology.Absolute,
		Symbol: Celsius.Symbol(), Offset: "273.15",
	})
	got, err := celsius.Of(25).Sub(celsius.Of(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "5 °C" {
		t.Errorf("got %s, want 5 °C", got)
	}
	if got.Kind() != metrology.Interval {
		t.Errorf("kind = %s, want interval", got.Kind())
	}
}

// The interval unit a scale declares is converted into, not merely labelled on.
//
// A Celsius scale whose differences are declared in degrees Rankine is not a
// realistic catalogue entry, but it separates the two things a difference needs:
// the linear scale it is computed on — the receiver's own factor, without the
// offset — and the unit it is finally read on. Labelling without converting
// would give 5 here, and 5 °R is not 5 K.
func TestDifferenceConvertsOntoTheDeclaredIntervalUnit(t *testing.T) {
	celsiusInRankine := metrology.MustUnit(metrology.UnitDef{
		Dimension: Celsius.Dimension(), Kind: metrology.Absolute,
		Symbol: Celsius.Symbol(), Offset: "273.15", Interval: &Rankine,
	})

	got, err := celsiusInRankine.Of(25).Sub(celsiusInRankine.Of(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "9 °R" {
		t.Errorf("got %s, want 9 °R", got)
	}
	kelvins, err := got.In[float64](Kelvin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kelvins != 5 {
		t.Errorf("= %v K, want 5", kelvins)
	}
}
