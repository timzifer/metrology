# Metrology — Architecture Concept

> A Go library for physical quantities: exact decimal arithmetic, runtime
> dimensional analysis, and one package per quantity.

| | |
|---|---|
| **Module** | `github.com/timzifer/metrology` |
| **Go** | 1.27 (minimum) |
| **State** | complete for the scope of section 6; `v1.0.0` awaits the API review of section 7 |

This document holds the architecture and the reasoning behind it. The decisions
are numbered D1 … D14 and referenced from code comments; a change that
contradicts one updates the decision first, in the same pull request, with the
reason. Silent divergence between this document and the code is the failure mode
it exists to prevent.

---

## Table of contents

1. [Scope](#1-scope)
2. [Guiding principles](#2-guiding-principles)
3. [Decisions](#3-decisions)
4. [The API](#4-the-api)
5. [Package layout](#5-package-layout)
6. [The catalogue](#6-the-catalogue)
7. [State and the road to v1.0.0](#7-state-and-the-road-to-v100)
8. [Deferred](#8-deferred)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)
11. [Appendix: verification log](#11-appendix-verification-log)

---

## 1. Scope

The library models a physical quantity as a decimal magnitude, a unit and a
dimension, and it does so in **concrete value types**: generic methods are
forbidden in interfaces, and an interface value boxes and allocates for a type
that may exist in the millions. That single constraint shapes the rest of this
document.

### Dimensional analysis is a runtime concern

Go's type system cannot carry it. Measured against the compiler (section 11):

| Question | Answer |
|---|---|
| Generic methods on concrete types | **supported** |
| Generic methods in interfaces | **forbidden** — `interface method must have no type parameters` |
| Heterogeneous storage of instantiated generic types | **impossible** |
| Integer type parameters for exponents (const generics) | **do not exist** |

A `Q[Length, Time]` with exponents checked by the type system cannot be built in
Go without code generation, and generating over the cross-product of all
dimensions is not a viable path. The library compensates with precise errors
instead of compile errors — and makes those errors a first-class part of its API
(D11).

It also compensates outside the type system: D13 ships a `go vet` pass that
proves what *can* be proven statically and stays silent about the rest. That
recovers a useful share of the safety const generics would have given, at the
point in the toolchain where Go conventionally puts such checks.

### Name and module path

`github.com/timzifer/metrology`. The reasoning, recorded because it will be asked
again:

- **not `units`** — semantically perfect, practically the worst option. At least
  six published Go modules already declare `package units` (alecthomas, docker,
  bcicen, ganehag, woweh, go.pitz.tech). No path conflict, but anyone importing
  ours alongside any of those needs an alias forever.
- **not `quantity`** — the closest contender, and the ISO 80000 term for exactly
  what is modelled. Rejected because *quantity* is already load-bearing vocabulary
  **inside** the library: the catalogue maps dimension to quantity, D6 speaks of
  kind-bearing quantities, and the YAML field is called `quantity`. A module of
  the same name makes the word do duty at two levels and forces every doc sentence
  to disambiguate.
- **not `si`** — the catalogue contains bar, torr, °C and kWh, and the planned
  sister package is non-SI by definition. A name that misdescribes its own
  contents from day one.
- **`metrology`** names the field, not a concept in the model. That leaves
  *quantity*, *measurement*, *unit*, *dimension* and *kind* free as internal
  vocabulary — which matters for a library whose entire purpose is precise
  terminology.

The one honest cost: metrology as a field also covers calibration, traceability
and uncertainty, and uncertainty is deferred (section 8). The name promises
slightly more than v1 delivers. `metrology/uncertainty` is the natural home when
that changes.

No `go-` prefix: that convention marks a Go binding to something else
(`go-redis`, `go-sql-driver`), and the last path element becomes the default
package name, which `go-metrology` could not be.

The same argument settles package names inside the module. `duration` is not
called `time`, because a program that measures durations usually imports the
standard library package of that name as well, and a library that forces an
alias on every consumer has picked the wrong name.

---

## 2. Guiding principles

Every decision is measured against these seven.

1. **A measurement is a value, not an object.** Copyable, comparable, no
   identity, no hidden behaviour.
2. **Nothing is mutated after the fact.** Every operation returns a new value; no
   existing value is ever written in place.
3. **Accuracy before speed** — but only in the core. Callers who need `float64`
   get it at the boundary, not in the middle.
4. **Conversion factors are exact.** Rounding happens once, at the end, by a
   documented rule — never in the catalogue.
5. **No state created by an import.** The catalogue is generated code, not
   runtime registration.
6. **Wrong physics is an error value, not a panic.** Panics exist only in
   explicitly named `Must` variants.
7. **Every hand-written line is covered.** 100 % statement coverage is enforced
   in CI — see D14 for what that does and does not mean.

---

## 3. Decisions

Each decision states what it costs. Where a decision makes later revision
expensive, that is noted.

### D1 — Measurement and Unit are concrete value types

`Measurement` is a struct of a decimal value and a unit, not an interface. `Unit`
likewise. There are no `Quantity`, `BaseUnit` or `DerivedUnit` abstractions
behind them, and `Symbol` is a tagged value type rather than a hierarchy: static,
SI-prefixable, gram, litre, product and quotient differ only in how they render
and which prefixes they accept, which is a switch, not a set of implementations.

**Why.** Two reasons converge. Generic methods are forbidden in interfaces, so
D10's `Of[N]` and `In[N]` cannot exist on one. And every interface value boxes
and allocates, which is measurable for a type that may exist in the millions.

**Cost.** Third-party extension happens through data — a catalogue of your own,
passed as a value — not through implementing types.

### D2 — apd/v3, not v2

`github.com/cockroachdb/apd/v3` as the single arithmetic dependency, never v2.

> **Measured, not assumed.**
> In **apd/v2**, `Decimal.Coeff` is a `math/big.Int`. An ordinary struct copy
> shares its slice: if either copy is written in place, the other changes
> silently — *at any digit count*, and passing a struct with an embedded
> `apd.Decimal` around by value is enough to trigger it. In **apd/v3** an inline
> optimisation covers values up to 38 digits; beyond that the same behaviour
> returns — which makes the bug rarer and therefore more treacherous.

**Consequence.** v3 does not solve the problem, it only moves the threshold. What
actually solves it is D3.

### D3 — Immutability as the load-bearing invariant

Every operation allocates its destination `Decimal` fresh. No function ever
writes into a `Decimal` reachable from an existing `Measurement`.
`Measurement.Decimal()` returns a copy taken via `Set`, never a pointer into the
interior.

**Why this suffices.** The aliasing from D2 only becomes dangerous when something
mutates. Because the invariant holds without exception, copies are safe — and the
types stay genuine Go values that can be passed and copied freely, which happens
on every `Measurement` copy. The invariant is therefore not a matter of style; it
is what carries correctness. (Equality is `Unit.Equal` and `Measurement.Equal`,
not `==`: a unit holds its decimals by pointer. `Dimension` is a plain word and
*is* comparable and usable as a map key.)

**Enforcement.** A guard test runs every public operation on a 200-digit value
and then asserts that copies taken beforehand are unchanged. Two hundred digits
is deliberate: below 38, apd/v3's inline optimisation hides the aliasing, so a
guard written at the sizes a test would otherwise use would pass while the defect
was present. The guard runs in CI under `-race`, and it is the test that fails
first on a regression.

### D4 — Factors as exact fractions

A derived unit carries numerator and denominator separately, plus an offset as an
exact decimal. Conversion to the base unit is `(v + offset) · num / den`,
performed as an exact multiplication followed by *one* division.

**Why.** The domain's most important factors are not finite in decimal:
Fahrenheit is 5/9, Torr is 101325/760. Stored as a pre-rounded decimal, every
conversion rounds twice. Stored as a fraction, it rounds once — and the catalogue
stays exactly what the SI Brochure says rather than an approximation of it.

**Auditability.** A catalogue entry can be compared to its source character by
character. `factor: 101325/760` is checkable; `133.32236842105263` is not. Every
entry therefore carries a `source:` citation and the generator refuses one that
does not: a conversion factor is a claim about the world, and a claim without a
citation cannot be checked.

**Exactness is a precondition, not a preference.** A unit whose factor is a
rational multiple of π — the degree of arc, the gon, the oersted — has no finite
decimal fraction and is *left out* of the catalogue rather than rounded into it.
Shipping a rounded factor silently because the unit is popular is how a catalogue
stops being auditable. What those units need is a symbolic factor; see section 8.

### D5 — Dimension: 7 packed exponents, kind held separately

`Dimension` is a packed integer holding seven `int8` exponents — comparable,
usable as a map key, allocation-free. `Kind` is its own field, outside that word.

| Bits | Field | Symbol |
|---:|---|---|
| 0–7 | time | T |
| 8–15 | length | L |
| 16–23 | mass | M |
| 24–31 | electric current | I |
| 32–39 | temperature | Θ |
| 40–47 | amount of substance | N |
| 48–55 | luminous intensity | J |
| 56–63 | reserved | — |

**Why they are separate.** Sharing one word produces two bugs at once: a
`WithoutKind` that clears four of eight kind bits through an operator precedence
mistake, and a `Product` that discards the kind entirely, so every multiplication
loses the absolute marker. Both are impossible by construction once kind is not a
bitfield in the same word — and kind gains room for more than eight values, which
D6 requires. The same argument later separated `Quantity` from `Kind`.

**Construction names its axes.** `New(Exponents{Time: -2, Length: 1})` allocates
nothing, names every axis at the call site, and has a matching inverse in
`Dimension.Exponents` — construction and destructuring read the same way, which
is what the generator of D8 emits.

**Exponent arithmetic wraps at the `int8` boundary and does not error.** Reaching
it takes 128 multiplications of the same axis. An error return on `Product` would
push a case that cannot occur into every caller of the D6 arithmetic, and D14
would then demand a test for a branch that is unreachable by construction.

The eight reserved bits are held for fractional exponents, which are deferred,
not forgotten (section 8).

### D6 — Kind and quantity, with explicit arithmetic rules

Two facts travel with a unit, and they are two fields for the same reason D5 took
the kind out of the dimension word: they are independent, and packing independent
facts into one value is how a `WithoutKind` ends up clearing four of eight bits.

**`Kind` — the affine distinction, *absolute vs. interval*.** 20 °C is a point
on a scale, 5 K is a distance along one. Two values, and the rules below.

**`Quantity` — which quantity a shared dimension is being read as.** The hertz
and the becquerel are both T⁻¹; the gray and the sievert are both L²T⁻²; a plane
angle, a solid angle and a bare ratio are all dimensionless; the candela and the
lumen are both J. A string tag rather than an enum, because the catalogue is
data (D8) and the set of quantities is open — the whole SI needs nine tags, and
none of them touches a line of the core.

The zero `Quantity` is untagged, and untagged is compatible with everything.
That is not laxity, it is the only workable rule: multiplication and division
drop the tag, so *every computed magnitude is untagged*, and a rule that refused
to name them would make each computation a dead end. The check fires only where
both sides make a claim and the claims differ — 50 Hz asked for in becquerel.

**Rules for addition and subtraction:**

| Operation | Result |
|---|---|
| absolute + interval | absolute — `20 °C + 5 K = 25 °C` |
| absolute − absolute | interval — `25 °C − 20 °C = 5 K` |
| interval ± interval | interval |
| absolute + absolute | error |
| interval − absolute | error |

A sum takes the tag of whichever operand carries one: an untagged T⁻¹ added to a
frequency is a frequency, and there is nothing else it could be.

**Rule for multiplication and division.** The result carries *neither* kind nor
quantity. A product of a torque and an angle is no longer a torque, and a system
that tries to guess will guess wrong. Naming the result is an explicit, checked
conversion — `q.To(pressure.Pascal)`, or `catalog.Canonical` for the unit that
dimension resolves to. Absolute values may not be multiplied at all: 20 °C times
2 is physically meaningless and returns an error.

**An interval unit may not carry an offset.** An offset is what makes a scale
affine, and an affine scale measures points. Rejecting the combination at
construction removes the case from every later operation: a unit that reaches the
arithmetic as an interval is linear, so a product never has to ask.

**An absolute unit declares the interval unit its differences are read on** — K
for °C, °R for °F. Without it `25 °C − 20 °C` would have to be 5 °C, which reads
like a temperature and is not one. The difference is *converted* onto the
declared unit, not merely labelled with it, so a scale declaring a counterpart
with a different factor still yields the right number. The scale the difference
is *computed* on is a third unit — the receiver's own factor without the offset —
and conflating the two is a live trap; a test with a Celsius scale declaring
degrees Rankine holds them apart.

**What the tag does not solve.** Two quantities on one dimension that also share
a *symbol* remain indistinguishable in the text form of D12 — `5 m²/s` is
kinematic viscosity and thermal diffusivity and a diffusion coefficient, and no
tag in the world makes that string read back to one of them. The catalogue
therefore carries one of the three and says so; the others wait for a text form
that carries the quantity as well (section 8).

### D7 — No global state, no init side effects

The catalogue is generated Go code: package-level `var`s and a map from dimension
to canonical unit, without a mutex and without runtime registration. There is no
`Register` and no `Lookup`.

**Why.** Runtime registration means every import of a quantity package creates
global state, and two packages claiming the same dimension panic at process
start, in import order, at the user's site, where nothing can be done about it.
For a published library that is the worst available failure mode. Generated code
does not have this failure class — collisions surface at generation time,
in-house, as a failed build.

**Package-level `var`s are not the state this forbids.** `pressure.Bar` reads the
way callers write it and a function would rebuild its decimals on every call.
Nothing writes to these after init, nothing registers itself, and there is no
runtime `Register` to race with. What D7 forbids is state that an *import*
creates. For the same reason the seven base dimensions are `const`, not `var`: a
constant is not state at all.

**For user-defined units.** A catalogue value the caller constructs and passes —
to `parse.New`, for instance. A value, not a global.

### D8 — The catalogue is data; the Go code is generated

Quantities, units, symbols, factors and source citations live in
`catalog/catalog.yaml`. `tools/catgen`, run through `go generate ./...`, produces
the quantity packages, `catalog/units_gen.go` and `unitvet/table_gen.go`.

**Why.** At the scope of section 6 this is hundreds of lines that would otherwise
be the same four lines per unit, with four chances for a transposed digit. As
data it is a table that can be checked against the SI Brochure and NIST SP 811 —
and one where each entry is independent.

**The generated files are declarations, nothing else.** A quantity package is a
list of `var X = metrology.MustUnit(…)`; the catalogue index is three composite
literals. There is no generated logic, so there is no generated branch anyone has
to write a test for, and the lookups that do have behaviour are hand-written in
`catalog/catalog.go`, where the coverage gate sees them.

**The generator validates before it writes,** and a rejected catalogue writes no
file. It refuses duplicate ids, duplicate Go identifiers, duplicate symbols
within a kind, two units claiming to be canonical for one dimension and quantity,
a dimension with no canonical unit at all, a factor that is not a number or is
zero, an offset on an interval unit, an interval reference that is missing,
absolute or of another dimension, a package declaring two dimensions, and a unit
without a source. **Unknown YAML keys are an error too:** a misspelled `factorr:`
would otherwise silently produce a unit with a factor of one, which is the one
defect a catalogue must not be able to have.

**The output is ordered, not iterated.** Units are emitted sorted by id, imports
deduplicated and sorted, so two runs are byte-identical and CI can check for a
dirty working tree after `go generate ./...`. A map range anywhere in the emitter
would turn that check into a coin toss.

**Side benefit.** The file doubles as documentation and can be exported as a
machine-readable unit catalogue.

### D9 — Precision belongs to the computation, not the value

A `Measurement` carries no `apd.Context`. Operations use a package default.
Callers needing more construct an explicit `Engine` value, whose zero value is
that same default — so there is no package-level context to configure and D7
stays intact.

**Why.** A context carried inside the value forces a rule deciding whose
precision wins in an addition, and such rules are not predictable for users.
Precision is a property of the *computation*, not of the measurement; it belongs
where the computing happens.

**The default is 20 significant digits.** decimal128 (34 digits) was the obvious
choice because it follows a citable standard; the measurement showed what the
standard costs, for one `bar → torr` conversion — one offset, one multiplication,
one division:

| Precision | Per conversion | Allocations |
|---|---:|---:|
| 20 digits | 783 ns | 1 |
| 34 digits (decimal128) | 1541 ns | 7 |
| 50 digits | 3089 ns | 15 |
| `float64` for reference | 0.34 ns | 0 |

The table is produced by `BenchmarkConvert` in `bench_test.go`, so it can be
rechecked rather than believed:

```sh
go test -run '^$' -bench BenchmarkConvert -benchmem .
```

Absolute figures are a property of the machine — a re-run on a shared cloud VM
gives 372 / 504 / 685 ns for the three precisions and 0.6 ns for `float64`, the
median of five runs, and README.md tabulates that whole run — and the ratios move
with the machine too. What does not move is the shape: the
default allocates once per conversion, decimal128 allocates five to seven times,
50 digits again about twice that, and every step costs time without benefiting
any physical measurement — no sensor in this domain delivers more than six to
eight trustworthy digits. That shape is what the decision rests on, and it is why
the benchmark is in the tree instead of only its output. decimal128 remains one
`NewEngine(34)` away.

**Multiplication and division round; addition and subtraction do not.** A sum of
two decimals is exact and stays exact. A chain of exact products doubles its digit
count at every step, so those round to the engine's precision — which is where
this decision says the rounding belongs.

**Every result is reduced before it is returned.** After a division `apd` pads to
the full context precision with zeros: `2.5 bar` otherwise becomes
`250000.0000000000000000000000000000 Pa`. Numerically correct, unusable as the
exchange format of D12.

**Error handling.** An inexact result is normal and not an error. Overflow,
division by zero and context violations are errors.

This also settles a related question: the library does **not** track significant
figures. Whether a value was measured as `2.50` or `2.5` is lost. That is
deliberate — measurement uncertainty is a topic of its own (section 8), not a
by-product of number representation.

### D10 — Generic methods at the system boundary

The core computes exclusively in `apd.Decimal`. Input and output in arbitrary
numeric types go through generic methods, which Go 1.27 permits on concrete
types.

```go
func (u Unit) Of[N Numeric](v N) Measurement
func (m Measurement) In[N Numeric](u Unit) (N, error)
```

`go.mod` declares `go 1.27` for this — the language version follows from that
line, not from the installed toolchain. It is also the library's minimum version.

**The exact readout is `DecimalIn`, not `In[*apd.Decimal]`.** A pointer type
cannot join a `~float64`-style type set, so the exact path is its own method.

**`Unit.Of` is total.** A NaN or an infinity is carried as the decimal form of
itself rather than rejected at the boundary, so construction never returns an
error a caller has to thread through. Both stay visible: they print as NaN and
Infinity, and asking for one as an integer is a `RangeError`.

**`Measurement.In` refuses rather than truncates.** A fractional magnitude read
into an integer, or one outside its range, is an error and not a silently altered
number — which is the failure mode this library exists to avoid.

### D11 — Errors are typed and comparable

One package-level set of error types: sentinel values for the class, struct types
for the context, everything usable with `errors.Is` / `errors.As`.

Because dimensional analysis happens at runtime per D1, the error message is what
the user gets instead of a compile error. It must name both dimensions in
readable form — `expected L²M¹T⁻², got L¹M¹T⁻²` — not merely
`dimensions not equal`.

**The dimension stringer uses a fixed axis order,** `L M T I Θ N J`, rather than
sorting by exponent. This message is read by someone comparing two dimension
strings; a fixed order means one differing exponent produces one differing
character, where sorting by exponent would permute `L²M¹T⁻²` against `L¹M¹T⁻²`.

### D12 — Text is the canonical exchange format

`MarshalText` / `UnmarshalText` as the foundation, with `json.Marshaler`,
`sql.Scanner` and `driver.Valuer` layered on top. A measurement serialises as
`"2.5 bar"` and round-trips losslessly.

**Why text and not number plus unit.** An object `{"value": 2.5, "unit": "bar"}`
forces every consumer through `float64` and thereby loses exactly what D2 through
D4 were built for. The text form preserves the decimal digits. The object form
stays available as an option but is not the default.

**Writing belongs to the measurement, reading does not.** Writing needs nothing
but the value: `Measurement` implements `MarshalText`, `MarshalJSON` and
`driver.Valuer` itself. Reading needs a catalogue — `"bar"` is a unit only
because something says so — and the core has no catalogue and no registry to put
one in (D7, D8). The standard decoding interfaces are handed no context, so a
`Measurement.UnmarshalText` could resolve symbols only out of a package-level
registry, and every program with units of its own would be locked out of exactly
the interfaces it needs most.

Reading therefore lives in `parse`, where a parser is a *value* holding the units
it knows — `parse.Measurement` over the shipped catalogue, `parse.New(mine)` over
any other — and `parse.Text` is the destination type that carries its parser into
`json.Unmarshal` and `sql.Scan`. The asymmetry is the honest shape of the
problem: a symbol table is context, and Go's decoding interfaces pass none.

**What the text does not carry.** It carries a magnitude and a symbol, and
neither the kind of D6 nor the quantity tag. `"5 K"` is a temperature and a
temperature difference written the same way; `"5 m²/s"` says nothing about which
of the quantities on that dimension was meant. A parser resolves both from what
it has: the kind from `Parser.Prefer` — defaulting to the interval reading,
because that is the one that composes with an absolute one — and the quantity
from the catalogue entry the symbol resolves to. An expression such as `"50 N/m²"`
resolves to no catalogue entry and is therefore untagged, which is what a
computed magnitude is too (D6), so it converts into any unit of its dimension.

**A symbol's spellings are enumerated, never guessed.** `Symbol.Spellings`
reports every way a symbol may be written and the parser indexes exactly those. A
static symbol admits no prefix at all, and a prefix is only ever read in front of
a symbol whose form declares one. That is what keeps `cd` the candela rather than
a centi-day, and `mmHg` a millimetre of mercury rather than a
milli-metre-of-mercury. A longest-prefix matcher over the alphabet would have to
guess, and would guess wrong on exactly these. Where a spelling collides, the
catalogue entry wins: `km` is the kilometre with the source citation, not the
prefixed metre — the same scale either way.

**Rendering has to be unambiguous before parsing can be exact.** A solidus and a
middle dot bind equally and from the left, so `m/(s/A)` and `b/(km/h)` need their
brackets — without them they read as `(m/s)/A` and `(b/km)/h`, which are
different dimensions. The bracketing rule lives on the rendered text rather than
on the symbol form, because a static symbol can join two units too. For the same
reason a product of a product is flattened on construction: it already rendered
flat, so keeping the nesting left two structures for one symbol and made
`Symbol.Equal` answer false for two symbols that print alike.

**An expression that spells a unit the parser knows *is* that unit.** `m²/ s` and
`m²/s` differ by a blank, and without this substitution only the second would
carry the quantity tag of D6. The substitution checks the scale and not only the
spelling: a caller's catalogue may spell something `m/s` that is not a metre per
second, and naming that would change the factor instead of the tag.

**A power of a power is bounded before it is computed.** `(Qm^127)^127` is a
factor of half a million digits, and one bracket more is sixty million — all in
fourteen characters of input. The parser multiplies the exponents of nested
brackets and refuses anything beyond `MaxPower`, the same bound a single power
has. Judging the result afterwards would be too late.

**The core validates the shape of a number itself,** through
`internal/decimaltext`, the one scanner both the core and the parser use: apd
accepts `".-1"` and renders it as `"0.-1"`, a value whose own text form is not a
value, and a zero with a positive exponent prints as `"00"`. Reading untrusted
text is what makes such defects reachable, but they are core defects and the
check belongs in `Unit.OfString`. The *range* of a magnitude stays apd's
business: it rejects an absurd exponent first, so a second check in the parser
would be dead code.

**Prefix selection is exact decimal arithmetic** (D9), not a logarithmic search:
`floor(log10 |v|)` comes from the digit count and the exponent of the
`apd.Decimal`, and applying the prefix shifts that exponent, so no digit is lost
and 1000 m is exactly 1 km where `math.Log` yields 999.9999999 m. The result is
trimmed of the trailing zeros the shift introduces but not reduced past the
decimal point: 250 kPa stays `250`, not `2.5E+2`. One prefix step on `m²` is a
factor of 10⁶ and on `m³` a factor of 10⁹, so the step scales with the power and
handles negative powers such as the wavenumber m⁻¹. The kilogram is a *gram*
symbol: magnitudes are in kilograms, prefixes attach to the gram, and the
unprefixed rendering is `kg` — the symbol always names the unit the magnitude is
in.

**`String` is the canonical form, `Prefixed` the display form.** The canonical
text keeps the unit the measurement is held in, because the text has to read back
as the same measurement. Prefix selection is a rendering choice and lives in its
own method.

### D13 — A `go vet` pass that checks dimensions statically

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
| a unit computed with `Div`, `Per` or `Pow`, then used | reported — the walk follows the composition |
| operand from another package's function with an invariant unit | reported, via facts |
| same dimension, different units (`bar` + `Pa`) | correctly silent |
| unit chosen at runtime (`if x { u = Bar }`) | silent — SSA φ-node, not provable |
| operand arriving as a function parameter | silent — unknown origin |
| a unit from the caller's own catalogue | silent — not in the generated table |
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
so a diagnostic and the error it predicts never disagree. What is *not* checked
is a declared result unit for `Div`: the quotient carries the unit its operands
gave it and is named in a separate, checked step.

**Four resolution rules that are easy to get wrong.**

- **A unit is trusted only where the catalogue names it.** The resolver reads a
  package-level variable when — and only when — the generated table has it, which
  is what keeps the pass from assuming a variable it does not know is never
  written to. A program with a catalogue of its own is silent rather than wrong.
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

### D14 — 100 % statement coverage of hand-written code, enforced

CI fails below 100 % statement coverage. The target applies to hand-written
packages; generated files are excluded from both numerator and denominator.

**Why this library and not every library.** Three properties make the target
reachable here rather than aspirational. The core is pure computation with no I/O,
no clock and no network — every branch is reachable from a table-driven test. The
catalogue is generated (D8), so the part that would otherwise dominate the line
count and be tested by copy-paste is excluded by rule. And the failure mode this
library must avoid is the silent wrong number, which is exactly what an untested
branch produces.

**What is excluded, and how.**

| Excluded | Mechanism |
|---|---|
| Generated catalogue code | files carrying the standard `// Code generated … DO NOT EDIT.` line are filtered out of the coverage profile |
| Defensive branches that cannot be triggered | require an explicit `//coverage:ignore` comment stating why, and are listed in `COVERAGE_EXCEPTIONS.md` with a rationale |
| `cmd/` and `tools/` | thin wrappers and build-time tools; the analysis pass itself is covered through `analysistest`, the generator through its own tests |

Anything else claiming an exemption is a design smell, not a testing problem. An
error branch that cannot be reached usually means the error cannot occur and the
check should go, or that the dependency needs to be injectable so it can be made
to fail.

Because the generated files are declarations only (D8), they currently contribute
*zero* statements, so the exclusion changes no number today. It is tested anyway,
against a generated fixture that does contain statements: the day a generated
file grows a function is not the day to discover the filter never worked.

**The trap, named explicitly.** Coverage measures execution, not verification. A
test that calls a function and asserts nothing raises the number and lowers the
value — it converts an untested line into a line everyone believes is tested,
which is worse than where we started. The rule is therefore: **coverage is a
floor, never the goal.** The correctness weight is carried by the property tests
of the dimension algebra, the round-trip tests of the parser, the kind-rule table
of D6, the aliasing guard of D3 and the NIST golden tests of the catalogue.
Coverage only ensures none of them has a blind spot.

**Mechanics.** `go test -covermode=atomic -coverpkg=./... -coverprofile=…` across
all packages, so cross-package calls count; `tools/covercheck` strips generated
files and the declared exceptions, then compares against 100. With `-coverpkg`
every test binary reports every block of every package, so a block covered by one
package's tests also appears with count 0 in the profiles of the others; merging
the repeated records is what makes the number right, and without it the gate
reports covered code as uncovered as soon as a second test package exists. Blocks
with no statements — an empty `AFact` method body — are neither covered nor
uncovered and are not listed. The per-function output is part of the CI log,
because "which function dropped" is the only useful form of a coverage failure.

---

## 4. The API

The shape, in the vocabulary the decisions establish. Runnable examples live in
the godoc of each package.

```go
// --- Core -------------------------------------------------------

type Dimension uint64          // 7 × int8, packed (D5)
type Kind      uint8           // absolute or interval, held separately (D5/D6)
type Quantity  string          // which quantity a shared dimension is read as (D6)

type Unit struct {             // value type, immutable (D1/D3)
    dim      Dimension
    kind     Kind
    quantity Quantity
    sym      Symbol
    num      *apd.Decimal      // exact fraction (D4)
    den      *apd.Decimal
    offset   *apd.Decimal
    interval *Unit             // the unit differences are read on (D6)
}

type Measurement struct {      // copyable; compare with Equal, not ==
    unit Unit
    val  apd.Decimal           // never written in place (D3)
}

// --- Construction and readout -----------------------------------

m  := pressure.Bar.Of(2.5)                     // implicit N = float64
m2, err := pressure.Bar.OfString("2.50000000001")

pa, err := m.In[float64](pressure.Pascal)      // 250000
d,  err := m.DecimalIn(pressure.Pascal)        // exact, not In[*apd.Decimal] (D10)

// --- Arithmetic -------------------------------------------------

t, _ := temperature.Celsius.Of(20).
        Add(interval.Kelvin.Of(5))             // 25 °C          (D6)

_, err = temperature.Celsius.Of(20).
        Add(temperature.Celsius.Of(5))         // ErrAbsoluteSum (D6)

q, _ := force.Newton.Of(100).
        Div(area.SquareMetre.Of(2))            // 50 N/m², kind and tag dropped

u, _ := catalog.Canonical(q.Dimension(), q.Kind(), q.Quantity())
p, _ := q.To(u)                                // 50 Pa, named in a checked step

// --- Composing units --------------------------------------------

perSecond, err := volume.CubicMetre.Per(duration.Second)  // Times, Per, Pow

// --- Precision --------------------------------------------------

e := metrology.NewEngine(34)                   // decimal128; zero Engine is
r, err := e.Mul(m, m)                          // DefaultPrecision = 20 (D9)

// --- Inspecting errors ------------------------------------------

var de *metrology.DimensionError
if errors.As(err, &de) {
    log.Printf("expected %s, got %s", de.Want, de.Got)
}

// --- Text: writing is a method, reading is a parser (D12) -------

text, _ := p.MarshalText()                     // "50 Pa"
data, _ := json.Marshal(p)                     // "50 Pa", quoted
disp    := p.Prefixed()                        // the display form

m3, err := parse.Measurement("250 kPa")        // the shipped catalogue
u2, err := parse.Unit("J/(kg·K)")              // expressions resolve too
mine    := parse.New(myUnits)                  // a catalogue of your own

var field parse.Text                           // and a destination that
err = json.Unmarshal(data, &field)             // carries its parser along
```

---

## 5. Package layout

| Package | Contents |
|---|---|
| `metrology` | `Measurement`, `Unit`, arithmetic, `Engine`, error types, serialisation |
| `dimension` | the packed word, product, quotient, reciprocal, stringer |
| `symbol` | SI prefixes, product, quotient and the special forms, spellings |
| `parse` | reading the text form, resolving unit expressions, `parse.Text` |
| `catalog` | the YAML source, the generated index, and the lookups over it |
| `unitvet`, `cmd/unitvet` | the static dimension checker of D13 |
| `internal/superscript` | superscript digits for both stringers, and reading them back |
| `internal/decimaltext` | the shape of a decimal, for the core and the parser |
| `tools/catgen` | the generator of D8 |
| `tools/covercheck` | the coverage gate of D14 |
| `length`, `pressure`, `temperature`, … | one package per quantity, fully generated |
| `interval` | the interval units the absolute scales subtract into |

**One package per quantity,** because it turns autocompletion into a search
function: `pressure.` lists exactly the pressure units. Nobody maintains those
packages by hand — they are generated (D8), one dimension per package, enforced
by the generator. Where one dimension carries two quantities they are two
packages, `frequency` and `activity`, each with its tag.

**The interval units live apart.** `temperature` holds the absolute scales — °C,
K, °F — and `interval` the spans they subtract into, so
`temperature.Celsius.Of(20).Add(interval.Kelvin.Of(5))` reads as what it is and
no package means two things.

**The `unitvet` corpus is a module of its own** under `unitvet/testdata`, with a
`replace` back to the repository root, because `analysistest` runs a directory
holding a `go.mod` in module mode. That is what lets the corpus import the real
quantity packages: a checker tested against a stand-in written next to it would
prove only that the stand-in matches the table. `go build ./...` at the root does
not reach it; CI builds it from its own directory, because `analysistest` loads
it with `GOPROXY=off` and a cold module cache would otherwise fail the run.

---

## 6. The catalogue

`catalog/catalog.yaml` holds **82 units across 43 quantity packages**: all seven
SI base units, all twenty-two named derived units, and the non-SI units of NIST
SP 811 that process engineering uses.

| Block | Contents |
|---|---|
| SI base | s, m, kg, A, K, mol, cd |
| Named derived SI | rad, sr, Hz, N, Pa, J, W, C, V, F, Ω, S, Wb, T, H, lm, lx, Bq, Gy, Sv, kat, °C |
| Mechanics, heat, material data | area, volume, velocity, acceleration, density, concentration, mass flow, volume flow, viscosity and kinematic viscosity, surface tension, thermal conductivity, specific heat |
| Process-engineering non-SI | bar, torr, mmHg, mmH₂O, atm, l and l/min, m³/h, kWh, ppm and ppb, °F, t, min, h, d |
| CGS and other legacy units | dyne, erg, poise, stokes, gauss, maxwell, curie, rem, calorie, electronvolt, ångström, barn, are, hectare |
| Dimensionless | ratio, plane angle, solid angle — separated by the quantity tag of D6 |

A golden test compares every non-SI unit against the conversion factors printed
in NIST SP 811. It compares to **eighteen significant digits, not to the last
one**: factors such as one 3600th have no finite decimal form, so the conversion
rounds once by D9 and the return trip cannot undo that rounding. Eighteen digits
is two below the engine default and far past where a pre-divided factor fails — a
torr stored as 133.32236842105263 goes wrong in the seventeenth.

**What the catalogue deliberately does not contain.**

- **Units defined through π.** The degree of arc is π/180 radians, the oersted is
  1000/4π A·m⁻¹. Neither has a finite decimal fraction, and D4 does not admit a
  rounded one. They wait for symbolic factors (section 8).
- **The absorbed-dose rad.** Its symbol is `rad`, which is the radian. The
  collision is real, it is in the standards, and the generator is right to refuse
  it. The rem is present; the CGS dose unit waits for a symbol namespace.
- **Thermal diffusivity and diffusion coefficients.** They are m²/s, like
  kinematic viscosity, and the quantity tag separates them in code but not in the
  text form of D12, where `5 m²/s` has to read back to exactly one unit. The
  catalogue carries kinematic viscosity and says so.

Dimension collisions cluster precisely in process engineering, which is why D6 is
not a footnote but the rule that makes this catalogue consistent in the first
place.

**Adding a unit** means editing the YAML and running `go generate ./...`; every
entry needs a `source:`, and no `*_gen.go` file is ever edited by hand.

---

## 7. State and the road to v1.0.0

Everything the decisions describe is implemented and enforced:

| Subsystem | State |
|---|---|
| `dimension`, `symbol` | complete; the property test asserts `Product(q, q.Reciprocal()) == One` for random dimensions |
| Core: `Measurement`, `Unit`, arithmetic, `Engine` | complete; the aliasing guard of D3 is green under `-race`, round-trip conversion reproduces exactly across the catalogue, and each of the five kind rules of D6 has a test |
| Catalogue and generator | complete for the scope of section 6; `go generate ./...` is reproducible and CI fails on a dirty tree |
| Text form: writing, `parse`, JSON, SQL | complete; the round-trip property holds across the whole catalogue and the parser is fuzzed against it |
| `unitvet` | complete; the corpus asserts the reported and the silent cases alike, and the pass runs clean over this repository, tests and examples included |
| Coverage gate | 100 % of hand-written statements, enforced in CI, `COVERAGE_EXCEPTIONS.md` empty |

**What remains before `v1.0.0` is a deliberate API review.** Until it happens the
module is tagged `v0.x` and the API may change without notice; any stability
promise made before the review is one to regret later. The review has to settle,
at minimum:

- the exported surface of `metrology` — which of `Times`, `Per`, `Pow`, `Prefixed`,
  `DecimalIn` and the `Engine` methods are load-bearing enough to freeze
- the naming of the error types and their exported fields, since D11 makes those
  the substitute for a compile error
- whether `parse.Text` is the right shape for the decoding boundary, or whether a
  parser-typed destination generated per catalogue would serve better
- what `Quantity` promises. It is a `string` (D6), so the tag is open by
  construction: a caller's catalogue may spell `"frequency"` and mean whatever it
  likes by it, and nothing links `metrology.Quantity("frequency")` to the
  catalogue entry of the same name — the tags are YAML data, not exported
  constants, and ten of the eighty-two units carry one. Three questions have to
  get one consistent answer before the type is frozen:
  **Is a quantity part of a unit's identity or an interpretation of it?**
  `Unit.Equal` compares the tag, arithmetic drops it (D6), and `parse` restores
  it by looking the expression's scale up in the catalogue. Those are three
  different answers today, defensible one at a time; the review has to say which
  one the type means.
  **Whose namespace is it?** Either the tags this module ships are reserved
  names with a documented meaning — in which case they belong in generated
  constants rather than in string literals, and a caller redefining one is doing
  something the library can name — or they are local to a catalogue, in which
  case two catalogues in one program may tag the same dimension differently and
  the compatibility rule of D6 is comparing spellings across namespaces that
  never agreed to share one.
  **What does untagged mean at a boundary?** Inside the core it is the wildcard
  that keeps a computed magnitude nameable, and that is settled. Crossing D12 it
  is what an expression carries when no catalogue entry spells it: `50 Hz` reads
  back tagged `frequency`, `50 s⁻¹` reads back untagged, and the two are the same
  scale. The text form cannot carry a tag of its own (section 8), so the spelling
  decides — which is a v1 question rather than a later one.
- `O1` in section 10, which decides whether `imperial` is a subpackage or a module

`cmd/unitvet` is versioned with the library but breaks nothing on its own: it is
additive, opt-in, and can ship a new version independently.

---

## 8. Deferred

| Topic | Rationale |
|---|---|
| Fractional exponents | Occur in correlations (e.g. `m·s⁻⁰·⁵`) but require rationals instead of `int8` in the dimension word. The eight reserved bits from D5 keep that door open. |
| Units defined through π | The degree of arc, the gon, the oersted. Their factors are rational multiples of π and have no finite decimal form, so D4 cannot store them exactly. Needs a symbolic factor — a fraction plus a π exponent — which changes the shape of every conversion. Left out of the catalogue rather than rounded into it. |
| Quantities sharing a dimension *and* a symbol | Thermal diffusivity and a diffusion coefficient print as `m²/s`, like kinematic viscosity. The quantity tag of D6 separates them in code, but the text form of D12 has to read back to one unit. Needs a text form that carries the quantity. |
| Measurement uncertainty | A large topic of its own with its own error propagation. Sensible as a layer on top, not in the core. |
| Non-linear scales | dB, pH, degrees Baumé. They do not fit the factor/offset model of D4 and need their own abstraction. |
| Localised output | Decimal comma, unit names per language. Maintainable only once the catalogue is settled. |
| Vector and tensor quantities | A different subject. A library for scalar quantities stays one. |

---

## 9. Risks

| Risk | Mitigation |
|---|---|
| The aliasing invariant breaks unnoticed | It is the one rule whose violation causes silent data corruption. Hence the dedicated guard test of D3, using values above 38 digits — below that threshold apd/v3 masks the bug. |
| Decimal arithmetic is too slow | Measured, and the measurement is in the tree: `BenchmarkConvert` puts a conversion three orders of magnitude above `float64` (D9). Irrelevant for design calculations and reporting, not irrelevant for a loop over millions of sensor readings — which is why README.md names the boundary rather than leaving a user to find it. If it binds, the escape is a fast path for values that fit losslessly in `int64` — not a return to `float64`. |
| Kind semantics proliferate | Every new kind needs a justification in the catalogue. No dimension collision and no affinity, no kind. |
| `unitvet` produces a false positive | The one failure mode that kills the tool, because users disable it and then get nothing. Every rule must be provable before it reports; `analysistest` asserts the silent cases as explicitly as the reported ones. Prefer missing a real bug over inventing one. |
| `unitvet` drifts from the library | Prevented by construction: its dimension table is generated from the catalogue of D8, in the same `go generate` run. A hand-maintained second table would be the defect waiting to happen. |
| The coverage target degrades into assertion-free tests | The known failure mode of a 100 % rule. Mitigated by keeping the correctness weight in property and golden tests, and by treating a coverage-only test in review as a defect. If the number is ever met by tests that assert nothing, the rule has done harm. |
| Go 1.27 as a minimum deters adopters | Accepted deliberately. The fallback is free functions instead of generic methods — a cost in ergonomics, not in substance. |

---

## 10. Open questions

### O1 — Non-SI units: subpackage or separate module?

**Status:** open, with a recommendation; decide before `v1.0.0`

A sister package mapping customary units — foot, stone, psi, BTU, gallon — is
planned. Two shapes are possible.

**Recommendation: a subpackage, `github.com/timzifer/metrology/imperial`.** Go
links only what is imported, so callers who never touch stones pay nothing for
their existence. And per D8 these are simply more catalogue entries, produced by
the same generator; a separate module means either exporting the generator or
duplicating it.

The argument for a separate module is real but narrower than it looks: the core
promises auditability against the SI Brochure, and customary units have different
provenance and uneven exactness — some are exact by international agreement
(1 in = 25.4 mm since 1959), others are historically muddy. Keeping that
distinction visible is worth doing. A subpackage with its own catalogue file and
its own source column achieves it without a second module path, a second release
cadence and a second CI pipeline.

---

## 11. Appendix: verification log

Every claim in this document about Go 1.27, apd, and runtime cost was measured,
not estimated. Reproduction steps:

### Go 1.27 language capabilities

```
git clone --depth 1 --branch go1.27.0 https://github.com/golang/go.git
cd go/src && GOROOT_BOOTSTRAP=<go1.24+> ./make.bash
```

| Construct | Result |
|---|---|
| `func (b Box[E]) Map[R any](f func(E) R) Box[R]` | compiles |
| `type I interface { M[T any](t T) }` | `interface method must have no type parameters` |
| `map[int]slice` where `type slice[A any] []details[A]` | `cannot use generic type slice[A any] without instantiation` |
| `var _ = Q[2, -1]{}` | `syntax error: unexpected -, expected ]` |
| `func Mul[A int, B int](…) Q[A + B, int]` | `syntax error: unexpected +, expected ]` |

### apd copy aliasing

Copy an `apd.Decimal` by value, mutate one copy in place, observe the other:

| Digits | apd/v2 | apd/v3 |
|---:|---|---|
| 10 | corrupted | intact |
| 30 | corrupted | intact |
| 38 | corrupted | intact |
| 40 | corrupted | **corrupted** |
| 100 | corrupted | **corrupted** |
| 500 | corrupted | **corrupted** |

This is why the guard test of D3 uses 200-digit values, and why "simplifying" it
to shorter ones would make it pass unconditionally.

### Conversion cost by precision

One `bar → torr` conversion — one offset, one multiplication, one division. The
table is reproduced in D9, where it decides the default.

| Precision | Per conversion | Allocations |
|---|---:|---:|
| 20 digits | 783 ns | 1 |
| 34 digits (decimal128) | 1541 ns | 7 |
| 50 digits | 3089 ns | 15 |
| `float64` for reference | 0.34 ns | 0 |

The measurement is code rather than a record of one:

```sh
go test -run '^$' -bench . -benchmem ./...
```

`bench_test.go` covers conversion, arithmetic, the `float64` boundary and the
text form; `parse/bench_test.go` covers reading. They assert nothing — a
benchmark that fails is not a defect, and the correctness weight stays in the
property, golden and guard tests of D14 — but they keep every runtime-cost claim
in this document checkable on the reader's own machine, which is the only sense
in which a quoted nanosecond figure is evidence at all.

### SSA and generic methods (D13)

`buildssa` runs with `ssa.BuilderMode(0)`, so generic methods are not instantiated
uniformly and a call to `Of[float64]` reports its name as `Of[float64]`.
Normalise through `(*ssa.Function).Origin()` before comparing method names, or
every generic constructor silently fails to resolve and the pass reports nothing
at all. This costs an hour to rediscover, which is why it is written down.

### Sources

- [Generic Methods in Go 1.27](https://go.dev/blog/generic-methods)
- [golang/go#77273 — spec: generic methods for Go](https://github.com/golang/go/issues/77273)
- SI Brochure, 9th edition (BIPM) — the catalogue's primary source
- NIST SP 811 — the source of the conversion golden tests
