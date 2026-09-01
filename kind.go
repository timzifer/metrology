package metrology

// Kind distinguishes an absolute quantity from an interval one (D6).
//
// 20 °C is a point on a scale; 5 K is a distance along it. The distinction is
// not a property of the dimension — both are Θ¹ — and it is not decoration:
// which of the two a value is decides whether a conversion applies the scale's
// offset, and which additions are meaningful at all.
//
// Kind is held next to the dimension, never inside it (D5). The word packing
// exponents has no room left for a second, unrelated fact, and packing one in
// is what made the previous layout drop the marker on every multiplication.
type Kind uint8

const (
	// Interval is the zero value: a difference, a span, a rate — anything for
	// which addition is meaningful without further thought. Most quantities
	// are intervals; only scales with an arbitrary zero need the distinction.
	Interval Kind = iota

	// Absolute marks a point on a scale whose zero is a convention: 20 °C, an
	// absolute pressure, a date. Two of them may be subtracted but not added.
	Absolute
)

// String names the kind.
func (k Kind) String() string {
	if k == Absolute {
		return "absolute"
	}
	return "interval"
}

// addKind applies the addition rules of D6 and returns the kind of the sum.
//
//	absolute + interval → absolute   20 °C + 5 K = 25 °C
//	interval + interval → interval
//	interval + absolute → absolute   addition commutes; subtraction does not
//	absolute + absolute → error      the sum of two points is meaningless
func addKind(left, right Kind) (Kind, bool) {
	if left == Absolute && right == Absolute {
		return 0, false
	}
	if left == Absolute || right == Absolute {
		return Absolute, true
	}
	return Interval, true
}

// subKind applies the subtraction rules of D6 and returns the kind of the
// difference.
//
//	absolute − absolute → interval   25 °C − 20 °C = 5 K
//	absolute − interval → absolute   25 °C − 5 K = 20 °C
//	interval − interval → interval
//	interval − absolute → error      a point cannot be taken from a span
func subKind(left, right Kind) (Kind, bool) {
	switch {
	case left == Absolute && right == Absolute:
		return Interval, true
	case left == Absolute:
		return Absolute, true
	case right == Absolute:
		return 0, false
	default:
		return Interval, true
	}
}
