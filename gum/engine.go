package gum

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
)

// Engine holds the precision a budget is computed with (D9).
//
// It is the counterpart of [metrology.Engine] and it exists for the same
// reason: precision belongs to the computation, not to the value, so a [Value]
// carries none and no rule is needed to decide whose wins when two of them
// meet. The zero Engine computes with [metrology.DefaultPrecision], which is
// what every method on Value uses.
//
//	e := gum.NewEngine(34)
//	sum, err := e.Add(a, b)
//
// Like the interval layer's engine it carries no rounding mode, and for the
// opposite reason: there, every bound has to round outward or it is not a
// bound; here, every contribution has to round to nearest or a cancellation
// stops cancelling. The one directed rounding is the square root in
// [Engine.Uncertainty], which rounds up because a combined uncertainty that
// rounded down would understate itself.
type Engine struct {
	core metrology.Engine
}

// NewEngine returns an Engine computing with the given number of significant
// digits. A precision of zero selects [metrology.DefaultPrecision].
func NewEngine(precision uint32) Engine {
	return Engine{core: metrology.NewEngine(precision)}
}

// Precision reports the number of significant digits this Engine keeps.
func (e Engine) Precision() uint32 { return e.core.Precision() }

// sqrt is the one operation this package computes for itself.
//
// Everything else goes through the core, so that D9 stays the single rounding
// policy of this library; a square root has to be built here because the core
// has none to borrow. The mode is a parameter because the two callers want
// different ones: a combined standard uncertainty rounds up, so that it never
// understates itself in its last digit, while a divisor of a Type B statement
// rounds to nearest like every other magnitude — it is an input to the budget,
// not the number the budget reports.
func (e Engine) sqrt(x *apd.Decimal, mode apd.Rounder) *apd.Decimal {
	ctx := apd.BaseContext
	ctx.Precision = e.Precision()

	// Sqrt refuses two operands, a negative one and a NaN, and neither reaches
	// here: the callers pass a sum of squares, a variance, and 1 − ρ² for a ρ
	// already checked against ±1 — and a NaN among the magnitudes has failed
	// one of the core's operations long before, because those do trap it.
	out := new(apd.Decimal)
	condition, _ := ctx.Sqrt(out, x)
	if mode == apd.RoundCeiling && condition.Inexact() {
		// apd's square root is correctly rounded to nearest and does not read
		// the context's rounding mode at all — measured, not assumed: all
		// three modes return the same digits. So the direction is applied
		// here, as one unit in the last place, and only where the root did not
		// come out exact. A combined uncertainty is then at least what it
		// should be, which is the only direction that does not mislead.
		_, _ = ctx.Add(out, out, apd.New(1, out.Exponent))
	}
	// D9: without this a root that came out exact is padded to the full
	// precision with zeros, and 0.5 prints as 0.50000000000000000000.
	out.Reduce(out)
	return out
}
