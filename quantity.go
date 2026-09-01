package metrology

// Quantity names what is being measured, where the dimension does not say it.
//
// Most dimensions belong to exactly one quantity and need no tag: L¹ is a
// length and nothing else. Some do not. Frequency and radioactivity are both
// T⁻¹, absorbed dose and dose equivalent are both L²T⁻², a plane angle and a
// solid angle are both dimensionless — and treating 5 Hz as 5 Bq because the
// exponents agree would be a wrong number delivered with confidence.
//
// This is the second job D6 gives the kind, and it is deliberately *not* a
// kind: absolute-versus-interval and which-quantity are independent facts, and
// packing two independent facts into one word is what D5 took apart. A unit
// carries both, separately.
//
// The zero Quantity is untagged, which is the right answer for most units and
// the only possible answer for a computed one: a quotient of a force by an area
// is a pressure, but the arithmetic that produced it knows only the exponents
// (D6). An untagged quantity converts into any unit of its dimension, so a
// computed result can still be named.
type Quantity string

// String returns the tag, or "untagged" for the zero value.
func (q Quantity) String() string {
	if q == "" {
		return "untagged"
	}
	return string(q)
}

// compatible reports whether a magnitude of quantity q may be expressed as one
// of quantity other.
//
// Two tags must agree, but an untagged operand goes either way: it is a
// magnitude on a dimension with no claim about which quantity it is, and
// refusing to name it would make every computed result a dead end. The check
// only fires where both sides make a claim and the claims differ — 5 Hz asked
// for in becquerel.
func (q Quantity) compatible(other Quantity) bool {
	return q == "" || other == "" || q == other
}

// resolve returns the tag a result carries when two operands meet: the one that
// makes a claim, if either does.
func (q Quantity) resolve(other Quantity) Quantity {
	if q == "" {
		return other
	}
	return q
}
