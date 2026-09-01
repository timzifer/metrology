package metrology

import "github.com/cockroachdb/apd/v3"

// DefaultPrecision is the number of significant digits an operation keeps when
// no [Engine] says otherwise.
//
// Twenty digits is not a compromise between speed and accuracy, it is the point
// past which accuracy stops arriving: the physical constants a conversion chain
// is built from are known to far fewer digits, and no measurement carries more.
// See O2 in CONCEPT.md for the measurement behind the number.
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
}

// NewEngine returns an Engine computing with the given number of significant
// digits. A precision of zero selects [DefaultPrecision].
func NewEngine(precision uint32) Engine {
	return Engine{precision: precision}
}

// Precision reports the number of significant digits this Engine keeps.
func (e Engine) Precision() uint32 {
	if e.precision == 0 {
		return DefaultPrecision
	}
	return e.precision
}

// context returns the rounding context for inexact operations.
func (e Engine) context() apd.Context {
	ctx := apd.BaseContext
	ctx.Precision = e.Precision()
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
