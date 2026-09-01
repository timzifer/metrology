package dimension_test

import (
	"testing"

	"github.com/timzifer/metrology/dimension"
)

func TestString(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    dimension.Dimension
		want string
	}{
		{"dimensionless", dimension.One, "1"},
		// D11 quotes exactly these two, because they are what a dimension
		// error prints instead of a compile error.
		{"energy", energy, "L²M¹T⁻²"},
		{"force", force, "L¹M¹T⁻²"},
		{"pressure", pressure, "L⁻¹M¹T⁻²"},
		{"velocity", velocity, "L¹T⁻¹"},
		{"single base unit", dimension.M, "M¹"},
		{"electric current", dimension.I, "I¹"},
		{"temperature", dimension.Θ, "Θ¹"},
		{"amount of substance", dimension.N, "N¹"},
		{"luminous intensity", dimension.J, "J¹"},
		{"axis order is fixed, not sorted by exponent", dimension.New(dimension.Exponents{
			Length: 1, Mass: 2, Time: 3,
		}), "L¹M²T³"},
		{"all seven axes", dimension.New(dimension.Exponents{
			Time: -2, Length: 1, Mass: 1, ElectricCurrent: -1,
			Temperature: 2, AmountOfSubstance: -3, LuminousIntensity: 4,
		}), "L¹M¹T⁻²I⁻¹Θ²N⁻³J⁴"},
		{"two-digit exponent", dimension.L.Pow(12), "L¹²"},
		{"two-digit negative exponent", dimension.L.Pow(-12), "L⁻¹²"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Two dimensions that differ in one exponent must produce strings differing in
// one place — that is what makes "expected …, got …" readable (D11).
func TestStringsOfNeighbouringDimensionsAlign(t *testing.T) {
	if got, want := energy.String(), "L²M¹T⁻²"; got != want {
		t.Fatalf("energy = %q", got)
	}
	if got, want := force.String(), "L¹M¹T⁻²"; got != want {
		t.Fatalf("force = %q", got)
	}
	if len([]rune(energy.String())) != len([]rune(force.String())) {
		t.Error("energy and force differ in length; the error message no longer aligns")
	}
}
