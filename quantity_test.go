package metrology_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

// Two quantities on one dimension, which is the case the tag exists for: T⁻¹ is
// how often something happens and also how fast something decays, and the two
// are not the same measurement (D6).
var (
	perSecond = dimension.T.Reciprocal()

	Hertz = metrology.MustUnit(metrology.UnitDef{
		Dimension: perSecond, Quantity: "frequency", Symbol: symbol.SI("Hz"),
	})
	Becquerel = metrology.MustUnit(metrology.UnitDef{
		Dimension: perSecond, Quantity: "radioactivity", Symbol: symbol.SI("Bq"),
	})
	Untagged = metrology.MustUnit(metrology.UnitDef{
		Dimension: perSecond, Symbol: symbol.Static("1/s"),
	})
)

func TestQuantityRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() error
		want error
	}{
		{"converting between two quantities of one dimension", func() error {
			_, err := Hertz.Of(50).To(Becquerel)
			return err
		}, metrology.ErrQuantity},
		{"adding them", func() error {
			_, err := Hertz.Of(50).Add(Becquerel.Of(1))
			return err
		}, metrology.ErrQuantity},
		{"subtracting them", func() error {
			_, err := Hertz.Of(50).Sub(Becquerel.Of(1))
			return err
		}, metrology.ErrQuantity},
		{"comparing them", func() error {
			_, err := Hertz.Of(50).Cmp(Becquerel.Of(1))
			return err
		}, metrology.ErrQuantity},

		// An untagged magnitude goes either way. It is the result of every
		// computation (D6 drops the tag on multiplication and division), and
		// refusing to name it would make every computed value a dead end.
		{"an untagged magnitude becomes a frequency", func() error {
			_, err := Untagged.Of(50).To(Hertz)
			return err
		}, nil},
		{"and equally a radioactivity", func() error {
			_, err := Untagged.Of(50).To(Becquerel)
			return err
		}, nil},
		{"a tagged magnitude becomes untagged", func() error {
			_, err := Hertz.Of(50).To(Untagged)
			return err
		}, nil},
		{"and adds to an untagged one", func() error {
			_, err := Hertz.Of(50).Add(Untagged.Of(1))
			return err
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// The dimension is checked before the quantity: adding a length to a frequency
// is a coarser mistake than adding a frequency to a radioactivity, and the
// message should say which one happened.
func TestDimensionIsReportedBeforeQuantity(t *testing.T) {
	_, err := Hertz.Of(1).Add(Metre.Of(1))
	if !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("error = %v, want ErrDimension", err)
	}
	if errors.Is(err, metrology.ErrQuantity) {
		t.Error("a dimension mismatch was reported as a quantity mismatch")
	}
}

// A sum takes the tag of whichever operand carries one: an untagged T⁻¹ added
// to a frequency is a frequency, and there is nothing else it could be.
func TestSumTakesTheTagThatIsThere(t *testing.T) {
	for _, tc := range []struct {
		name string
		get  func() (metrology.Measurement, error)
		want metrology.Quantity
	}{
		{"tagged plus untagged", func() (metrology.Measurement, error) {
			return Hertz.Of(50).Add(Untagged.Of(1))
		}, "frequency"},
		{"untagged plus tagged", func() (metrology.Measurement, error) {
			return Untagged.Of(50).Add(Hertz.Of(1))
		}, "frequency"},
		{"untagged plus untagged", func() (metrology.Measurement, error) {
			return Untagged.Of(50).Add(Untagged.Of(1))
		}, ""},
		{"tagged minus untagged", func() (metrology.Measurement, error) {
			return Hertz.Of(50).Sub(Untagged.Of(1))
		}, "frequency"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.get()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Quantity() != tc.want {
				t.Errorf("quantity = %q, want %q", got.Quantity(), tc.want)
			}
		})
	}
}

// D6: multiplication and division drop the kind, and they drop the tag with it.
// A product of a frequency and a duration is a count, not a frequency.
func TestProductsAreUntagged(t *testing.T) {
	product, err := Hertz.Of(50).Mul(Metre.Of(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.Quantity() != "" {
		t.Errorf("a product carries the tag %q", product.Quantity())
	}

	quotient, err := Hertz.Of(50).Div(Hertz.Of(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quotient.Quantity() != "" {
		t.Errorf("a quotient carries the tag %q", quotient.Quantity())
	}
}

func TestQuantityString(t *testing.T) {
	if got := metrology.Quantity("frequency").String(); got != "frequency" {
		t.Errorf("String() = %q", got)
	}
	// The zero value is a value, not a missing one: most units have no tag, and
	// the error message has to say something about them.
	if got := metrology.Quantity("").String(); got != "untagged" {
		t.Errorf("the empty quantity prints as %q", got)
	}
}

func TestQuantityErrorMessage(t *testing.T) {
	_, err := Hertz.Of(50).To(Becquerel)

	var qe *metrology.QuantityError
	if !errors.As(err, &qe) {
		t.Fatalf("error = %v, want a *QuantityError", err)
	}
	if qe.Left != "frequency" || qe.Right != "radioactivity" {
		t.Errorf("quantities = %s/%s", qe.Left, qe.Right)
	}
	want := "metrology: To: frequency and radioactivity share a dimension but are different quantities"
	if got := qe.Error(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if errors.Is(err, metrology.ErrDimension) {
		t.Error("a quantity mismatch also matches ErrDimension")
	}
}

// The tag is part of what makes two units the same unit.
func TestUnitEqualityIncludesTheQuantity(t *testing.T) {
	same := metrology.MustUnit(metrology.UnitDef{
		Dimension: perSecond, Quantity: "frequency", Symbol: symbol.SI("Hz"),
	})
	if !Hertz.Equal(same) {
		t.Error("two identical definitions are not equal")
	}
	if Hertz.Equal(Untagged) {
		t.Error("a tagged unit equals an untagged one")
	}
	if Hertz.Quantity() != "frequency" {
		t.Errorf("Quantity() = %q", Hertz.Quantity())
	}
}
