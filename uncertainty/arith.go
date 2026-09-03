package uncertainty

import (
	"cmp"

	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
)

// To expresses this range on another scale of the same dimension and kind.
//
// Each bound converts outward: the lower one toward −∞, the upper one toward
// +∞. That is the whole of D15's finding — round a bound the way D9 rounds a
// point and the interval comes back narrower than it went in, and a narrower
// interval can disagree with a source that agreed.
func (r Range) To(u metrology.Unit) (Range, error) { return Engine{}.To(r, u) }

// To is [Range.To] at this engine's precision (D9).
func (e Engine) To(r Range, u metrology.Unit) (Range, error) {
	lo, loErr := e.floor().To(r.Lo(), u)
	hi, hiErr := e.ceiling().To(r.Hi(), u)
	if err := cmp.Or(loErr, hiErr); err != nil {
		return Range{}, err
	}
	return ordered(lo, hi), nil
}

// Add returns the sum of two ranges, at [metrology.DefaultPrecision].
//
// The sum of [a, b] and [c, d] is [a+c, b+d], and the kind rules of D6 decide
// what the sum is on: an absolute range plus an interval one is absolute, two
// absolute ranges have no sum. Nothing about that is new here — every bound
// goes through [metrology.Engine.Add], which already refuses what it refuses.
func (r Range) Add(o Range) (Range, error) { return Engine{}.Add(r, o) }

// Add is [Range.Add] at this engine's precision (D9).
func (e Engine) Add(r, o Range) (Range, error) {
	lo, loErr := e.floor().Add(r.Lo(), o.Lo())
	hi, hiErr := e.ceiling().Add(r.Hi(), o.Hi())
	if err := cmp.Or(loErr, hiErr); err != nil {
		return Range{}, err
	}
	return ordered(lo, hi), nil
}

// Sub returns the difference of two ranges, at [metrology.DefaultPrecision].
//
// The difference of [a, b] and [c, d] is [a−d, b−c]: the smallest difference
// takes the largest away from the smallest. Crossing the bounds is the whole
// of it, and getting it wrong yields an interval that is too narrow rather than
// an error, which is why it is stated here and asserted in the tests.
func (r Range) Sub(o Range) (Range, error) { return Engine{}.Sub(r, o) }

// Sub is [Range.Sub] at this engine's precision (D9).
func (e Engine) Sub(r, o Range) (Range, error) {
	lo, loErr := e.floor().Sub(r.Lo(), o.Hi())
	hi, hiErr := e.ceiling().Sub(r.Hi(), o.Lo())
	if err := cmp.Or(loErr, hiErr); err != nil {
		return Range{}, err
	}
	return ordered(lo, hi), nil
}

// Mul returns the product of two ranges, at [metrology.DefaultPrecision].
//
// All four products of the bounds are computed, and the result runs from the
// least to the greatest. That is not caution, it is arithmetic: an interval
// that straddles zero does not take its extreme at the corner one would guess —
// [−2, 3] · [−2, 3] is [−6, 9], and the minimum sits at lo·hi, not at lo·lo.
//
// As with [metrology.Measurement.Mul] the result carries no kind, its unit is
// the product of both units, and a point on a scale has no product at all.
func (r Range) Mul(o Range) (Range, error) { return Engine{}.Mul(r, o) }

// Mul is [Range.Mul] at this engine's precision (D9).
func (e Engine) Mul(r, o Range) (Range, error) {
	return e.corners(r, o, metrology.Engine.Mul)
}

// Div returns the quotient of two ranges, at [metrology.DefaultPrecision].
//
// A divisor whose interval covers zero is an error rather than a bound: the
// quotient runs to infinity in both directions, and no pair of finite bounds
// encloses it. Reporting one would be a lie about the data, which is the one
// thing a library for checking published numbers must not do.
//
// The divisor is examined before the operands' kinds are, because a divisor
// covering zero has no quotient whatever the kinds say.
func (r Range) Div(o Range) (Range, error) { return Engine{}.Div(r, o) }

// Div is [Range.Div] at this engine's precision (D9).
func (e Engine) Div(r, o Range) (Range, error) {
	if coversZero(o) {
		return Range{}, &UnboundedError{Op: "Div", Divisor: o.String()}
	}
	return e.corners(r, o, metrology.Engine.Div)
}

// Pow returns the range raised to the n-th power, at
// [metrology.DefaultPrecision].
//
// An even power is where an interval stops being a pair of numbers one can
// compute with separately: [−2, 3]² is [0, 9], not [4, 9] and not [−6, 9]. The
// square of an interval straddling zero has its minimum at zero, which is at
// neither bound.
//
// A negative power is the reciprocal of the positive one, so an interval
// covering zero has none — the same [ErrUnbounded] as [Range.Div].
//
// The magnitude is raised by repeated multiplication, which rounds n−1 times
// where a single power would round once. Every one of those roundings goes
// outward, so the result is a wider enclosure and never a narrower one — the
// property that matters is kept, and the extra width is the price of the core
// having no power on a magnitude to delegate to.
func (r Range) Pow(n int) (Range, error) { return Engine{}.Pow(r, n) }

// Pow is [Range.Pow] at this engine's precision (D9).
func (e Engine) Pow(r Range, n int) (Range, error) {
	// The unit refuses an absolute scale and a power out of range, exactly as
	// it does for a measurement (D6, D5) — the rules are not restated here.
	unit, err := r.unit.Pow(n)
	if err != nil {
		return Range{}, err
	}
	if n == 0 {
		// Every range to the zeroth power is the point 1 on the dimensionless
		// scale, whatever it was a range of.
		one := unit.Of(1)
		return newRange(one, one), nil
	}
	if n < 0 && coversZero(r) {
		return Range{}, &UnboundedError{Op: "Pow", Divisor: r.String()}
	}

	magnitude := n
	if magnitude < 0 {
		magnitude = -magnitude
	}
	lo, hi, err := e.powerBounds(r, magnitude)
	if err == nil && n < 0 {
		// The reciprocal is decreasing on an interval free of zero, so the
		// bounds change places.
		lo, hi, err = e.reciprocal(lo, hi)
	}
	if err != nil {
		return Range{}, err
	}
	return ordered(unit.OfDecimal(lo), unit.OfDecimal(hi)), nil
}

// Mid returns the midpoint of the range.
//
// It is computed on the magnitudes, within the one scale both bounds are read
// on, and not as (Lo + Hi) / 2 through the arithmetic: the sum of two absolute
// magnitudes is not a magnitude at all (D6), and an affine scale nevertheless
// has a midpoint — an affine map preserves midpoints, so the midpoint of the
// scale's own coordinates is the midpoint of the quantity.
//
// A midpoint is a point and not a bound, so it rounds the way D9 rounds every
// other point. On an interval narrower than the engine's precision that
// rounding can put it outside the bounds; use [Range.Lo] and [Range.Hi] where
// the answer has to be an enclosure.
func (r Range) Mid() (metrology.Measurement, error) { return Engine{}.Mid(r) }

// Mid is [Range.Mid] at this engine's precision (D9).
func (e Engine) Mid(r Range) (metrology.Measurement, error) {
	var s steps
	sum := s.do(bare(e.core, metrology.Engine.Add, &r.lo, &r.hi))
	mid := s.do(bare(e.core, metrology.Engine.Div, sum, apd.New(2, 0)))
	if s.err != nil {
		return metrology.Measurement{}, s.err
	}
	return r.unit.OfDecimal(mid), nil
}

// Width returns how wide the range is, as an interval-kind measurement.
//
// This one is free: Hi − Lo is absolute − absolute for an absolute range, which
// D6 already makes an interval, expressed in the interval unit the scale
// declares — the width of 20 … 21 °C is 1 K. It rounds toward +∞, because a
// width reported too small is a claim the data does not support.
//
// It reports an error where the subtraction does, which for a scale whose
// declared interval unit carries a conflicting quantity tag it can.
func (r Range) Width() (metrology.Measurement, error) { return Engine{}.Width(r) }

// Width is [Range.Width] at this engine's precision (D9).
func (e Engine) Width(r Range) (metrology.Measurement, error) {
	return e.ceiling().Sub(r.Hi(), r.Lo())
}

// Overlaps reports whether two ranges have any magnitude in common.
//
// Where the two are on different scales the other range is converted onto this
// one's, outward — so a rounding can only report an overlap that is not there,
// never hide one that is. That direction is the point: a wider interval never
// invents a disagreement, and this method exists to check for agreement.
func (r Range) Overlaps(o Range) (bool, error) { return Engine{}.Overlaps(r, o) }

// Overlaps is [Range.Overlaps] at this engine's precision (D9).
func (e Engine) Overlaps(r, o Range) (bool, error) {
	converted, err := e.To(o, r.unit)
	if err != nil {
		return false, err
	}
	return r.lo.Cmp(&converted.hi) <= 0 && converted.lo.Cmp(&r.hi) <= 0, nil
}

// corners applies a multiplicative operation to all four pairs of bounds and
// returns the interval from the least result to the greatest.
//
// Each pair is computed twice, once toward −∞ and once toward +∞, because which
// pair holds the extreme is not known before they are all computed and the
// direction a bound must round is decided by which end it becomes. Eight
// operations where a sign analysis would need four — and a sign analysis is
// four more cases to get wrong, over an interval that may straddle zero, for a
// saving nobody has measured as binding.
func (e Engine) corners(
	r, o Range,
	apply func(metrology.Engine, metrology.Measurement, metrology.Measurement) (metrology.Measurement, error),
) (Range, error) {
	pairs := [4][2]metrology.Measurement{
		{r.Lo(), o.Lo()},
		{r.Lo(), o.Hi()},
		{r.Hi(), o.Lo()},
		{r.Hi(), o.Hi()},
	}
	var low, high metrology.Measurement
	for i, pair := range pairs {
		down, downErr := apply(e.floor(), pair[0], pair[1])
		up, upErr := apply(e.ceiling(), pair[0], pair[1])
		if err := cmp.Or(downErr, upErr); err != nil {
			return Range{}, err
		}
		// Every result is on the one unit the operands' units compose to, so
		// the magnitudes compare as they are written.
		if i == 0 || down.Decimal().Cmp(low.Decimal()) < 0 {
			low = down
		}
		if i == 0 || up.Decimal().Cmp(high.Decimal()) > 0 {
			high = up
		}
	}
	return newRange(low, high), nil
}

// powerBounds returns the magnitudes of the two bounds of r raised to the
// positive power n.
//
// Three cases, and the third is the one an interval library is written for: an
// even power of an interval straddling zero has its minimum at zero, which is
// at neither bound.
func (e Engine) powerBounds(r Range, n int) (lo, hi *apd.Decimal, err error) { //nolint:nonamedreturns
	var s steps
	switch {
	case n%2 == 1 || r.lo.Sign() >= 0:
		// Monotone increasing over the whole interval.
		lo = s.do(e.power(e.floor(), &r.lo, n))
		hi = s.do(e.power(e.ceiling(), &r.hi, n))
	case r.hi.Sign() <= 0:
		// An even power of a wholly negative interval decreases: the bound
		// further from zero becomes the larger result.
		lo = s.do(e.power(e.floor(), &r.hi, n))
		hi = s.do(e.power(e.ceiling(), &r.lo, n))
	default:
		far := &r.lo
		if abs(&r.hi).Cmp(abs(&r.lo)) > 0 {
			far = &r.hi
		}
		lo = apd.New(0, 0)
		hi = s.do(e.power(e.ceiling(), far, n))
	}
	return lo, hi, s.err
}

// power raises a magnitude to a positive power by repeated multiplication, each
// step rounding the way the engine says.
func (e Engine) power(with metrology.Engine, v *apd.Decimal, n int) (*apd.Decimal, error) {
	var s steps
	out := apd.New(1, 0)
	for range n {
		out = s.do(bare(with, metrology.Engine.Mul, out, v))
	}
	return out, s.err
}

// reciprocal returns the bounds of 1/[lo, hi], which the caller has established
// does not cover zero. The reciprocal decreases, so the bounds change places.
func (e Engine) reciprocal(lo, hi *apd.Decimal) (*apd.Decimal, *apd.Decimal, error) {
	var s steps
	one := apd.New(1, 0)
	low := s.do(bare(e.floor(), metrology.Engine.Div, one, hi))
	high := s.do(bare(e.ceiling(), metrology.Engine.Div, one, lo))
	return low, high, s.err
}

// bare carries two magnitudes through one of the core's operations on the
// dimensionless scale and hands back the magnitude of the result.
//
// This package computes on the two magnitudes inside a Range; the core computes
// on measurements. Putting a magnitude on [scalar] is how the one reaches the
// other. The alternative — an apd.Context built here — would be a second
// rounding policy beside D9 and a second implementation of the path D4
// describes, which is what D15 refuses in as many words.
func bare(
	e metrology.Engine,
	apply func(metrology.Engine, metrology.Measurement, metrology.Measurement) (metrology.Measurement, error),
	a, b *apd.Decimal,
) (*apd.Decimal, error) {
	m, err := apply(e, scalar.OfDecimal(a), scalar.OfDecimal(b))
	if err != nil {
		return nil, err
	}
	return m.Decimal(), nil
}

// steps accumulates the first error of a chain of magnitude operations, so that
// a chain reads as the arithmetic it is rather than as six identical branches
// in the way of it. It is the calc of convert.go, one layer up and for the same
// reason.
//
// A failed step yields a zero rather than a nil, because the steps after it run
// anyway and a nil magnitude would be a panic where an error is already on its
// way back.
type steps struct{ err error }

func (s *steps) do(d *apd.Decimal, err error) *apd.Decimal {
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		return apd.New(0, 0)
	}
	return d
}

// ordered pairs two bounds computed on one unit, putting them the right way
// round.
//
// Every operation here is monotone in each bound, so the results arrive in
// order — unless a unit's factor is negative, which nothing in the catalogue is
// and [metrology.NewUnit] does not forbid. One comparison settles it for every
// operation at once, and saves each of them from having to argue that it cannot
// happen there.
func ordered(lo, hi metrology.Measurement) Range {
	if lo.Decimal().Cmp(hi.Decimal()) > 0 {
		lo, hi = hi, lo
	}
	return newRange(lo, hi)
}

// coversZero reports whether the interval contains zero, which is what makes a
// divisor unusable and a negative power unbounded.
func coversZero(r Range) bool {
	return r.lo.Sign() <= 0 && r.hi.Sign() >= 0
}

// abs returns |d| as a fresh decimal, sharing nothing with d (D3).
func abs(d *apd.Decimal) *apd.Decimal {
	out := new(apd.Decimal).Set(d)
	out.Negative = false
	return out
}
