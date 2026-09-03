package uncertainty

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
)

// Engine holds the precision an interval computation is carried out with (D9).
//
// It is the counterpart of [metrology.Engine] and it exists for the same
// reason: precision belongs to the computation, not to the value, so a Range
// carries none and no rule is needed to decide whose wins when two of them
// meet. The zero Engine computes with [metrology.DefaultPrecision], which is
// what every method on Range uses.
//
//	e := uncertainty.NewEngine(34)
//	sum, err := e.Add(a, b)
//
// What it does not carry is a rounding mode. The mode is not the caller's to
// choose here: a lower bound rounds toward −∞ and an upper bound toward +∞, or
// the interval is not one (D15).
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

// floor is the engine a lower bound is computed with: every rounding goes
// toward −∞, so the bound never moves up into the interval.
func (e Engine) floor() metrology.Engine { return e.core.Rounding(apd.RoundFloor) }

// ceiling is the engine an upper bound is computed with: every rounding goes
// toward +∞.
func (e Engine) ceiling() metrology.Engine { return e.core.Rounding(apd.RoundCeiling) }
