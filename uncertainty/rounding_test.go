package uncertainty_test

import (
	"testing"

	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/pressure"
)

// contains reports whether outer encloses inner. Both must be on one scale.
func contains(t *testing.T, outer, inner uncertainty.Range) bool {
	t.Helper()
	loCmp, err := outer.Lo().Cmp(inner.Lo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hiCmp, err := outer.Hi().Cmp(inner.Hi())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return loCmp <= 0 && hiCmp >= 0
}

// The finding D15 rests on, stated as the property it is: a conversion never
// narrows a range.
//
// It is asserted over the whole catalogue rather than on one pair, because a
// bound that rounds inward does so only where the factor is not a power of ten,
// and picking the examples by hand is how the case that does it gets missed.
// Every unit is converted into every other unit it can reach, and back.
func TestConversionNeverNarrows(t *testing.T) {
	units := catalog.Units()
	for _, from := range units {
		original, err := uncertainty.Between(
			of(t, from, "1.0000000000000000001"),
			of(t, from, "2.9999999999999999999"),
		)
		if err != nil {
			t.Fatalf("%s: %v", from, err)
		}
		for _, to := range units {
			if from.Dimension() != to.Dimension() || from.Kind() != to.Kind() {
				continue
			}
			converted, err := original.To(to)
			if err != nil {
				// A quantity conflict: a hertz has no reading in becquerel.
				continue
			}
			back, err := converted.To(from)
			if err != nil {
				t.Fatalf("%s → %s → %s: %v", from, to, from, err)
			}
			if !contains(t, back, original) {
				t.Errorf("%s → %s → %s narrowed %s to %s", from, to, from, original, back)
			}
		}
	}
}

// The same property where it bites: 1.0000000000000000001 bar in torr needs
// more than twenty digits, so the lower bound has to round, and rounding it the
// way D9 rounds a point would move it up — into the interval.
func TestABoundRoundsOutwardWhereAPointWouldNot(t *testing.T) {
	lo := of(t, pressure.Bar, "1.0000000000000000001")
	hi := of(t, pressure.Bar, "2.0000000000000000001")
	r, err := uncertainty.Between(lo, hi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	converted, err := r.To(pressure.Torr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// What the core does for a point, which is right for a point and wrong for
	// a bound: the lower bound lands above where the range starts.
	asPoints, err := metrology.Engine{}.To(lo, pressure.Torr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmp, err := converted.Lo().Cmp(asPoints); err != nil || cmp >= 0 {
		t.Fatalf("the lower bound %s did not go below the rounded point %s (%v)",
			converted.Lo(), asPoints, err)
	}

	// And the exact value is inside the converted range, which is the whole
	// claim: the interval still holds what it held.
	exact := metrology.NewEngine(60)
	for _, bound := range []metrology.Measurement{lo, hi} {
		reference, err := exact.To(bound, pressure.Torr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		low, err := converted.Lo().Cmp(reference)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		high, err := converted.Hi().Cmp(reference)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if low > 0 || high < 0 {
			t.Errorf("%s converts to %s, which is outside %s", bound, reference, converted)
		}
	}
}

// A conversion can only report an overlap that is not there, never hide one
// that is. That direction is the point of rounding outward — a wider interval
// never invents a disagreement, and inventing one is the failure this package
// exists to avoid.
func TestConversionNeverBreaksAnOverlap(t *testing.T) {
	// Two ranges that touch at exactly one magnitude, which is the case a
	// narrowing conversion would pull apart.
	a, err := uncertainty.Between(
		of(t, pressure.Bar, "1.0000000000000000001"),
		of(t, pressure.Bar, "2.0000000000000000003"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := uncertainty.Between(
		of(t, pressure.Bar, "2.0000000000000000003"),
		of(t, pressure.Bar, "3.0000000000000000007"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overlaps, err := a.Overlaps(b); err != nil || !overlaps {
		t.Fatalf("the two ranges touch: %v, %v", overlaps, err)
	}

	for _, to := range catalog.Units() {
		if to.Dimension() != pressure.Bar.Dimension() || to.Kind() != pressure.Bar.Kind() {
			continue
		}
		left, err := a.To(to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		right, err := b.To(to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		overlaps, err := left.Overlaps(right)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !overlaps {
			t.Errorf("converting to %s pulled %s and %s apart: %s and %s", to, a, b, left, right)
		}
	}
}

// The two directed modes are not interchangeable, and the test that they are
// both used is that the two bounds of one conversion round the opposite way.
func TestTheTwoBoundsRoundOppositeWays(t *testing.T) {
	third, err := uncertainty.Between(
		of(t, pressure.Bar, "1"),
		of(t, pressure.Bar, "1"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// One bar in torr does not terminate, so both bounds of this point range
	// have to round — and they part company, which a point never does.
	converted, err := third.To(pressure.Torr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, err := converted.Lo().Cmp(converted.Hi())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmp >= 0 {
		t.Fatalf("a point converted to a point: %s", converted)
	}
	floor, err := metrology.Engine{}.Rounding(apd.RoundFloor).To(of(t, pressure.Bar, "1"), pressure.Torr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !converted.Lo().Equal(floor) {
		t.Errorf("the lower bound is %s, want the floor %s", converted.Lo(), floor)
	}
}

// Precision belongs to the computation here too (D9): an engine of its own
// carries a range further, and the wider engine's answer is inside the
// default's, because both are enclosures of the same exact interval.
func TestEnginePrecision(t *testing.T) {
	r, err := uncertainty.Between(
		of(t, pressure.Bar, "1"),
		of(t, pressure.Bar, "2"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wide := uncertainty.NewEngine(40)
	if got := wide.Precision(); got != 40 {
		t.Errorf("Precision() = %d, want 40", got)
	}
	if got := (uncertainty.Engine{}).Precision(); got != metrology.DefaultPrecision {
		t.Errorf("the zero engine computes with %d digits, want %d", got, metrology.DefaultPrecision)
	}

	narrow, err := r.To(pressure.Torr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	precise, err := wide.To(r, pressure.Torr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(t, narrow, precise) {
		t.Errorf("the 40-digit enclosure %s is not inside the 20-digit one %s", precise, narrow)
	}
}
