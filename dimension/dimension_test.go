package dimension_test

import (
	"math"
	"testing"

	"github.com/timzifer/metrology/dimension"
)

// pressure is L⁻¹M¹T⁻², the dimension every example in docs/ uses.
var pressure = dimension.New(dimension.Exponents{Length: -1, Mass: 1, Time: -2})

func TestNewRoundTripsEveryAxis(t *testing.T) {
	// Each axis is set alone, with a negative exponent, so that a sign bleeding
	// into a neighbouring axis shows up as a non-zero exponent elsewhere.
	for _, tc := range []struct {
		name string
		e    dimension.Exponents
	}{
		{"time", dimension.Exponents{Time: -3}},
		{"length", dimension.Exponents{Length: -3}},
		{"mass", dimension.Exponents{Mass: -3}},
		{"electric current", dimension.Exponents{ElectricCurrent: -3}},
		{"temperature", dimension.Exponents{Temperature: -3}},
		{"amount of substance", dimension.Exponents{AmountOfSubstance: -3}},
		{"luminous intensity", dimension.Exponents{LuminousIntensity: -3}},
		{"all at once", dimension.Exponents{
			Time: -2, Length: 1, Mass: 3, ElectricCurrent: -4,
			Temperature: 5, AmountOfSubstance: -6, LuminousIntensity: 7,
		}},
		{"extremes", dimension.Exponents{
			Time: math.MinInt8 + 1, Length: math.MaxInt8,
			Mass: math.MinInt8 + 1, ElectricCurrent: math.MaxInt8,
			Temperature: math.MinInt8 + 1, AmountOfSubstance: math.MaxInt8,
			LuminousIntensity: math.MinInt8 + 1,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := dimension.New(tc.e).Exponents()
			if got != tc.e {
				t.Errorf("New(%+v).Exponents() = %+v", tc.e, got)
			}
		})
	}
}

func TestAccessorsReadTheirOwnAxis(t *testing.T) {
	e := dimension.Exponents{
		Time: -2, Length: 1, Mass: 3, ElectricCurrent: -4,
		Temperature: 5, AmountOfSubstance: -6, LuminousIntensity: 7,
	}
	d := dimension.New(e)
	for _, tc := range []struct {
		name string
		got  dimension.Exponent
		want dimension.Exponent
	}{
		{"Time", d.Time(), e.Time},
		{"Length", d.Length(), e.Length},
		{"Mass", d.Mass(), e.Mass},
		{"ElectricCurrent", d.ElectricCurrent(), e.ElectricCurrent},
		{"Temperature", d.Temperature(), e.Temperature},
		{"AmountOfSubstance", d.AmountOfSubstance(), e.AmountOfSubstance},
		{"LuminousIntensity", d.LuminousIntensity(), e.LuminousIntensity},
	} {
		if tc.got != tc.want {
			t.Errorf("%s() = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// The base constants must agree with New, because the catalogue will be written
// in terms of one and tested in terms of the other.
func TestBaseConstantsMatchNew(t *testing.T) {
	for _, tc := range []struct {
		name   string
		const_ dimension.Dimension
		built  dimension.Dimension
	}{
		{"T", dimension.T, dimension.New(dimension.Exponents{Time: 1})},
		{"L", dimension.L, dimension.New(dimension.Exponents{Length: 1})},
		{"M", dimension.M, dimension.New(dimension.Exponents{Mass: 1})},
		{"I", dimension.I, dimension.New(dimension.Exponents{ElectricCurrent: 1})},
		{"Θ", dimension.Θ, dimension.New(dimension.Exponents{Temperature: 1})},
		{"N", dimension.N, dimension.New(dimension.Exponents{AmountOfSubstance: 1})},
		{"J", dimension.J, dimension.New(dimension.Exponents{LuminousIntensity: 1})},
	} {
		if tc.const_ != tc.built {
			t.Errorf("%s = %#x, New = %#x", tc.name, uint64(tc.const_), uint64(tc.built))
		}
	}
}

// D5: the top byte is reserved for fractional exponents and must stay clear,
// otherwise a later widening of the word changes existing values.
func TestReservedBitsStayZero(t *testing.T) {
	d := dimension.New(dimension.Exponents{
		Time: -1, Length: -1, Mass: -1, ElectricCurrent: -1,
		Temperature: -1, AmountOfSubstance: -1, LuminousIntensity: -1,
	})
	if reserved := uint64(d) >> 56; reserved != 0 {
		t.Errorf("reserved bits = %#x, want 0", reserved)
	}
}

func TestIsOne(t *testing.T) {
	if !dimension.One.IsOne() {
		t.Error("One.IsOne() = false")
	}
	if dimension.New(dimension.Exponents{}).IsOne() != true {
		t.Error("New(zero).IsOne() = false")
	}
	if pressure.IsOne() {
		t.Error("pressure.IsOne() = true")
	}
}

// A Dimension is meant to be usable as a map key, which is what makes the
// catalogue of D8 a plain map literal.
func TestUsableAsMapKey(t *testing.T) {
	catalogue := map[dimension.Dimension]string{pressure: "Pa"}
	if got := catalogue[dimension.New(dimension.Exponents{Length: -1, Mass: 1, Time: -2})]; got != "Pa" {
		t.Errorf("lookup by an equal dimension returned %q", got)
	}
}
