# D15 — Uncertainty as a layer: `metrology/uncertainty`

**Status:** built and frozen. `Range`, its arithmetic, its text form, its
parser and the `unitvet` receiver all exist, [section 7](../status.md) records what each is
measured by, and it has since settled that the type ships inside `v1.0.0` —
see **The freeze** at the end of this decision.

Five things this decision said turned out to be wrong when it met the code. They
are corrected in place below and marked **Correction**, rather than quietly
edited away, because a decision that reads as if it had been right all along
teaches nobody what the writing of it missed: `Mid` and `Width` are not total,
the argument given for why `±` cannot be canonical does not hold (a better one
does), the `apd` rounding type is named `apd.Rounder`, D9 rounds half-*up* and
not half-even, and the layer needed two additions to the core and not one.

[Section 8](../deferred.md) has deferred measurement uncertainty from the beginning, with a
justification that still holds: it is a large topic of its own with its own
error propagation, and it belongs on top of the core rather than in it. What
that entry did not say is *where* the layer lives, and this decision answers
only that — the layer is a subpackage of this module, `metrology/uncertainty`,
and not a separate library.

**Why here and not elsewhere.** The alternative was a library of its own, and it
loses three things at once. Conversion is where the layer is hardest (see the
rounding finding below), and a conversion implemented outside this module would
be a second implementation of D4 — exactly the drift the generated `unitvet`
table exists to prevent one level up. The kind rules of D6 apply to an interval
unchanged and would otherwise be restated by somebody who has not read them. And
`unitvet` resolves receivers by type name against a table generated from the
catalogue: a type outside the module is a type the checker cannot be taught
without exporting its table.

**Why not in the core.** The refusal in [section 8](../deferred.md) was right about the substance.
This is *interval arithmetic*, not uncertainty propagation in the sense of the
GUM: it gives worst-case bounds and it has the dependency problem — `x − x` is
not zero, `x / x` is not one, and a formula naming a variable twice over-widens.
For checking a published number that is acceptable and even conservative: a
wider interval never invents a disagreement, it can only hide one. As a general
uncertainty budget it is wrong, and a `Measurement` that carried bounds would
put that wrongness in every value in the library. The package name has to carry
the warning too, and the package doc says it on the first line.

**The type.** One unit, two exact magnitudes:

```go
package uncertainty

// Range is a magnitude known only to lie between two bounds, on one scale.
type Range struct { /* unit Unit; lo, hi apd.Decimal */ }

func Of(m metrology.Measurement) Range                       // a point
func Between(lo, hi metrology.Measurement) (Range, error)    // same unit required
func Symmetric(m, tol metrology.Measurement) (Range, error)  // 3.7 ± 0.2

func (r Range) Lo() metrology.Measurement
func (r Range) Hi() metrology.Measurement
func (r Range) Mid() (metrology.Measurement, error)
func (r Range) Width() (metrology.Measurement, error)        // an interval-kind measurement
func (r Range) Overlaps(o Range) (bool, error)
func (r Range) To(u metrology.Unit) (Range, error)
func (r Range) Add(o Range) (Range, error)                   // Sub, Mul, Div, Pow
func (r Range) String() string                               // "[3.65, 3.75] mm"
func (r Range) PlusMinus() (string, bool)                    // "3.7 ± 0.05 mm"
func (r Range) MarshalText() ([]byte, error)                 // MarshalJSON, Value

type Engine struct{ /* core metrology.Engine */ }            // D9, not the mode
type Parser struct{ /* units parse.Parser */ }               // D7/D12
type Text struct{ Range; /* … */ }                           // the decoding side

func Parse(text string) (Range, error)                       // Default().Range
```

**Correction, `Mid` and `Width` return an error.** Both were sketched above as
total, and neither is. `Width` is `Hi − Lo`, which for a scale whose declared
interval unit carries a conflicting quantity tag is refused by D6 — the
catalogue has no such scale, a caller's catalogue may. `Mid` divides, and a
division at the edge of the exponent range overflows. Both errors are reachable
and therefore tested; a branch nobody can reach would have been deleted instead
(D14).

**How `Mid` is computed, and why it is not `(Lo + Hi) / 2`.** The sum of two
absolute magnitudes is not a magnitude (D6), so an absolute range has no
midpoint by that route — and it plainly has one. The midpoint is therefore taken
on the two magnitudes *inside* the one scale they share: an affine map preserves
midpoints, so the midpoint of the scale's own coordinates is the midpoint of the
quantity. It is a point and not a bound, so it rounds the way D9 rounds every
other point.

**`Pow`, and the power the core does not have.** The core has `Unit.Pow` and no
`Measurement.Pow`: a magnitude has never needed raising. `Range.Pow` does, so it
raises by repeated multiplication under the directed engine and takes its unit
from `Unit.Pow`, which is exact — `(v·f)ⁿ = vⁿ·fⁿ`, so the magnitude alone is
raised. That rounds n−1 times where a single power would round once. It is
sound, because every one of those roundings goes outward and a wider enclosure
is still an enclosure; the godoc says so rather than leaving a reader to
discover it. An even power of an interval straddling zero is the case worth
naming: `[−2, 3]²` is `[0, 9]`, and its minimum is at neither bound.

One unit and two magnitudes rather than two measurements, because a range whose
ends are in different units is a state the type must not be able to hold — and
because it makes the inheritance from D6 exact.

**How it composes with the decisions already made.**

| Decision | How it applies |
|---|---|
| D3, immutability | Two `apd.Decimal` fields, written only at construction via `Set`, exactly as `Measurement` does. The aliasing guard grows a `Range` case; the 200-digit rule applies unchanged, and for the same reason. |
| D4, exact fractions | Conversion is the same `(v + offset) · num / den` on each bound — with one amendment, below. |
| D6, kind and quantity | Free, and pleasingly so. Both bounds share one unit, so an absolute range such as 20 ± 0.5 °C is meaningful; `Width` returns an **interval**-kind measurement because absolute − absolute = interval is already the rule; and multiplying two absolute ranges is already an error. D6 needs no new clause. |
| D7, no global state | Value types and functions. Reading text needs a symbol table, so a range parser is a value built over a `parse.Parser`, and the asymmetry of D12 repeats verbatim. |
| D9, precision | The significant-digit rule stays *outside* the type. Deriving `[3.65, 3.75]` from the literal `3.7` is a reading rule and lives in the parser, not in `Range`. D9 does not start tracking significant figures. Precision itself is where D9 puts it: `uncertainty.Engine` is the counterpart of `metrology.Engine` and carries it. What it does not carry is the rounding mode, which is not the caller's to choose here. |
| D14, coverage | Pure computation, no I/O, no clock. The 100 % rule holds with no exception. |

**The text form, and why `±` cannot be the canonical one.** The canonical form
is the bracket form, `[3.65, 3.75] mm²/s`, which states the two magnitudes the
range holds and reads back as exactly what it says. `±` and the compact
parenthesis form are **accepted on input** and `Range.PlusMinus` produces the
first of them. That is the split D12 already makes between `String` and
`Prefixed`, for the same reason: the canonical text has to read back as the same
value, and the pleasant form is a rendering choice.

**Correction, the reason `±` is not canonical.** This decision first said that a
product of two ranges is asymmetric and that `±` cannot write an asymmetric
interval. That is not true and the code proved it: every closed interval has a
midpoint and a half-width, so every range has a ± form. Two reasons survive, and
they are the ones the code implements.

The first is what the form *says*. `[1, 4] m` written as `2.5 ± 1.5 m` asserts a
centre, and a range that came out of a quotient has no centre anybody claimed —
the arithmetic produced two bounds, and reading a centre into them is a claim
about the data that the data does not make. The brackets state what is there.

The second is arithmetic and is what `PlusMinus` reports on. The midpoint and
the half-width need one digit more than the bounds do, so a range whose bounds
already fill the engine's precision has no ± form that reads back as the same
range. `PlusMinus` returns `(string, bool)` and the second result is false
there, rather than the first result rounding — and it checks by reconstruction,
putting `mid − tol` and `mid + tol` back against the bounds, because addition
never rounds and so a mismatch can only have come from the one division.

**The tolerance beside a point is a span.** `20 ± 0.5 °C` reads its tolerance on
the interval unit the scale declares — 0.5 K — because a tolerance is a distance
along a scale and never a place on it, which is D6 and not a rule of this layer.
A scale declaring no interval unit has nowhere to read one, and the kind rules
say so when the range is built.

**The finding that makes this more than a wrapper: interval conversion must
round outward.** D9 rounds to the context precision at the one division of a
conversion. For a point that is right. For an interval *bound* it is wrong:
rounding a bound inward narrows the interval, and a narrowed interval can turn
an overlap into a disjoint pair — a disagreement manufactured by the conversion
and standing in no source. So `Lo` rounds toward −∞ and `Hi` toward +∞. `apd`
has both modes; what this module did not expose was a way to ask an engine for
one:

```go
func (e Engine) Rounding(mode apd.Rounder) Engine
```

The zero `Engine` is unchanged and D9 is intact — this is a second rounding
policy in a library that had exactly one, invisible unless asked for. The
alternative, `uncertainty` building an `apd.Context` of its own and duplicating
D4's conversion path, is worse for the reason at the top of this decision.

**Correction, the type is `apd.Rounder`.** This decision wrote `apd.Rounding`,
which is the name of the *field* on `apd.Context`. The type is `apd.Rounder`, a
string, with `apd.RoundFloor` and `apd.RoundCeiling` among its constants.

**Correction, D9 rounds half-*up*, not half-even.** This decision asserted
half-even in passing. `Engine.context` is `apd.BaseContext` with the precision
set, `apd.BaseContext` leaves `Rounding` empty, and an empty `apd.Rounder` means
`RoundHalfUp`. D9 itself names no mode and is unaffected; the sentence here was
simply wrong, and it mattered enough to check because the whole finding above is
about which way a bound moves.

**Correction, two additive changes to the core and not one.** The second is:

```go
func (u Unit) OfDecimal(d *apd.Decimal) Measurement
```

`Range` holds a unit and two bare `apd.Decimal`, as the type above says it
does — and the core had no exported way back from a magnitude to a measurement.
`Unit.Of` goes through `float64` or `int64` and loses digits; `Unit.OfString` is
exact but runs every bound through text and back, on every `Lo`, every `Hi` and
every result of every operation. `OfDecimal` is the exported counterpart of the
existing `Measurement.Decimal`: what one hands out, the other takes back, and it
copies on the way in exactly as `Decimal` copies on the way out (D3). It is one
line, it is additive, and the asymmetry it closes was one before D15 existed.
[Section 7](../status.md)'s freeze list gains it.

**The property this rests on is a test, not an argument.** `uncertainty` asserts
over the whole catalogue that converting a range into every other unit of its
dimension and back yields a range that *contains* the original, and separately
that a conversion never pulls two overlapping ranges apart. Directed rounding
that had been got backwards would fail both within a second.

**`unitvet` learns the second receiver type (D13).** A range added to a range of
another dimension is exactly the provable class the checker exists for. Staying
silent on it would not be D13's silence on doubt — that rule is about operands
the pass cannot resolve, not about a type it was never told exists. This is the
largest single cost of D15 and it is not optional: a checker that goes blind
precisely where the arithmetic moved would be worse than no checker, because
users would not notice.

**What `unitvet` actually needed.** Less than feared, and in one more place than
expected. The rules did not change at all: a range's dimension, kind and
quantity are its bounds', so `Add` on a `Range` is `Add` on a `Measurement` as
far as anything provable is concerned, and the existing rule bodies apply
unmodified. What changed is that the pass now looks for its types in two
packages instead of one, that `coreCall` reports which type a call was on as
well as which method — `Unit.Of` and `uncertainty.Of` share a name and do not
share an argument — and that the three constructors had to be recognised
*despite having no receiver*, because every range enters through one of them and
without them nothing about a range resolves at all. The one thing the pass will
not say is which interval unit the width of an absolute range is read on: the
scale declares it, the generated table does not record it, and that is the
silence a difference of two points already gets.

**What it costs.** A second arithmetic surface — five operations, each with the
corner analysis that a point does not need: all four products for `Mul`, because
an interval straddling zero does not take its extreme at the corner one would
guess, and a divisor whose interval covers zero is an error rather than a bound,
because the quotient is unbounded and reporting a bound for it would be a lie
about the data. A rounding mode on `Engine` where there was one, and a decimal
constructor on `Unit` where there was none. A `unitvet` receiver. And [section 7](../status.md)
gains two items: whether `Range` is part of the `v1.0.0` surface or ships behind
it, and whether `Unit.OfDecimal` belongs in the frozen surface beside
`Engine.Rounding`.

**The freeze.** Both of those items are now answered, and both answers are yes.
`Range` ships inside `v1.0.0`, which makes its exported shape a compatibility
promise rather than a sketch, and the three things the layer argued about are
frozen as they were built. `Mid` and `Width` keep the error the correction above
gave them, because a total signature would have to swallow a failure that is
reachable and tested. `PlusMinus` keeps `(string, bool)`, because there is one
reason for a no and one thing to do about it — `String` — so an error would
carry nothing the caller does not already have. `uncertainty.Engine` stays a
type of its own *because* it carries no rounding mode: it is the enforcement of
the finding above, and a `metrology.Engine` in its place would hand the caller
the one knob this decision takes away. `Engine.Rounding` and `Unit.OfDecimal`
are frozen with it, for the reason [section 7](../status.md) gives: two shipped layers ask for
exactly those two, and freezing a layer without the hooks it stands on would be
freezing nothing.

What the freeze does not close is addition. `Range` has no `Contains`, no
`Equal` and no `Cmp`, and a later release may add all three: a method added is
additive, where a signature changed is a `v2`. The freeze is about the shapes,
and the shapes are the three above.
