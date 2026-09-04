package gum

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
)

// Correlated returns two values built from inputs known to be correlated. It is
// [Engine.Correlated] with the default precision.
func Correlated(a, b Input, correlation string) (Value, Value, error) {
	return Engine{}.Correlated(a, b, correlation)
}

// Correlated returns two values built from inputs with a declared correlation
// coefficient — two readings from one instrument, two lengths off one gauge.
//
// The correlation is resolved here and never again: the pair is decomposed into
// two independent sources, the first of which both values draw on. Everything
// downstream is then the ordinary arithmetic of independent inputs, and a
// covariance matrix consulted at combination time — which would mean a context
// object threaded through every call, or a registry (D7) — is not needed
// anywhere.
//
// The coefficient is given as text, exactly as a catalogue factor is (D4): it
// is an exact decimal in [−1, 1], and a float64 spelling of 0.8 is not that.
func (e Engine) Correlated(a, b Input, correlation string) (Value, Value, error) {
	rho, _, err := apd.NewFromString(correlation)
	if err != nil {
		return Value{}, Value{}, &metrology.SyntaxError{Op: "Correlated", Input: correlation, Err: err}
	}
	if rho.Form != apd.Finite || abs(rho).Cmp(apd.New(1, 0)) > 0 {
		return Value{}, Value{}, &InputError{
			Op: "Correlated", Name: b.Name, Why: "a correlation coefficient outside [−1, 1]",
		}
	}

	first, err := e.Of(a)
	if err != nil {
		return Value{}, Value{}, err
	}
	second, err := e.Of(b)
	if err != nil {
		return Value{}, Value{}, err
	}

	// The second value is rebuilt on the pair's two independent sources:
	// ρ of its uncertainty follows the first input, and the rest is its own.
	// That is the 2×2 Cholesky factor, written out.
	var s steps
	u := &second.terms[0].c
	shared := s.do(e.bare(metrology.Engine.Mul, rho, u))
	square := s.do(e.bare(metrology.Engine.Mul, rho, rho))
	rest := s.do(e.bare(metrology.Engine.Sub, apd.New(1, 0), square))
	own := s.do(e.bare(metrology.Engine.Mul, e.sqrt(rest, apd.RoundHalfUp), u))
	if s.err != nil {
		return Value{}, Value{}, s.err
	}

	second.terms = []term{
		newTerm(first.terms[0].src, shared),
		newTerm(second.terms[0].src, own),
	}
	return first, second, nil
}

// Sample evaluates an input from repeated observations. It is [Engine.Sample]
// with the default precision.
func Sample(name string, observations []metrology.Measurement) (Value, error) {
	return Engine{}.Sample(name, observations)
}

// Sample evaluates an input the way the GUM calls Type A: from n observations,
// with the arithmetic mean as the estimate and the experimental standard
// deviation of the mean as the standard uncertainty (JCGM 100 §4.2).
//
// The degrees of freedom are n − 1, which is the number this evaluation exists
// to produce: a Type B input is usually known well enough to be [Infinite], and
// a budget whose effective degrees of freedom mean anything has a Type A input
// somewhere in it.
//
// The mean is taken on the observations' own coordinates rather than by adding
// them, because the sum of two points on a scale is not a point on it (D6) — an
// affine map preserves means, so a mean of temperatures in degrees Celsius is
// the temperature it looks like. That is the argument [uncertainty.Range.Mid]
// makes for a midpoint, and it is the same one.
func (e Engine) Sample(name string, observations []metrology.Measurement) (Value, error) {
	if len(observations) < 2 {
		return Value{}, &InputError{
			Op: "Sample", Name: name, Why: "fewer than two observations, which have no standard deviation",
		}
	}

	span, err := spanUnit(observations[0])
	if err != nil {
		return Value{}, err
	}

	var s steps
	unit := observations[0].Unit()
	magnitudes := make([]*apd.Decimal, len(observations))
	sum := apd.New(0, 0)
	for i, observation := range observations {
		magnitudes[i] = s.measure(e.core.To(observation, unit)).Decimal()
		sum = s.do(e.bare(metrology.Engine.Add, sum, magnitudes[i]))
	}

	count := apd.New(int64(len(observations)), 0)
	estimate := unit.OfDecimal(s.do(e.bare(metrology.Engine.Div, sum, count)))

	// The deviations are differences of two magnitudes on one scale, so the
	// core reads them on the scale a difference belongs on (D6) — which is the
	// span unit the contribution has to be expressed on anyway.
	squares := apd.New(0, 0)
	for _, magnitude := range magnitudes {
		deviation := s.measure(e.core.Sub(unit.OfDecimal(magnitude), estimate)).Decimal()
		square := s.do(e.bare(metrology.Engine.Mul, deviation, deviation))
		squares = s.do(e.bare(metrology.Engine.Add, squares, square))
	}

	// s²/n in one division: Σ(xᵢ − x̄)² / (n(n − 1)) is the variance of the
	// mean, and dividing once rounds once (D4).
	scaled := s.do(e.bare(metrology.Engine.Mul, count, apd.New(int64(len(observations)-1), 0)))
	variance := s.do(e.bare(metrology.Engine.Div, squares, scaled))
	if s.err != nil {
		return Value{}, s.err
	}

	return Value{
		est:  estimate,
		span: span,
		// The root rounds up: this is a standard uncertainty leaving the
		// evaluation, not a divisor on the way into one.
		terms: []term{newTerm(newSource(name, len(observations)-1), e.sqrt(variance, apd.RoundCeiling))},
	}, nil
}

// Rectangular returns the standard uncertainty of a quantity known only to lie
// within ±halfWidth, with no reason to prefer any value inside: a digitiser's
// resolution, a specification limit, a rounding in a data sheet. It is a
// half-width divided by √3 (JCGM 100 §4.3.7).
func Rectangular(halfWidth metrology.Measurement) (metrology.Measurement, error) {
	return Engine{}.Rectangular(halfWidth)
}

// Rectangular is the Type B evaluation of a rectangular distribution: a/√3.
func (e Engine) Rectangular(halfWidth metrology.Measurement) (metrology.Measurement, error) {
	return e.divideByRoot("Rectangular", halfWidth, 3)
}

// Triangular returns the standard uncertainty of a quantity within ±halfWidth
// whose middle is more likely than its edges — two rectangular contributions
// convolved, as a fitting tolerance or a well-centred setting. It is a/√6
// (JCGM 100 §4.3.9).
func Triangular(halfWidth metrology.Measurement) (metrology.Measurement, error) {
	return Engine{}.Triangular(halfWidth)
}

// Triangular is the Type B evaluation of a triangular distribution: a/√6.
func (e Engine) Triangular(halfWidth metrology.Measurement) (metrology.Measurement, error) {
	return e.divideByRoot("Triangular", halfWidth, 6)
}

// UShaped returns the standard uncertainty of a quantity within ±halfWidth that
// spends most of its time near the edges — a sinusoidal drift, a thermostat
// cycling between two limits. It is a/√2.
func UShaped(halfWidth metrology.Measurement) (metrology.Measurement, error) {
	return Engine{}.UShaped(halfWidth)
}

// UShaped is the Type B evaluation of a U-shaped distribution: a/√2.
func (e Engine) UShaped(halfWidth metrology.Measurement) (metrology.Measurement, error) {
	return e.divideByRoot("UShaped", halfWidth, 2)
}

// FromExpanded returns the standard uncertainty behind an expanded one: a
// calibration certificate states U and the coverage factor k it used, and the
// budget needs u = U/k. It is [Engine.FromExpanded] with the default precision.
func FromExpanded(expanded metrology.Measurement, k int) (metrology.Measurement, error) {
	return Engine{}.FromExpanded(expanded, k)
}

// FromExpanded returns U/k, the standard uncertainty behind an expanded one.
func (e Engine) FromExpanded(expanded metrology.Measurement, k int) (metrology.Measurement, error) {
	if k < 1 {
		return metrology.Measurement{}, &InputError{
			Op: "FromExpanded", Why: "a coverage factor below one",
		}
	}
	if err := span("FromExpanded", expanded); err != nil {
		return metrology.Measurement{}, err
	}
	u, err := e.bare(metrology.Engine.Div, expanded.Decimal(), apd.New(int64(k), 0))
	if err != nil {
		return metrology.Measurement{}, err
	}
	return expanded.Unit().OfDecimal(u), nil
}

// rootGuard is how many digits past the engine's precision the divisor of a
// Type B distribution is taken at.
//
// Dividing by a root that was itself rounded to the engine's precision rounds
// twice, and the two roundings compound into the last digit the caller reads.
// Guard digits put the first one out of reach of the second — the same
// arrangement D20 makes where π enters a conversion.
const rootGuard = 10

// divideByRoot is the shape every Type B distribution here has: a half-width
// over the square root of a small integer, the integer being what the
// distribution contributes.
//
// The root rounds to nearest rather than outward. It is an input to the budget
// and not the number the budget reports, and the one place this package rounds
// conservatively is where a combined uncertainty leaves it.
func (e Engine) divideByRoot(op string, halfWidth metrology.Measurement, of int64) (metrology.Measurement, error) {
	if err := span(op, halfWidth); err != nil {
		return metrology.Measurement{}, err
	}
	wide := NewEngine(e.Precision() + rootGuard)
	u, err := e.bare(metrology.Engine.Div, halfWidth.Decimal(), wide.sqrt(apd.New(of, 0), apd.RoundHalfUp))
	if err != nil {
		return metrology.Measurement{}, err
	}
	return halfWidth.Unit().OfDecimal(u), nil
}

// span refuses a point where a span belongs.
//
// A half-width is a distance along a scale and never a place on it, which is
// D6 and not a rule of this package — but the core only enforces it where two
// measurements meet, and here one arrives alone.
func span(op string, m metrology.Measurement) error {
	if m.Kind() != metrology.Absolute {
		return nil
	}
	return &metrology.KindError{
		Op: op, Left: m.Kind(), Right: m.Kind(),
		Why: "a tolerance is a span along a scale, not a point on it",
	}
}

// abs returns |d| as a fresh decimal, sharing nothing with d (D3).
func abs(d *apd.Decimal) *apd.Decimal {
	return new(apd.Decimal).Abs(d)
}
