// Package uncertainty is interval arithmetic, not uncertainty propagation: a
// [Range] gives worst-case bounds, and it has the dependency problem — x − x is
// not zero, x / x is not one, and a formula naming a variable twice over-widens
// (D15).
//
// For checking a published number that is acceptable and even conservative: a
// wider interval never invents a disagreement, it can only hide one. As a
// general uncertainty budget it is wrong, and this sentence is here rather than
// further down because a reader who takes the package for a GUM implementation
// gets numbers that look right. Quadrature combination, correlated quantities
// and coverage factors are none of this and stay deferred.
//
// # One scale, two magnitudes
//
// A Range holds one [metrology.Unit] and two bounds. A range whose ends are in
// different units is a state the type cannot hold, which is what makes the
// inheritance from D6 exact: both bounds share one unit, so an absolute range
// such as 20 ± 0.5 °C is meaningful, [Range.Width] is a span because
// absolute − absolute = interval is already the rule, and the product of two
// absolute ranges is already an error. D6 needs no new clause here.
//
// # A bound rounds outward
//
// D9 rounds a magnitude to the context precision at the one division of a
// conversion. For a point that is right. For a bound it is wrong: rounding a
// bound inward narrows the interval, and a narrowed interval can turn an
// overlap into a disjoint pair — a disagreement manufactured by the conversion
// and standing in no source. So every lower bound is computed with
// [apd.RoundFloor] and every upper bound with [apd.RoundCeiling], through
// [metrology.Engine.Rounding].
//
// # The zero value
//
// The zero Range is not a range: its unit is the zero [metrology.Unit], which
// has no scale, exactly as the zero [metrology.Measurement] has none. Build one
// with [Of], [Between] or [Symmetric], or read one with [Parse].
//
// Everything that has to read that scale — [Range.Width], [Range.To],
// [Range.Pow], [Range.Overlaps] — answers [metrology.ErrNoScale] rather than
// panicking. [Range.Mid] still answers, and the asymmetry is D15's rather than
// an oversight: a midpoint is computed on the two magnitudes inside the one
// scale, so it never asks the scale anything, where a width is a difference of
// two points and is therefore read on an interval unit.
package uncertainty

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

// Range is a magnitude known only to lie between two bounds, on one scale.
//
// It is an ordinary Go value (D1) — copyable, passable, free of identity. It is
// not comparable with ==; use [Range.Overlaps], or compare the bounds.
//
// Nothing inside a Range is ever written after construction (D3). Every
// operation allocates its result, and every bound is handed out through
// [metrology.Unit.OfDecimal], which copies — because an apd.Decimal shares its
// coefficient with every struct copy of itself, and one in-place write would
// silently corrupt every range derived from it.
type Range struct {
	unit   metrology.Unit
	lo, hi apd.Decimal
}

// scalar is the dimensionless scale a bare magnitude travels on.
//
// This package computes on the two magnitudes inside a Range, and the core
// computes on measurements. Rather than build an apd.Context of its own — a
// second rounding policy beside D9, which is exactly what D15 refuses — a bare
// magnitude is put on this scale, run through the core's own arithmetic, and
// taken off again. The factor is 1 and the offset 0, so nothing is converted
// and no digit moves.
//
// It is a package-level value and not global mutable state (D7): nothing writes
// to it, for the same reason nothing writes to a catalogue unit.
var scalar = metrology.MustUnit(metrology.UnitDef{
	Dimension: dimension.One,
	Symbol:    symbol.Static("1"),
})

// newRange pairs two bounds that are already known to be on one scale and in
// the right order.
//
// The decimals are copied in through Set (D3) — a struct copy of an apd.Decimal
// shares its coefficient, and a Range that shared one with the measurement it
// was built from would be corrupted by a write nobody could see.
func newRange(lo, hi metrology.Measurement) Range {
	r := Range{unit: lo.Unit()}
	r.lo.Set(lo.Decimal())
	r.hi.Set(hi.Decimal())
	return r
}

// Of returns the range of a single measurement: a point, with both bounds on
// it.
//
// It is the entry from a magnitude that is known exactly, or known well enough
// that the interval is not worth carrying — and it makes every operation of
// this package reachable from an ordinary [metrology.Measurement].
func Of(m metrology.Measurement) Range {
	return newRange(m, m)
}

// Between returns the range from lo to hi.
//
// Both bounds must be on the same scale, not merely of the same dimension: a
// range holds one unit, and with two there would be no answer to which of them
// it holds. Convert first — Between(lo, hi) after hi.To(lo.Unit()) is the
// spelling, and it says which scale was chosen.
//
// A lower bound above the upper one is an error rather than a silent swap. It
// is almost always a caller's mistake, and a layer that quietly repaired it
// would hide the mistake in a value that looks fine.
func Between(lo, hi metrology.Measurement) (Range, error) {
	if lo.Dimension() != hi.Dimension() {
		return Range{}, &metrology.DimensionError{Op: "Between", Want: lo.Dimension(), Got: hi.Dimension()}
	}
	if !lo.Unit().Equal(hi.Unit()) {
		return Range{}, &ScaleError{Op: "Between", Lower: lo.Unit(), Upper: hi.Unit()}
	}
	// One scale, so the magnitudes compare as they are written: no conversion,
	// nothing to round, and no error a caller could act on.
	if lo.Decimal().Cmp(hi.Decimal()) > 0 {
		return Range{}, &ReversedError{Op: "Between", Lower: lo.String(), Upper: hi.String()}
	}
	return newRange(lo, hi), nil
}

// Symmetric returns the range m ± tol: 3.7 ± 0.2.
//
// The tolerance is a span along the scale, never a point on it, and the kind
// rules of D6 say so without a clause of their own — m.Sub(tol) refuses an
// absolute tolerance and m.Add(tol) refuses the sum of two points. A negative
// tolerance turns the bounds around and is reported as such.
func Symmetric(m, tol metrology.Measurement) (Range, error) {
	lo, err := m.Sub(tol)
	if err != nil {
		return Range{}, err
	}
	hi, err := m.Add(tol)
	if err != nil {
		return Range{}, err
	}
	return Between(lo, hi)
}

// Unit returns the scale both bounds are read on.
func (r Range) Unit() metrology.Unit { return r.unit }

// Dimension returns what is measured.
func (r Range) Dimension() dimension.Dimension { return r.unit.Dimension() }

// Kind reports whether the bounds are points on a scale or spans along it (D6).
func (r Range) Kind() metrology.Kind { return r.unit.Kind() }

// Quantity reports what is measured, where the dimension does not say it.
func (r Range) Quantity() metrology.Quantity { return r.unit.Quantity() }

// Lo returns the lower bound.
func (r Range) Lo() metrology.Measurement { return r.unit.OfDecimal(&r.lo) }

// Hi returns the upper bound.
func (r Range) Hi() metrology.Measurement { return r.unit.OfDecimal(&r.hi) }
