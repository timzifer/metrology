# D13 — A `go vet` pass that checks dimensions statically

The library ships `cmd/unitvet`, a `golang.org/x/tools/go/analysis` pass that
parses third-party Go code, resolves which unit each `Measurement` carries, and
reports arithmetic and conversions across incompatible dimensions — without
running the code.

```
go vet -vettool=$(go env GOPATH)/bin/unitvet ./...

app/app.go:12:33: Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹
app/app.go:19:14: Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it
app/app.go:26:21: To on incompatible quantities: frequency and radioactivity
```

The dimensions are written the way D11 requires of the run-time errors, because
the two messages are read by the same person about the same mistake, and a
checker with a notation of its own would make them compare two spellings instead
of two dimensions.

**How it works.** The pass consumes SSA from `buildssa` and walks each operand
backwards to its origin. When an operand traces to a catalogue unit —
`pressure.Bar.Of(2.5)` resolves to the package-level `Bar` variable — its
dimension is known. The dimension table is generated from the same YAML catalogue
as the library itself (D8), in the same `catgen` run, so the checker and the
runtime cannot drift apart. Cross-package analysis uses the framework's **fact**
mechanism: a function that provably always returns one dimension exports that as
a fact, which importing packages consume.

**The governing rule: silence on doubt.** The pass reports only *provable*
conflicts. Where an operand's unit cannot be resolved with certainty, it says
nothing. A dimension checker that produces false positives is a dimension checker
that gets switched off, and then it catches nothing at all. False negatives are
acceptable; the runtime check remains the backstop.

**What is decidable.** Every row is a function in `unitvet/testdata`, and both
columns are asserted:

| Pattern | Result |
|---|---|
| `pressure.Bar.Of(2.5).Add(temperature.Celsius.Of(20))` | reported |
| assignment to local variables, then `p.Sub(t)` | reported |
| `temperature.Celsius.Of(20).Add(temperature.Celsius.Of(5))` | reported — the affine rule of D6 |
| `frequency.Hertz.Of(50).To(activity.Becquerel)` | reported — the quantity tag of D6 |
| `becquerel.Mul(ratio)`, then `.Add(hertz)` | reported — the dropped tag of D16; the run time accepts it |
| the same across a package boundary | reported — the fact carries the provenance |
| `becquerel.Mul(metre)`, then `.Div(metre)`, then `.Add(hertz)` | silent — the dimension changed, so the tag is gone for good |
| a plane angle times a solid angle, then converted | silent — two surviving tags disagree, which is no answer |
| a unit computed with `Div`, `Per` or `Pow`, then used | reported — the walk follows the composition |
| operand from another package's function with an invariant unit | reported, via facts |
| same dimension, different units (`bar` + `Pa`) | correctly silent |
| unit chosen at runtime (`if x { u = Bar }`) | silent — SSA φ-node, not provable |
| operand arriving as a function parameter | silent — unknown origin |
| a unit from the caller's own catalogue | silent — not in the generated table |
| `dose.Sievert = length.Metre` | reported — the write is what makes the table untrue |
| a write to a variable of the program's own | silent — the resolver never trusted it |
| a unit held in a map, a slice or a struct field | silent |
| `25 °C − 20 °C`, then the result used | silent — the interval unit is not in the table |
| a method value, `add := m.Add` | silent — the receiver is bound out of sight |

Units arriving from deserialisation are equally out of reach. This is a lint that
catches the statically obvious subset, not a proof system.

**Scope.** Beyond `Add` and `Sub` the same machinery checks the target of a
conversion — `To`, `In` and `DecimalIn` — and `Cmp`, and the affine and quantity
rules of D6 wherever they are decidable: two points added, a point multiplied, a
hertz converted into a becquerel. Those are the same walk with a different
comparison, and leaving them out would have shipped a checker that had the
information to prove `20 °C + 5 °C` wrong and declined to say so. Every rule the
pass applies is one the run time applies, in the order the run time applies it,
so a diagnostic and the error it predicts never disagree — with the single
exception D16 adds, the tag a product dropped, which the run time has no way
left to see and which says so in its own message. What is *not* checked
is a declared result unit for `Div`: the quotient carries the unit its operands
gave it and is named in a separate, checked step.

**Six resolution rules that are easy to get wrong.**

- **A unit is trusted only where the catalogue names it.** The resolver reads a
  package-level variable when — and only when — the generated table has it, which
  is what keeps the pass from assuming a variable it does not know is never
  written to. A program with a catalogue of its own is silent rather than wrong.
- **The trust that remains is checked, not assumed.** A catalogue unit is an
  exported package-level variable, and Go lets an importer assign to it (D7 keeps
  these variables deliberately: a function would rebuild its decimals on every
  call, and `pressure.Bar` is how callers write it). Resolving one by name
  therefore assumes nobody writes to it — so a direct store to a catalogue unit
  is itself reported, at the write rather than at the uses it invalidates. What
  it costs the program is small and local: the catalogue's own maps hold copies
  taken at init, so `catalog.BySymbol("Sv")` still answers with the sievert while
  the variable no longer does. What it costs this pass is the whole proof, which
  is why the rule is here and not in a comment. A write through a pointer taken
  elsewhere, or one inside a dependency the vet run does not cover, stays out of
  reach.
- **A forbidden operation has no result.** `Add` on two absolute magnitudes
  returns the zero `Measurement`, so the walk stops there rather than propagating
  the scale it would have had. One mistake reports once; without this a single
  wrong line reports again at every operation downstream of it, which is how a
  linter teaches people to ignore it.
- **The difference of two points is not resolved.** `25 °C − 20 °C` is read on
  the interval unit the scale declares — K for °C — and which unit that is is not
  something the table records. The dimension is settled and the tag is not, so
  the pass says nothing about the result rather than guessing. Recording the
  declared interval unit in the table would close this; it is a table entry, not
  a redesign.
- **A dropped tag is provenance, not a tag.** The checker records the quantity a
  product discarded, and never claims the value still carries it: an untagged
  T⁻¹ computed from a becquerel converts into a curie without a word, and only a
  *conflicting* tag is reported (D16). Merging the two fields would turn every
  legitimate naming of a computed magnitude into a diagnostic, which is the
  false-positive failure this pass is built to avoid.
- **A method value is refused, not unpacked.** It binds the receiver into a
  closure and calls a wrapper without it, and the wrapper carries the original
  method's own object — so reading the operands off such a call by position reads
  the wrong ones. A receiver bound out of sight is not provable anyway.

**Silencing a report.** A test that asserts an operation fails is an operation
the pass is right to report and nobody wants reported. A
`//unitvet:ignore <reason>` line comment silences the diagnostic on its own line
and on the line below it, spelled to match the `//coverage:ignore` markers of D14
rather than inventing a second convention. The pass does not read the reason; the
next reader does. This is the one escape hatch, and it is deliberately a
source-level one: a flag that switched a rule off globally would hide the next
real defect along with the deliberate one.

**Why it earns its place.** D1 established that Go cannot express dimensional
analysis in its type system. D13 recovers a useful part of what const generics
would have given — at the point in the toolchain where Go actually puts this kind
of check, and without asking users to change how they write code. It is opt-in,
it composes with existing `go vet` and CI setups, and third parties can run it
against their own code without depending on it.
