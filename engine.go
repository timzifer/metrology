package metrology

import "github.com/cockroachdb/apd/v3"

// DefaultPrecision is the number of significant digits an operation keeps when
// no [Engine] says otherwise.
//
// Twenty digits is not a compromise between speed and accuracy, it is the point
// past which accuracy stops arriving: the physical constants a conversion chain
// is built from are known to far fewer digits, and no measurement carries more.
// See D9 in CONCEPT.md for the measurement behind the number.
const DefaultPrecision = 20

// Engine holds the precision a computation is carried out with (D9).
//
// Precision belongs to the computation, not to the value: a [Measurement]
// carries no context, so no rule is needed to decide whose precision wins when
// two of them meet. The zero Engine computes with [DefaultPrecision], which is
// what every method on Measurement uses.
//
//	e := metrology.NewEngine(60)
//	sum, err := e.Add(a, b)
type Engine struct {
	precision uint32

	// rounding is the mode the one inexact operation of a computation uses.
	// The empty Rounder is apd's own default and the policy of D9; only the
	// interval layer of D15 asks for another one.
	rounding apd.Rounder
}

// NewEngine returns an Engine computing with the given number of significant
// digits. A precision of zero selects [DefaultPrecision].
func NewEngine(precision uint32) Engine {
	return Engine{precision: precision}
}

// Rounding returns an Engine that rounds the way mode says, at the same
// precision.
//
// D9 gives this library one rounding policy, and for a point it is the right
// one. For the bound of an interval it is wrong: rounding a bound inward
// narrows the interval, and a narrowed interval can turn an overlap into a
// disjoint pair — a disagreement manufactured by the conversion and standing in
// no source. So the Range of the uncertainty package converts its lower bound
// with [apd.RoundFloor] and its upper one with [apd.RoundCeiling], and this
// method is the only reason it can (D15).
//
// The zero Engine is unchanged: a second rounding policy in a library that had
// exactly one, invisible unless asked for.
//
//	lo := metrology.Engine{}.Rounding(apd.RoundFloor)
func (e Engine) Rounding(mode apd.Rounder) Engine {
	e.rounding = mode
	return e
}

// Precision reports the number of significant digits this Engine keeps.
func (e Engine) Precision() uint32 {
	if e.precision == 0 {
		return DefaultPrecision
	}
	return e.precision
}

// context returns the rounding context for inexact operations.
//
// This is the one place a rounding mode reaches the arithmetic: the single
// division of a conversion (D4) and the products and quotients of D9 all round
// here and nowhere else, so [Engine.Rounding] needs no second implementation.
func (e Engine) context() apd.Context {
	ctx := apd.BaseContext
	ctx.Precision = e.Precision()
	ctx.Rounding = e.rounding
	return ctx
}

// exactContext returns the context for operations that cannot round: addition,
// subtraction and multiplication of decimals never need to, which is the whole
// point of keeping factors as fractions (D4) — only the single division rounds.
//
// It is a function rather than a package variable because a package variable
// would be global mutable state, and D7 does not allow any.
func exactContext() apd.Context {
	ctx := apd.BaseContext
	ctx.Precision = 0
	return ctx
}
