package metrology

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/internal/pi"
)

// piGuard is how many digits past the engine's precision π is taken at.
//
// The crossing conversion of D20 rounds twice where D4 rounds once: at π, and
// at the division. Guard digits are what keep the first rounding from reaching
// the last place of the second — the reference test in convert_test.go computes
// every crossing conversion in the catalogue again at a far higher precision
// and requires the same answer.
const piGuard = 10

// calc accumulates the first error of a chain of decimal operations.
//
// apd reports a condition and an error per call. Threading that through six
// call sites by hand would put six identical branches in the way of the
// arithmetic; this keeps one.
type calc struct {
	err error
}

func (c *calc) do(_ apd.Condition, err error) {
	if c.err == nil {
		c.err = err
	}
}

// convert expresses a magnitude read on the from scale as one read on the to
// scale, both being units of the same dimension and kind (the callers check).
//
// D4: the conversion is
//
//	v' = ((v + offset_from) · num_from · den_to) / (den_from · num_to) − offset_to
//
// arranged so that everything before the division is exact and there is exactly
// one division. Converting via the base unit instead would divide twice and
// round twice, which is how 760 torr stops being exactly 101325 Pa.
//
// An interval carries no offset: 5 K is 5 °C-worth of difference, and adding
// 273.15 to a difference is the classic affine bug this kind system exists to
// prevent (D6).
//
// D20 adds one term: the π exponents of the two units subtract. They cancel for
// every conversion that stays inside the π units — degree to arcsecond, gon to
// degree — which is therefore exactly as exact as it was before. Only a
// conversion that crosses between a π unit and a π-free one puts a number in
// place of π, and only that one rounds a second time.
func (e Engine) convert(op string, v *apd.Decimal, from, to Unit) (*apd.Decimal, error) {
	// Before the fast path below, not after it. Equal answers true for two zero
	// Units and for a zero Unit beside a constructed one that happens to render
	// the same way, so a check placed under the fast path would let both
	// through and return a magnitude read on no scale at all.
	if !from.hasScale() || !to.hasScale() {
		return nil, &NoScaleError{Op: op}
	}
	if from.Equal(to) {
		// Not an optimisation: a conversion of a value to its own unit must
		// return the value, and a division would round it to the engine's
		// precision first.
		return reduced(v), nil
	}

	var c calc
	exact := exactContext()
	numerator := new(apd.Decimal)
	if from.kind == Absolute {
		c.do(exact.Add(numerator, v, from.offset))
	} else {
		numerator.Set(v)
	}
	c.do(exact.Mul(numerator, numerator, from.num))
	c.do(exact.Mul(numerator, numerator, to.den))

	denominator := new(apd.Decimal)
	c.do(exact.Mul(denominator, from.den, to.num))

	if delta := int(from.pi) - int(to.pi); delta != 0 {
		// The direction π is rounded in depends on the sign of the quotient,
		// which is why this happens here and not before the multiplications.
		negative := (numerator.Sign() < 0) != (denominator.Sign() < 0)
		power, err := e.piPower(delta, negative)
		if err != nil {
			return nil, fmt.Errorf("metrology: %s: convert %s to %s: %w", op, from, to, err)
		}
		if delta > 0 {
			c.do(exact.Mul(numerator, numerator, power))
		} else {
			c.do(exact.Mul(denominator, denominator, power))
		}
	}

	result := new(apd.Decimal)
	rounding := e.context()
	c.do(rounding.Quo(result, numerator, denominator))

	if to.kind == Absolute {
		c.do(exact.Sub(result, result, to.offset))
	}
	if c.err != nil {
		return nil, fmt.Errorf("metrology: %s: convert %s to %s: %w", op, from, to, c.err)
	}
	// D9: without this the quotient is padded to the full precision with
	// zeros, and 2.5 bar serialises as 250000.0000000000000000 Pa.
	result.Reduce(result)
	return result, nil
}

// reduced returns a fresh copy of d with trailing zeros removed (D9, D3).
func reduced(d *apd.Decimal) *apd.Decimal {
	out := new(apd.Decimal)
	out.Reduce(d)
	return out
}

// piPower returns π raised to the absolute value of delta, at the engine's
// precision plus [piGuard] digits, rounded in the direction that keeps the
// engine's own mode true of the conversion's result (D20).
func (e Engine) piPower(delta int, negative bool) (*apd.Decimal, error) {
	precision := uint64(e.Precision()) + piGuard
	if precision > pi.MaxPrecision {
		return nil, &PrecisionError{
			Op:        "convert",
			Requested: e.Precision(),
			Max:       pi.MaxPrecision - piGuard,
		}
	}
	n := delta
	if n < 0 {
		n = -n
	}
	// The exponent is a difference of two int8 fields and the precision is
	// checked above, so neither of Power's refusals can fire here.
	power, _ := pi.Power(n, uint32(precision), piRounding(e.rounding, delta, negative))
	return power, nil
}

// piRounding picks the direction πⁿ is rounded in.
//
// D15 needs a converted bound to move outward, and it says so by handing this
// engine [apd.RoundFloor] or [apd.RoundCeiling]. A bound is only a bound if
// every rounding on the way to it goes the same way, and π is not always on the
// same side of the fraction: it multiplies the numerator for a positive
// exponent difference and the denominator for a negative one, and the quotient
// itself may be negative. Both flips invert which direction makes the result
// smaller, so the two are combined here rather than guessed at the call site.
//
// Any other mode — the default of D9 among them — takes π to nearest. There is
// nothing to preserve in that case: the mode describes one rounding of a point,
// not an enclosure, and the guard digits put the difference far below the last
// place reported.
func piRounding(mode apd.Rounder, delta int, negative bool) apd.Rounder {
	if mode != apd.RoundFloor && mode != apd.RoundCeiling {
		return apd.RoundHalfUp
	}
	// grows says the result rises with π: true when π multiplies a positive
	// quotient or divides a negative one.
	grows := (delta > 0) != negative
	if (mode == apd.RoundFloor) == grows {
		return apd.RoundFloor
	}
	return apd.RoundCeiling
}
