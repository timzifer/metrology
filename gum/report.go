package gum

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/uncertainty"
)

// Uncertainty returns the combined standard uncertainty. It is
// [Engine.Uncertainty] with the default precision.
func (v Value) Uncertainty() (metrology.Measurement, error) { return Engine{}.Uncertainty(v) }

// Uncertainty returns the combined standard uncertainty u_c: the root of the
// sum of the squared contributions (JCGM 100 §5.1).
//
// There is no covariance term to add, and that is the whole design rather than
// a simplification: every contribution names an independent input, because a
// declared correlation was decomposed into independent inputs when the values
// were built. Correlated quantities are handled by the merge in the arithmetic,
// where the coefficients meet, not by a matrix here.
//
// It is a span (D6) — an uncertainty is a distance along a scale and never a
// point on it — so an uncertainty beside 20 °C comes back in kelvin. The root
// rounds up: understating a combined uncertainty in its last digit is the one
// direction that misleads.
func (e Engine) Uncertainty(v Value) (metrology.Measurement, error) {
	if len(v.terms) == 1 {
		// The root of one square is the magnitude itself, and taking it
		// through a squaring and a root would round twice for no reason: a
		// value with one input reports exactly the uncertainty it was given.
		lone := abs(&v.terms[0].c)
		// D9: a coefficient that came out of a division carries the padding
		// zeros the division left on it, and a reported uncertainty should not.
		lone.Reduce(lone)
		return v.span.OfDecimal(lone), nil
	}

	var s steps
	squares := apd.New(0, 0)
	for i := range v.terms {
		square := s.do(e.bare(metrology.Engine.Mul, &v.terms[i].c, &v.terms[i].c))
		squares = s.do(e.bare(metrology.Engine.Add, squares, square))
	}
	if s.err != nil {
		return metrology.Measurement{}, s.err
	}
	return v.span.OfDecimal(e.sqrt(squares, apd.RoundCeiling)), nil
}

// Expanded returns the interval y ± k·u_c. It is [Engine.Expanded] with the
// default precision.
func (v Value) Expanded[N metrology.Numeric](k N) (uncertainty.Range, error) {
	return Engine{}.Expanded(v, k)
}

// Expanded returns the interval y ± k·u_c, as a range of the interval layer.
//
// This is where the two layers meet, and the direction is deliberate: a budget
// produces a number, and the interval layer already has the text form, the
// parser and the outward rounding for reporting it. There is no way back — an
// [uncertainty.Range] states two bounds and claims no distribution, and reading
// it as a rectangular input here would invent a claim the data does not make.
//
// The coverage factor is the caller's. Where it comes from a confidence level
// it comes with degrees of freedom: [Value.EffectiveFreedom] computes ν_eff,
// and Table G.2 of the GUM turns that into k. This package ships no copy of
// that table, because a table of quantiles transcribed into a repository is a
// page of numbers with nothing to check them against — which is exactly what
// D4 refuses for a conversion factor.
func (e Engine) Expanded[N metrology.Numeric](v Value, k N) (uncertainty.Range, error) {
	u, err := e.Uncertainty(v)
	if err != nil {
		return uncertainty.Range{}, err
	}
	expanded, err := e.core.Mul(u, scalar.Of(k))
	if err != nil {
		return uncertainty.Range{}, err
	}
	// Mul composes the units, so the product of a span and a bare number is
	// spelled "K·1" until it is put back on the scale it never left.
	return uncertainty.Symmetric(v.est, u.Unit().OfDecimal(expanded.Decimal()))
}

// EffectiveFreedom returns the effective degrees of freedom. It is
// [Engine.EffectiveFreedom] with the default precision.
func (v Value) EffectiveFreedom() (int, error) { return Engine{}.EffectiveFreedom(v) }

// EffectiveFreedom returns the effective degrees of freedom of the combined
// uncertainty, by the Welch-Satterthwaite formula (JCGM 100 §G.4.1):
//
//	ν_eff = u_c⁴ / Σ (cᵢ⁴ / νᵢ)
//
// It is what a confidence level needs: with ν_eff, Table G.2 of the GUM gives
// the coverage factor to hand to [Value.Expanded]. An input with [Infinite]
// degrees of freedom contributes nothing to the sum, which is why a budget of
// nothing but Type B inputs comes back [Infinite] — there is no estimate of an
// estimate anywhere in it.
//
// The result is truncated towards zero, as §G.4.1 says to. A ν_eff too large
// for an int is reported as [Infinite], because the t-factor at that end of the
// table is the normal one and has been for some thousands of degrees of freedom.
func (e Engine) EffectiveFreedom(v Value) (int, error) {
	u, err := e.Uncertainty(v)
	if err != nil {
		return 0, err
	}
	var s steps
	fourth := e.fourthPower(&s, u.Decimal())

	sum := apd.New(0, 0)
	for i := range v.terms {
		if v.terms[i].src.freedom == Infinite {
			continue
		}
		contribution := e.fourthPower(&s, &v.terms[i].c)
		share := s.do(e.bare(metrology.Engine.Div, contribution,
			apd.New(int64(v.terms[i].src.freedom), 0)))
		sum = s.do(e.bare(metrology.Engine.Add, sum, share))
	}
	if s.err != nil {
		return 0, s.err
	}
	if sum.Sign() == 0 {
		return Infinite, nil
	}

	effective := s.do(e.bare(metrology.Engine.Div, fourth, sum))
	if s.err != nil {
		return 0, s.err
	}

	ctx := apd.BaseContext
	ctx.Precision = e.Precision()
	truncated := new(apd.Decimal)
	// Floor refuses nothing a division can produce: an infinity floors to
	// itself, and the Int64 below is what reports it.
	_, _ = ctx.Floor(truncated, effective)

	whole, err := truncated.Int64()
	if err != nil || int64(int(whole)) != whole {
		return Infinite, nil
	}
	return int(whole), nil
}

// fourthPower squares a magnitude twice, through the core.
func (e Engine) fourthPower(s *steps, d *apd.Decimal) *apd.Decimal {
	square := s.do(e.bare(metrology.Engine.Mul, d, d))
	return s.do(e.bare(metrology.Engine.Mul, square, square))
}

// Contribution is one input's share of a value's uncertainty: the row of an
// uncertainty budget.
type Contribution struct {
	// Source is the input it comes from, with the name and the degrees of
	// freedom it was given.
	Source Source

	// Value is (∂y/∂x)·u(x): what this input alone contributes to the
	// uncertainty of the result, as a span on the result's scale. Its sign is
	// the sensitivity's, and it is worth reading — two contributions of
	// opposite sign are what cancel when the inputs are the same.
	Value metrology.Measurement
}

// Contributions returns the uncertainty budget: one row per independent input,
// in the order the inputs first reached this value.
//
// It is a projection of what the value already holds and not a second
// computation, which is what makes it worth printing — the numbers in the table
// are the numbers the combination used.
func (v Value) Contributions() []Contribution {
	rows := make([]Contribution, len(v.terms))
	for i := range v.terms {
		rows[i] = Contribution{
			Source: v.terms[i].src,
			Value:  v.span.OfDecimal(&v.terms[i].c),
		}
	}
	return rows
}

// String renders the estimate and its combined standard uncertainty, with the
// unit on both sides: "3.7 mm ± 0.05 mm".
//
// Repeating the unit is what tells this apart from the ± form of the interval
// layer, where "3.7 ± 0.05 mm" is a range with two bounds and no distribution
// behind it. Two spellings that read back as different values must not look the
// same (D12), and a standard uncertainty and a worst-case interval are as
// different as this library gets.
//
// A value whose uncertainty is not a number — an estimate that is one, say —
// prints as its estimate alone rather than as an arithmetic accident.
func (v Value) String() string {
	u, err := v.Uncertainty()
	if err != nil {
		return v.est.String()
	}
	return v.est.String() + " ± " + u.String()
}
