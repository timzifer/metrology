# 7. State and the v1.0.0 surface

Everything the decisions describe is implemented and enforced:

| Subsystem | State |
|---|---|
| `dimension`, `symbol` | complete; the property test asserts `Product(q, q.Reciprocal()) == One` for random dimensions |
| Core: `Measurement`, `Unit`, arithmetic, `Engine` | complete; the aliasing guard of D3 is green under `-race`, round-trip conversion reproduces exactly across the catalogue, and each of the five kind rules of D6 has a test |
| Catalogue and generator | complete for the scope of [section 6](catalogue.md); `go generate ./...` is reproducible and CI fails on a dirty tree |
| Text form: writing, `parse`, JSON, SQL | complete; the round-trip property holds across the whole catalogue and the parser is fuzzed against it |
| `unitvet` | complete; the corpus asserts the reported and the silent cases alike, and the pass runs clean over this repository, tests and examples included |
| Coverage gate | 100 % of hand-written statements, enforced in CI, `COVERAGE_EXCEPTIONS.md` empty |
| Symbolic factors (D20) | complete; the constant is checked against Machin's formula, every crossing conversion in the catalogue is checked against the same conversion at sixty digits, the four sign combinations of a directed bound are asserted case by case, and the property test of D15 runs over the π units as it does over the rest |
| GUM propagation, `metrology/gum` (D21) | complete; `x − x` and `x / x` are asserted to have no uncertainty at all, the product rule and the four correlation coefficients are checked against numbers computed by hand, a worked budget is a runnable example, the aliasing guard of D3 covers a value at 200 digits, and the `unitvet` corpus asserts the reported and the silent cases over values as it does over ranges; its exported surface is inside `v1.0.0` |
| The `int64` fast path (D17) | **decided, not built.** It is additive and invisible in the API, so it does not gate `v1.0.0`; what D17 fixes is that there is no type parameter and no facade to build it behind |
| `uncertainty` (D15) | complete; a conversion is asserted over the whole catalogue never to narrow a range and never to pull two overlapping ranges apart, the four-corner table of `Mul` and the even-power case of `Pow` are tested case by case, the aliasing guard of D3 covers both bounds at 200 digits, `FuzzRange` holds the text form to a fixed point, and the `unitvet` corpus asserts the reported and the silent cases over ranges as it does over measurements; its exported surface is inside `v1.0.0` |

**The deliberate API review before `v1.0.0` is complete.** Every item it had
to settle is settled below, on 2026-09-04, and nothing stands between the
module and the tag. Until the review happened the module was tagged `v0.x` and
the API could change without notice, because any stability promise made before
it would have been one to regret later. The items are struck through rather
than deleted, so that a reader who remembers the question finds the answer
where the question was:

- ~~the exported surface of `metrology` — which of `Times`, `Per`, `Pow`,
  `Prefixed`, `DecimalIn` and the `Engine` methods are load-bearing enough to
  freeze~~ — **settled: all of them.** The candidates for a cut were the ones a
  caller can compose from something else, and each turns out to be the one
  place a rule is enforced rather than a convenience. `Times`, `Per` and `Pow`
  are where D12's one-spelling rule lives — `Symbol.Product` gathers a
  repeated prefixable multiplicand into a power there and nowhere else, so a
  caller building a derived unit by hand would get a second spelling of the
  square metre. `Prefixed` is the other half of the `String`/`Prefixed` split of
  D12: writing is a method, and a caller that wants the prefixed form has no
  parser to get it from. `DecimalIn` is `In` without the numeric boundary — the
  decimal the library computed rather than a `float64` or an `int64` cut from
  it — and it is the one way to read an exact magnitude in a scale other than
  the one the measurement carries without constructing a second `Measurement`
  to take `Decimal` of; `unitvet` resolves it beside `To` and `In` for that
  reason. The `Engine` methods are the arithmetic, `Cmp` and `To` under an
  explicit precision, and every method on `Measurement` is the zero `Engine`
  applied to the same operation (D9); dropping one from the `Engine` would
  leave that operation with no way to run at another precision. Nothing in the
  list is a duplicate of anything else, so nothing is cut
- ~~the naming of the error types and their exported fields, since D11 makes
  those the substitute for a compile error~~ — **settled: as built.** Seven
  classes, one sentinel and one struct each, every struct with an `Op` field
  naming the operation and the operands beside it — `Want`/`Got` on a
  `DimensionError`, `Left`/`Right` on a `KindError` and a `QuantityError`,
  `Requested`/`Max` on a `PrecisionError`, `Value`/`Type` on a `RangeError`,
  `Input`/`Err` on a `SyntaxError` — and `NoScaleError` with `Op` alone, for the
  reason D11 gives. The names say what a compile error would have said, in the
  order it would have said it, and `errors.Is` reaches the class while
  `errors.As` reaches the fields. No rename was found that a reader would have
  needed
- ~~whether `parse.Text` is the right shape for the decoding boundary, or
  whether a parser-typed destination generated per catalogue would serve
  better~~ — **settled: `parse.Text` is the shape.** A generated destination
  type per catalogue would give every catalogue a type of its own to unmarshal
  into, and that is one type per catalogue the caller has to name in every
  struct that receives one — and a program with two catalogues, which D12's
  reasoning is built around, would have two of them for one measurement. `Text`
  carries the parser as a value instead, so the zero `Text` reads the shipped
  catalogue, `Parser.Text` reads any other, and the field type is the same in
  both cases. That is the value-not-registry rule of D7 applied to the decoding
  boundary, and generating a type around it would have moved the choice of
  catalogue from the value to the type without making it any more checkable
- ~~what `Quantity` promises~~ — **settled, and kept here so the answers stay
  findable.** **Identity or interpretation?** Identity (D16): a hertz is not a
  becquerel, the run time refuses the conflict in every additive operation and
  every conversion, and `unitvet` follows the tag through the products that drop
  it. The type stays a `string`, because the catalogue is data (D8) and the set
  of quantities is open — ten of the ninety units carry a tag.
  **Whose namespace?** This catalogue's (D16). Every tagged package declares its
  own constant — `frequency.Quantity` — and the unit definitions are generated
  from it. Not the *type's*: a caller's catalogue declares its own tags, two
  catalogues in one program may still tag a dimension differently, and D6's rule
  still compares spellings across them. What changed is that each side has one
  place where its spelling is written down.
  **What does untagged mean at a boundary?** Inside the core, the wildcard that
  keeps a computed magnitude nameable, with the enforcement it gives up moved
  into the static checker (D16). Crossing the text form, the spelling is the
  statement (D12): `50 Hz` reads back tagged, `50 s⁻¹` untagged, one scale and
  two readings, because a text that made no claim should not have one invented
  for it. A caller with an expectation converts — `m.To(frequency.Hertz)` — which
  returns hertz or an error, whichever spelling arrived.
- ~~whether `Engine.Rounding` and `Unit.OfDecimal` belong in the frozen
  surface~~ — **settled: both do**, and settled by the item below rather than on
  their own. Both are what D15 needed from the core — an interval bound has to
  round outward or a conversion can manufacture a disagreement that stands in no
  source, and a type holding bare magnitudes has to be able to label them again.
  Both are additive and invisible to a caller who does not ask: the zero
  `Engine` is unchanged, and `OfDecimal` is the counterpart of a `Decimal` that
  was already exported. Against them stood that `Rounding` is a second rounding
  policy in a library that had exactly one, and that `OfDecimal` widens the door
  into `Measurement` from two constructors to three — which is why the review
  had to see both rather than inherit them. What decides it is that neither is
  one layer's convenience: D21 needs exactly these two and nothing else, D20
  needs `Rounding` for the direction a π factor moves a bound in, and both of
  those layers ship inside `v1.0.0`. A hook two shipped layers ask for is a
  hook, not an accident — and freezing the layers without freezing the two
  hooks they are built on would be freezing nothing
- ~~the shape of `Unit.Factor`~~ — **settled by building D20 first.** The getter
  returns a `Factor` struct carrying the π exponent beside the fraction, because
  a caller reading the fraction alone computes a wrong number and the compiler
  has to stop that caller. This was the one item whose *order* was decided rather
  than open, and it is done: the change was free in `v0.x` and would have been a
  `v2` after the freeze. What the review still sees is the struct itself
- ~~whether `uncertainty.Range` and `gum.Value` ship inside `v1.0.0` or behind
  it~~ — **settled: both ship inside, and each of the six arguments is frozen as
  built.** Both layers exist, both are the first serious consumers of the core,
  and the table above records what each is measured by. A layer that is complete
  and measured and still held behind the freeze is a layer nobody can depend on,
  for no reason anybody can name.

  **The interval layer.** `Mid` and `Width` keep their error, because D15's own
  correction established that neither is total and that both failures are
  reachable and tested; a signature without the error would have to swallow one
  of them. `PlusMinus` keeps `(string, bool)`: there is exactly one reason for a
  no and exactly one thing to do about it, which is `String`, so an error value
  would carry nothing the caller does not already have — it is the
  `String`/`Prefixed` split of D12 with the same rule. And `uncertainty.Engine`
  stays a type of its own precisely *because* it carries no rounding mode.
  Borrowing `metrology.Engine` would hand the caller the one knob D15 takes
  away, and outward rounding is the property the layer exists for; the type is
  the enforcement.

  **The propagation layer.** `Input` and `Standard` both stay, because they are
  not two spellings of one thing. `Input` is the extensible form, so a field
  added after the freeze is additive; `Standard` is the two-argument case that
  never has to grow, and D21 already records a second reason for it — a
  composite literal is a container read `unitvet` does not follow, so the
  positional constructor is the one that keeps the common case provable. Keeping
  only the struct puts every budget in ceremony and blinds the checker; keeping
  only the pair leaves a name and a degree of freedom nowhere to go. `Apply`
  keeps its shape — an estimate and one `Partial` per input, with the derivative
  a `Measurement` rather than a number, so the core checks ∂f/∂x · u(x) against
  the span unit of the result and a derivative in the wrong units is a dimension
  error instead of a plausible answer. `EffectiveFreedom` stays: ν_eff is
  computed *from* the budget by Welch-Satterthwaite and only the budget holds
  the contributions that go into it, whereas Table G.2 is a lookup that needs no
  budget and would arrive here with nothing to check it against — the thing D4
  refuses for a conversion factor. Dropping the method would leave ν_eff
  computable nowhere; shipping the table would put a page of unchecked numbers
  in a repository that refuses them everywhere else.

  **What the freeze does not close.** Additions. `Range` has no `Contains`, no
  `Equal` and no `Cmp` where the core has two of the three, and the freeze does
  not decide that: a method added later is additive and costs a caller nothing.
  What a freeze cannot repair afterwards is a *shape* — an error that should not
  be there, a positional pair that should have been a struct — and those are the
  six settled above
- ~~`O1`, whether `imperial` is a subpackage or a module~~ — settled as D19: a
  subpackage, `units/customary`, from a catalogue file of its own
- ~~`O3`, what covers the units the two customary systems disagree about~~ —
  settled as part of D19: `units/customary`, with `us` and `imperial` below it

What the review no longer has to settle is whether the arithmetic is a type
parameter of `Measurement`: D17 says it is not, and that was the one open
question whose answer could not be deferred past `v1.0.0`. The `int64` fast path
it leaves open is invisible in the API and lands whenever it is written.

`cmd/unitvet` is versioned with the library but breaks nothing on its own: it is
additive, opt-in, and can ship a new version independently.
