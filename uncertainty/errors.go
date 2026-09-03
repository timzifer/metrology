package uncertainty

import (
	"errors"
	"fmt"

	"github.com/timzifer/metrology"
)

// The error classes this package adds (D11).
//
// They sit beside the core's, they do not replace them: a range whose bounds
// are of different dimensions reports a [metrology.DimensionError] and matches
// [metrology.ErrDimension], because a caller that already handles the core's
// classes should not have to learn a second set for the same mistake. What is
// here is what interval arithmetic has and a point does not.
var (
	// ErrReversed reports a lower bound above the upper one. See
	// [ReversedError].
	ErrReversed = errors.New("the lower bound is above the upper bound")

	// ErrUnbounded reports a quotient or a reciprocal with no bound: the
	// divisor's interval covers zero, so the result runs to infinity in both
	// directions. See [UnboundedError].
	ErrUnbounded = errors.New("the result is unbounded")

	// ErrScale reports two bounds of one dimension read on two different
	// scales — 2.5 bar and 300000 Pa. See [ScaleError].
	ErrScale = errors.New("the bounds are on different scales")
)

// ReversedError names the two bounds that arrived the wrong way round.
type ReversedError struct {
	Op    string // the operation that failed: "Between", "Symmetric"
	Lower string // the bound given as the lower one
	Upper string // the bound given as the upper one
}

func (e *ReversedError) Error() string {
	return fmt.Sprintf("uncertainty: %s: the lower bound %s is above the upper bound %s", e.Op, e.Lower, e.Upper)
}

// Is reports that every ReversedError matches [ErrReversed].
func (e *ReversedError) Is(target error) bool { return target == ErrReversed }

// UnboundedError names the interval that covers zero.
//
// Reporting a bound for such a quotient would be a lie about the data: the
// quotient runs to infinity in both directions, and there is no pair of finite
// bounds that encloses it. A caller who knows the divisor is really one-signed
// says so by narrowing the divisor, not by reading a made-up bound.
type UnboundedError struct {
	Op      string // the operation that failed: "Div", "Pow"
	Divisor string // the interval that covers zero, in its own units
}

func (e *UnboundedError) Error() string {
	return fmt.Sprintf("uncertainty: %s: the divisor %s covers zero, so the result has no bound", e.Op, e.Divisor)
}

// Is reports that every UnboundedError matches [ErrUnbounded].
func (e *UnboundedError) Is(target error) bool { return target == ErrUnbounded }

// ScaleError names the two scales a range was asked to hold at once.
//
// It is a separate class from [metrology.DimensionError] because the dimensions
// match: the numbers would convert, and that is exactly what makes the mistake
// worth reporting rather than guessing at.
type ScaleError struct {
	Op    string         // the operation that failed: "Between"
	Lower metrology.Unit // the scale of the lower bound
	Upper metrology.Unit // the scale of the upper bound
}

func (e *ScaleError) Error() string {
	return fmt.Sprintf("uncertainty: %s: a range holds one scale, got %s and %s; convert one of them first",
		e.Op, e.Lower, e.Upper)
}

// Is reports that every ScaleError matches [ErrScale].
func (e *ScaleError) Is(target error) bool { return target == ErrScale }
