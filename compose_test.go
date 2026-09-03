package metrology_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
)

func TestUnitTimes(t *testing.T) {
	torque, err := Newton.Times(Metre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := torque.String(), "N·m"; got != want {
		t.Errorf("symbol = %q, want %q", got, want)
	}
	if got, want := torque.Dimension(), dimension.New(dimension.Exponents{Length: 2, Mass: 1, Time: -2}); got != want {
		t.Errorf("dimension = %s, want %s", got, want)
	}
	// The factors multiply exactly: a kilometre-newton is a thousand newton
	// metres, and the fraction stays a fraction (D4).
	scaled, err := Newton.Times(Kilometre)
	if err != nil {
		t.Fatal(err)
	}
	m, err := scaled.Of(1).To(torque)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.String(), "1000 N·m"; got != want {
		t.Errorf("1 N·km = %q, want %q", got, want)
	}
}

// A product of a unit with itself is the unit squared, and squared is what Pow
// answers: one scale has one spelling, so the two agree and both equal the
// catalogue entry (D12). Before they gathered, m·m and m² were the same scale
// under two names, and only one of them read back as the catalogue unit.
func TestTimesGathersIntoAPower(t *testing.T) {
	squared, err := Metre.Times(Metre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := squared.String(), "m²"; got != want {
		t.Errorf("m times m = %q, want %q", got, want)
	}
	powered, err := Metre.Pow(2)
	if err != nil {
		t.Fatal(err)
	}
	if !squared.Equal(powered) {
		t.Errorf("m·m = %s and m² = %s are not the same unit", squared, powered)
	}
	if !squared.Equal(SquareMetre) {
		t.Errorf("m times m = %s, not the square metre", squared)
	}
	// The gathering is on the symbol and changes no number: a square metre is
	// still a square metre.
	cubed, err := squared.Times(Metre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cubed.String(), "m³"; got != want {
		t.Errorf("m² times m = %q, want %q", got, want)
	}
}

func TestUnitPer(t *testing.T) {
	pressure, err := Newton.Per(SquareMetre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pressure.String(), "N/m²"; got != want {
		t.Errorf("symbol = %q, want %q", got, want)
	}
	// A pascal is a newton per square metre, so the conversion is the identity
	// and says so exactly.
	m, err := pressure.Of(50).To(Pascal)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.String(), "50 Pa"; got != want {
		t.Errorf("50 N/m² = %q, want %q", got, want)
	}
}

// A point on a scale has no product and no quotient (D6): 20 °C times anything
// is not a quantity, and the error has to say which operand was the problem.
func TestUnitComposeRejectsAbsolute(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func() (metrology.Unit, error)
	}{
		{"times, on the left", func() (metrology.Unit, error) { return Celsius.Times(Metre) }},
		{"times, on the right", func() (metrology.Unit, error) { return Metre.Times(Celsius) }},
		{"per, on the left", func() (metrology.Unit, error) { return Celsius.Per(Metre) }},
		{"per, on the right", func() (metrology.Unit, error) { return Metre.Per(Celsius) }},
		{"pow", func() (metrology.Unit, error) { return Celsius.Pow(2) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.fn()
			if !errors.Is(err, metrology.ErrKind) {
				t.Fatalf("error = %v, want ErrKind", err)
			}
		})
	}
}

func TestUnitPow(t *testing.T) {
	for _, tc := range []struct {
		name string
		unit metrology.Unit
		n    int
		want string
	}{
		{"the first power is the unit itself", Metre, 1, "m"},
		{"cubed", Metre, 3, "m³"},
		{"reciprocal", Metre, -1, "m⁻¹"},
		{"the zeroth power is dimensionless", Metre, 0, "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, err := tc.unit.Pow(tc.n)
			if err != nil {
				t.Fatal(err)
			}
			if got := u.String(); got != tc.want {
				t.Errorf("symbol = %q, want %q", got, tc.want)
			}
			if got, want := u.Dimension(), tc.unit.Dimension().Pow(dimension.Exponent(tc.n)); got != want {
				t.Errorf("dimension = %s, want %s", got, want)
			}
		})
	}
}

// The factor is raised exactly, and a negative power turns the fraction over
// rather than dividing into it: 1 km⁻¹ is 0.001 m⁻¹ and 1 km² is 10⁶ m².
func TestUnitPowFactor(t *testing.T) {
	squareKilometre, err := Kilometre.Pow(2)
	if err != nil {
		t.Fatal(err)
	}
	m, err := squareKilometre.Of(1).To(SquareMetre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.String(), "1000000 m²"; got != want {
		t.Errorf("1 km² = %q, want %q", got, want)
	}

	perKilometre, err := Kilometre.Pow(-1)
	if err != nil {
		t.Fatal(err)
	}
	perMetre, err := Metre.Pow(-1)
	if err != nil {
		t.Fatal(err)
	}
	m, err = perKilometre.Of(1).To(perMetre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.String(), "0.001 m⁻¹"; got != want {
		t.Errorf("1 km⁻¹ = %q, want %q", got, want)
	}
}

// A dimension holds seven int8 exponents (D5), so a power beyond that range is
// not a dimension anyone can write down.
func TestUnitPowOutOfRange(t *testing.T) {
	for _, n := range []int{128, -128, 1000} {
		_, err := Metre.Pow(n)
		if !errors.Is(err, metrology.ErrRange) {
			t.Errorf("Pow(%d) error = %v, want ErrRange", n, err)
		}
		var re *metrology.RangeError
		if errors.As(err, &re) && re.Op != "Pow" {
			t.Errorf("Pow(%d) error Op = %q, want %q", n, re.Op, "Pow")
		}
	}
}

// Composition drops the quantity tag (D6): a quotient of two units knows the
// exponents and nothing else, which is what lets the result be named later.
func TestComposeDropsQuantityAndKind(t *testing.T) {
	hertz := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.T.Pow(-1), Quantity: "frequency", Symbol: Metre.Symbol(),
	})
	u, err := hertz.Times(Metre)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Quantity(); got != "" {
		t.Errorf("quantity = %q, want the empty one", got)
	}
	if got := u.Kind(); got != metrology.Interval {
		t.Errorf("kind = %s, want interval", got)
	}
}
