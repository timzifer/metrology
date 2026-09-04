# D21 — GUM propagation: `metrology/gum`, linear terms with provenance

**Status:** built and frozen. [Section 8](../deferred.md) had deferred uncertainty *propagation*
since the beginning and D15 built only the interval half; this is the other
half, a second layer beside `uncertainty` and not an extension of it. [Section 7](../status.md)
records what each part is measured by, and it has since settled that `Value`
ships inside `v1.0.0` — see **The freeze** at the end of this decision. Six
things this decision said turned out to be wrong or incomplete when it met the
code, and they are corrected in place below and marked **Correction**, as D15's
and D20's are.

**Why not inside `uncertainty`.** The two models disagree on purpose. In the
interval layer `x − x` is not zero and must not be, because the layer knows
nothing about where its two operands came from and a worst-case enclosure of two
unrelated magnitudes is what it promises. In a GUM budget `x − x` *is* zero,
exactly, because the two are the same input and their contributions cancel. Both
answers are right in their own model and neither is right in the other; a package
holding both types would be a package whose name means two things, and
`uncertainty`'s doc line exists precisely to stop a reader taking the one for the
other. So: `metrology/gum`, a sibling. The name is jargon, and it is the jargon
of the standard it implements (JCGM 100:2008) — a reader who does not recognise
it is a reader who should be reading the standard before the package.

Not a separate module either, and for D15's three reasons unchanged: conversion
would be a second implementation of D4, the kind rules of D6 would be restated by
somebody who has not read them, and `unitvet` cannot be taught a type outside
this module.

**The model.** The law of propagation of uncertainty, first order (JCGM 100 §5):

```
u_c²(y) = Σ (∂f/∂xᵢ)² u²(xᵢ) + 2 ΣΣ (∂f/∂xᵢ)(∂f/∂xⱼ) u(xᵢ, xⱼ)
```

A value therefore carries not one number but the *decomposition* that produced
it — a sparse list of contributions, each tagged with the independent input it
came from:

```go
package gum

// Value is an estimate of a measurand together with where its uncertainty
// came from.
type Value struct {
    est   metrology.Measurement
    terms []term          // sorted by source, one entry per independent input
}

type term struct {
    src Source            // identity of an independent input
    c   *apd.Decimal      // (∂y/∂x)·u(x), on the estimate's interval scale
}

// Source identifies one independent input. Two values sharing a Source are
// correlated through it, which is the whole mechanism.
type Source struct{ /* opaque */ }
```

Correlation is then not a second mechanism: two values are correlated exactly
where their term lists name the same source, and the cross terms fall out of the
sum. A declared correlation between two *inputs* is handled at construction, by
building the pair out of two independent sources — the 2×2 Cholesky factor is
two lines — rather than by consulting a covariance matrix at combination time.
That keeps `Value` self-contained and every operation total, which a
matrix-on-the-side does not: `a.Add(b)` cannot look up a ρ it was never given,
and giving it one would mean a context object threaded through every call or a
registry, which is D7.

**Contributions are spans, and the layer inherits that rather than restating
it.** `u(x)` is a distance along a scale and never a place on it, so a term lives
on the estimate's *interval* unit — 0.3 K beside 20 °C — which is the rule D6
already states and `uncertainty.Symmetric` already follows. Converting a `Value`
converts the estimate as a point and every term as a span, and `Unit.OfDecimal`
(D15) is how a bare term becomes a measurement again. The layer needs **nothing
new from the core**: `Engine.Rounding` and `Unit.OfDecimal` were added for D15
and cover D21 as they stand. That is worth recording where [section 7](../status.md) asks
whether those two belong in the frozen surface — a second consumer answers the
question the first one raised.

**The rounding finding, and it is the opposite of D15's.** In the interval layer
every bound rounds outward, because the bound *is* the answer. Here the terms
are intermediate and cancellation is the feature: round `|c|` up on both terms of
`x − x` and the two no longer cancel, so the layer would report an uncertainty
for a quantity that has none — the dependency problem re-introduced by the
rounding policy that was meant to be conservative. So **terms round to nearest
(D9) and only the final combination rounds up**, with `apd.RoundCeiling` on the
square root. One directed rounding, at the one place where the number leaves the
layer.

**Correction, `apd` does not round a square root the way it is asked.** This
decision assumed the mode on the context would carry, as it does for every other
operation and as `Engine.Rounding` relies on in D15. It does not: `Sqrt` returns
the correctly rounded nearest value under `RoundFloor`, `RoundCeiling` and
`RoundHalfUp` alike — measured, all three give the same digits. So the direction
is applied afterwards, as one unit in the last place and only where the root
came back inexact. A root that is exact is left alone, which is what keeps
u = 0.5 from being reported as 0.50000000000000000001.

**Correction, one contribution is its own combination.** A value with a single
input would otherwise be squared and rooted to get back the number it was given,
and the round trip rounds twice — enough to turn a stated 0.3 K into 0.30000…1.
The root of one square is the magnitude itself, so that is what is returned.

**Correction, `Pow` is not repeated multiplication.** The sketch argued that
raising a value by multiplying it by itself is exact about the dependency for
free, which is true. What it costs is the *spelling* of the unit: the reciprocal
that a negative power needs comes out as `1/(m·m)` where the core's own
`Unit.Pow` says `m⁻²`, and two spellings of one scale is what D12 refuses. So
the unit is `Unit.Pow` and the sensitivity is the n·xⁿ⁻¹ the chain rule asks
for — with xⁿ⁻¹ falling out of the loop that raises the magnitude, so nothing is
differentiated symbolically after all.

**Constructing an input.** Type B evaluation is a handful of divisors and is
where a budget actually starts:

```go
func Exactly(m metrology.Measurement) Value                          // u = 0, no terms
func Standard(est, u metrology.Measurement) (Value, error)           // u given directly
func Rectangular(est, halfWidth metrology.Measurement) (Value, error) // u = a/√3
func Triangular(est, halfWidth metrology.Measurement) (Value, error)  // u = a/√6
func Coverage(est, U metrology.Measurement, k int) (Value, error)     // u = U/k
func Sample(ms []metrology.Measurement) (Value, error)                // Type A: mean, s/√n
func Correlated(a, b Input, rho *apd.Decimal) (Value, Value, error)
```

There is deliberately **no constructor from an `uncertainty.Range`**. An interval
states two bounds and claims no distribution; reading it as rectangular would
invent the claim, and inventing a claim about the data is the one thing both
layers refuse. The reverse direction exists, because it is a report and not an
assumption.

**Correction, the distributions produce an uncertainty and not a value.** The
sketch above made every Type B evaluation a constructor of its own —
`Rectangular(est, halfWidth)` beside `Standard(est, u)` — which multiplies the
constructors by the distributions for no gain. What shipped factors the two
apart: `Rectangular`, `Triangular`, `UShaped` and `FromExpanded` take a
half-width and return the standard uncertainty behind it, and `Of` or
`Standard` turns that into a value. Four functions and two constructors instead
of eight constructors, and a caller can read a/√3 in the one place it is
computed.

**Operating and reporting.**

```go
func (v Value) Add(o Value) (Value, error)                  // Sub, Mul, Div, Pow, Scale, To
func (v Value) Apply(y metrology.Measurement, p ...Partial) (Value, error)
func (v Value) Estimate() metrology.Measurement
func (v Value) Uncertainty() (metrology.Measurement, error) // combined standard u, interval kind
func (v Value) Expanded(k int) (uncertainty.Range, error)   // y ± k·u, as a Range
func (v Value) Contributions() []Contribution               // the budget table
```

`Add` and `Sub` add and subtract the term lists; `Mul` and `Div` are the first
order of the product rule, `∂(xy)/∂x = y`; `Pow` is `n·xⁿ⁻¹`. Anything that is
not arithmetic — a flow coefficient, a calibration polynomial, a steam table —
goes through `Apply`, where the caller supplies the partial derivatives as
measurements. **The library does not differentiate Go functions and will not
pretend to:** automatic differentiation over `Measurement` is a different design,
and a partial the caller computed and can cite beats one the library inferred.

`Expanded` returning an `uncertainty.Range` is where the two layers meet: the
budget produces the number, the interval layer already has the text form, the
parser and the outward rounding for reporting it. `Contributions` is the table a
practitioner actually wants — source, sensitivity, contribution, share of the
variance — and it is a projection of the term list, not a second computation.

**What stays out.** Monte Carlo evaluation (JCGM 101, GUM Supplement 1): it needs
a random source, which is state, and floating point to be affordable, which is a
second arithmetic — if it is ever wanted it is its own decision, not a corner of
this one. Second-order terms. Distributions as objects. Effective degrees of
freedom by Welch–Satterthwaite: a natural second milestone, needs a ν per source,
and is deliberately not in the first.

**Correction, the span unit is asked for, and it can be refused.** The layer
does not decide which scale a contribution is read on; it asks the core, because
a measurement minus itself is a span by D6 and those rules already know the
answer. What the decision missed is that the question has no answer on every
scale: a caller's catalogue may declare an interval unit whose quantity tag
conflicts with its own scale's, and then the difference of two of its points is
a magnitude on a scale nobody can name. So a `Value` carries the span unit
resolved once, at construction, and every constructor can refuse — except
`Exactly`, which has no contribution to express and stands the estimate's own
scale in for the span it will never use.

**Correction, the degrees of freedom are in the first version.** This decision
put Welch-Satterthwaite in a second milestone. `Value.EffectiveFreedom`
implements it (JCGM 100 §G.4.1), every input carries a ν — `Infinite` by
default, n − 1 out of `Sample` — and a ν_eff past what an `int` holds is
reported as `Infinite`, because the t-factor at that end of the table has been
the normal one for some thousands of degrees of freedom.

What is *not* here is Table G.2 itself. Turning ν_eff into a coverage factor
needs a table of quantiles, and a table of quantiles transcribed into this
repository would be a page of numbers with nothing to check them against — which
is exactly what D4 refuses for a conversion factor and what the π constant of
D20 avoids by carrying its own recomputation. The caller reads the standard's
table and passes the factor to `Expanded`.

**`unitvet` learns a third receiver type**, as D15 predicted the second would
cost. The rule bodies do not change — a `Value`'s dimension, kind and quantity
are its estimate's — and three constructors are recognised without a receiver,
exactly as `uncertainty.Of`, `Between` and `Symmetric` are: `Exactly`,
`Standard` and `Apply`, each of which takes its estimate as an argument the
pass can resolve. `Standard` is also *checked*, by the rule `Symmetric` already
had: a standard uncertainty is a span along a scale and never a point on it.

**Correction, three constructors are silent and that is the design.** `Of` takes
an `Input` struct, `Sample` a slice and `Correlated` a pair of structs, and a
field of a composite literal is the container read D13 does not follow. A value
built by one of them resolves to nothing and the pass says nothing about it. The
alternative — teaching the checker to track stores into a local struct — is the
guessing D13 forbids, and the positional `Standard` exists partly so that the
common case stays provable.

**The freeze.** `Value` ships inside `v1.0.0`, and the three things this layer
argued about are frozen as they were built. `Input` and `Standard` both stay:
they are not two spellings of one thing, because `Input` is the extensible form
— a field added after the freeze is additive — and `Standard` is the
two-argument case that never has to grow *and* the one the correction above
keeps provable for `unitvet`. Keeping only the struct would put every budget in
ceremony and blind the checker on the common case; keeping only the pair would
leave a name and a degree of freedom nowhere to go. `Apply` keeps its shape,
with the derivative a `Measurement` rather than a number, so that the core
checks ∂f/∂x · u(x) against the span unit of the result and a derivative in the
wrong units is a dimension error instead of a plausible answer; there is still
deliberately no automatic differentiation. And `EffectiveFreedom` stays in a
package that ships no table, because ν_eff is computed *from* the budget and
only the budget holds the contributions it needs, where Table G.2 is a lookup
that needs no budget and would arrive here with nothing to check it against.
Dropping the method would leave ν_eff computable nowhere; adding the table would
put a page of unchecked numbers in a repository that refuses them everywhere
else.

`Engine.Rounding` and `Unit.OfDecimal` are frozen with this layer as with D15's:
this package needs exactly those two from the core and nothing else, which is
half of [section 7](../status.md)'s reason for freezing them.
