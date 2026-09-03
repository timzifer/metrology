// Package gum propagates measurement uncertainty in the sense of the GUM: a
// [Value] carries an estimate and the decomposition that produced its
// uncertainty, and combining two of them applies the law of propagation of
// uncertainty to first order (JCGM 100:2008 §5, D21).
//
// It is the sibling of [github.com/timzifer/metrology/uncertainty] and not a
// second type inside it, because the two models disagree on purpose. In the
// interval layer x − x is not zero and must not be: a Range knows nothing about
// where its bounds came from, and a worst-case enclosure of two unrelated
// magnitudes is what it promises. Here x − x is exactly zero, because the two
// are the same input and their contributions cancel. Both answers are right in
// their own model and neither is right in the other.
//
// # Provenance is the mechanism
//
// A Value holds one contribution per independent input — a [Source] and the
// product of the sensitivity with that input's standard uncertainty. Two values
// are correlated exactly where their contributions name the same source, and no
// covariance matrix is consulted at combination time: a declared correlation
// between two inputs is decomposed into independent sources when they are
// built, by [Correlated]. That is what keeps every operation total and every
// Value self-contained, which is D7 and not a preference.
//
// # What rounds, and which way
//
// The contributions round the way D9 rounds every other magnitude, to nearest.
// They are intermediate, and cancellation is the feature: rounding them outward
// the way an interval bound rounds (D15) would leave x − x with an uncertainty
// it does not have. Only the combined standard uncertainty rounds, and it
// rounds up — one directed rounding, at the one place a number leaves this
// package.
//
// # What this is not
//
// Monte Carlo evaluation (JCGM 101), second-order terms, distributions as
// objects, and automatic differentiation of Go functions. A quantity that is
// not reached by the arithmetic here is reached by [Value.Apply], where the
// caller supplies the partial derivatives — a derivative that can be cited
// beats one this library inferred.
//
// # The zero value
//
// The zero Value is not a value: its estimate is the zero
// [metrology.Measurement], which has no scale. Build one with [Exactly], [Of],
// [Correlated] or [Sample].
package gum

import (
	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

// Value is an estimate of a quantity together with where its uncertainty comes
// from.
//
// It is an ordinary Go value (D1) — copyable, passable, free of identity — and
// nothing inside one is written after construction (D3). Every operation
// allocates its own contributions, so two Values sharing a slice share only
// numbers that never change.
type Value struct {
	est metrology.Measurement

	// span is the scale the contributions are read on: the interval unit an
	// absolute scale declares (D6), and the estimate's own scale otherwise. It
	// is resolved once, where the value is built, because every operation and
	// every report needs it and the core can refuse to name it — a caller's
	// catalogue may declare an interval unit that its own scale cannot convert
	// onto, and that is an error at construction rather than a surprise later.
	span metrology.Unit

	// terms holds one contribution per independent input, in the order the
	// inputs first reached this value. Each coefficient is (∂y/∂x)·u(x), a
	// bare magnitude on the span unit.
	terms []term
}

// term is one input's contribution to a value's uncertainty.
type term struct {
	src Source
	c   apd.Decimal
}

// Source identifies one independent input quantity.
//
// Identity is a pointer and not a counter, because a counter is package state
// and D7 allows none — two values built independently, in two goroutines or two
// programs, must never collide. The name and the degrees of freedom travel with
// it so that a budget can be printed without a second lookup.
type Source struct {
	id      *sourceID
	name    string
	freedom int
}

// sourceID is one byte, and it has to be: a zero-sized allocation has no
// identity in Go, because every pointer to one may be the same address.
type sourceID byte

// Infinite is the degrees of freedom of an input whose standard uncertainty is
// known rather than estimated — the usual case for a Type B evaluation, where
// the number comes from a certificate or a specification.
//
// It is the zero value, so an [Input] that says nothing about degrees of
// freedom says the thing that is almost always meant.
const Infinite = 0

// Name reports the name this input was given, and the empty string for an
// unnamed one.
func (s Source) Name() string { return s.name }

// Freedom reports the degrees of freedom of this input, or [Infinite].
func (s Source) Freedom() int { return s.freedom }

// same reports whether two contributions come from one input.
func (s Source) same(other Source) bool { return s.id == other.id }

// Input describes one independent input quantity of a measurement model.
type Input struct {
	// Estimate is the value of the input quantity.
	Estimate metrology.Measurement

	// Uncertainty is its standard uncertainty: a span along the estimate's
	// scale, never a point on it (D6). For an input on an absolute scale that
	// is the interval unit the scale declares — 0.3 K beside 20 °C.
	//
	// A Type B statement that is not already a standard uncertainty is turned
	// into one first: see [Rectangular], [Triangular], [UShaped] and
	// [FromExpanded].
	Uncertainty metrology.Measurement

	// Name labels the input in [Value.Contributions]. Optional, and worth
	// setting: a budget whose rows are unnamed is a budget nobody can act on.
	Name string

	// Freedom is the degrees of freedom of the uncertainty above: n − 1 for a
	// Type A evaluation from n observations, and [Infinite] where the
	// uncertainty is known rather than estimated.
	Freedom int
}

// scalar is the dimensionless scale a bare magnitude travels on.
//
// This package computes on the coefficients inside a Value, and the core
// computes on measurements. Putting a magnitude on this scale is how the one
// reaches the other, so that every rounding here is the rounding of D9 rather
// than a policy of this package's own. The one operation the core does not have
// is the square root, and that is the only place a context is built here.
//
// It is a package-level value and not global mutable state (D7): nothing writes
// to it, for the same reason nothing writes to a catalogue unit.
var scalar = metrology.MustUnit(metrology.UnitDef{
	Dimension: dimension.One,
	Symbol:    symbol.Static("1"),
})

// spanUnit returns the scale a span along a measurement's scale is read on: the
// interval unit an absolute scale declares, and the scale itself otherwise.
//
// It asks the core rather than deciding: a measurement minus itself is a span
// by the kind rules of D6, and those rules already know which unit a difference
// of two points is expressed in. A rule restated here would be a rule that can
// disagree with the one in the core.
//
// The core can refuse, and the error is worth propagating rather than papering
// over: a scale whose declared interval unit carries a conflicting quantity tag
// has no span this library can name, and every uncertainty on it would be a
// magnitude on a scale nobody chose.
func spanUnit(m metrology.Measurement) (metrology.Unit, error) {
	// The question is about the scale and not about the magnitude, so it is
	// asked with a zero: an infinity has no difference with itself, and a
	// value carrying one would otherwise have no span unit to be built on.
	zero := m.Unit().Of(0)
	span, err := zero.Sub(zero)
	if err != nil {
		return metrology.Unit{}, err
	}
	return span.Unit(), nil
}

// Exactly returns a value with no uncertainty at all: a constant of the model,
// a count, or a quantity whose uncertainty is negligible beside the others.
//
// It has no contributions, so it adds nothing to any budget it enters and it
// never appears in [Value.Contributions].
//
// It is the one constructor that cannot fail. Where the core refuses to name a
// span along the estimate's scale, the estimate's own scale stands in: an exact
// value has no contribution to express on it, and the only magnitude that ever
// reaches it is a zero, which is a zero on every scale of its dimension.
func Exactly(m metrology.Measurement) Value {
	span, err := spanUnit(m)
	if err != nil {
		span = m.Unit()
	}
	return Value{est: m, span: span}
}

// Standard returns the value of an independent input from its estimate and its
// standard uncertainty, unnamed and with [Infinite] degrees of freedom. It is
// [Engine.Standard] with the default precision.
//
// It is the short form of [Of] and the common one: most inputs of a budget are
// a number and an uncertainty from a certificate. Name them — [Of] takes an
// [Input] with a Name and the degrees of freedom — as soon as the budget has
// enough rows that a reader has to ask which is which.
func Standard(estimate, u metrology.Measurement) (Value, error) {
	return Engine{}.Standard(estimate, u)
}

// Standard returns the value of an unnamed independent input.
func (e Engine) Standard(estimate, u metrology.Measurement) (Value, error) {
	return e.Of(Input{Estimate: estimate, Uncertainty: u})
}

// Of returns the value of one independent input quantity. It is [Engine.Of]
// with the default precision.
func Of(in Input) (Value, error) { return Engine{}.Of(in) }

// Of returns the value of one independent input quantity.
//
// The uncertainty is converted onto the span unit of the estimate, which is
// where the kind rules of D6 put a tolerance, so an input stated in kelvin
// beside a temperature in degrees Celsius is read as the span it is. A negative
// standard uncertainty is refused: it is not a smaller one, it is a mistake.
func (e Engine) Of(in Input) (Value, error) {
	if in.Freedom < 0 {
		return Value{}, &InputError{Op: "Of", Name: in.Name, Why: "degrees of freedom below zero"}
	}
	span, err := spanUnit(in.Estimate)
	if err != nil {
		return Value{}, err
	}
	u, err := e.core.To(in.Uncertainty, span)
	if err != nil {
		return Value{}, err
	}
	if u.Decimal().Negative {
		return Value{}, &InputError{Op: "Of", Name: in.Name, Why: "a negative standard uncertainty"}
	}
	return Value{
		est:   in.Estimate,
		span:  span,
		terms: []term{newTerm(newSource(in.Name, in.Freedom), u.Decimal())},
	}, nil
}

// newSource mints an input identity. Every call returns a source that is equal
// to no other, which is what makes two values independent unless they were
// built to share one.
func newSource(name string, freedom int) Source {
	return Source{id: new(sourceID), name: name, freedom: freedom}
}

// newTerm copies a coefficient into a contribution (D3): an apd.Decimal shares
// its coefficient slice with every struct copy of itself, so a term that held
// the caller's decimal would change when the caller's did.
func newTerm(src Source, c *apd.Decimal) term {
	t := term{src: src}
	t.c.Set(c)
	return t
}

// Estimate returns the value of the quantity, without its uncertainty.
func (v Value) Estimate() metrology.Measurement { return v.est }

// Unit returns the scale the estimate is read on.
func (v Value) Unit() metrology.Unit { return v.est.Unit() }

// Dimension returns what the estimate measures.
func (v Value) Dimension() dimension.Dimension { return v.est.Dimension() }

// Kind reports whether the estimate is a point on a scale or a span along one.
func (v Value) Kind() metrology.Kind { return v.est.Kind() }

// Quantity reports the quantity tag of the estimate's unit (D6).
func (v Value) Quantity() metrology.Quantity { return v.est.Quantity() }
