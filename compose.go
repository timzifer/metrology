package metrology

import (
	"strconv"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/dimension"
)

// MaxPower is the largest power [Unit.Pow] accepts, in either direction.
//
// A dimension holds seven int8 exponents (D5), so a power beyond this range
// cannot be represented at all — and no quantity comes within two orders of
// magnitude of it. A reader of unit expressions needs the same bound, to refuse
// a power before computing it: raising a unit to the 127th and that result to
// the 127th again is a factor with sixty million digits, written in fourteen
// characters.
const MaxPower = 127

// Times returns the unit of a product: the newton times the metre is N·m.
//
// This is the unit half of [Measurement.Mul], available on its own for the
// cases where a unit has to be built before there is a magnitude to put on it —
// a parser reading "N·m", or a program naming the result of a computation.
//
// The result carries neither kind nor quantity (D6): a product of a force and a
// length is not a torque until someone says so, and a system that guesses here
// guesses wrong. A point on a scale has no product at all, and saying so is
// what the kind is for.
func (u Unit) Times(other Unit) (Unit, error) {
	if err := intervalUnits("Times", u, other); err != nil {
		return Unit{}, err
	}
	return u.times(other), nil
}

// Per returns the unit of a quotient: the metre per the second is m/s.
//
// It is [Unit.Times] the other way round, and drops kind and quantity for the
// same reason.
func (u Unit) Per(other Unit) (Unit, error) {
	if err := intervalUnits("Per", u, other); err != nil {
		return Unit{}, err
	}
	return u.byUnit(other), nil
}

// Pow returns the unit raised to the n-th power: the metre cubed is m³, the
// second to the −1 is s⁻¹, and any unit to the 0 is the dimensionless one.
//
// The factor is raised exactly, by repeated multiplication rather than by a
// logarithm: (101325/760)² is a fraction of two integers and stays one (D4).
// A π exponent is multiplied by n for the same reason and with the same
// exactness — the square degree is the degree squared (D20).
// As with [Unit.Times] the result carries neither kind nor quantity.
func (u Unit) Pow(n int) (Unit, error) {
	if u.kind == Absolute {
		return Unit{}, &KindError{
			Op: "Pow", Left: u.kind, Right: u.kind,
			Why: "a point on a scale has no power",
		}
	}
	if n < -MaxPower || n > MaxPower {
		return Unit{}, &RangeError{Op: "Pow", Value: strconv.Itoa(n), Type: "a dimension exponent"}
	}
	if n == 0 {
		// Every unit to the zeroth power is the same dimensionless one, and
		// the factor of the base is gone with the dimension: m⁰ is 1, not
		// 1 m⁰ worth of metres.
		return Unit{
			dim:    dimension.One,
			sym:    u.sym.Pow(0),
			num:    apd.New(1, 0),
			den:    apd.New(1, 0),
			offset: apd.New(0, 0),
		}, nil
	}
	num, den := powExact(u.num, n), powExact(u.den, n)
	if n < 0 {
		num, den = den, num
	}
	return Unit{
		dim:    u.dim.Pow(dimension.Exponent(n)),
		sym:    u.sym.Pow(n),
		num:    num,
		den:    den,
		pi:     int8(int(u.pi) * n),
		offset: apd.New(0, 0),
	}, nil
}

// powExact raises a finite decimal to the |n|-th power without rounding.
//
// The exponent is small — a dimension exponent — so the loop is shorter than
// the square-and-multiply that would replace it, and every step is an exact
// multiplication of two finite decimals (D4).
func powExact(d *apd.Decimal, n int) *apd.Decimal {
	if n < 0 {
		n = -n
	}
	out := apd.New(1, 0)
	for i := 0; i < n; i++ {
		out = mulExact(out, d)
	}
	return out
}

// intervalUnits rejects composing a scale whose zero is a convention (D6): the
// square of 20 °C is not 400 of anything, and neither is 20 °C per second.
func intervalUnits(op string, left, right Unit) error {
	if left.kind == Absolute || right.kind == Absolute {
		return &KindError{
			Op: op, Left: left.kind, Right: right.kind,
			Why: "a point on a scale has no product",
		}
	}
	return nil
}
