package metrology

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// Add returns the sum of two measurements, at [DefaultPrecision].
//
// The kind rules of D6 decide what the sum is:
//
//	20 °C + 5 K  = 25 °C   absolute + interval → absolute
//	5 K  + 3 K   = 8 K     interval + interval → interval
//	20 °C + 5 °C = error   two points on a scale have no sum
//
// The result is expressed in the unit of the absolute operand, or in the
// receiver's unit when both are intervals.
func (m Measurement) Add(other Measurement) (Measurement, error) {
	return Engine{}.Add(m, other)
}

// Add is [Measurement.Add] at this engine's precision (D9).
func (e Engine) Add(left, right Measurement) (Measurement, error) {
	if err := sameQuantity("Add", left, right); err != nil {
		return Measurement{}, err
	}
	kind, ok := addKind(left.unit.kind, right.unit.kind)
	if !ok {
		return Measurement{}, &KindError{
			Op: "Add", Left: left.unit.kind, Right: right.unit.kind,
			Why: "the sum of two points on a scale is not a point on it",
		}
	}
	// Addition commutes, so the absolute operand — if there is one — decides
	// the unit of the result, and the other is converted onto its scale.
	if kind == Absolute && right.unit.kind == Absolute {
		left, right = right, left
	}
	return e.combine("Add", left, right, (*apd.Context).Add, left.unit)
}

// Sub returns the difference of two measurements, at [DefaultPrecision].
//
// The kind rules of D6 decide what the difference is:
//
//	25 °C − 20 °C = 5 K     absolute − absolute → interval
//	25 °C − 5 K   = 20 °C   absolute − interval → absolute
//	5 K  − 3 K    = 2 K     interval − interval → interval
//	5 K  − 20 °C  = error   a point cannot be taken from a span
//
// The difference of two absolute magnitudes is expressed in the interval unit
// declared for the receiver's scale — K for °C — or on the receiver's own scale
// where none is declared.
func (m Measurement) Sub(other Measurement) (Measurement, error) {
	return Engine{}.Sub(m, other)
}

// Sub is [Measurement.Sub] at this engine's precision (D9).
func (e Engine) Sub(left, right Measurement) (Measurement, error) {
	if err := sameQuantity("Sub", left, right); err != nil {
		return Measurement{}, err
	}
	kind, ok := subKind(left.unit.kind, right.unit.kind)
	if !ok {
		return Measurement{}, &KindError{
			Op: "Sub", Left: left.unit.kind, Right: right.unit.kind,
			Why: "a point on a scale cannot be subtracted from a span along it",
		}
	}
	if kind == Interval && left.unit.kind == Absolute {
		// Both operands are points, so their difference is a span. The
		// subtraction happens on the linear scale underneath the receiver —
		// the offsets cancel — and the span is then read on the interval unit
		// the scale declares: 25 °C − 20 °C is 5 K.
		diff, err := e.combine("Sub", left, right, (*apd.Context).Sub, left.unit.linearScale())
		if err != nil {
			return Measurement{}, err
		}
		return e.To(diff, left.unit.intervalScale())
	}
	return e.combine("Sub", left, right, (*apd.Context).Sub, left.unit)
}

// combine converts right onto the scale of left, applies op, and labels the
// result with the given unit.
//
// The conversion target is the linear scale of the receiver, not the receiver
// itself: an interval operand must be converted without the offset, or 5 K
// turns into 278.15 K on its way into a Celsius sum (D6). The result unit
// carries the same factor as the receiver, so the arithmetic needs no second
// conversion.
func (e Engine) combine(
	op string,
	left, right Measurement,
	apply func(*apd.Context, *apd.Decimal, *apd.Decimal, *apd.Decimal) (apd.Condition, error),
	result Unit,
) (Measurement, error) {
	target := left.unit
	if right.unit.kind == Interval {
		target = left.unit.linearScale()
	}
	converted, err := e.convert(op, &right.val, right.unit, target)
	if err != nil {
		return Measurement{}, err
	}
	// An untagged operand takes on the quantity of the tagged one: adding an
	// untagged T⁻¹ to a frequency yields a frequency, and there is nothing
	// else it could yield.
	result.quantity = left.unit.quantity.resolve(right.unit.quantity)

	value := new(apd.Decimal)
	exact := exactContext()
	if _, err := apply(&exact, value, &left.val, converted); err != nil {
		return Measurement{}, fmt.Errorf("metrology: %s: %w", op, err)
	}
	return result.measurement(value), nil
}

// Mul returns the product of two measurements, at [DefaultPrecision].
//
// The result carries no kind and its unit is the product of both units (D6):
// 100 N times 2 m is 200 N·m, and it is not a torque until someone says so.
// Absolute magnitudes have no product at all — twice 20 °C is not 40 °C, and
// there is no reading of it that is.
func (m Measurement) Mul(other Measurement) (Measurement, error) {
	return Engine{}.Mul(m, other)
}

// Mul is [Measurement.Mul] at this engine's precision (D9).
func (e Engine) Mul(left, right Measurement) (Measurement, error) {
	if err := intervalUnits("Mul", left.unit, right.unit); err != nil {
		return Measurement{}, err
	}
	return e.scaleBy("Mul", left, right, (*apd.Context).Mul, left.unit.times(right.unit))
}

// Div returns the quotient of two measurements, at [DefaultPrecision].
//
//	100 N / 2 m² → 50 N/m²
//
// As with [Measurement.Mul] the result carries no kind, and its unit is derived
// from both operands.
func (m Measurement) Div(other Measurement) (Measurement, error) {
	return Engine{}.Div(m, other)
}

// Div is [Measurement.Div] at this engine's precision (D9).
func (e Engine) Div(left, right Measurement) (Measurement, error) {
	if err := intervalUnits("Div", left.unit, right.unit); err != nil {
		return Measurement{}, err
	}
	return e.scaleBy("Div", left, right, (*apd.Context).Quo, left.unit.byUnit(right.unit))
}

// scaleBy applies a multiplicative operation to two magnitudes.
//
// Unlike addition, no conversion happens first: both operands keep their unit
// and the result unit records the combination, so 100 N ÷ 2 m² is 50 N/m² with
// no factor lost anywhere. The operation rounds to the engine's precision,
// because a chain of exact products doubles its digit count at every step.
func (e Engine) scaleBy(
	op string,
	left, right Measurement,
	apply func(*apd.Context, *apd.Decimal, *apd.Decimal, *apd.Decimal) (apd.Condition, error),
	result Unit,
) (Measurement, error) {
	value := new(apd.Decimal)
	ctx := e.context()
	if _, err := apply(&ctx, value, &left.val, &right.val); err != nil {
		return Measurement{}, fmt.Errorf("metrology: %s: %w", op, err)
	}
	value.Reduce(value) // D9
	return result.measurement(value), nil
}

// Cmp compares two measurements of the same dimension and kind, returning -1,
// 0 or +1 as the receiver is less than, equal to or greater than other.
//
// The comparison is by value, not by representation: 1 bar and 100000 Pa
// compare equal, and so do 2.5 and 2.50.
func (m Measurement) Cmp(other Measurement) (int, error) {
	return Engine{}.Cmp(m, other)
}

// Cmp is [Measurement.Cmp] at this engine's precision (D9).
func (e Engine) Cmp(left, right Measurement) (int, error) {
	if err := sameQuantity("Cmp", left, right); err != nil {
		return 0, err
	}
	if left.unit.kind != right.unit.kind {
		return 0, &KindError{
			Op: "Cmp", Left: left.unit.kind, Right: right.unit.kind,
			Why: "a point on a scale and a span along it are not comparable",
		}
	}
	converted, err := e.convert("Cmp", &right.val, right.unit, left.unit)
	if err != nil {
		return 0, err
	}
	return left.val.Cmp(converted), nil
}

// Equal reports whether two measurements are the same quantity, whichever unit
// each is held in. A dimension or kind mismatch is not an error here, it is an
// answer: they are not equal.
func (m Measurement) Equal(other Measurement) bool {
	cmp, err := m.Cmp(other)
	return err == nil && cmp == 0
}

// sameQuantity is the runtime check that stands in for the compile error Go
// cannot give (D1/D11): the dimensions must match, and where both operands say
// which quantity they are, those must match too.
//
// The dimension is checked first, because it is the coarser mistake and the
// more useful message: adding a length to a pressure is a different kind of
// wrong from adding a frequency to a radioactivity.
func sameQuantity(op string, left, right Measurement) error {
	if left.unit.dim != right.unit.dim {
		return &DimensionError{Op: op, Want: left.unit.dim, Got: right.unit.dim}
	}
	if !left.unit.quantity.compatible(right.unit.quantity) {
		return &QuantityError{Op: op, Left: left.unit.quantity, Right: right.unit.quantity}
	}
	return nil
}

// linearScale returns the scale underneath a unit: the same factor, without the
// offset and without the kind.
//
// It is what an interval operand is converted onto before it meets an absolute
// one, and the scale a difference of two absolute magnitudes is computed on.
// It is derived, never the declared interval unit: the magnitudes being
// computed with are written in this unit's own numbers, and relabelling them
// without converting is how a factor goes missing.
func (u Unit) linearScale() Unit {
	if u.kind != Absolute {
		return u
	}
	return Unit{dim: u.dim, sym: u.sym, num: u.num, den: u.den, offset: apd.New(0, 0)}
}

// intervalScale returns the unit a difference of two magnitudes on this scale
// is read on: the declared counterpart — K for °C — or the linear scale
// underneath where none is declared.
func (u Unit) intervalScale() Unit {
	if u.interval != nil {
		return *u.interval
	}
	return u.linearScale()
}
