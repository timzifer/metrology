// Package pi holds π to more digits than any [apd.Context] in this module can
// ask for, and hands it out as an enclosure rather than as a value.
//
// It exists for D20. A unit factor is num/den·πᵖ, and the exponents of two
// units subtract on conversion, so a conversion that stays inside the π units —
// degree to arcsecond, gon to degree — never needs a digit of π. Only one that
// crosses between a π unit and a π-free one does, and that is the single place
// in this library where an exact factor meets a number that has no exact form.
//
// The digits are a constant and not a computation. Computing them per
// conversion is too slow to consider, computing them once needs a cache, and a
// cache is the global mutable state D7 forbids — there is nowhere in this
// library to keep one. What makes a constant trustworthy is the same thing that
// makes a catalogue factor trustworthy (D4): a check against its source.
// pi_test.go recomputes every digit from Machin's formula in math/big rationals
// and compares them character by character.
//
// [Power] rounds the way it is asked rather than the way D9 rounds. That is
// what lets an interval bound take the direction that widens it (D15): πᵏ
// rounded down is below πᵏ, and a product with it stays below the true product.
package pi

import (
	"errors"

	"github.com/cockroachdb/apd/v3"
)

// MaxPrecision is the largest precision [Power] serves.
//
// The constant below holds ten digits more, so a caller working with guard
// digits still gets an answer rather than a truncation it cannot see.
const MaxPrecision = 1000

// MaxExponent is the largest exponent [Power] accepts: the widest difference
// two int8 unit exponents can have. It is what makes the multiplication below
// unable to overflow, because π²⁵⁴ is a number with 127 digits before the
// point and the context's exponent range reaches far past that.
const MaxExponent = 254

// ErrPrecision is returned by [Power] for a precision above [MaxPrecision].
//
// The alternative is to serve the request with the digits that exist, which
// would return fewer correct digits than the caller asked for without saying
// so. A conversion that is quietly less precise than its engine promises is
// the failure this library is built to prevent.
var ErrPrecision = errors.New("metrology/internal/pi: precision above MaxPrecision")

// ErrExponent is returned by [Power] for an exponent outside 1…[MaxExponent].
// Callers pass the absolute value of a difference of two unit exponents and
// handle the zero case — where nothing needs computing — before they get here.
var ErrExponent = errors.New("metrology/internal/pi: exponent out of range")

// digits is π, taken from the computation in pi_test.go and checked against
// it there. Ten digits past MaxPrecision, so a caller with guard digits is
// still served exactly rather than truncated.
const digits = "3." +
	"141592653589793238462643383279502884197169399375105820974944" +
	"592307816406286208998628034825342117067982148086513282306647" +
	"093844609550582231725359408128481117450284102701938521105559" +
	"644622948954930381964428810975665933446128475648233786783165" +
	"271201909145648566923460348610454326648213393607260249141273" +
	"724587006606315588174881520920962829254091715364367892590360" +
	"011330530548820466521384146951941511609433057270365759591953" +
	"092186117381932611793105118548074462379962749567351885752724" +
	"891227938183011949129833673362440656643086021394946395224737" +
	"190702179860943702770539217176293176752384674818467669405132" +
	"000568127145263560827785771342757789609173637178721468440901" +
	"224953430146549585371050792279689258923542019956112129021960" +
	"864034418159813629774771309960518707211349999998372978049951" +
	"059731732816096318595024459455346908302642522308253344685035" +
	"261931188171010003137838752886587533208381420617177669147303" +
	"598253490428755468731159562863882353787593751957781857780532" +
	"17122680661300192787661119590921642019893809525720"

// Power returns πⁿ to prec significant digits, rounded the way mode says.
//
// Every step rounds in the requested direction, so a directed mode gives a
// bound and not merely an approximation: with [apd.RoundFloor] the result is at
// most πⁿ, with [apd.RoundCeiling] at least it. That is what D15 needs from a
// conversion that crosses out of the π units.
func Power(n int, prec uint32, mode apd.Rounder) (*apd.Decimal, error) {
	if n < 1 || n > MaxExponent {
		return nil, ErrExponent
	}
	if prec > MaxPrecision {
		return nil, ErrPrecision
	}
	ctx := apd.BaseContext
	ctx.Precision = prec
	ctx.Rounding = mode

	// The constant is a literal in this file and parses; rounding it to the
	// context's precision is inexact, which apd does not trap.
	d := new(apd.Decimal)
	_, _, _ = ctx.SetString(d, digits)

	// n is a difference of two unit exponents and therefore small, so the
	// obvious loop is shorter than the square-and-multiply that would replace
	// it — the same reasoning as powExact in the core.
	out := new(apd.Decimal).Set(d)
	for i := 1; i < n; i++ {
		// Bounded by MaxExponent, so this cannot overflow and the only
		// condition it raises is the inexactness that is the point of it.
		_, _ = ctx.Mul(out, out, d)
	}
	return out, nil
}
