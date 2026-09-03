# Metrology — Architecture Concept

> A Go library for physical quantities: exact decimal arithmetic, runtime
> dimensional analysis, and one package per quantity.

| | |
|---|---|
| **Module** | `github.com/timzifer/metrology` |
| **Go** | 1.27 (minimum) |
| **State** | complete for the scope of section 6; `v1.0.0` awaits the API review of section 7 |

This document holds the architecture and the reasoning behind it. The decisions
are numbered D1 … D19 and referenced from code comments; a change that
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
and uncertainty, and uncertainty propagation is deferred (section 8). The name
promises slightly more than v1 delivers. `metrology/uncertainty` was named here
as the natural home when that changes; D15 made it the decided one for the
interval half of the topic and it is now built, and section 8 says which half
stays deferred.

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

The tag is a `string` and the type is open, but the *spellings this catalogue
uses* are not left to a string literal at the call site: every tagged package
declares its own — `frequency.Quantity`, `activity.Quantity` — and the generator
writes the unit definitions in terms of that same constant, so a package has one
spelling of the fact rather than two. A caller with a catalogue of its own
declares its own constants; the type stays open, the *names* are owned by
whoever generated them (D16).

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

The drop is unconditional and stays that way: the tag a becquerel loses to a
product is a tag the run time cannot get back, and putting it back would be the
guessing this rule exists to forbid. What *can* remember it is the static
checker, which walks the operands anyway — D16 has the case and the three rules
that keep it provable.

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

**The rounding mode is `apd`'s default, and there is now a second.** The context
is `apd.BaseContext` with the precision set, which leaves `Rounding` empty, and
an empty `apd.Rounder` means `RoundHalfUp`. That is the policy for a point and
this decision does not change it. `Engine.Rounding` (D15) returns an engine that
rounds another way, because an interval bound must round outward or the
conversion narrows the interval — the zero `Engine` is untouched, and a caller
who never asks never meets it.

**Error handling.** An inexact result is normal and not an error. Overflow,
division by zero and context violations are errors.

This also settles a related question: the library does **not** track significant
figures. Whether a value was measured as `2.50` or `2.5` is lost. That is
deliberate — measurement uncertainty is a topic of its own (section 8), not a
by-product of number representation. D15 does not change this: the rule that
reads `[3.65, 3.75]` out of the literal `3.7` lives in a parser, at the
boundary, and never in a magnitude.

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

**The spelling is the statement**, and that is the answer to the last of the
three questions section 7 asked about `Quantity` (D16). `"50 Hz"` reads back
tagged and `"50 s⁻¹"` untagged, for one scale, and the asymmetry is honest
rather than accidental: someone who writes `Hz` means a frequency, and someone
who writes `s⁻¹` has written a rate and said nothing about which quantity it is.
Untagged is not a failure to determine the tag — it is the correct reading of a
text that made no claim.

The alternatives were worse. Resolving by scale rather than by spelling would
name `kg·m/s²` a newton, and D6 is explicit that a product of a force and a
length is not a torque until someone says so. Carrying the tag in the text needs
a notation that no gauge, no standard and no other program writes, and section 8
is where that waits. An asymmetry that can be stated in one sentence beats
either.

**A reader with an expectation states it, and the API already has one way to.**

```go
m, err := parse.Measurement(text)     // whatever the text said
hz, err := m.To(frequency.Hertz)      // a frequency in hertz, or an error
```

`To` is the checked step D6 requires everywhere else: an untagged magnitude goes
in and a tagged one comes out, a conflicting tag is an error rather than a
silent reinterpretation, and a wrong dimension is an error too. A program that
expects hertz gets hertz or gets told, whichever spelling arrived. There is
deliberately no second name for it — an API about to be frozen (section 7) is
the worst place to grow a synonym for an operation it already has.

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

The same rule reaches one place it had missed: a **quotient in any but the first
place of a product brackets itself**. `N·m/s` reads back as `(N·m)/s`, so a
product of a newton and a metre-per-second has to render `N·(m/s)` or it renders
a unit it is not. The first multiplicand needs none — `m/s·N` already reads as
`(m/s)·N`, which is what it is. This went unnoticed while `m·m²/s` read back as
a product of `m` and `m²` and rendered the same string again: two structures for
one spelling, which is the defect the flattening rule above describes, hiding
the wrong spelling underneath it.

**Repeated prefixable factors gather into a power.** `Times` used to spell
`m·m` where `Pow` spelled `m²`, and the two are one scale. That is not a
cosmetic difference: `Unit.Equal` compares symbols, so `m.Times(m)` was *not*
the square metre; the substitution below looks the rendered symbol up in the
catalogue, so `m·m/s` missed `m²/s` and a magnitude lost the quantity tag of D6
to a notation. `Symbol.Product` now adds the powers of repeated multiplicands —
`m·m` is `m²`, `m²·m` is `m³`, `N·m` is untouched — and a base that cancels to
the zeroth power drops out.

**Only a prefixable symbol gathers**, and the restriction is what makes the rule
sound rather than merely nice. An SI symbol records its power as a number, so a
power can be added to it and taken off again. Every other form carries its power
in its text — `Pow` of a static torr is the static `"torr²"` — and a static that
has been raised cannot be recognised as a power of anything afterwards.
Gathering those renders `1·1·1` as `1²·1`, then `1²·1²`, then `(1²)²`: a
spelling that reads back as a different symbol every time it is written. The
fuzzer found that within thirty seconds of the rule being written without the
restriction, and the input is in the corpus.

**Where the gathering stops.** It normalises a *spelling*, never a *name*. Two
limits follow and both are deliberate. A quotient does not cancel: `mm/m` is a
strain and `1` is not what an engineer asked to see, so `m/m` stays `m/m`. And
`m²·s⁻¹` is the same scale as `m²/s` and still reads back untagged, because the
substitution is a lookup of the rendered symbol and `m²·s⁻¹` is not a spelling
the catalogue holds. Widening it to *any* known unit of the same scale would
turn `kg·m/s²` into `N`, and D6 is explicit that a product of a force and a
length is not a torque until someone says so.

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

### D15 — Uncertainty as a layer: `metrology/uncertainty`

**Status:** built. `Range`, its arithmetic, its text form, its parser and the
`unitvet` receiver all exist, and section 7 records what each is measured by.

Five things this decision said turned out to be wrong when it met the code. They
are corrected in place below and marked **Correction**, rather than quietly
edited away, because a decision that reads as if it had been right all along
teaches nobody what the writing of it missed: `Mid` and `Width` are not total,
the argument given for why `±` cannot be canonical does not hold (a better one
does), the `apd` rounding type is named `apd.Rounder`, D9 rounds half-*up* and
not half-even, and the layer needed two additions to the core and not one.

Section 8 has deferred measurement uncertainty from the beginning, with a
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

**Why not in the core.** The refusal in section 8 was right about the substance.
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
Section 7's freeze list gains it.

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
constructor on `Unit` where there was none. A `unitvet` receiver. And section 7
gains two items: whether `Range` is part of the `v1.0.0` surface or ships behind
it, and whether `Unit.OfDecimal` belongs in the frozen surface beside
`Engine.Rounding`.

---

### D16 — The quantity tag is identity, and `unitvet` follows the tag a product drops

D6 gave `Quantity` three behaviours that were each defensible on their own and
never reconciled: `Unit.Equal` compares the tag, `Add`, `Sub`, `Cmp` and `To`
refuse a conflict, `Mul` and `Div` throw the tag away, and `parse` puts one back
by looking the expression's scale up in a catalogue. Before the type can be
frozen (section 7) it has to mean one thing.

**It means identity.** A hertz and a becquerel are both T⁻¹ and are not the same
measurement; treating one as the other is a wrong number delivered with
confidence, which is the failure this library exists to prevent. So the tag is
part of what a unit *is*, not a label on it, and the run time already enforces
that: `sameQuantity` guards every additive operation and `Engine.To` guards
every conversion. Nothing in the core changes.

**The untagged wildcard is not an exception to it, it is what makes it usable.**
Multiplication and division drop the tag — they must, because a becquerel times
a metre is not anything the catalogue names — so every computed magnitude is
untagged, and a rule that refused to let an untagged magnitude meet a tagged one
would make each computation a dead end. The wildcard is the price of the drop,
and it is worth paying at the run time, where the alternative is a library in
which no quotient can ever be named.

**But it opens a hole, and the hole is static.** The tag can be laundered:

```go
scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
_, _ = scaled.Add(frequency.Hertz.Of(50))          // accepted at run time
```

The product left the dimension untouched and dropped the tag, so what reaches
the sum is an untagged T⁻¹ and the wildcard lets it through. The magnitude is
still a radioactivity. The run time cannot know that — it holds a value, a unit
and a dimension, and the tag it discarded is gone — and giving it a memory would
mean putting the tag back into `Mul` and `Div`, which is exactly the guessing D6
forbids and would make the product of a torque and an angle a torque.

**`unitvet` has the information the run time discarded.** It walks the operands
backwards anyway (D13), so it can carry the dropped tag as *provenance*: the
quantity a multiplicative operation discarded while leaving the dimension
intact. That provenance conflicts with a real tag exactly where the discarded
one would have:

```
app/app.go:12:9: Add on incompatible quantities: a magnitude computed from
                 radioactivity and frequency; Mul and Div drop the tag (D6),
                 so the run time no longer sees the conflict
```

**Three rules keep it provable rather than clever.**

- **Provenance survives only where the dimension does.** A becquerel scaled by a
  plain number is still a radioactivity; a becquerel times a metre is a T⁻¹L¹
  that names no quantity, and dividing the metre back out does not recover one.
  Where the result dimension differs from the operand's, the tag is gone for
  good and the checker forgets it too.
- **Two surviving tags that disagree are no answer.** A plane angle times a
  solid angle is dimensionless and is neither, so the product carries no
  provenance rather than one of the two.
- **It is provenance, never a tag.** The checker never claims the value *is* a
  radioactivity — it is not, the arithmetic untagged it, and `To(activity.Curie)`
  on it is legal and stays silent. The two fields are separate for the same
  reason `Kind` and `Quantity` are.

**This is the one diagnostic that predicts no run-time error**, and it is the
reason this is a decision rather than a rule inside D13. Everywhere else the
pass and the run time agree by construction, and a reader who runs the code can
confirm the diagnostic. Here the code runs and produces a number, and the
message says so in its own text, because a checker that reports something the
reader cannot reproduce is a checker the reader stops believing. The escape is
the one D13 already ships: `//unitvet:ignore` on the line, with a reason, for
the case where the reinterpretation is deliberate.

**The namespace follows from it.** If the tag is identity then a string literal
at a call site is a second spelling of an identity, with nothing to keep the two
in step — so every tagged package declares the constant and `catgen` writes the
unit definitions from it:

```go
const Quantity metrology.Quantity = "frequency"   // frequency/frequency_gen.go

u, ok := catalog.Canonical(dim, kind, frequency.Quantity)
```

It goes in the quantity package rather than in `catalog`, because that is the
package a caller already imports for the units, and reaching a tag through
`catalog` would pull all forty-three quantity packages in behind it. The tags
are therefore *reserved names of this catalogue*, not of the type: `Quantity`
stays a `string` and a caller's own catalogue declares its own constants for its
own tags. Two catalogues in one program that spell one dimension differently are
still two namespaces, and D6's compatibility rule still compares spellings
across them — but each side now has one place where its spelling is written
down, which is what makes a collision something a person can look up.

**What it leaves to D12.** The third question section 7 asked — what untagged
means crossing the text boundary — is answered there, and answered by leaving it
alone: `50 Hz` reads back tagged and `50 s⁻¹` untagged for one scale, because
the spelling is the statement and a text that made no claim should not have one
invented for it. D16 says what the tag is and who owns the spelling; D12 says
what a text carrying none of it means.

### D17 — One arithmetic, and a fast path made of integers

This was O2, and it had to be answered before `v1.0.0` because only half of it
is additive: parameterising `Measurement` later renames every type in the API,
while the other half is invisible in the API and can land at any time.

Two proposals travel under one name and they do not have the same answer. One
hands the arithmetic in from outside; the other keeps one arithmetic and lets a
magnitude be a machine integer when it can be. The first is refused below on a
measurement, the second is not.

#### Reading 1 — the arithmetic passed in from outside

`apd.Decimal` by default and, where a simulation wants speed over exactness, a
float-backed implementation passed in its place — as an interface, or as a type
parameter of `Measurement`.

Both readings were measured, on `bench_test.go` and on prototypes of each shape;
the numbers are in section 11. `BenchmarkKernel` was added for this question and
stays, because it is the comparison a reader will want to repeat before asking
it again.

**A fast mode is a change of representation, not a change of operations.** This
is the finding that settles most of the question. Take the proposal literally —
keep `val apd.Decimal` in the struct and swap the operations behind a facade —
and the float backend has to unpack a decimal, compute, and pack a decimal
again. That costs **327 ns against the 44 ns of the decimal multiplication it
replaces**: the fast arithmetic is seven times slower than the exact arithmetic
it was brought in to avoid. A facade over the operations cannot be fast, in any
of its spellings, because the representation is where the time is.

What *is* fast is a type parameter, because it changes the storage: a magnitude
held as `float64` behind a `Backend[V]` constraint multiplies in **1.6 ns with no
allocation**, and Go 1.27 does permit the generic method of D10 on such a type
(verified, section 11). The interface spelling of the same idea — a magnitude
behind a `Value` interface — costs 18 ns and one allocation per operation, which
is D1's boxing argument in this exact setting.

**What the type parameter costs the rest of the design.** `Measurement[V, B]`
does not stay in `measurement.go`:

- **The catalogue instantiates.** The units of section 6 are package-level
  `var`s of type `metrology.Unit` across forty-four packages (D7, D8). Generic,
  each of them is *one* instantiation, so a second backend needs a second set —
  the generator emits every quantity package twice, or the fast units are
  converted from the exact ones at run time. If it is conversion, the type
  parameter bought nothing a separate type would not have given.
- **`parse` instantiates with it.** `parse.Text` embeds `metrology.Measurement`
  and `parse.Measurement(text)` returns one, so both become generic and both
  have to pick a backend at the package-level entry points of D12.
- **`unitvet` resolves by type name.** The pass looks `Measurement`, `Unit` and
  `Engine` up in the core's scope and compares receiver types; with a generic
  receiver every one of those is an instantiation needing `Origin()`
  normalisation. Section 11 already records an hour lost to exactly that class
  of bug with `Of[float64]`, and that was for a method alone.
- **Every user signature instantiates.** `func f(m metrology.Measurement)`
  becomes a spelling with a backend in it. A generic alias hides the spelling,
  not the fact that two backends are two types — which is D1's
  heterogeneous-storage row a second time.

**And the text form would stop being honest.** A float-backed magnitude marshals
through the same `MarshalText` as an exact one. A thousand additions of 0.1 bar
is `100.0 bar` in the core and `99.9999999999986 bar` in `float64`; both are
well-formed text, both parse, and *nothing in the string says which engine
produced it*. D12 makes text the exchange format, so an approximate value that
carries no mark of its provenance is the failure mode the whole D2–D4 chain
exists to prevent — arriving through the one door the design opened on purpose.

**The workload measurement, which is what actually decides it.** A window of 64
readings multiplied and summed:

| Kernel | Time | Allocations |
|---|---:|---:|
| every intermediate a `Measurement` (`BenchmarkKernel/Exact`) | 58 900 ns | 769 |
| a full fast mode (prototype: float magnitude, float unit) | 5 900 ns | 64 |
| **units left at the boundary** (`BenchmarkKernel/Boundary`) | **849 ns** | **6** |

The third row is principle 3 written out, it is available today through
`In[float64]`, and it is **seven times faster than the fast mode** — because a
fast mode still builds a result unit on every operation, while the boundary
crosses twice for the whole loop. A swapped-in arithmetic is therefore not the
fast option. It is the option that keeps the dimension check *inside* the loop,
and that, not speed, is the only thing it sells.

**On this reading the answer is no: do not parameterise the core.** Should the
dimension check inside the loop turn out to be worth paying for, it belongs in a
concrete type of its own — `metrology/fast`, built from catalogue units at the
boundary, with no `MarshalText` and an explicitly named lossy readout. That
shape costs the core nothing, needs no second catalogue, leaves `unitvet` and
`parse` alone, and — being additive — can be decided *after* `v1.0.0`.
Parameterising cannot: it renames every type in the API.

#### Reading 2 — a magnitude that is sometimes a machine integer

The other proposal keeps *one* arithmetic and changes what a magnitude is held
in: an `int64` coefficient with an exponent where the value fits one, an
`apd.Decimal` otherwise. `Add`, `Sub` and `Mul` check whether both operands are
the cheap form and take the shortcut; anything else promotes both to decimals
and runs the path that exists today. Nothing is passed in and nothing is
chosen — the value decides, at run time, and the caller never sees which path
ran.

That difference is the whole difference. Reading 1 asks a caller to trade
exactness for speed; reading 2 trades nothing, because **the shortcut computes
what the slow path would have computed**. Measured, on the accumulation the
fast path exists for — 64 additions on one scale:

| 64 additions | Time | Allocations |
|---|---:|---:|
| the core today | 23 500 ns | 257 |
| the core with the identity precheck below | 17 800 ns | 257 |
| **an `int64` fast path (prototype)** | **2 440 ns** | **0** |
| units left at the boundary, for reference | 605 ns | 5 |

Nine times, and every allocation gone: an integer magnitude has no coefficient
slice to share, so the defensive copy D3 exists for has nothing to copy and
`Reduce` has nothing to reduce. The boundary is still faster, but it is the row
that gives up both the dimension check and the exact arithmetic; this one gives
up neither. That is what reading 1 could not offer.

**Three conditions, and the third is the one that surprises.**

**It has to hold an integer, not a float.** An `int64` coefficient with an
exponent *is* a decimal, so `0.1 + 0.2` is `0.3` down both paths and a thousand
additions of 0.1 is exactly 100. A `float64` shortcut answers
`0.30000000000000004` and `99.999999999998593` — the same expression with two
answers, decided by how the operands happened to be constructed, invisible to
the caller and to the text form of D12. A fast path that is not the same
arithmetic is reading 1 wearing a different hat.

**It has to be a tagged struct, not an interface member.** With the magnitude
behind a `NumericHolder` interface the shortcut measures 60 ns and **allocates
16 bytes per value**, because a non-pointer value stored in an interface
escapes; as a tagged field of the struct it measures 45 ns and allocates
nothing. This is D1's argument arriving a third time, and it is worth being
exact about: the *type switch* is not the cost — the boxing is.

**It needs no generics at all.** `Measurement` stays one concrete type, the
catalogue stays one set of `var`s, the generator emits exactly what it emits
today, and `parse` and `unitvet` are untouched. The tag is a field, and a field
is not a type parameter. Every cost that made reading 1 unaffordable is absent
here — which is the strongest single argument for this reading and the reason
it is worth separating the two proposals rather than answering them together.

**Where the fast path stops, which bounds what it can be sold as.** It covers
addition, subtraction, comparison and multiplication — a product of two
coefficients is exact until it overflows. It does **not** cover division, which
is not exact in general, and it does not survive a conversion: a magnitude that
has been through the single division of D4 carries twenty significant digits
and no longer fits an `int64`. So a loop that converts on every iteration falls
off the fast path on the first one and never returns to it. And a product chain
keeps a cost the fast path cannot touch: `Mul` rebuilds the result unit every
time, so the kernel of reading 1 measures 20 700 ns on the fast path against
849 ns at the boundary. The magnitude was never the expensive part of a
product — see below.

**What it costs.** `Measurement` grows from 152 to 176 bytes. Every arithmetic
operation gains a promotion branch, and D14 wants each of them tested: both
forms, mixed forms, overflow, mismatched exponents. The aliasing guard of D3
grows a second form to guard. None of that is structural, which is exactly what
separates it from reading 1.

**The decision.** Reading 1: **no** — the core is not parameterised, and no
arithmetic is passed into it in any spelling. Reading 2: **yes**, as a tagged
field behind the existing API — no interface, no type parameter, integers only.
Its prerequisite is the identity check below, which is done and was most of its
cost. Being invisible in the API it is additive and does not have to precede
`v1.0.0`; being invisible in the API is also why it must not change a single
answer, and the test that says so is that the fast and slow paths agree on every
operand pair.

#### The prerequisite both readings kept running into — done

`Unit.Equal` cross-multiplied two factor fractions to answer whether two units
are the same scale, and that answer is on the path of every same-unit addition,
every comparison and every conversion into a unit a value already holds. Two
references to one catalogue `var` share their decimals, so the pointers answer
the question before the arithmetic does — and D3 is what makes reading them
sound rather than lucky: nothing ever writes to a unit's decimals, so sharing a
pointer means holding the same number for as long as both units exist. Without
D3 it would be a cache with no invalidation.

`sameScale` in `unit.go` now asks the pointers first. Measured back to back on
one machine, medians of seven runs:

| | before | after |
|---|---:|---:|
| `Cmp` | 154 ns | 75 ns |
| conversion into the same unit | 240 ns | 110 ns |
| `Add` | 371 ns | 284 ns |
| `Sub` | 377 ns | 302 ns |
| the `int64` fast path of reading 2 (prototype) | 105 ns | 45 ns |

The last row is why this belonged in the decision rather than in a later patch:
without it **the unit check is most of the fast path**, and a fast path built on
top of it would have been measuring `Unit.Equal` instead of the arithmetic it
set out to avoid.

**What it does not reach, which is the same boundary reading 2 ran into.** The
shortcut fires when both operands name the same unit *object*. A unit a `Mul`
just built is a fresh object with fresh decimals, so an accumulation of computed
products — `BenchmarkKernel/Exact` — compares two equal-but-distinct units on
every iteration and pays the cross multiplication anyway. The two findings are
one finding seen twice: the library recomputes units it already has, and both a
magnitude fast path and an identity check stop at that.

**Where the rest of the exact core's headroom is.** Of the 480 ns of a `Mul`,
**229 ns and half the allocations are the unit half** — the exact multiplication
of two factor fractions (D4), which `BenchmarkCompose/Times` measures on its
own — against some 50 ns for the magnitude. That fraction is invariant across a
loop and is rebuilt on every iteration anyway. It is the one place where the
library recomputes something it already knows, and no fast path for magnitudes
reaches it.


**What the decision freezes, and what it leaves free.** `Measurement` and
`Unit` stay concrete, single-arithmetic types, and that is a `v1.0.0` promise:
there is no `Backend`, no `Value` interface and no type parameter, now or
later. Everything else here is additive and needs no second decision — the
`int64` fast path may land in any `v0.x` or long after `v1.0.0` without
changing a signature, and so may a concrete `metrology/fast` if the in-loop
dimension check ever turns out to be worth paying for. Reopening this means
running `BenchmarkKernel` first: it is in the tree so that the comparison can
be repeated rather than re-argued.

### D18 — The generated packages live under `units/`

Forty-three of the fifty top-level directories were generated quantity packages.
The seven that were not — `dimension`, `symbol`, `parse`, `catalog`, `internal`,
`unitvet`, `tools` — are the library, and at the module root they were the ones
a reader had to search for. The generated packages therefore move one level
down:

```
github.com/timzifer/metrology/units/pressure
github.com/timzifer/metrology/units/temperature
```

**It is a directory move and nothing else.** A package's name is the last
element of its path, so `pressure.Bar` is still `pressure.Bar` and every call
site in every program is untouched; only import lines change. `catgen` writes
the directory (`unitsDir` in `tools/catgen/catalog.go`), so nothing chooses it
by hand, and the same run rewrites the import paths in `catalog/units_gen.go`
and the keys of `unitvet/table_gen.go` — the checker resolves units by import
path, and that is exactly why the table is generated with them rather than
beside them (D13).

**Why now.** An import path is API. After `v1.0.0` this costs a major version or
a set of forwarding packages; before it, it costs one `go generate` and a
`gofmt`. The decision is cheap in exactly one window and it is this one.

**What it buys, stated honestly.** Not collision avoidance: a program that
already has a `duration` package still has to alias one of them, because the
package *name* did not change. What it buys is that the module root now reads as
what it is — a library with a catalogue attached — and that the catalogue has a
name as a group. `units/doc.go` is the only hand-written file under it and
declares nothing; it exists so the group has a doc page saying what it is and
where the lookups live.

**`interval` moves with them.** It is a generated quantity package like the
others — the spans that absolute scales subtract into — and its place beside
`temperature` under `units/` is the one that reads correctly.

### D19 — Customary units are a subpackage: `units/customary`

A sister package mapping customary units — foot, pound, psi, gallon — goes in
this module, under `units/`, generated by the same `catgen` run from a catalogue
file of its own.

**Why not a separate module.** Per D8 these are more catalogue entries and
nothing else, so a second module means either exporting the generator or
duplicating it — and a second release cadence, a second CI pipeline and a second
version to keep in step with the core it depends on. Go links only what is
imported, so a caller who never touches stones pays nothing for a subpackage
either way. The argument for a module was auditability, and a subpackage with its
own catalogue file and its own `source:` column carries that without the rest.

**Why a package boundary at all**, when the catalogue already holds the bar, the
torr, the ångström and the curie among the SI units. There is a line and those
are on the near side of it: they are the non-SI units *accepted for use with the
SI*, NIST SP 811 Appendix B.8, exactly defined and internationally sanctioned.
The foot is not in that class. What this module promises is auditability against
the SI Brochure and SP 811 (section 6), and the honest way to keep that promise
while shipping the foot is to put the foot somewhere else and say so in the
import path.

**The one exception it forces.** `units/customary` holds more than one dimension,
which every other generated package is forbidden to do (section 5, enforced in
`catgen`). The rule is not being relaxed, it is being read: a package is one
*group*, and for every package generated so far the group is a quantity. Here it
is a provenance. The generator learns to emit a dimension per unit rather than
one per package, and the check becomes one that fires everywhere the group is a
quantity.

The alternative was `units/customary/length` beside `units/length`, and it is
worse for the reason D18 was careful about: a package's name is the last element
of its path, so those two are both `length` and every program that converts feet
into metres — which is every program that wants this package at all — would have
to alias one of them.

**What it does not change.** `catalog` indexes both, so `Canonical` and
`BySymbol` answer across the whole catalogue; `parse` reads both, because a
parser is a value holding the units it was given (D12); `unitvet` keys its table
by import path and needs nothing at all.

**How the generator knows.** A catalogue group carries `group: provenance`, and
the default is `quantity`. It is spelled out rather than inferred from whether a
dimension was given, because a package holding two dimensions by accident is a
defect `catgen` has always caught and should keep catching: with the marker, the
one-dimension check still fires everywhere the group is a quantity, and the
per-unit dimension is required exactly where the package has none. The two
catalogue files are read by one run and validated as one document — a symbol
that resolves to two units is a defect whichever file each half is written in.

**What shipped, and what did not.** The first batch is the eight units that are
the same in both customary systems and exactly defined in NIST SP 811 Appendix
B.6: the inch, foot, yard and mile, the pound and ounce, the pound-force, and
the pound per square inch. Every one is a finite decimal multiple of the inch or
the pound, so D4 holds without symbolic factors; the golden test compares them
against the standard to twenty-eight significant digits, which is where a
pre-divided psi would already have failed.

Left out, and for one reason each rather than for a general shortage of nerve:

- **The British thermal unit**, which is at least five units — International
  Table, thermochemical, and three more — sharing one spelling. D12 cannot hold
  two units under one symbol, so this one needs its spellings decided before it
  needs its factors. It is the one entry still waiting.

**Where the two systems disagree, the package does too** — this is O3, and it is
answered here because the answer is the same shape as the question D19 already
asked. The US liquid gallon is 231 in³ and the imperial gallon is 4.54609 L, a
fifth apart, and the same goes for the fluid ounce and the ton. They live in
`units/customary/us` and `units/customary/imperial`; the units the two systems
agree on stay in the parent. The package names are `us` and `imperial`, so
`us.Gallon` and `imperial.Gallon` are two names and a program using both aliases
neither — which is exactly what a per-quantity split under `units/customary`
could not have given.

**A second package does not make a second symbol table, and that is the harder
half.** `catgen` keys the symbol index by spelling and kind with no package in
the key, because `catalog.BySymbol` and `parse` resolve against the whole
catalogue: a spelling that resolves to two units is a text form that does not
read back. So the question was never only where the Go names live.

**An ambiguous spelling therefore names no unit.** `gal` is a US gallon in one
country and an imperial gallon in another, so it names nothing here: `galUS` and
`galImp` do, and `parse` refuses the bare `gal` rather than returning whichever
entry was written first. This is the choice D6 makes for a quantity tag and D13
for a dimension it cannot prove, applied to notation — say nothing rather than
say something wrong. A test asserts the refusal, because a spelling that is
absent on purpose looks exactly like a spelling that was forgotten.

**Those spellings are this library's, and the entry says so.** NIST SP 811 writes
both gallons in words, so no standard supplies a symbol that tells them apart. A
space would have — `US gal` is what a data sheet says — but a space cannot be
part of a symbol here: `parse` treats it as a separator so that `m² / s` copied
off a data sheet still reads (D12), and a grammar that had to decide which
blanks bind and which separate would be guessing. Suffixing the base with the
system is the shortest spelling left that names what distinguishes the two.

**What the implementation has to get right**, recorded here because these are
the entries an unwary catalogue gets wrong:

- **The exact ones are exact, and D4 requires them to stay that way.** The
  international yard and pound agreement of 1959 makes 1 in exactly 25.4 mm and
  1 lb exactly 0.45359237 kg. Both are finite decimals, so both are exact
  fractions and neither waits for symbolic factors.
- **The gallon is two units.** The US liquid gallon is 231 in³, the imperial
  gallon is 4.54609 L, and they differ by a fifth. Two catalogue entries with two
  symbols, or the library ships an ambiguity in the one place D12 cannot tolerate
  one.
- **The pound is a mass and a force.** `lb` and `lbf` are different dimensions,
  and the quantity tag of D6 does not help here because the dimensions already
  differ. Two entries, two Go names, no cleverness.
- **Anything historically muddy stays out** until a source can be cited. D8
  already refuses an entry without one, which is the check that keeps this
  package from becoming a folklore collection.

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

// --- Bounds instead of a point: the interval layer (D15) ------

r, err := uncertainty.Parse("2.55 ± 0.05 bar") // also [2.5, 2.6] bar, 2.55(5) bar
lo      := r.Lo()                              // 2.5 bar
w, err  := r.Width()                           // 0.1 bar
inTorr, err := r.To(pressure.Torr)             // each bound rounds outward
agree, err  := r.Overlaps(specified)           // so a conversion never
                                               // manufactures a disagreement
text, ok := r.PlusMinus()                      // the display form, and whether
                                               // it says exactly what r says
```

---

## 5. Package layout

| Package | Contents |
|---|---|
| `metrology` | `Measurement`, `Unit`, arithmetic, `Engine`, error types, serialisation |
| `dimension` | the packed word, product, quotient, reciprocal, stringer |
| `symbol` | SI prefixes, product, quotient and the special forms, spellings |
| `parse` | reading the text form, resolving unit expressions, `parse.Text` |
| `uncertainty` | the interval layer of D15 — `Range`, its arithmetic with outward-rounding bounds, its text form, its parser and `uncertainty.Text` |
| `catalog` | the YAML source, the generated index, and the lookups over it |
| `unitvet`, `cmd/unitvet` | the static dimension checker of D13 |
| `internal/superscript` | superscript digits for both stringers, and reading them back |
| `internal/decimaltext` | the shape of a decimal, for the core and the parser |
| `tools/catgen` | the generator of D8 |
| `tools/covercheck` | the coverage gate of D14 |
| `units/length`, `units/pressure`, … | one package per quantity, fully generated (D18) |
| `units/interval` | the interval units the absolute scales subtract into |
| `units/customary` | the customary units both systems agree on, one package by provenance (D19) |
| `units/customary/us`, `units/customary/imperial` | the units the two systems disagree about — the gallon, the fluid ounce, the ton |

**One package per quantity,** because it turns autocompletion into a search
function: `pressure.` lists exactly the pressure units. Nobody maintains those
packages by hand — they are generated (D8), one dimension per package, enforced
by the generator. Where one dimension carries two quantities they are two
packages, `frequency` and `activity`, each with its tag.

**They live under `units/`** (D18). Forty-three of them against seven
hand-written packages, and at the module root the seven were the ones that had
to be searched for. The import path gains a segment, the call sites gain
nothing — `pressure.Bar` is still `pressure.Bar`, because the package name is
the last element and that did not change.

**One package is a provenance rather than a quantity, and it is the only one.**
`units/customary` (D19) holds the customary units, which are several dimensions
in one package, because what they have in common is where they come from and not
what they measure. Every other generated package is one quantity and the
generator says so.

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

The catalogue is two files. `catalog/catalog.yaml` holds **82 units across 43
quantity packages**: all seven SI base units, all twenty-two named derived
units, and the non-SI units of NIST SP 811 that process engineering uses.
`catalog/customary.yaml` holds **14 customary units in three packages** grouped
by provenance rather than by quantity (D19): eight the two systems agree on, and
three each where they do not. Both are read by one `catgen` run and
validated as one document, because a symbol resolving to two units is a defect
whichever file each half is written in.

| Block | Contents |
|---|---|
| SI base | s, m, kg, A, K, mol, cd |
| Named derived SI | rad, sr, Hz, N, Pa, J, W, C, V, F, Ω, S, Wb, T, H, lm, lx, Bq, Gy, Sv, kat, °C |
| Mechanics, heat, material data | area, volume, velocity, acceleration, density, concentration, mass flow, volume flow, viscosity and kinematic viscosity, surface tension, thermal conductivity, specific heat |
| Process-engineering non-SI | bar, torr, mmHg, mmH₂O, atm, l and l/min, m³/h, kWh, ppm and ppb, °F, t, min, h, d |
| CGS and other legacy units | dyne, erg, poise, stokes, gauss, maxwell, curie, rem, calorie, electronvolt, ångström, barn, are, hectare |
| Dimensionless | ratio, plane angle, solid angle — separated by the quantity tag of D6 |
| Customary (`units/customary`, D19) | in, ft, yd, mi, lb, oz, lbf, psi — every one exact by the 1959 agreement |
| Customary, by system | `us` and `imperial`: galUS/galImp, flozUS/flozImp, tonUS/tonImp — the bare `gal` names nothing |

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
| The `int64` fast path (D17) | **decided, not built.** It is additive and invisible in the API, so it does not gate `v1.0.0`; what D17 fixes is that there is no type parameter and no facade to build it behind |
| `uncertainty` (D15) | complete; a conversion is asserted over the whole catalogue never to narrow a range and never to pull two overlapping ranges apart, the four-corner table of `Mul` and the even-power case of `Pow` are tested case by case, the aliasing guard of D3 covers both bounds at 200 digits, `FuzzRange` holds the text form to a fixed point, and the `unitvet` corpus asserts the reported and the silent cases over ranges as it does over measurements |

**What remains before `v1.0.0` is a deliberate API review.** Until it happens the
module is tagged `v0.x` and the API may change without notice; any stability
promise made before the review is one to regret later. The review has to settle,
at minimum — items already settled are struck through rather than deleted, so
that a reader who remembers the question finds the answer where the question
was:

- the exported surface of `metrology` — which of `Times`, `Per`, `Pow`, `Prefixed`,
  `DecimalIn` and the `Engine` methods are load-bearing enough to freeze
- the naming of the error types and their exported fields, since D11 makes those
  the substitute for a compile error
- whether `parse.Text` is the right shape for the decoding boundary, or whether a
  parser-typed destination generated per catalogue would serve better
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
- whether `Engine.Rounding` and `Unit.OfDecimal` belong in the frozen surface.
  Both are what D15 needed from the core — an interval bound has to round
  outward or a conversion can manufacture a disagreement that stands in no
  source, and a type holding bare magnitudes has to be able to label them again.
  Both are additive and invisible to a caller who does not ask: the zero
  `Engine` is unchanged, and `OfDecimal` is the counterpart of a `Decimal` that
  was already exported. But `Rounding` is a second rounding policy in a library
  that had exactly one, and `OfDecimal` widens the door into `Measurement` from
  two constructors to three. The review should see both rather than inherit them
- whether `uncertainty.Range` ships inside `v1.0.0` or behind it. The layer now
  exists and is the first serious consumer of the core, which is what the
  question was waiting for. What it argues about is `Mid` and `Width` returning
  errors, `PlusMinus` returning `(string, bool)`, and whether `uncertainty` gets
  its own `Engine` or borrows the core's
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

---

## 8. Deferred

| Topic | Rationale |
|---|---|
| Fractional exponents | Occur in correlations (e.g. `m·s⁻⁰·⁵`) but require rationals instead of `int8` in the dimension word. The eight reserved bits from D5 keep that door open. |
| Units defined through π | The degree of arc, the gon, the oersted. Their factors are rational multiples of π and have no finite decimal form, so D4 cannot store them exactly. Needs a symbolic factor — a fraction plus a π exponent — which changes the shape of every conversion. Left out of the catalogue rather than rounded into it. |
| Quantities sharing a dimension *and* a symbol | Thermal diffusivity and a diffusion coefficient print as `m²/s`, like kinematic viscosity. The quantity tag of D6 separates them in code, but the text form of D12 has to read back to one unit. Needs a text form that carries the quantity. |
| Measurement uncertainty — propagation | Still deferred, and the reason is unchanged: it is a large topic of its own with its own error propagation, and it belongs on top of the core rather than in it. What has since been decided and built is only the *interval* half — `metrology/uncertainty`, D15: worst-case bounds, with the dependency problem and with every bound rounding outward. Quadrature combination, correlated quantities and coverage factors are none of it and stay here. |
| Non-linear scales | dB, pH, degrees Baumé. They do not fit the factor/offset model of D4 and need their own abstraction. |
| Localised output | Decimal comma, unit names per language. Maintainable only once the catalogue is settled. |
| Vector and tensor quantities | A different subject. A library for scalar quantities stays one. |

---

## 9. Risks

| Risk | Mitigation |
|---|---|
| The aliasing invariant breaks unnoticed | It is the one rule whose violation causes silent data corruption. Hence the dedicated guard test of D3, using values above 38 digits — below that threshold apd/v3 masks the bug. |
| Decimal arithmetic is too slow | Measured, and the measurement is in the tree: `BenchmarkConvert` puts a conversion three orders of magnitude above `float64` (D9). Irrelevant for design calculations and reporting, not irrelevant for a loop over millions of sensor readings — which is why README.md names the boundary rather than leaving a user to find it, and why `BenchmarkKernel` measures that boundary as faster than any arithmetic swapped in behind it (D17). If it binds, the escape is a fast path for values that fit losslessly in `int64`, which D17 measures rather than proposes: nine times on an accumulation, no allocations, and the same answer as the slow path — not a return to `float64`, which D17 measures as slower than the boundary it would replace and as a different arithmetic besides. |
| Kind semantics proliferate | Every new kind needs a justification in the catalogue. No dimension collision and no affinity, no kind. |
| `unitvet` produces a false positive | The one failure mode that kills the tool, because users disable it and then get nothing. Every rule must be provable before it reports; `analysistest` asserts the silent cases as explicitly as the reported ones. Prefer missing a real bug over inventing one. |
| The dropped-tag rule of D16 becomes a false positive | It is the one diagnostic that predicts no run-time error, so it is the one rule whose noise would be indistinguishable from a bug in the checker. Held down by the two limits D16 states — provenance dies with the dimension, and two disagreeing tags leave none — both asserted in the silent half of the corpus. If it ever reports a deliberate reinterpretation more often than a mistake, the rule goes, not the marker that silences it. |
| `unitvet` drifts from the library | Prevented by construction: its dimension table is generated from the catalogue of D8, in the same `go generate` run. A hand-maintained second table would be the defect waiting to happen. |
| The coverage target degrades into assertion-free tests | The known failure mode of a 100 % rule. Mitigated by keeping the correctness weight in property and golden tests, and by treating a coverage-only test in review as a defect. If the number is ever met by tests that assert nothing, the rule has done harm. |
| Go 1.27 as a minimum deters adopters | Accepted deliberately. The fallback is free functions instead of generic methods — a cost in ergonomics, not in substance. |

---

## 10. Open questions

**None.** There were three, and all three are decided:

| | |
|---|---|
| O1 — non-SI units: subpackage or separate module? | **D19** — a subpackage, `units/customary`, generated by the same run from a catalogue of its own |
| O2 — a fast mode: swappable arithmetic or an adaptive fast path? | **D17** — no type parameter and no facade, one arithmetic, and an `int64` fast path that is additive |
| O3 — what covers the units the two customary systems disagree about? | **D19** — `units/customary` with `us` and `imperial` below it, and an ambiguous spelling naming no unit at all |

The entries stay rather than being deleted: a reader who remembers the question
should find the answer where the question was. What remains before `v1.0.0` is
the API review of section 7, which is a review and not an open question — it has
to *look at* a surface that is already decided.

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
| `func (u Unit[V, B]) Of[N Numeric](v N) Measurement[V, B]` — D10's generic method on a *generic* type, which the refused reading of D17 needs | compiles and runs |
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

### Fast mode: where the time goes (D17)

Same machine. **One multiplication of two magnitudes**, by where the value is
held — this is the table that decides D17, because it separates changing the
*operations* from changing the *representation*:

| Magnitude held as | Time | Allocations |
|---|---:|---:|
| `apd.Decimal`, called directly (what the core does) | 44 ns | 1 |
| `apd.Decimal` behind an interface facade, decimal backend | 44 ns | 1 |
| `apd.Decimal` behind an interface facade, **`float64` backend** | 327 ns | 4 |
| behind a `Value` interface, `float64` implementation | 18 ns | 1 |
| a type parameter with an `apd` backend | 41 ns | 1 |
| a type parameter with a `float64` backend | 1.6 ns | 0 |
| plain `float64` | 0.6 ns | 0 |

The third row is the proposal read literally, and it is seven times slower than
the arithmetic it replaces: unpacking a decimal to a float and packing the
result back costs more than multiplying the decimals. The float backend used
apd's own `SetFloat64`, so this is not a slow conversion routine — it is the
conversion itself.

**A whole `Measurement.Mul`**, which shows that the magnitude is not where the
time goes either:

| Shape | Time | Allocations |
|---|---:|---:|
| the exact core today (`BenchmarkArithmetic/Mul`) | 480 ns | 8 |
| — of which the unit half (`BenchmarkCompose/Times`, the fractions of D4) | 229 ns | 4 |
| — of which the magnitude half (prototype: the `apd` multiplication alone) | 52 ns | 1 |
| prototype: `float64` magnitude, exact `Unit` kept | 240 ns | 4 |
| prototype: `float64` magnitude and `float64` unit | 94 ns | 1 |
| prototype: the same, result unit hoisted out of the loop | 9.4 ns | 0 |

**The kernel** — 64 readings multiplied and summed — is tabulated in D17. Its
first and third rows are `BenchmarkKernel/Exact` and `BenchmarkKernel/Boundary`;
the middle row is the prototype, which the repository does not carry, because a
benchmark of a design that was not adopted is a maintenance cost with no reader.

**The adaptive fast path (reading 2).** One addition of two magnitudes on one
scale, by what the magnitude is held in and whether `Unit.Equal` tries identity
first:

| Add | plain | with the identity precheck |
|---|---:|---:|
| the core today | 372 ns, 4 allocs | 282 ns, 4 allocs |
| `int64` behind a `NumericHolder` interface | 124 ns, 1 alloc | 60 ns, 1 alloc |
| `int64` as a tagged field of the struct | 105 ns, 0 allocs | 45 ns, 0 allocs |

The interface row allocates because a non-pointer value stored in an interface
escapes; the type switch itself is free. Both fast rows carry the full kind and
quantity rules of D6 — adding them changed no measurable time, which is why the
unit check and not the rule checking is what the precheck row removes.

Accumulating 64 additions on one scale: 23 500 ns / 257 allocs in the core,
17 800 ns with the precheck, 2 440 ns / **0 allocs** on the fast path, 605 ns at
the boundary. The same 64 readings multiplied and summed stay at 20 700 ns on
the fast path, because `Mul` rebuilds the result unit each time and no magnitude
shortcut reaches that.

`unsafe.Sizeof`: `Measurement` 152 B today, 176 B with a tagged `int64` field
and its exponent, 136 B with an interface member that then allocates per value.

**Accuracy, for the "imprecise" half of the proposal.** A thousand additions of
0.1 bar: `100.0 bar` exactly, `99.9999999999986 bar` in `float64`. Ten million
of them drift by 1.6·10⁻⁴ absolute. The size of the error is not the point — the
point is that both render as valid text in the form of D12 and neither says
which engine produced it. The same test run against an `int64` coefficient
returns the exact answer down both paths, `0.1 + 0.2 = 0.3` included, which is
the line between the two readings drawn as a measurement.

### The rounding mode a bound needs (D15)

Two facts about `apd` that the decision got wrong from memory, and that cost
nothing to check once the code existed:

| Claim in D15 as written | What apd/v3 v3.2.1 says |
|---|---|
| the type is `apd.Rounding` | `apd.Rounding` is the *field* on `apd.Context`; the type is `apd.Rounder`, a string |
| D9 rounds half-even | `apd.BaseContext` leaves `Rounding` empty, and an empty `Rounder` means `RoundHalfUp` (`round.go`, `Rounder.ShouldAddOne`, default case) |

Both matter more than a spelling would, because the whole finding of D15 is
about which way a bound moves. `Context.Quo` and `Context.Mul` both consult
`c.Rounding` — `Quo` at the remainder, `Mul` through `c.round` — so setting it
on the one context `Engine.context` builds reaches every rounding the library
does, and no second implementation is needed.

The property itself is a test rather than an argument, and it is the one to run
first after touching any of this:

```sh
go test -run 'TestConversionNeverNarrows|TestConversionNeverBreaksAnOverlap' ./uncertainty
```

It converts a range into every other unit of its dimension in the catalogue and
back, and asserts the result *contains* what went in. Directed rounding got
backwards fails it in under a second.

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
