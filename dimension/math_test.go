package dimension_test

import (
	"math/rand/v2"
	"testing"

	"github.com/timzifer/metrology/dimension"
)

var (
	length   = dimension.L
	time_    = dimension.T
	mass     = dimension.M
	area     = dimension.New(dimension.Exponents{Length: 2})
	velocity = dimension.New(dimension.Exponents{Length: 1, Time: -1})
	force    = dimension.New(dimension.Exponents{Length: 1, Mass: 1, Time: -2})
	energy   = dimension.New(dimension.Exponents{Length: 2, Mass: 1, Time: -2})
)

func TestProduct(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  dimension.Dimension
		want dimension.Dimension
	}{
		{"empty product is dimensionless", dimension.Product(), dimension.One},
		{"single factor is itself", dimension.Product(force), force},
		{"length·length is area", dimension.Product(length, length), area},
		{"force·length is energy", dimension.Product(force, length), energy},
		{"three factors", dimension.Product(mass, velocity, velocity), energy},
		{"negative exponents cancel", dimension.Product(velocity, time_), length},
		{"all seven axes", dimension.Product(
			dimension.T, dimension.L, dimension.M, dimension.I,
			dimension.Θ, dimension.N, dimension.J,
		), dimension.New(dimension.Exponents{
			Time: 1, Length: 1, Mass: 1, ElectricCurrent: 1,
			Temperature: 1, AmountOfSubstance: 1, LuminousIntensity: 1,
		})},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, tc.got, tc.want)
		}
	}
}

func TestQuotient(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  dimension.Dimension
		want dimension.Dimension
	}{
		{"length/time is velocity", dimension.Quotient(length, time_), velocity},
		{"energy/length is force", dimension.Quotient(energy, length), force},
		{"force/area is pressure", dimension.Quotient(force, area), pressure},
		{"anything by itself is dimensionless", dimension.Quotient(energy, energy), dimension.One},
		{"one over time", dimension.Quotient(dimension.One, time_), time_.Reciprocal()},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, tc.got, tc.want)
		}
	}
}

func TestPow(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  dimension.Dimension
		want dimension.Dimension
	}{
		{"length² is area", length.Pow(2), area},
		{"exponent zero clears every axis", energy.Pow(0), dimension.One},
		{"exponent one is identity", energy.Pow(1), energy},
		{"exponent minus one is the reciprocal", energy.Pow(-1), energy.Reciprocal()},
		{"exponent scales negative axes too", velocity.Pow(3),
			dimension.New(dimension.Exponents{Length: 3, Time: -3})},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, tc.got, tc.want)
		}
	}
}

// Every axis is negated, the amount of substance included: an axis skipped here
// turns mol⁻¹ back into mol with no error anywhere.
func TestReciprocalCoversEveryAxis(t *testing.T) {
	all := dimension.New(dimension.Exponents{
		Time: 1, Length: 2, Mass: 3, ElectricCurrent: 4,
		Temperature: 5, AmountOfSubstance: 6, LuminousIntensity: 7,
	})
	want := dimension.New(dimension.Exponents{
		Time: -1, Length: -2, Mass: -3, ElectricCurrent: -4,
		Temperature: -5, AmountOfSubstance: -6, LuminousIntensity: -7,
	})
	if got := all.Reciprocal(); got != want {
		t.Errorf("Reciprocal() = %s, want %s", got, want)
	}
}

// M1's definition of done: Product(q, q.Reciprocal()) == One for random
// dimensions.
func TestProductWithReciprocalIsOne(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	randomExponent := func() dimension.Exponent {
		// Physical exponents live in a narrow band; -8…8 covers the catalogue
		// several times over and stays clear of the int8 wrap.
		return dimension.Exponent(r.IntN(17) - 8)
	}
	for i := 0; i < 1000; i++ {
		d := dimension.New(dimension.Exponents{
			Time:              randomExponent(),
			Length:            randomExponent(),
			Mass:              randomExponent(),
			ElectricCurrent:   randomExponent(),
			Temperature:       randomExponent(),
			AmountOfSubstance: randomExponent(),
			LuminousIntensity: randomExponent(),
		})
		if got := dimension.Product(d, d.Reciprocal()); got != dimension.One {
			t.Fatalf("Product(%s, %s) = %s, want 1", d, d.Reciprocal(), got)
		}
		if got := d.Reciprocal().Reciprocal(); got != d {
			t.Fatalf("Reciprocal twice on %s gave %s", d, got)
		}
		if got := dimension.Quotient(d, d); got != dimension.One {
			t.Fatalf("Quotient(%s, %s) = %s, want 1", d, d, got)
		}
	}
}

// Product is associative and commutative because addition of exponents is; a
// catalogue that derives the same unit along two paths depends on it.
func TestProductIsAssociativeAndCommutative(t *testing.T) {
	left := dimension.Product(dimension.Product(force, length), time_)
	right := dimension.Product(force, dimension.Product(length, time_))
	if left != right {
		t.Errorf("associativity: %s vs %s", left, right)
	}
	if a, b := dimension.Product(force, time_), dimension.Product(time_, force); a != b {
		t.Errorf("commutativity: %s vs %s", a, b)
	}
}
