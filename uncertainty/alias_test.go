package uncertainty_test

import (
	"strings"
	"testing"

	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/length"
)

// D3, the invariant everything else rests on, now over two magnitudes instead
// of one. apd.Decimal shares its coefficient slice with every struct copy of
// itself, so a single in-place write reaches every range derived from the same
// value.
//
// 200 digits on purpose: apd/v3 stores coefficients up to 38 digits inline,
// which hides the aliasing at the sizes a test would otherwise use. This is the
// bug that passes tests and fails in production, and shortening these values is
// how it comes back.
func TestNoAliasing(t *testing.T) {
	lo := strings.Repeat("1234567890", 20)
	hi := strings.Repeat("9876543210", 20)
	original := span(t, length.Metre, lo, hi)
	before := original.String()

	t.Run("Lo and Hi hand out copies", func(t *testing.T) {
		low, high := original.Lo(), original.Hi()
		low.Decimal().Coeff.SetInt64(1)
		high.Decimal().Coeff.SetInt64(1)
		d := low.Decimal()
		d.Coeff.SetInt64(7)
		d.Exponent = 0
		if original.String() != before {
			t.Fatalf("writing through a bound changed the range:\n%s", original)
		}
	})

	t.Run("the bounds do not share with each other", func(t *testing.T) {
		d := original.Lo().Decimal()
		d.Coeff.SetInt64(3)
		if got := original.Hi().String(); got != hi+" m" {
			t.Fatalf("the upper bound followed the lower one: %s", got)
		}
	})

	t.Run("Mid and Width hand out copies", func(t *testing.T) {
		mid, err := original.Mid()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		width, err := original.Width()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		mid.Decimal().Coeff.SetInt64(1)
		width.Decimal().Coeff.SetInt64(1)
		if original.String() != before {
			t.Fatalf("writing through Mid or Width changed the range:\n%s", original)
		}
	})

	t.Run("a copied range is independent", func(t *testing.T) {
		copied := original
		d := copied.Lo().Decimal()
		d.Coeff.SetInt64(7)
		if original.String() != before || copied.String() != before {
			t.Fatal("a copy shares storage with its original")
		}
	})

	t.Run("arithmetic does not write to its operands", func(t *testing.T) {
		other := span(t, length.Metre, "1", "2")
		otherBefore := other.String()
		if _, err := original.Add(other); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.Sub(other); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.Mul(other); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.To(length.Kilometre); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.Pow(2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if original.String() != before {
			t.Errorf("the receiver changed:\n%s", original)
		}
		if other.String() != otherBefore {
			t.Errorf("the operand changed:\n%s", other)
		}
	})

	t.Run("a constructor copies its measurements", func(t *testing.T) {
		low := of(t, length.Metre, lo)
		r := uncertainty.Of(low)
		low.Decimal().Coeff.SetInt64(5)
		if got := r.Lo().String(); got != lo+" m" {
			t.Fatalf("the range followed the measurement it was built from: %s", got)
		}
	})
}
