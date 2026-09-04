package metrology

import (
	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

// Unit is a scale on which a magnitude is read: a dimension, a symbol, and the
// exact relation to the base unit of that dimension.
//
// A Unit is a value (D1) and immutable (D3). It is not comparable with ==
// because it holds decimals by pointer; use [Unit.Equal]. Those decimals are
// never written to, so copying a Unit — which happens on every Measurement
// copy — shares them safely.
//
// The relation to the base unit is an exact fraction plus an offset (D4):
//
//	base = (magnitude + offset) · numerator / denominator
//
// stored as three decimals rather than one pre-divided factor. 101325/760 is
// the definition of the torr and can be checked against the SI Brochure
// character by character; 133.32236842105263 is an approximation of it that
// rounds a second time on every conversion.
//
// # The zero value
//
// The zero Unit is not a scale. A concrete value type has a zero value no
// constructor produced (D1), and this one holds no factor and no offset, so
// there is nothing to read a magnitude on — every operation that would have to
// read one returns [ErrNoScale] rather than dereferencing what is not there.
// The zero [Measurement] carries it, and so does the Unit every failing
// constructor returns, which is how a caller who ignored one error reaches the
// arithmetic with it.
type Unit struct {
	dim      dimension.Dimension
	kind     Kind
	quantity Quantity
	sym      symbol.Symbol
	num      *apd.Decimal
	den      *apd.Decimal
	offset   *apd.Decimal

	// interval is the unit a difference of two absolute magnitudes is
	// expressed in: 25 °C − 20 °C is 5 K, not 5 °C. Optional; without it the
	// difference stays on the receiver's own scale.
	interval *Unit
}

// UnitDef describes a unit for [NewUnit].
//
// The factor and the offset are given as text, not as float64 or *apd.Decimal:
// text is what the catalogue of D8 holds, it is exact, and it is what an
// auditor compares against the standard.
type UnitDef struct {
	// Dimension of the quantity this unit measures.
	Dimension dimension.Dimension

	// Kind distinguishes a point on a scale from a span along it (D6). The
	// zero value, [Interval], is right for every unit without an offset.
	Kind Kind

	// Quantity names what is measured where the dimension does not say it:
	// the hertz and the becquerel are both T⁻¹ and are not the same thing.
	// Empty for every unit whose dimension belongs to one quantity only.
	Quantity Quantity

	// Symbol renders the unit and selects its prefixes.
	Symbol symbol.Symbol

	// Numerator and Denominator relate this unit to the base unit of its
	// dimension as an exact fraction. Empty means 1 — a base unit needs
	// neither.
	Numerator   string
	Denominator string

	// Offset is added to a magnitude before the factor is applied. Empty
	// means 0. Only an [Absolute] unit may carry one.
	Offset string

	// Interval is the unit a difference of two magnitudes on this scale is
	// expressed in: for °C that is K. Optional, and only meaningful for an
	// absolute unit.
	Interval *Unit
}

// NewUnit builds a unit from its definition.
func NewUnit(def UnitDef) (Unit, error) {
	num, err := decimalFromText("NewUnit", def.Numerator, 1)
	if err != nil {
		return Unit{}, err
	}
	den, err := decimalFromText("NewUnit", def.Denominator, 1)
	if err != nil {
		return Unit{}, err
	}
	offset, err := decimalFromText("NewUnit", def.Offset, 0)
	if err != nil {
		return Unit{}, err
	}
	if num.Sign() == 0 || den.Sign() == 0 {
		return Unit{}, ErrZeroFactor
	}
	if offset.Sign() != 0 && def.Kind != Absolute {
		return Unit{}, ErrOffsetKind
	}
	if def.Interval != nil {
		if def.Interval.kind != Interval {
			return Unit{}, ErrOffsetKind
		}
		if def.Interval.dim != def.Dimension {
			return Unit{}, &DimensionError{Op: "NewUnit", Want: def.Dimension, Got: def.Interval.dim}
		}
	}
	return Unit{
		dim:      def.Dimension,
		kind:     def.Kind,
		quantity: def.Quantity,
		sym:      def.Symbol,
		num:      num,
		den:      den,
		offset:   offset,
		interval: def.Interval,
	}, nil
}

// MustUnit is [NewUnit] for definitions that are known good at authoring time —
// the generated catalogue of D8, and tests. It panics where NewUnit would
// return an error.
func MustUnit(def UnitDef) Unit {
	u, err := NewUnit(def)
	if err != nil {
		panic("metrology: MustUnit: " + err.Error())
	}
	return u
}

// decimalFromText parses one field of a [UnitDef], defaulting an empty string.
func decimalFromText(op, text string, fallback int64) (*apd.Decimal, error) {
	if text == "" {
		return apd.New(fallback, 0), nil
	}
	d, _, err := apd.NewFromString(text)
	if err != nil {
		return nil, &SyntaxError{Op: op, Input: text, Err: err}
	}
	return d, nil
}

// Dimension reports what this unit measures.
func (u Unit) Dimension() dimension.Dimension { return u.dim }

// Kind reports whether magnitudes in this unit are points or spans (D6).
func (u Unit) Kind() Kind { return u.kind }

// Quantity reports what this unit measures where its dimension is shared by
// more than one quantity, and the empty [Quantity] where it is not.
func (u Unit) Quantity() Quantity { return u.quantity }

// Symbol returns the unit's symbol.
func (u Unit) Symbol() symbol.Symbol { return u.sym }

// String renders the unit's symbol without a prefix.
func (u Unit) String() string { return u.sym.String() }

// Factor returns the exact fraction relating this unit to the base unit of its
// dimension. The decimals are copies: a unit never hands out its own (D3).
//
// The zero Unit reports 1/1 — the identity [NewUnit] would have defaulted to —
// because an accessor has no error channel and a nil decimal would only move
// the dereference into the caller. It is not a claim that the zero Unit is a
// scale: the arithmetic still refuses it with [ErrNoScale], and that is where a
// caller finds out.
func (u Unit) Factor() (numerator, denominator *apd.Decimal) {
	if !u.hasScale() {
		return apd.New(1, 0), apd.New(1, 0)
	}
	return copyDecimal(u.num), copyDecimal(u.den)
}

// Offset returns the value added to a magnitude before the factor is applied,
// as a copy (D3). The zero Unit reports 0, for the reason [Unit.Factor] gives.
func (u Unit) Offset() *apd.Decimal {
	if !u.hasScale() {
		return apd.New(0, 0)
	}
	return copyDecimal(u.offset)
}

// IntervalUnit returns the unit a difference of two magnitudes on this scale is
// expressed in — K for °C — and whether one was declared.
func (u Unit) IntervalUnit() (Unit, bool) {
	if u.interval == nil {
		return Unit{}, false
	}
	return *u.interval, true
}

// Equal reports whether two units are the same scale: same dimension, same
// kind, same quantity, same symbol, and the same exact factor and offset.
//
// The fraction is compared as a value, not digit by digit, so 1/2 equals 5/10.
func (u Unit) Equal(other Unit) bool {
	return u.dim == other.dim &&
		u.kind == other.kind &&
		u.quantity == other.quantity &&
		u.sym.Equal(other.sym) &&
		sameScale(u, other)
}

// sameScale reports whether two units stand in the same relation to their base
// unit: the same offset, and the same fraction.
//
// The pointers are asked before the arithmetic is, and that is not a
// micro-optimisation of the comparison below it — it is what makes the
// comparison affordable where it is hot. Every same-unit addition, every
// comparison and every conversion into the unit a value already holds arrives
// here, and in each of those the two units are usually the *same* catalogue
// variable, whose decimals are therefore literally the same objects.
//
// D3 is what makes reading the pointers sound rather than merely lucky: nothing
// ever writes to a unit's decimals, so two units sharing one means they hold the
// same number for as long as both exist, not just at the instant of the
// comparison. Without D3 this would be a cache with no invalidation.
func sameScale(u, other Unit) bool {
	if u.num == other.num && u.den == other.den && u.offset == other.offset {
		// Two zero Units land here as well, on three nil pointers, and the
		// answer stays true: Equal reports that two units are the same scale,
		// not that either is a usable one.
		return true
	}
	if !u.hasScale() || !other.hasScale() {
		// Exactly one of them came from a constructor. They are not the same
		// scale, and the comparison below would dereference the other's
		// absent decimals to say so.
		return false
	}
	// Distinct decimals still have to be compared as numbers: 1/2 and 5/10 are
	// one scale written two ways, and a catalogue is free to write either.
	return u.offset.Cmp(other.offset) == 0 &&
		sameRatio(u.num, u.den, other.num, other.den)
}

// hasScale reports whether u came from a constructor.
//
// Every path that builds a Unit — NewUnit, times, byUnit, Pow, linearScale —
// sets all three decimals or none of them, so the numerator alone settles it.
// That invariant is what makes this a complete test rather than a heuristic;
// a new constructor that sets only some of the three would break it.
func (u Unit) hasScale() bool { return u.num != nil }

// sameRatio reports whether a/b and c/d are the same number, by cross
// multiplication — exact, and without a division that could round.
func sameRatio(a, b, c, d *apd.Decimal) bool {
	var left, right apd.Decimal
	ctx := exactContext()
	// Multiplication of two finite decimals cannot fail in an exact context.
	_, _ = ctx.Mul(&left, a, d)
	_, _ = ctx.Mul(&right, c, b)
	return left.Cmp(&right) == 0
}

// copyDecimal returns a Decimal sharing nothing with d (D3): apd.Decimal shares
// its coefficient on a plain struct copy, so a copy must go through Set.
func copyDecimal(d *apd.Decimal) *apd.Decimal {
	return new(apd.Decimal).Set(d)
}

// times returns the unit of a product, byUnit the unit of a quotient.
//
// The result carries no kind (D6): a product of a torque and an angle is not a
// torque, and a system that guesses here guesses wrong. Both operands are
// intervals — the callers check that — and an interval unit has no offset by
// construction, so the resulting scale is linear.
func (u Unit) times(other Unit) Unit {
	return Unit{
		dim:    dimension.Product(u.dim, other.dim),
		sym:    symbol.Product(u.sym, other.sym),
		num:    mulExact(u.num, other.num),
		den:    mulExact(u.den, other.den),
		offset: apd.New(0, 0),
	}
}

func (u Unit) byUnit(other Unit) Unit {
	return Unit{
		dim:    dimension.Quotient(u.dim, other.dim),
		sym:    symbol.Quotient(u.sym, other.sym),
		num:    mulExact(u.num, other.den),
		den:    mulExact(u.den, other.num),
		offset: apd.New(0, 0),
	}
}

// mulExact multiplies two finite decimals without rounding.
func mulExact(a, b *apd.Decimal) *apd.Decimal {
	d := new(apd.Decimal)
	ctx := exactContext()
	_, _ = ctx.Mul(d, a, b)
	return d
}
