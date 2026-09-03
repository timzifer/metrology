# Working in this repository

Read `CONCEPT.md` before making design choices. It holds the architecture and,
more importantly, the reasoning — decisions are numbered D1 … D19 and referenced
from code comments. If a change contradicts a decision, the decision gets updated
first, in the same pull request, with the reason. Silent divergence between
`CONCEPT.md` and the code is the failure mode to avoid.

## What this is

A Go library for physical quantities: exact decimal arithmetic, runtime
dimensional analysis, one package per quantity. Module path
`github.com/timzifer/metrology`, minimum Go 1.27.

**The library is complete for its stated scope.** The core computes, the
catalogue holds 82 units across 43 quantity packages, plus 8 customary units in
`units/imperial` (D19) — all seven SI base units,
all twenty-two named derived units, the CGS and legacy units, and the non-SI
units of NIST SP 811 that process engineering uses — the text form of D12 reads
and writes (`metrology` writes, `parse` reads), and `unitvet` checks dimensions
statically per D13. What remains before `v1.0.0` is the deliberate API review of
section 7 of `CONCEPT.md`; until then the module is `v0.x` and the API may
change. Section 7 also records what is complete and what each subsystem is
measured by.

One thing is **decided and not built**: `metrology/uncertainty`, the interval
layer of D15 — a magnitude known only to lie between two bounds. Read D15 before
touching it, in particular the rule that an interval bound rounds *outward* and
the additive `Engine.Rounding` that requires. Uncertainty *propagation* —
quadrature, correlations, coverage factors — stays deferred in section 8 and is
not what D15 describes.

The generated quantity packages live under `units/` (D18) —
`units/pressure`, `units/temperature`. The seven hand-written packages stay at
the module root. A new quantity package goes under `units/` because that is
where `catgen` writes it; nothing chooses the directory by hand. One package to come is a
provenance and not a quantity — `units/imperial` (D19), several dimensions in
one package — and it is the only one the one-dimension rule does not apply to.

A customary unit that the US and imperial systems disagree about does **not** go
in yet — that is O3, and a second package would not help, because the symbol
index of D12 is global and two units spelling `gal` are rejected wherever they
live.

Adding a unit means editing `catalog/catalog.yaml` and running
`go generate ./...` — never editing a `*_gen.go` file. Every entry needs a
`source:`; the generator rejects one without. A group with a `quantity:` also
gets a generated `const Quantity` in its package — `frequency.Quantity` — and
the unit definitions are written from that constant, so the tag has one spelling
per package (D16). Compare against the constant, never against a string literal. One `catgen` run writes the
quantity packages, `catalog/units_gen.go` **and** `unitvet/table_gen.go`: the
checker resolves units against the catalogue it was generated from, and that is
the only reason it cannot drift out of step with the run time.

## Invariants — breaking one of these is a defect, not a style question

**Never mutate a `Decimal` reachable from an existing value (D3).** Every
operation allocates its destination fresh. `Measurement.Decimal()` returns a copy
taken via `Set`, never a pointer into the interior. This is not fastidiousness:
`apd.Decimal` shares its coefficient slice on a plain struct copy, so mutating
one copy silently corrupts the other. In apd/v3 an inline optimisation hides this
below 38 digits, which makes it a bug that passes tests and fails in production.
The guard test uses 200-digit values for that reason — do not "simplify" it.

**Use apd/v3, never v2 (D2).** v2 has the same aliasing behaviour at every size.

**No interfaces in the core (D1).** `Measurement` and `Unit` are concrete value
types. Generic methods are forbidden in interfaces, interface values box and
allocate, and the old sealed interfaces were never implementable anyway. Adding
an interface "for testability" here is the wrong trade — inject values, not
behaviour.

**Factors are exact fractions, never pre-rounded decimals (D4).** Store
numerator and denominator. Convert as `(v + offset) · num / den`: exact
multiplication, then one division. `factor: 101325/760` is auditable against the
SI Brochure; `133.32236842105263` is not.

**Reduce every result before returning it (D9).** After a division `apd` pads to
full context precision with zeros, so `2.5 bar` serialises as
`250000.0000000000000000000000000000 Pa` without it. This has already cost one
round of failing tests.

**No global mutable state, no `init()` side effects (D7).** The catalogue is
generated code. There is no runtime `Register`. If you find yourself adding a
package-level map that something writes to at init time, stop and re-read D7.

**Kind rules are explicit, not inferred (D6).** absolute + interval = absolute;
absolute − absolute = interval; absolute + absolute is an error; multiplication
and division drop the kind entirely. Do not add heuristics that guess a kind for
a computed result.

**Writing is a method, reading is a parser (D12).** `Measurement` marshals
itself; nothing in the core resolves a symbol, because resolving one needs a
catalogue and the core has nowhere to keep it (D7). Reading lives in `parse`,
where a `Parser` is a value holding its units and `parse.Text` carries one into
`json.Unmarshal` and `sql.Scan`. Do not add an `UnmarshalText` to `Measurement`
that reaches for a package-level registry — it would lock out every program with
units of its own, which is the case the design is built for.

**One scale has one spelling (D12).** `Symbol.Product` gathers repeated
*prefixable* multiplicands into a power, so `m.Times(m)` is `m²` and equals the
catalogue's square metre. Only prefixable: a static carries its power in its
text, so `Static("torr²")` cannot be recognised as a power of torr again, and
gathering it makes one spelling read back as another — the fuzzer proves this in
seconds. Do not "finish the job" by cancelling quotients either: `mm/m` is a
strain, and `1` is not what it means.

**A symbol's spellings are enumerated, never guessed (D12).** `Symbol.Spellings`
reports every way a symbol may be written, and the parser indexes exactly those.
A static symbol takes no prefix at all — that is what keeps `cd` the candela and
not a centi-day. Do not replace it with a matcher that strips a leading letter
and hopes.

**A quantity tag is identity, and its spelling belongs to the catalogue that
generated it (D16).** `Quantity` stays a `string` so a caller can have tags of
its own, but this module's tags are declared as constants in the packages that
own them. Do not reintroduce string literals for them, and do not move the
constants into `catalog` — that would pull all forty-three quantity packages in
behind a tag comparison.

**Kind and quantity are separate fields (D6).** `Kind` is absolute versus
interval. `Quantity` is which quantity a shared dimension is being read as — the
hertz and the becquerel are both T⁻¹. The empty quantity is compatible with
everything, because multiplication and division drop the tag and every computed
magnitude is therefore untagged. Do not merge the two back into one value.

**Every catalogue factor is exact.** A unit whose factor involves π — the degree
of arc, the oersted — does not go into the catalogue with a rounded decimal. It
waits for symbolic factors. See D4 and section 8 of `CONCEPT.md`.

## Commands

```sh
go build ./...
go vet ./...
go test -race ./...

# coverage gate (D14)
go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go run ./tools/covercheck -profile coverage.out

# the dimension checker over this repository (D13); it must stay silent
go build -o /tmp/unitvet ./cmd/unitvet
go vet -vettool=/tmp/unitvet ./...

# the benchmarks behind every runtime-cost claim in CONCEPT.md and README.md
go test -run '^$' -bench . -benchmem ./...
```

A benchmark asserts nothing and cannot fail; CI only runs it once per case to
keep it compiling. If you change a runtime-cost claim in the documentation,
change the benchmark that produces it in the same pull request — a quoted
nanosecond figure with no benchmark behind it is the same failure mode as a
`CONCEPT.md` that has drifted from the code.

The `unitvet` corpus under `unitvet/testdata` is a module of its own with a
`replace` back to the root — that is what lets it import the real quantity
packages. `go build ./...` at the root does not reach it; build it from its own
directory when changing it.

## Coverage policy (D14)

100 % statement coverage of hand-written code, enforced in CI. Generated files
and `cmd/`, `tools/` are excluded automatically by `covercheck`.

**Coverage is a floor, not the goal.** A test that executes a function and
asserts nothing raises the number and lowers the value — it turns an untested
line into a line everyone believes is tested. In review, a test that exists only
to move the percentage counts as a defect. The correctness weight belongs in:

- property tests (dimension algebra, round-trip conversion)
- the aliasing guard of D3
- the kind-rule table of D6
- golden tests against NIST SP 811 (from M4)

If a branch genuinely cannot be reached, mark it `//coverage:ignore <reason>` and
add it to `COVERAGE_EXCEPTIONS.md`. Prefer deleting the unreachable check: an
error branch that cannot fire usually means the error cannot occur.

## Conventions

- **Errors, not panics.** Panics only in explicitly named `Must` variants.
  Dimension errors must name both dimensions readably — `expected L²M¹T⁻², got
  L¹M¹T⁻²` — because at runtime this message is what replaces a compile error.
- **Reference decisions in comments** where code encodes one: `// D4: exact
  fraction, one rounding`. Someone will otherwise "simplify" it back.
- **Never hand-edit generated files.** They carry
  `// Code generated … DO NOT EDIT.` Change the YAML catalogue and re-run
  `go generate ./...`.
- **Godoc on every exported symbol**, with runnable examples for anything a user
  will actually call.
- Comments explain *why*. The *what* is in the code.

## Things that will look like bugs but are not

- `Dimension` reserves 8 unused bits (D5). They are held for fractional
  exponents, which are deferred, not forgotten.
- The default precision is deliberately modest, 20 significant digits. See D9 in
  `CONCEPT.md` — the benchmark data is there; going higher costs measurably and
  buys nothing for physical measurements. decimal128 is one `NewEngine(34)` away.
- There is no swappable arithmetic and no float-backed fast mode, and there will
  not be one: D17 measures a facade over the operations as
  *slower* than the decimals it replaces, because the representation is where the
  time is, and a loop that leaves its units at the boundary beats every variant
  of it anyway. `BenchmarkKernel` is that comparison; run it before proposing one
  again. This one is decided, not open — `Measurement` and `Unit` stay concrete
  single-arithmetic types, and that is a `v1.0.0` promise. What D17 does *not*
  refuse is the adaptive fast path — an `int64`
  coefficient in place of a decimal where the value fits one, promoting to apd
  where it does not. That one is measured favourably and stays open, on three
  conditions: an integer and never a float, because the shortcut has to compute
  what the slow path computes; a tagged struct field and never an interface
  member, which allocates per value; and no generics, which it does not need.
- `unitvet` reports exactly one thing the run time accepts, and it is on
  purpose (D16): a quantity tag that a `Mul` or a `Div` dropped while leaving
  the dimension intact still conflicts, and the run time has no tag left to
  check. The message says so in its own text. The provenance is not a tag — a
  becquerel scaled by a number still converts into a curie silently — and it
  dies with the dimension. Do not "fix" this by putting the tag back into
  `Mul`/`Div`; that is the guessing D6 forbids.
- A catalogue unit is an exported `var` and an importer can assign to it. That
  is not an oversight (D7), and `unitvet` reports the write: it resolves such a
  variable by name, so a store to one is what makes its table untrue. Do not
  turn the catalogue into accessor functions to close this — D13 records what
  the write actually costs, and it is not the shape of the API.
- `unitvet` stays silent on cases it cannot prove (D13). That is the design: a
  dimension checker with false positives gets switched off and then catches
  nothing at all. Do not make it "smarter" by guessing. In particular it trusts
  a package-level unit variable only where the generated table names it, and it
  does not resolve `MustUnit` calls, phi nodes, parameters or container reads.
- A test in this repository that asserts an operation fails is an operation
  `unitvet` is right to report. Mark it `//unitvet:ignore <reason>` — do not
  weaken the test to hide it from the checker, and do not add a rule exempting
  test files, which would blind the checker to every real defect in one.
