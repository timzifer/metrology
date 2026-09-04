# D20 — Symbolic factors: one π exponent beside the fraction

**Status:** built. This is the decision D4 and [section 6](../catalogue.md) had been pointing at
since they refused to round a factor, and [section 7](../status.md) records what each part of it
is measured by. Three things it said turned out to be wrong when it met the
code; they are corrected in place below and marked **Correction**, as D15's
were, because a decision that reads as if it had been right all along teaches
nobody what the writing of it missed.

D4 stores a factor as `num/den` and admits nothing else, so a unit whose factor
is a rational multiple of π has no entry: the degree of arc, the gon, the
oersted. They are not exotic — the degree is in Table 8 of the SI Brochure, and a
catalogue of plane angle holding only the radian is a catalogue nobody can use
for an angle they read off a drawing. The factor therefore gains one term:

```go
// base = (magnitude + offset) · num / den · πᵖ
type Unit struct { /* … */ num, den *apd.Decimal; pi int8 }
```

**One constant, not a table and not an algebra.** π is the only irrational the
catalogue needs, and that is not a close call: every candidate below is a rational
multiple of a power of π, and no unit in NIST SP 811 or the SI Brochure needs
`e`, a root or a logarithm. A table of named constants would cost a resolution
rule and a comparison rule for a case that does not exist; an expression tree
would put a computer algebra system inside a data file whose whole purpose (D4)
is that a human can check it against a standard character by character.
`factor: {num: "1", den: "180", pi: 1}` is still that.

**What it unblocks.** With sources, in one generator run:

| Unit | Factor | p |
|---|---|---|
| degree of arc, arcminute, arcsecond | `1/180`, `1/10800`, `1/648000` rad | 1 |
| gon | `1/200` rad | 1 |
| oersted | `1000/4` A·m⁻¹ | −1 |
| parsec | `648000/1` au | −1 |

The exponent is the whole extension: four of the six units are p = 1 and two are
p = −1, and both directions have to be there, because the sign of the exponent
decides which side of the fraction π lands on. The parsec is written against
the astronomical unit above because that is how the IAU defines it; the
catalogue stores every factor against the base unit of the dimension, so the
entry is `648000 · 149597870700` metres over π, and the astronomical unit — exact
since 2012 and a useful unit in its own right — comes with it.

**The algebra is the exponent's arithmetic and nothing else.** `times` adds p,
`byUnit` subtracts it, `Pow(n)` multiplies it by n and gets the same range check
the dimension exponents already have, and `Pow(0)` clears it with the rest of
the scale. `linearScale` keeps it, because dropping the offset does not touch
the factor.

**Why `Equal` may compare p separately, and why that is exact.** Two scales are
the same iff they have the same p *and* the same fraction. That is not an
approximation of a comparison of the products: π is transcendental, so `πᵈ` is
rational only for `d = 0`, and two factors with different exponents can never be
the same number. `sameRatio` stays exactly as it is, and the pointer shortcut in
`sameScale` stays sound for the same reason it is sound today (D3).

**The conversion, and the one amendment D4 takes.** The exponents subtract:

```
v' = ((v + off_f) · num_f · den_t) / (den_f · num_t) · π^(p_f − p_t) − off_t
```

Where `p_f == p_t` — degree → arcsecond, gon → degree, oersted → oersted — the
π factor is `π⁰`, the expression is what `convert` already computes, and the
conversion is exactly as exact as it is today. **Only a conversion that crosses
between a π unit and a π-free one materialises π at all**, and that one rounds a
second time: once for the quotient D4 already has, once for π at the engine's
precision. D4 gains that sentence and keeps everything else — the catalogue
factor is still exact, still auditable, still one division.

The second rounding is bounded rather than argued away: π enters at the engine's
precision plus a guard, so the double rounding is invisible in the last place the
engine reports. That is a test and not a claim — every crossing conversion in the
catalogue is computed a second time at precision + 40 and rounded down to the
engine's precision, and the two must agree.

**π enters as an enclosure, not as a value.** `internal/pi` exposes the constant
twice, `Floor(prec)` and `Ceiling(prec)` — the same digits with the last place
down and up. A point conversion takes either (they differ below what it reports);
an interval bound takes the one that widens, chosen by the sign of the magnitude
and the sign of `p_f − p_t`. This is D15's outward rule reaching one level
further down, and it needs no new policy: `Engine.Rounding` already exists, and
the property test in `uncertainty/rounding_test.go` runs over the whole
catalogue, so the first π unit to enter the catalogue is policed by a test that
is already written. Getting the direction backwards fails it in a second.

**Why the constant is stored and not computed.** A computed π would have to be
cached to be affordable, and a cache is the global mutable state D7 forbids —
there is nowhere in this library to keep one. So the digits are a string
constant, and their auditability is handled the way D4 handles a conversion
factor: a test recomputes π from Machin's formula in `math/big` rationals and
compares it to the constant digit by digit. The constant is a claim about the
world like `101325/760`, and it carries its check.

**The precision ceiling is an error, not a silent truncation.** The constant is
finite, so an `Engine` asking for more digits than it holds minus the guard gets
a `RangeError` naming the limit. A conversion that quietly returns fewer correct
digits than the engine promises is the failure this library exists to prevent.

**The cost, and where it falls due.** One exported signature changes:

```go
func (u Unit) Factor() (numerator, denominator *apd.Decimal)   // today
func (u Unit) Factor() Factor                                   // Factor{Num, Den *apd.Decimal; Pi int}
```

A caller that reads `Factor()` and ignores a π exponent computes a wrong number,
so the exponent could not be bolted on as a second getter — the compiler has to
stop that caller. **This is why D20 was built before the API review of [section 7](../status.md)
closed and not after it:** the change was free in `v0.x` and would have been a
`v2` after the freeze. `UnitDef` gained `Pi int` and the YAML a `pi:` key, both
additive and both defaulting to zero, so no existing entry moved.

The arcminute and arcsecond ship as `′` and `″` only: the ASCII `'` and `"` are
quoting characters in the formats `parse` is embedded in (D12), and a symbol
that has to be escaped to be written is a symbol that will be written wrong.
`rad` stays refused for the absorbed dose ([section 6](../catalogue.md)), unchanged.

**Correction, `°` next to `°C` is not a hazard.** This decision predicted that
the parser would need a longest-match rule, because the degree symbol is a prefix
of the degree Celsius and the catalogue had never held such a pair. It needs
nothing: `parse` splits the magnitude from the unit text and then looks the whole
text up as one key (D12), so `°` and `°C` are two entries in a map and never
compete. The prediction was about a parser that scans, and this one does not.

**Correction, the two units this decision promised and the catalogue does not
have.** The square degree would need `deg²` as a second spelling of the degree,
and two spellings for one unit is the ambiguity D12 exists to refuse;
`angle.Degree.Pow(2)` builds that scale without naming it, and the exponent
algebra is tested there instead. The gilbert would need a magnetomotive-force
group whose canonical unit is the ampere over again, which is a question about
D6 and not about factors. In their place the catalogue gained the astronomical
unit, which the parsec is defined against and which is exact in its own right —
so the batch is six units and two of them were not on the list.

**Correction, `internal/pi` exposes a power and not two bounds.** The sketch
above named `Floor(prec)` and `Ceiling(prec)`. What a conversion needs is
`Power(n, prec, mode)`: the exponent has to be *raised* in the direction it is
rounded, because π² computed from a rounded-down π is only a lower bound if
every multiplication on the way rounds down too. Putting the loop inside the
constant's package is what keeps that true; a caller multiplying two bounds
itself would have had to know it.

**What it cost.** One field on `Unit`, one on `UnitDef`, one key in the YAML, one
exponent check in the generator, a `Factor` struct where two return values were,
a `PrecisionError`, and `internal/pi` — the digits, one function, and the test
that recomputes them. Everything else is the four lines where the exponent adds,
subtracts, multiplies and compares. The conversion grew one branch, and it is the
branch that runs only when the exponents fail to cancel.
