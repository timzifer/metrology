package metrology

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

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
func (e Engine) convert(v *apd.Decimal, from, to Unit) (*apd.Decimal, error) {
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

	result := new(apd.Decimal)
	rounding := e.context()
	c.do(rounding.Quo(result, numerator, denominator))

	if to.kind == Absolute {
		c.do(exact.Sub(result, result, to.offset))
	}
	if c.err != nil {
		return nil, fmt.Errorf("metrology: convert %s to %s: %w", from, to, c.err)
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
