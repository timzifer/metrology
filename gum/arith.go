package gum

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
)

// To expresses the value on another scale. It is [Engine.To] with the default
// precision.
func (v Value) To(u metrology.Unit) (Value, error) { return Engine{}.To(v, u) }

// To expresses the value on another scale.
//
// The estimate converts as a point and every contribution as a span, which is
// not the same conversion: a point on an absolute scale carries the offset and
// a difference along it does not (D6). Both go through the core, so a π factor
// or an affine scale is handled once, where it is implemented.
func (e Engine) To(v Value, u metrology.Unit) (Value, error) {
	est, err := e.core.To(v.est, u)
	if err != nil {
		return Value{}, err
	}
	span, err := spanUnit(est)
	if err != nil {
		return Value{}, err
	}
	terms, err := e.rescale(v.terms, v.span, span)
	if err != nil {
		return Value{}, err
	}
	return Value{est: est, span: span, terms: terms}, nil
}

// Add returns the sum. It is [Engine.Add] with the default precision.
func (v Value) Add(o Value) (Value, error) { return Engine{}.Add(v, o) }

// Add returns the sum of two values, with the uncertainty of the sum.
//
// Contributions from the same input are added rather than combined in
// quadrature, which is what makes this a budget and not a guess: two values
// that came from one measurement move together, and the arithmetic knows it
// because they name the same [Source].
func (e Engine) Add(left, right Value) (Value, error) {
	return e.combine(left, right, metrology.Engine.Add, false)
}

// Sub returns the difference. It is [Engine.Sub] with the default precision.
func (v Value) Sub(o Value) (Value, error) { return Engine{}.Sub(v, o) }

// Sub returns the difference of two values, with the uncertainty of the
// difference.
//
// x.Sub(x) has no uncertainty at all, and that is the whole point of carrying a
// decomposition: the two contributions are the same input's, they cancel
// exactly, and nothing is left to combine. The interval layer answers the same
// question with a wider interval, on purpose (D15, D21).
func (e Engine) Sub(left, right Value) (Value, error) {
	return e.combine(left, right, metrology.Engine.Sub, true)
}

// combine is Add and Sub: the core decides what the estimate and its unit are,
// and the contributions follow with a sensitivity of ±1.
func (e Engine) combine(
	left, right Value,
	apply func(metrology.Engine, metrology.Measurement, metrology.Measurement) (metrology.Measurement, error),
	subtract bool,
) (Value, error) {
	est, err := apply(e.core, left.est, right.est)
	if err != nil {
		return Value{}, err
	}
	span, err := spanUnit(est)
	if err != nil {
		return Value{}, err
	}

	aligned, err := e.aligned(span, left, right)
	if err != nil {
		return Value{}, err
	}
	terms, err := e.merge(aligned[0], aligned[1], subtract)
	if err != nil {
		return Value{}, err
	}
	return Value{est: est, span: span, terms: terms}, nil
}

// Mul returns the product. It is [Engine.Mul] with the default precision.
func (v Value) Mul(o Value) (Value, error) { return Engine{}.Mul(v, o) }

// Mul returns the product of two values.
//
// The sensitivities are the other operand's estimate — ∂(xy)/∂x = y — so a
// contribution of x is scaled by y and the other way round. A value multiplied
// by itself keeps one contribution per input rather than two, and it comes out
// as 2x·u(x): the product rule, arrived at by the same merge that makes a sum
// of correlated values right.
func (e Engine) Mul(left, right Value) (Value, error) {
	est, err := e.core.Mul(left.est, right.est)
	if err != nil {
		return Value{}, err
	}
	fromLeft, err := e.scale(left.terms, right.est.Decimal())
	if err != nil {
		return Value{}, err
	}
	fromRight, err := e.scale(right.terms, left.est.Decimal())
	if err != nil {
		return Value{}, err
	}
	terms, err := e.merge(fromLeft, fromRight, false)
	if err != nil {
		return Value{}, err
	}
	// A product of two spans is a span, so its own scale is the one its
	// contributions are read on — there is no interval unit to look up.
	return Value{est: est, span: est.Unit(), terms: terms}, nil
}

// Div returns the quotient. It is [Engine.Div] with the default precision.
func (v Value) Div(o Value) (Value, error) { return Engine{}.Div(v, o) }

// Div returns the quotient of two values.
//
// ∂(x/y)/∂x is 1/y and ∂(x/y)/∂y is −x/y², written here as −z/y so that the
// quotient the core has already computed does the work of the square. A divisor
// whose estimate is zero is refused by the core before any of this runs.
func (e Engine) Div(left, right Value) (Value, error) {
	est, err := e.core.Div(left.est, right.est)
	if err != nil {
		return Value{}, err
	}
	divisor := right.est.Decimal()

	var s steps
	fromLeft := make([]term, len(left.terms))
	for i := range left.terms {
		fromLeft[i] = newTerm(left.terms[i].src,
			s.do(e.bare(metrology.Engine.Div, &left.terms[i].c, divisor)))
	}
	fromRight := make([]term, len(right.terms))
	for i := range right.terms {
		scaled := s.do(e.bare(metrology.Engine.Mul, est.Decimal(), &right.terms[i].c))
		fromRight[i] = newTerm(right.terms[i].src,
			neg(s.do(e.bare(metrology.Engine.Div, scaled, divisor))))
	}
	if s.err != nil {
		return Value{}, s.err
	}

	terms, err := e.merge(fromLeft, fromRight, false)
	if err != nil {
		return Value{}, err
	}
	return Value{est: est, span: est.Unit(), terms: terms}, nil
}

// Pow raises the value to the n-th power. It is [Engine.Pow] with the default
// precision.
func (v Value) Pow(n int) (Value, error) { return Engine{}.Pow(v, n) }

// Pow raises the value to the n-th power.
//
// The sensitivity is n·xⁿ⁻¹, and the power before the last one is where it
// comes from — so the loop that raises the magnitude produces the derivative on
// the way past, and nothing is differentiated symbolically. The unit is
// [metrology.Unit.Pow], which is exact and spells the result the way the rest
// of the library spells it: m⁻² and not 1/(m·m).
//
// The magnitude rounds once per multiplication, where a closed form would round
// once. That is the trade [uncertainty.Range.Pow] makes for the same reason:
// the core has no power over a magnitude, and a chain of exact multiplications
// with one rounding each is what it does have.
//
// The zeroth power is the exact dimensionless one: x⁰ is 1 however uncertain x
// is, and a point on a scale has no power at all (D6).
func (e Engine) Pow(v Value, n int) (Value, error) {
	unit, err := v.est.Unit().Pow(n)
	if err != nil {
		return Value{}, err
	}
	if n == 0 {
		return Exactly(unit.Of(1)), nil
	}

	magnitude := v.est.Decimal()
	absolute := n
	if absolute < 0 {
		absolute = -absolute
	}

	// below is x^(|n|−1) and raised is x^|n|.
	var s steps
	below := apd.New(1, 0)
	for i := 1; i < absolute; i++ {
		below = s.do(e.bare(metrology.Engine.Mul, below, magnitude))
	}
	raised := s.do(e.bare(metrology.Engine.Mul, below, magnitude))

	// For a positive power the estimate is x^n and the derivative n·x^(n−1).
	// For a negative one they are the reciprocals: 1/x^|n| and −|n|/x^(|n|+1).
	estimate, derivative := raised, s.do(e.bare(metrology.Engine.Mul, apd.New(int64(n), 0), below))
	if n < 0 {
		estimate = s.do(e.bare(metrology.Engine.Div, apd.New(1, 0), raised))
		derivative = s.do(e.bare(metrology.Engine.Div, apd.New(int64(n), 0),
			s.do(e.bare(metrology.Engine.Mul, raised, magnitude))))
	}
	if s.err != nil {
		return Value{}, s.err
	}

	terms, err := e.scale(v.terms, derivative)
	if err != nil {
		return Value{}, err
	}
	return Value{est: unit.OfDecimal(estimate), span: unit, terms: terms}, nil
}

// Partial is one input's sensitivity in [Value.Apply]: the value the model
// depends on, and ∂f/∂x at the estimate.
type Partial struct {
	// Of is the input quantity.
	Of Value

	// Derivative is ∂f/∂x, as a measurement. Its unit times the input's span
	// unit has to be the span unit of the result, and the core says so if it
	// is not — which is the whole reason the derivative is a measurement here
	// and not a number.
	Derivative metrology.Measurement
}

// Apply propagates uncertainty through a model this package cannot
// differentiate. It is [Engine.Apply] with the default precision.
func Apply(y metrology.Measurement, partials ...Partial) (Value, error) {
	return Engine{}.Apply(y, partials...)
}

// Apply propagates uncertainty through a model this package cannot
// differentiate: a flow coefficient, a calibration polynomial, a steam table.
//
// The caller supplies the estimate y = f(x₁ … xₙ) and each ∂f/∂xᵢ, and the
// contributions are formed and merged exactly as the arithmetic above forms
// them — including the merge, so a model naming one input twice is handled
// correctly rather than double-counted.
//
// There is deliberately no automatic differentiation. It is a different design
// with a different type, and a derivative the caller computed and can cite in a
// document beats one this library inferred.
func (e Engine) Apply(y metrology.Measurement, partials ...Partial) (Value, error) {
	span, err := spanUnit(y)
	if err != nil {
		return Value{}, err
	}
	terms := []term{}
	for _, p := range partials {
		contributions := make([]term, len(p.Of.terms))
		for i := range p.Of.terms {
			product, err := e.core.Mul(p.Derivative, p.Of.span.OfDecimal(&p.Of.terms[i].c))
			if err != nil {
				return Value{}, err
			}
			onSpan, err := e.core.To(product, span)
			if err != nil {
				return Value{}, err
			}
			contributions[i] = newTerm(p.Of.terms[i].src, onSpan.Decimal())
		}
		if terms, err = e.merge(terms, contributions, false); err != nil {
			return Value{}, err
		}
	}
	return Value{est: y, span: span, terms: terms}, nil
}

// aligned expresses both operands' contributions on the scale the result is
// read on.
//
// The two are one step and not two because the failure is one failure: a
// contribution that cannot be converted onto the result's span, whichever
// operand it came from. Which operand that is depends on which of the two
// scales the core read the result on, and a caller has both in front of it.
func (e Engine) aligned(span metrology.Unit, operands ...Value) ([][]term, error) {
	out := make([][]term, len(operands))
	for i, operand := range operands {
		terms, err := e.rescale(operand.terms, operand.span, span)
		if err != nil {
			return nil, err
		}
		out[i] = terms
	}
	return out, nil
}

// rescale expresses every contribution on another span unit.
//
// Where the two units are one scale there is nothing to do, and skipping the
// conversion is not an optimisation: a conversion into the unit a magnitude
// already holds would round it to the engine's precision (D4).
func (e Engine) rescale(terms []term, from, to metrology.Unit) ([]term, error) {
	if from.Equal(to) {
		return terms, nil
	}
	out := make([]term, len(terms))
	for i := range terms {
		converted, err := e.core.To(from.OfDecimal(&terms[i].c), to)
		if err != nil {
			return nil, err
		}
		out[i] = newTerm(terms[i].src, converted.Decimal())
	}
	return out, nil
}

// scale multiplies every contribution by one magnitude — the sensitivity a
// product gives the other operand's contributions.
func (e Engine) scale(terms []term, by *apd.Decimal) ([]term, error) {
	var s steps
	out := make([]term, len(terms))
	for i := range terms {
		out[i] = newTerm(terms[i].src, s.do(e.bare(metrology.Engine.Mul, &terms[i].c, by)))
	}
	return out, s.err
}

// merge puts two contribution lists together, adding the coefficients of any
// input that appears in both — subtracting them where subtract says so.
//
// This is where correlation happens, and it is the only place. Two values that
// share an input share a [Source], the coefficients meet here, and a difference
// of two values built from one measurement cancels to nothing at all. Nothing
// rounds outward on the way: cancellation is the property this package exists
// to get right, and a coefficient nudged conservatively would destroy it.
func (e Engine) merge(left, right []term, subtract bool) ([]term, error) {
	var s steps
	out := make([]term, 0, len(left)+len(right))
	out = append(out, left...)

	for i := range right {
		c := &right[i].c
		if subtract {
			c = neg(c)
		}
		if at := indexOf(out, right[i].src); at >= 0 {
			out[at] = newTerm(out[at].src, s.do(e.bare(metrology.Engine.Add, &out[at].c, c)))
			continue
		}
		out = append(out, newTerm(right[i].src, c))
	}
	return out, s.err
}

// indexOf finds an input in a contribution list, or reports −1.
//
// A linear scan, because a budget has inputs a person can name: ten of them is
// a large model, and a map keyed by identity would cost more than it saves and
// would have to answer what its iteration order means for a printed budget.
func indexOf(terms []term, src Source) int {
	for i := range terms {
		if terms[i].src.same(src) {
			return i
		}
	}
	return -1
}

// bare runs two magnitudes through one of the core's operations on the
// dimensionless scale and hands back the magnitude of the result.
//
// It is the idiom of the interval layer and it is here for the same reason: an
// apd.Context built in this package would be a second rounding policy beside
// D9. The one exception is the square root, which the core does not have.
func (e Engine) bare(
	apply func(metrology.Engine, metrology.Measurement, metrology.Measurement) (metrology.Measurement, error),
	a, b *apd.Decimal,
) (*apd.Decimal, error) {
	m, err := apply(e.core, scalar.OfDecimal(a), scalar.OfDecimal(b))
	if err != nil {
		return nil, err
	}
	return m.Decimal(), nil
}

// steps accumulates the first error of a chain of magnitude operations, so that
// a chain reads as the arithmetic it is rather than as six identical branches
// in the way of it. It is the steps of the interval layer, one package over and
// for the same reason.
//
// A failed step yields a zero rather than a nil, because the steps after it run
// anyway and a nil magnitude would be a panic where an error is already on its
// way back.
type steps struct{ err error }

func (s *steps) do(d *apd.Decimal, err error) *apd.Decimal {
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		return apd.New(0, 0)
	}
	return d
}

// measure is do for an operation of the core that yields a measurement.
func (s *steps) measure(m metrology.Measurement, err error) metrology.Measurement {
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		return scalar.Of(0)
	}
	return m
}

// neg returns −d as a fresh decimal, sharing nothing with d (D3).
func neg(d *apd.Decimal) *apd.Decimal {
	return new(apd.Decimal).Neg(d)
}
