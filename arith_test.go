package metrology_test

import (
	"errors"
	"math"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
)

func TestArithmetic(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func(a, b metrology.Measurement) (metrology.Measurement, error)
		a, b metrology.Measurement
		want string
	}{
		{"addition in one unit", metrology.Measurement.Add, Metre.Of(2), Metre.Of(3), "5 m"},
		// The operands keep their own units; the result is read on the
		// receiver's, and the conversion happens on the way in.
		{"addition across units", metrology.Measurement.Add,
			Metre.Of(500), Kilometre.Of(1), "1500 m"},
		{"addition the other way round", metrology.Measurement.Add,
			Kilometre.Of(1), Metre.Of(500), "1.5 km"},
		{"subtraction across units", metrology.Measurement.Sub,
			Kilometre.Of(1), Metre.Of(250), "0.75 km"},
		{"an exact factor stays exact through a sum", metrology.Measurement.Add,
			Pascal.Of(0), Torr.Of(760), "101325 Pa"},

		{"multiplication", metrology.Measurement.Mul, Newton.Of(100), Metre.Of(2), "200 N·m"},
		{"division", metrology.Measurement.Div, Newton.Of(100), SquareMetre.Of(2), "50 N/m²"},
		{"division into a dimensionless ratio", metrology.Measurement.Div,
			Metre.Of(3), Metre.Of(2), "1.5 m/m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.op(tc.a, tc.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A quotient of a force and an area is a pressure, whatever its symbol says. It
// is the dimension that decides, and the dimension is what the next operation
// checks against.
func TestProductAndQuotientDimensions(t *testing.T) {
	quotient, err := Newton.Of(100).Div(SquareMetre.Of(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quotient.Dimension() != Pascal.Dimension() {
		t.Fatalf("dimension = %s, want %s", quotient.Dimension(), Pascal.Dimension())
	}
	// And because the dimension matches, it converts into the named unit — the
	// factors of both operands travel with it.
	pascals, err := quotient.In[float64](Pascal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pascals != 50 {
		t.Errorf("= %v Pa, want 50", pascals)
	}

	product, err := Newton.Of(2).Mul(Metre.Of(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := dimension.Product(Newton.Dimension(), Metre.Dimension())
	if product.Dimension() != want {
		t.Errorf("dimension = %s, want %s", product.Dimension(), want)
	}
}

// A unit whose factor is not one keeps that factor in the derived unit: 1 km²
// is 10⁶ m², not 1.
func TestDerivedUnitsCarryTheirFactors(t *testing.T) {
	area, err := Kilometre.Of(2).Mul(Kilometre.Of(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := area.String(); got != "6 km·km" {
		t.Errorf("got %s, want 6 km·km", got)
	}
	inSquareMetres, err := area.In[float64](SquareMetre)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inSquareMetres != 6e6 {
		t.Errorf("= %v m², want 6e6", inSquareMetres)
	}
}

func TestArithmeticErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() error
		want error
	}{
		{"adding across dimensions", func() error {
			_, err := Bar.Of(1).Add(Metre.Of(1))
			return err
		}, metrology.ErrDimension},
		{"subtracting across dimensions", func() error {
			_, err := Bar.Of(1).Sub(Metre.Of(1))
			return err
		}, metrology.ErrDimension},
		{"comparing across dimensions", func() error {
			_, err := Bar.Of(1).Cmp(Metre.Of(1))
			return err
		}, metrology.ErrDimension},
		{"comparing across kinds", func() error {
			_, err := Kelvin.Of(1).Cmp(KelvinAbsolute.Of(1))
			return err
		}, metrology.ErrKind},
		{"dividing by zero", func() error {
			_, err := Metre.Of(1).Div(Metre.Of(0))
			return err
		}, nil}, // apd reports it; the class is not one of ours
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("expected an error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// An operand that cannot be computed with stops the operation rather than
// producing a number that looks fine.
func TestArithmeticOnNonFiniteOperands(t *testing.T) {
	huge := mustOf(Metre, "9E+99999")
	if _, err := huge.Mul(huge); err == nil {
		t.Error("a product beyond the exponent range must fail")
	}
	if _, err := Metre.Of(math.NaN()).Add(Metre.Of(1)); err != nil {
		// A quiet NaN propagates rather than trapping, and stays visible as
		// one.
		t.Fatalf("unexpected error: %v", err)
	}
	sum, _ := Metre.Of(math.NaN()).Add(Metre.Of(1))
	if sum.String() != "NaN m" {
		t.Errorf("got %s, want NaN m", sum)
	}
}

func TestCmpAndEqual(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b metrology.Measurement
		want int
	}{
		{"the same measurement", Bar.Of(1), Bar.Of(1), 0},
		// Comparison is by value: 1 bar and 100000 Pa are one quantity written
		// two ways, and 2.5 and 2.50 are one number written two ways.
		{"across units", Bar.Of(1), Pascal.Of(100000), 0},
		{"trailing zeros do not matter", mustOf(Bar, "2.5"), mustOf(Bar, "2.50"), 0},
		{"less than", Bar.Of(1), Bar.Of(2), -1},
		{"greater than", Pascal.Of(200000), Bar.Of(1), 1},
		{"absolute scales compare too", Celsius.Of(0), KelvinAbsolute.Of(273.15), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.a.Cmp(tc.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Cmp = %d, want %d", got, tc.want)
			}
			if equal := tc.a.Equal(tc.b); equal != (tc.want == 0) {
				t.Errorf("Equal = %v, want %v", equal, tc.want == 0)
			}
		})
	}

	// Equal answers rather than erroring: two quantities of different
	// dimensions are not equal, which is a fact and not a failure.
	if Bar.Of(1).Equal(Metre.Of(1)) {
		t.Error("a pressure equals a length")
	}
}
