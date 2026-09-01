# Metrology — Architecture Concept

> A Go library for physical quantities with exact decimal arithmetic and runtime
> dimensional analysis, targeting a published module covering SI and process
> engineering.

| | |
|---|---|
| **Status** | M5 implemented — the text form reads and writes, and round-trips across the catalogue |
| **Date** | 2026-09-01 |
| **Module** | `github.com/timzifer/metrology` |
| **Go** | 1.27 (minimum) |

---

## Table of contents

1. [Scope](#1-scope)
2. [Guiding principles](#2-guiding-principles)
3. [Decisions](#3-decisions)
4. [API sketch](#4-api-sketch)
5. [Package layout](#5-package-layout)
6. [Quantity catalogue](#6-quantity-catalogue)
7. [Milestones](#7-milestones)
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

### Verified against the compiler

Go 1.27.0 built from source. Results:

| Question | Answer |
|---|---|
| Generic methods on concrete types | **supported** |
| Generic methods in interfaces | **forbidden** — `interface method must have no type parameters` |
| Heterogeneous storage of instantiated generic types | **still impossible** |
| Integer type parameters for exponents (const generics) | **do not exist** |

The consequence is the premise everything else rests on: **dimensional analysis
stays a runtime concern.** A `Q[Length, Time]` with exponents checked by the type
system cannot be built in Go without code generation, and generating over the
cross-product of all dimensions is not a viable path. The library compensates
with precise errors instead of compile errors — and makes those errors a
first-class part of its API.

It also compensates outside the type system: D13 adds a `go vet` pass that proves
what *can* be proven statically and stays silent about the rest. That recovers a
useful share of the safety const generics would have given, at the point in the
toolchain where Go conventionally puts such checks.

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
- **not `si`** — the catalogue already contains bar, torr, °C and kWh, and the
  planned sister package is non-SI by definition. A name that misdescribes its
  own contents from day one.
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

---

## 2. Guiding principles

Every later decision is measured against these seven.

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
7. **Every hand-written line is covered.** 100 % statement coverage is the
   standing target, enforced in CI — see D14 for what that does and does not
   mean.

---

## 3. Decisions

Each decision states what it costs. Where a decision makes later revision
expensive, that is noted.

### D1 — Measurement and Unit are concrete value types

**Status:** decided

`Measurement` is a struct of a decimal value and a unit, not an interface. `Unit`
likewise. There are no `Quantity`, `BaseUnit` or `DerivedUnit` abstractions
behind them.

**Why.** Two reasons converge. Generic methods are forbidden in interfaces, so
D10's `Of[N]` and `In[N]` cannot exist on one. And every interface value boxes
and allocates, which is measurable for a type that may exist in the millions.

**Cost.** Third-party extension happens through data (registering custom units),
not through implementing types.

### D2 — apd/v3, not v2

**Status:** decided

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

**Status:** decided

Every operation allocates its destination `Decimal` fresh. No function ever
writes into a `Decimal` reachable from an existing `Measurement`.
`Measurement.Decimal()` returns a copy taken via `Set`, never a pointer into the
interior.

**Why this suffices.** The aliasing from D2 only becomes dangerous when something
mutates. If the invariant holds without exception, copies are safe — and the type
stays a genuine Go value that can be passed freely, used as a map key, and
compared with `==` for unit equality. The invariant is therefore not a matter of
style; it is what carries correctness.

**Enforcement.** A test package that runs every public operation on a 200-digit
value and then asserts that copies taken beforehand are unchanged. Runs in CI
under `-race`. This is the test that fails first on a regression.

### D4 — Factors as exact fractions

**Status:** decided

A derived unit carries numerator and denominator separately, plus an offset as an
exact decimal. Conversion to the base unit is `(v + offset) · num / den`,
performed as an exact multiplication followed by *one* division.

**Why.** The domain's most important factors are not finite in decimal:
Fahrenheit is 5/9, Torr is 101325/760. Stored as a pre-rounded decimal, every
conversion rounds twice. Stored as a fraction, it rounds once — and the catalogue
stays exactly what the SI Brochure says rather than an approximation of it.

**Auditability.** A catalogue entry can be compared to its source character by
character. `factor: 101325/760` is checkable; `133.32236842105263` is not.

### D5 — Dimension: 7 packed exponents, kind held separately

**Status:** decided

`Dimension` remains a packed integer holding seven `int8` exponents — comparable,
usable as a map key, allocation-free. `Kind` moves out of that word into its own
field.

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

**Why separate them.** The current shared word produces two bugs at once.
`WithoutKind()` clears only four of the eight kind bits because of an operator
precedence mistake, and `Product()` discards the kind entirely, so every
multiplication loses the absolute marker. Both disappear by construction once
kind is no longer a bitfield in the same word — and kind gains room for more than
eight values, which D6 requires.

### D6 — Kind and quantity, with explicit arithmetic rules

**Status:** decided; revised in M4, when the collisions arrived

Two facts have to travel with a unit, and the original plan gave both to the
kind. They are now two fields, for the same reason D5 took the kind out of the
dimension word: they are independent, and packing independent facts into one
value is how a `WithoutKind` ends up clearing four of eight bits.

**`Kind` — the affine distinction, *absolute vs. interval*.** 20 °C is a point
on a scale, 5 K is a distance along one. Two values, and the rules below.

**`Quantity` — which quantity a shared dimension is being read as.** The hertz
and the becquerel are both T⁻¹; the gray and the sievert are both L²T⁻²; a plane
angle, a solid angle and a bare ratio are all dimensionless; the candela and the
lumen are both J. A string tag rather than an enum, because the catalogue is
data (D8) and the set of quantities is open — M4 added seven tags without
touching a line of the core.

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

**What the tag does not solve.** Two quantities on one dimension that also share
a *symbol* remain indistinguishable in the text form of D12 — `5 m²/s` is
kinematic viscosity and thermal diffusivity and a diffusion coefficient, and no
tag in the world makes that string read back to one of them. The catalogue
therefore carries one of the three, and the others wait for a text form that
carries the quantity as well.

### D7 — No global state, no init side effects

**Status:** decided

The catalogue is generated Go code: a map from dimension to canonical unit, as a
package variable, without a mutex and without runtime registration. `Register`,
`Lookup` and the error classes `SymbolAlreadyRegistered` /
`QuantityAlreadyRegistered` are removed.

**Why.** Today every import of a quantity package creates global state, and
`internal.Require` panics when two packages claim the same dimension. For a
published library this is the worst possible failure mode: it happens at the
user's site, at process start, depending on import order, and there is nothing
they can do about it. Generated code does not have this failure class —
collisions surface at generation time, in-house.

**For user-defined units.** An explicit `Registry` value that the caller
constructs and passes. A value, not a global.

### D8 — The catalogue is data; the Go code is generated

**Status:** decided

Quantities, units, symbols, factors and source citations live in a versioned YAML
file. A `go:generate` tool produces the quantity packages, the catalogue map and
part of the tests.

**Why.** At the target scope in section 6 this is several hundred units. Written
by hand that is the same line four times per unit, with four chances for a
transposed digit. As data it is a table that can be checked against the SI
Brochure and NIST SP 811 — and one that parallelises well with Claude Code,
because each entry is independent.

**Side benefit.** The file doubles as documentation and can be exported as a
machine-readable unit catalogue.

### D9 — Precision belongs to the computation, not the value

**Status:** decided

A `Measurement` carries no `apd.Context`. Operations use a package default
context. Callers needing more construct an explicit `Engine` value.

**Why.** A context carried inside the value forces a rule deciding whose
precision wins in an addition, and such rules are not predictable for users.
Precision is a property of the *computation*, not of the measurement; it belongs
where the computing happens.

**Error handling.** An inexact result is normal and not an error. Overflow,
division by zero and context violations are errors.

> **Addendum from the prototype.**
> Every result must be **reduced** before being returned. After a division `apd`
> pads to the full context precision with zeros: `2.5 bar` otherwise becomes
> `250000.0000000000000000000000000000 Pa`. Numerically correct, unusable as the
> exchange format of D12. Without this step four of eight prototype tests failed;
> with `Reduce` applied to every return value, all pass.

This also settles a related question: the library does **not** track significant
figures. Whether a value was measured as `2.50` or `2.5` is lost. That is
deliberate — measurement uncertainty is the topic in section 8, not a by-product
of number representation.

### D10 — Generic methods at the system boundary

**Status:** decided

The core computes exclusively in `apd.Decimal`. Input and output in arbitrary
numeric types go through generic methods, which Go 1.27 permits on concrete
types.

```go
func (u Unit) Of[N Numeric](v N) Measurement
func (m Measurement) In[N Numeric](u Unit) (N, error)
```

`go.mod` must declare `go 1.27` for this — the language version follows from that
line, not from the installed toolchain. This is also the library's minimum
version and belongs in the README.

### D11 — Errors are typed and comparable

**Status:** decided

One package for errors, replacing today's split across `errors/` and `internal/`.
Sentinel values for the class, struct types for the context, everything usable
with `errors.Is` / `errors.As`.

Because dimensional analysis happens at runtime per D1, the error message is what
the user gets instead of a compile error. It must name both dimensions in
readable form — `expected L²M¹T⁻², got L¹M¹T⁻²` — not merely
`dimensions not equal`.

### D12 — Text is the canonical exchange format

**Status:** decided; refined in M5, where the reading half met D7

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

### D13 — A `go vet` pass that checks dimensions statically

**Status:** decided, prototype demonstrated

The library ships `cmd/unitvet`, a `golang.org/x/tools/go/analysis` pass that
parses third-party Go code, resolves which unit each `Measurement` carries, and
reports additions, subtractions and conversions across incompatible dimensions —
without running the code.

```
go vet -vettool=$(which unitvet) ./...

app/app.go:12:33:            Add on incompatible dimensions: L-1M1T-2 and Th1
app/app.go:19:14:            Sub on incompatible dimensions: L-1M1T-2 and Th1
consumer/consumer.go:10:34:  Add on incompatible dimensions: L-1M1T-2 and L1
```

**How it works.** The pass consumes SSA from `buildssa` and walks each `Add` /
`Sub` call's operands backwards to their origin. When an operand traces to a
catalogue unit — `pressure.Bar.Of(2.5)` resolves to the package-level `Bar`
variable — its dimension is known. The dimension table is generated from the
same YAML catalogue as the library itself (D8), so the checker and the runtime
cannot drift apart. Cross-package analysis uses the framework's **fact**
mechanism: a function that provably always returns one dimension exports that as
a fact, which importing packages consume — this is how the `consumer.go`
diagnostic above is produced, from a call into another package.

**The governing rule: silence on doubt.** The pass reports only *provable*
conflicts. Where an operand's unit cannot be resolved with certainty, it says
nothing. A dimension checker that produces false positives is a dimension checker
that gets switched off, and then it catches nothing at all. False negatives are
acceptable; the runtime check of D1 remains the backstop.

**What is decidable, measured on the prototype:**

| Pattern | Result |
|---|---|
| `pressure.Bar.Of(2.5).Add(temperature.Celsius.Of(20))` | reported |
| assignment to local variables, then `p.Sub(t)` | reported |
| operand from another package's function with an invariant unit | reported, via facts |
| same dimension, different units (`bar` + `Pa`) | correctly silent |
| unit chosen at runtime (`if x { u = Bar }`) | silent — SSA φ-node, not provable |
| operand arriving as a function parameter | silent — unknown origin |

Units held in slices, maps or struct fields, or arriving from deserialisation,
are equally out of reach. This is a lint that catches the statically obvious
subset, not a proof system.

**Why it earns its place.** D1 established that Go cannot express dimensional
analysis in its type system. D13 recovers a useful part of what const generics
would have given — at the point in the toolchain where Go actually puts this kind
of check, and without asking users to change how they write code. It is opt-in,
it composes with existing `go vet` and CI setups, and third parties can run it
against their own code without depending on it.

**Scope beyond Add/Sub.** The same machinery checks `ConvertTo` targets, and — in
the API of section 4, where `Div` takes an explicit result unit — whether that
declared result unit matches the computed dimension. Those are the same walk with
a different comparison.

### D14 — 100 % statement coverage of hand-written code, enforced

**Status:** decided

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
| `cmd/` main functions | thin wrappers over `singlechecker.Main`; the pass itself is covered through `analysistest` |

Anything else claiming an exemption is a design smell, not a testing problem. An
error branch that cannot be reached usually means the error cannot occur and the
check should go, or that the dependency needs to be injectable so it can be made
to fail.

**The trap, named explicitly.** Coverage measures execution, not verification. A
test that calls a function and asserts nothing raises the number and lowers the
value — it converts an untested line into a line everyone believes is tested,
which is worse than where we started. The rule is therefore: **coverage is a
floor, never the goal.** The correctness weight is carried by the property tests
of M1, the round-trip and kind-rule tests of M2, the aliasing guard of D3 and the
NIST golden tests of M4. Coverage only ensures none of them has a blind spot.

**Mechanics.** `go test -covermode=atomic -coverpkg=./... -coverprofile=…` across
all packages, so cross-package calls count; a script strips generated files and
the declared exceptions, then `go tool cover -func` yields the number CI compares
against 100. The per-function output is part of the CI log, because "which
function dropped" is the only useful form of a coverage failure.

## 4. API sketch

Not finished code — the shape against which the decisions are checked for mutual
consistency.

> **The prototype runs.** The core of this section is built as a working package
> against Go 1.27 and apd/v3: dimensional algebra, exact fractions, all five kind
> rules, generic methods, error types, and the aliasing guard from D3 with
> 200-digit values. **Eight tests, green under `-race`.** The run exposed one
> design flaw, now recorded in D9 — which is what it was for.

```go
// --- Core -------------------------------------------------------

type Dimension uint64          // 7 × int8, packed (D5)
type Kind      uint16          // bitflags, held separately (D5/D6)

type Unit struct {             // value type, immutable (D1/D3)
    dim    Dimension
    kind   Kind
    sym    Symbol
    num    *apd.Decimal        // exact fraction (D4)
    den    *apd.Decimal
    offset *apd.Decimal
}

type Measurement struct {      // 40 bytes, copyable
    unit Unit
    val  apd.Decimal           // never written in place (D3)
}

// --- Construction and readout -----------------------------------

m  := pressure.Bar.Of(2.5)                     // implicit N = float64
m2 := pressure.Bar.OfString("2.50000000001")   // no float detour

pa, err := m.In[float64](pressure.Pascal)      // 250000
d,  err := m.In[*apd.Decimal](pressure.Pascal) // exact

// --- Arithmetic -------------------------------------------------

t, _ := temperature.Celsius.Of(20).
        Add(interval.Kelvin.Of(5))             // 25 °C          (D6)

_, err = temperature.Celsius.Of(20).
        Add(temperature.Celsius.Of(5))         // ErrAbsoluteSum (D6)

p, _ := force.Newton.Of(100).
        Div(area.SquareMeter.Of(2))            // -> 50 Pa, kind dropped

// --- Inspecting errors ------------------------------------------

var de *metrology.DimensionError
if errors.As(err, &de) {
    log.Printf("expected %s, got %s", de.Want, de.Got)
}

// --- Text: writing is a method, reading is a parser (D12) -------

text, _ := p.MarshalText()                     // "2.5 bar"
data, _ := json.Marshal(p)                     // "2.5 bar", quoted

m,  err := parse.Measurement("250 kPa")        // the shipped catalogue
u,  err := parse.Unit("J/(kg·K)")              // expressions resolve too
mine   := parse.New(myUnits)                   // a catalogue of your own

var field parse.Text                           // and a destination that
err = json.Unmarshal(data, &field)             // carries its parser along
```

Text output picks the SI prefix automatically, in decimal arithmetic rather than
through `math.Log`: a logarithmic search is prone to rounding errors at exactly
the powers of ten where the prefix changes.

---

## 5. Package layout

| Package | Contents | Status |
|---|---|---|
| `metrology` | Measurement, Unit, arithmetic, error types, serialisation | M2 |
| `dimension` | packing, product, quotient, reciprocal, stringer | done (M1) |
| `symbol` | SI prefixes, product, quotient and special forms | done (M1) |
| `internal/superscript` | superscript digits for both stringers, and reading them back | done (M1, M5) |
| `internal/decimaltext` | the shape of a decimal, for the core and the parser | done (M5) |
| `parse` | reading the text form, resolving unit expressions | done (M5) |
| `catalog` | YAML source plus generator | M3 |
| `unitvet`, `cmd/unitvet` | static dimension checker per D13 | M6 |
| `imperial` | customary units, planned; shape per O1 | after M4 |
| `length`, `pressure`, … | one package per quantity, fully generated | M3 onward |
| `internal/testutil` | property tests, aliasing guard, catalogue checks | M2 |

One package per quantity, because it turns autocompletion into a search
function: `pressure.` lists exactly the pressure units. Nobody maintains those
packages by hand — they are generated (D8).

---

## 6. Quantity catalogue

The existing catalogue covers mechanics and thermodynamics reasonably well — 39
registered base units and 24 derived ones. The gap is contiguous: the
electromagnetic, photometric and radiological part of SI.

| Block | Status | Missing |
|---|---|---|
| SI base quantities | 5 of 7 | electric current (A), luminous intensity (cd) |
| Named derived SI units | 5 of 22 | rad, sr, Hz, C, V, F, Ω, S, Wb, T, H, lm, lx, Bq, Gy, Sv, kat |
| Mechanics, heat, material data | largely complete | mass flow rate, kinematic viscosity, surface tension, thermal diffusivity |
| Process-engineering non-SI units | started | l/min, m³/h, kWh, mbar, mmH₂O, ppm/ppb, Nm³ |
| Dimensionless numbers | `ratio` only | Re, Pr, Nu, Gr as named quantities on dimension 1 |

The electromagnetic block is the largest single item and also the most
mechanical: seven base units whose definitions all sit in the same source. It is
therefore the first stress test for the generator from D8 — if the catalogue
carries this block, it carries the rest.

Dimension collisions cluster precisely in process engineering: `m²/s` is
kinematic viscosity, thermal diffusivity and diffusion coefficient at once;
`J/kg` is specific energy and specific enthalpy. D6 is therefore not a footnote
but the rule that makes this catalogue consistent in the first place.

---

## 7. Milestones

The order is not arbitrary: M1 through M3 establish the invariants that M4 then
scales against. Parallelising only makes sense from M4 onward.

### M0 — Repository, name, scaffolding

New repo at `github.com/timzifer/metrology`, `go 1.27`, CI running build, vet,
test under `-race` and the coverage gate of D14, licence, README with the target
picture, `CLAUDE.md` carrying the invariants for agent sessions.

**Done when:** CI is green on the empty module and the coverage gate demonstrably
fails when a deliberately uncovered function is added.

### M1 — Dimension and symbol

The packed dimension word of D5 with the kind held outside it, and the symbol
system with prefix selection in decimal arithmetic. Table-driven tests for
product, quotient and reciprocal across all seven axes with negative exponents.

**Done when:** a property test confirms `Product(q, q.Reciprocal()) == One` for
random dimensions, and stringer output is correct for all target quantities.

**Status: done.** Both packages are in the tree, `go vet` is clean, the suite is
green under `-race`, and the coverage gate of D14 reports 100 %. What the
implementation decided, beyond what was written above:

- **`New` takes an `Exponents` struct.** `New(Exponents{Time: -2, Length: 1})`
  allocates nothing, names every axis at the call site and gives
  `Dimension.Exponents` a matching inverse — construction and destructuring read
  the same way, which is what the generator of D8 will emit.
- **The seven base dimensions are constants**, `dimension.L`, `dimension.T` and
  so on, not package variables. D7 forbids global mutable state; a `const` is not
  state at all.
- **The stringer uses a fixed axis order**, `L M T I Θ N J`, rather than sorting
  by exponent. D11's error message is read by someone comparing two dimension
  strings; a fixed order means one differing exponent produces one differing
  character, and sorting by exponent would permute `L²M¹T⁻²` against `L¹M¹T⁻²`.
- **Exponent arithmetic wraps at the `int8` boundary and does not error.**
  Reaching it takes 128 multiplications of the same axis. The alternative — an
  error return on `Product` — would push a case that cannot occur into every
  caller of the D6 arithmetic, and D14 would then demand a test for a branch that
  is unreachable by construction.
- **`Symbol` is a tagged value type, not an interface** (D1). Static,
  SI-prefixable, gram, litre, product and quotient differ only in how they render
  and which prefixes they accept — a switch, not a hierarchy.
- **Prefix selection is exact decimal arithmetic** (D9). `floor(log10 |v|)` comes
  from the digit count and the exponent of the `apd.Decimal`, and applying the
  prefix is a shift of that exponent, so no digit is lost and 1000 m is exactly
  1 km — a logarithmic search yields 999.9999999 m for the same input. The result
  is trimmed of the trailing zeros the shift introduces, but not reduced past the
  decimal point: 250 kPa stays `250`, not `2.5E+2`.
- **One prefix step on `m²` is a factor of 10⁶**, on `m³` a factor of 10⁹.
  `SIPow(text, power)` scales the step with the power and handles negative powers
  such as the wavenumber m⁻¹.
- **The kilogram is `symbol.Gram()`.** Magnitudes are in kilograms, prefixes
  attach to the gram, and the unprefixed rendering is `kg` — the symbol always
  names the unit the magnitude is in.
- **`covercheck` merges the repeated records of a multi-package profile.** With
  `-coverpkg=./...` every test binary reports every block of every package, so a
  block covered by one package's tests also appears with count 0 in the profiles
  of the others. Summing them is what `go tool cover` does; without it the gate
  reported covered code as uncovered as soon as a second test package existed.

### M2 — Core: Measurement, Unit, arithmetic

The pivotal step. Value types per D1, apd/v3 per D2, immutability per D3, exact
fractions per D4, precision policy per D9. A hand-maintained mini catalogue of
eight units, just enough to exercise the arithmetic.

**Done when:**
- the aliasing guard from D3 is green
- round-trip conversion across all mini-catalogue pairs reproduces exactly
- each of the five kind rules from D6 has a test
- `-race` is clean
- coverage is 100 % per D14

**Status: done.** All five conditions hold. What the implementation decided,
beyond what was written above:

- **`Kind` marks the affine distinction only.** D6 gives the kind two jobs:
  absolute versus interval, and resolving dimension collisions such as torque
  against energy. The second one is not implemented as a kind, because packing
  two independent facts into one word is precisely what D5 took apart. It
  arrives with the catalogue as a separate quantity tag in M4, where the first
  collisions actually appear; until then a kind is `Interval` or `Absolute` and
  nothing else.
- **An interval unit may not carry an offset.** An offset is what makes a scale
  affine, and an affine scale measures points. Rejecting the combination at
  construction removes the case from every later operation: a unit that reaches
  the arithmetic as an interval is linear, so a product never has to ask.
- **A unit may declare the interval unit its differences are read on** — K for
  °C, °R for °F. Without it `25 °C − 20 °C` would have to be 5 °C, which reads
  like a temperature and is not one. The difference is *converted* onto that
  unit, not merely labelled with it, so a scale declaring a counterpart with a
  different factor still yields the right number.
- **The scale a difference is computed on is derived, never the declared one.**
  Those are two different units — the receiver's own factor without the offset,
  and the unit the result is read on — and the first implementation conflated
  them. A test with a Celsius scale declaring degrees Rankine holds them apart.
- **`Unit.Of` is total.** A NaN or an infinity is carried as the decimal form of
  itself rather than rejected at the boundary, so construction never returns an
  error a caller has to thread through. Both stay visible: they print as NaN and
  Infinity, and asking for one as an integer is a `RangeError`.
- **`Measurement.In` refuses rather than truncates.** A fractional magnitude
  read into an integer, or one outside its range, is an error and not a silently
  altered number — which is the failure mode this library exists to avoid.
- **The exact readout is `DecimalIn`, not `In[*apd.Decimal]`.** The API sketch
  in section 4 wrote the latter; a pointer type cannot join a `~float64`-style
  type set, so the exact path is its own method.
- **`Engine` is a value with a precision, and its zero value is the default.**
  Every operation exists twice — as a method on `Measurement` at
  `DefaultPrecision`, and on `Engine` for callers who need more. There is no
  package-level context to configure, which keeps D7 intact.
- **Multiplication and division round; addition and subtraction do not.** A sum
  of two decimals is exact and stays exact. A chain of exact products doubles its
  digit count at every step, so those round to the engine's precision — which is
  where D9 says the rounding belongs.
- **`String` is the canonical form, `Prefixed` the display form.** The canonical
  text keeps the unit the measurement is held in, because D12 requires the text
  to read back as the same measurement. Prefix selection is a rendering choice
  and lives in its own method.

### M3 — Catalogue format and generator

YAML schema, generator, generated quantity packages, generated catalogue map. The
mini catalogue from M2 becomes the generator's first input. The generator checks
for duplicate symbols and duplicate dimension/kind pairs at generation time and
aborts, rather than panicking at runtime.

**Done when:** `go generate ./...` reproducibly emits identical code, CI verifies
that nothing ungenerated was committed, and the coverage filter demonstrably
excludes the generated files.

**Status: done.** What the implementation decided:

- **The generator lives in `tools/catgen`, the catalogue in `catalog/`.** The
  YAML and the generated index sit together; the generator sits where D14
  already excludes it from the coverage figure, and is tested on its own terms.
- **The generated files are declarations, nothing else.** A quantity package is
  a list of `var X = metrology.MustUnit(…)`, and the catalogue index is three
  composite literals. There is no generated logic, so there is no generated
  branch anyone has to write a test for — and the lookups that do have behaviour
  are hand-written in `catalog/catalog.go`, where the coverage gate sees them.
- **Consequence for D14:** the shipped generated files contribute *zero*
  statements to the coverage profile, so the exclusion currently changes no
  number. That is not a reason to leave it untested: `covercheck` is tested
  against a generated fixture that does contain statements, because the day a
  generated file grows a function is not the day to discover the filter never
  worked.
- **Package-level `var`s, not accessor functions.** `pressure.Bar` reads the way
  the API sketch in section 4 writes it, and a function would rebuild the
  decimals on every call. This is not the global mutable state D7 rules out:
  nothing writes to these after init, nothing registers itself, and there is no
  runtime `Register` to race with. What D7 forbids is state that an import
  *creates*; a table of constants is not that.
- **One package per dimension, and the interval units live in `interval`.**
  `temperature` holds the absolute scales — °C, K, °F — and `interval` the spans
  they subtract into. `temperature.Celsius.Of(20).Add(interval.Kelvin.Of(5))`
  reads as what it is, and the split keeps a package from meaning two things.
- **Every unit carries a source, and the generator refuses one that does not.**
  A conversion factor is a claim about the world; a claim without a citation
  cannot be checked, which is the whole argument of D4.
- **The generator validates before it writes.** Duplicate ids, duplicate Go
  identifiers, duplicate symbols within a kind, two units claiming to be
  canonical for one dimension and kind, a dimension with no canonical unit at
  all, a factor that is not a number or is zero, an offset on an interval unit,
  an interval reference that is missing, absolute, or of another dimension, and
  a package declaring two dimensions. All of them abort, and a rejected
  catalogue writes no file — the failure is a broken build, never a panic in
  somebody else's program (D7).
- **Unknown YAML keys are an error.** A misspelled `factorr:` would silently
  produce a unit with a factor of one, which is the one defect a catalogue must
  not be able to have.
- **The output is ordered, not iterated.** Units are emitted sorted by id and
  imports are deduplicated and sorted, so two runs are byte-identical. CI checks
  exactly that by looking for a dirty working tree after `go generate ./...`; a
  map range anywhere in the emitter would turn that check into a coin toss.
- **The mini catalogue of M2 stays in `catalog_test.go`.** The core is tested
  against units defined by the test, not against the shipped catalogue: a test
  that uses the catalogue to test the engine fails twice for one defect and
  tells you neither time which one it was.

### M4 — Scaling the catalogue

The actual breadth, now as data work. Order: electromagnetics as the stress test,
then photometry and radiology, then the process-engineering gaps, finally the
non-SI units. Every entry with a source citation.

**Done when:**
- all 7 base and 22 named derived SI units are present
- a golden test reproduces the conversion tables from NIST SP 811
- every unit has a source recorded in the catalogue

**Status: done.** 82 units across 43 packages. What the work decided:

- **The dimension collisions arrived, and D6 changed to meet them.** `Quantity`
  is now its own field next to `Kind`, not a second meaning inside it. Seven
  tags were enough for the whole SI: frequency, radioactivity, absorbed dose,
  dose equivalent, luminous intensity, luminous flux, plane angle, solid angle,
  kinematic viscosity.
- **Every factor in the catalogue is exact, and units that cannot be are left
  out.** The degree of arc is π/180 radians and the oersted is 1000/4π A·m⁻¹:
  neither has a finite decimal fraction, so neither is in the catalogue. A
  rounded factor would be precisely what D4 forbids, and shipping one silently
  because the unit is popular is how a catalogue stops being auditable. What is
  needed is a symbolic factor — a rational multiple of π — which is section 8's
  problem, not M4's.
- **Two quantities that share a dimension *and* a symbol cannot both ship.**
  Thermal diffusivity and a diffusion coefficient are m²/s, like kinematic
  viscosity; the tag separates them in the type system but not in the text form
  of D12, where `5 m²/s` has to read back to exactly one unit. The catalogue
  carries kinematic viscosity and says so in a comment.
- **The absorbed-dose rad is not in the catalogue.** Its symbol is `rad`, which
  is the radian. That collision is real, it is in the standards, and the
  generator is right to refuse it — the rem is there, and the CGS dose unit
  waits for a symbol namespace.
- **The golden test compares to eighteen significant digits, not to the last
  one.** Factors such as one 3600th have no finite decimal form, so the
  conversion rounds once by D9 and the return trip cannot undo that rounding.
  Eighteen digits is two below the engine default and far past where a
  pre-divided factor fails: a torr stored as 133.32236842105263 goes wrong in
  the seventeenth.
- **`duration`, not `time`.** A program that measures durations usually imports
  the standard library package of that name too, and a library that forces an
  alias on every consumer has picked the wrong name — the same argument that
  settled the module name in section 1.
- **The package is the quantity.** One dimension per package, enforced by the
  generator; where one dimension carries two quantities they are two packages,
  `frequency` and `activity`, each with its tag. Autocompletion stays a
  catalogue rather than a pile.

### M5 — Edges: parsing, serialisation, documentation

Reading and writing the text form, JSON and SQL, godoc with runnable examples per
package, README. Fuzz test on the parser.

**Done when:**
- the round-trip property holds across the entire catalogue
- fuzzing finds no crash in one hour
- every exported symbol is documented
- coverage is 100 % per D14, `COVERAGE_EXCEPTIONS.md` reviewed and short

**Status: done.** The `parse` package reads what the library writes, across the
whole catalogue, both in the canonical form and in the prefixed display form of
D9. What the work decided:

- **The parser is a value, not a registry.** D12 above records why, and the shape
  it forced: `Parser` holds its units, `parse.Text` carries a parser into
  `json.Unmarshal` and `sql.Scan`, and nothing is registered anywhere at init.
- **Spellings are enumerated, not guessed.** Every symbol reports the ways it may
  be written — `Symbol.Spellings` — and the parser indexes exactly those. That is
  what keeps `cd` the candela rather than a centi-day, and `mmHg` a millimetre of
  mercury rather than a milli-metre-of-mercury: a static symbol admits no prefix
  at all, and a prefix is only ever read in front of a symbol whose form declares
  it. A longest-prefix matcher over the alphabet would have to guess, and would
  guess wrong on exactly these.
- **A collision is resolved towards the unit that spells itself that way.** `km`
  is the kilometre of the catalogue, not the prefixed metre — the same scale
  either way, but the catalogue entry is the one with the source citation.
- **The renderer had to become unambiguous before the parser could be exact.**
  A solidus and a middle dot bind equally and from the left, so `m/(s/A)` and
  `b/(km/h)` need their brackets — without them they are `(m/s)/A` and `(b/km)/h`,
  which are different dimensions. The bracketing rule is now on the rendered text
  rather than on the symbol form, because a static symbol can join two units too.
  For the same reason a product of a product is flattened on construction: it
  already rendered flat, so keeping the nesting left two structures for one
  symbol and made `Symbol.Equal` answer false for two symbols that print alike.
- **Composing units became public API.** `Unit.Times`, `Unit.Per` and `Unit.Pow`
  are what an expression is built out of, and a program naming a computed result
  wants them anyway. They drop kind and quantity exactly as the arithmetic of D6
  does.
- **An expression that spells a unit the parser knows is that unit.** `m²/ s`
  and `m²/s` differ by a blank, and without this only the second would carry the
  quantity tag of D6. The substitution checks the scale and not only the
  spelling: a caller's catalogue may spell something `m/s` that is not a metre
  per second, and naming that would change the factor instead of the tag.
- **A power of a power is bounded before it is computed.** `(Qm^127)^127` is a
  factor of half a million digits, and one bracket more is sixty million — all
  in fourteen characters of input. The parser therefore multiplies the exponents
  of nested brackets and refuses anything beyond `MaxPower`, the same bound a
  single power has. Judging the result afterwards would be too late: the fuzzer
  found this as four workers at full CPU and no executions at all.
- **The fuzzer earned its place five times over.** Besides the hang above it
  found a lexer that walked off the end of its input on invalid UTF-8 (the
  replacement rune is three bytes long and the byte it stands for is one); apd
  accepting `".-1"` and rendering it as `"0.-1"`, a value whose own text form is
  not a value; a zero with a positive exponent printing as `"00"`; and the
  nested-product structure above. The middle two are core defects, not parser
  defects — reading untrusted text is simply what makes them reachable.
  `Unit.OfString` now checks the shape of a number itself, through
  `internal/decimaltext`, which is the one scanner both the core and the parser
  use.
- **The magnitude range stays apd's business.** A first draft rejected an
  exponent beyond ±100000 in the parser; apd rejects it first, so the check was
  dead code and went.

### M6 — `unitvet`, the static dimension checker

Per D13. Depends on M3, because the pass's dimension table is generated from the
same catalogue. Can be started as soon as the core API from M2 is stable, and
should be — every catalogue entry added in M4 then extends its reach for free.

The pass runs against the library's own test corpus first: a testdata package
holding one function per pattern in the D13 table, half of them expected to be
reported and half expected to stay silent. `analysistest` makes both directions
assertable.

**Done when:**
- `go vet -vettool=unitvet ./...` reports every seeded conflict in testdata
- and reports nothing on the not-provable cases
- the dimension table is generated, never hand-maintained
- running the pass over the library's own examples and tests is clean

Tagging starts at M2 as `v0.x`. `v1.0.0` comes only after M5 and after a
deliberate API review — any stability promise made before that is one to regret
later. M6 may ship after v1.0 as a separate tool version; it is additive and
breaks nothing.

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
| The aliasing invariant breaks unnoticed | It is the one rule whose violation causes silent data corruption. Hence the dedicated guard test in M2, using values above 38 digits — below that threshold apd/v3 masks the bug. |
| Decimal arithmetic is too slow | Measured: 783 ns per conversion at 20 digits, versus 0.34 ns for `float64`. Irrelevant for design calculations and reporting, not irrelevant for a loop over millions of sensor readings. Carry benchmarks from M2 onward. If it binds, the escape is a fast path for values that fit losslessly in `int64` — not a return to `float64`. |
| The generator becomes a project of its own | Hard time box on M3. It may be ugly; it only has to be deterministic. |
| Kind semantics proliferate | Every new kind needs a justification in the catalogue. No dimension collision and no affinity, no kind. |
| `unitvet` produces a false positive | The one failure mode that kills the tool, because users disable it and then get nothing. Every rule must be provable before it reports; `analysistest` asserts the silent cases as explicitly as the reported ones. Prefer missing a real bug over inventing one. |
| `unitvet` drifts from the library | Prevented by construction: its dimension table is generated from the catalogue of D8, in the same `go generate` run. A hand-maintained second table would be the defect waiting to happen. |
| The coverage target degrades into assertion-free tests | The known failure mode of a 100 % rule. Mitigated by keeping the correctness weight in property and golden tests, and by treating a coverage-only test in review as a defect. If the number is ever met by tests that assert nothing, the rule has done harm. |
| Go 1.27 as a minimum deters adopters | Accepted deliberately. By v1.0, 1.27 will be one release behind current. The fallback is free functions instead of generic methods — a cost in ergonomics, not in substance. |

---

## 10. Open questions

### O1 — Non-SI units: subpackage or separate module?

**Status:** open, with a recommendation

A sister package mapping customary units — foot, stone, psi, BTU, gallon — is
planned for the medium term. Two shapes are possible.

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

Decide before M4, because it determines whether the generator emits into one
module or two.

### O2 — Default precision — now decidable from data

**Status:** open, with a recommendation

D9 named 34 digits because that matches decimal128 and therefore follows a
citable standard. The prototype shows what the standard costs — one `bar → torr`
conversion, i.e. one offset, one multiplication, one division:

| Precision | Per conversion | Allocations |
|---|---:|---:|
| 20 digits | 783 ns | 1 |
| 34 digits (decimal128) | 1541 ns | 7 |
| 50 digits | 3089 ns | 15 |
| `float64` for reference | 0.34 ns | 0 |

Going from 20 to 34 costs twice the time and seven times the allocations without
benefiting any physical measurement — no sensor in this domain delivers more than
six to eight trustworthy digits. **Recommendation: 20 digits as the default**,
with decimal128 reachable through the `Engine` value. The decision must be made
before v1, because it fixes the rounding behaviour of every result.

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

### Static dimension checking (D13)

A working `go/analysis` pass was built against `golang.org/x/tools` and run over a
seven-function test corpus, both standalone and as `go vet -vettool=`. Results
matched the design intent exactly: three seeded conflicts reported — including one
resolved across a package boundary through the fact mechanism — and four cases
correctly left silent, of which two are conflicts that are not statically
provable (runtime-selected unit, unit arriving as a parameter).

The one implementation detail worth recording, because it costs an hour to
rediscover: `buildssa` runs with `ssa.BuilderMode(0)`, so generic methods are not
instantiated uniformly, and a call to `Of[float64]` reports its name as
`Of[float64]`. Normalise through `(*ssa.Function).Origin()` before comparing
method names, or every generic constructor silently fails to resolve and the pass
reports nothing at all.

### Sources

- [Generic Methods in Go 1.27](https://go.dev/blog/generic-methods)
- [golang/go#77273 — spec: generic methods for Go](https://github.com/golang/go/issues/77273)
- SI Brochure, 9th edition (BIPM) — for catalogue verification in M4
- NIST SP 811 — for the conversion golden tests in M4
