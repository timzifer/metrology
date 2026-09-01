# Working in this repository

Read `CONCEPT.md` before making design choices. It holds the architecture and,
more importantly, the reasoning — decisions are numbered D1 … D14 and referenced
from code comments. If a change contradicts a decision, the decision gets updated
first, in the same pull request, with the reason. Silent divergence between
`CONCEPT.md` and the code is the failure mode to avoid.

## What this is

A Go library for physical quantities: exact decimal arithmetic, runtime
dimensional analysis, one package per quantity. Module path
`github.com/timzifer/metrology`, minimum Go 1.27.

**Current milestone: M4 → M5.** The core is implemented and the catalogue holds
the SI: all seven base units, all twenty-two named derived units, and the non-SI
units of NIST SP 811 that process engineering uses. M5 is the text form — parsing,
serialisation, fuzzing. The status notes under M1 … M4 in `CONCEPT.md` record what
each implementation decided. See section 7 of `CONCEPT.md` for the sequence and
the definition of done for each step.

Adding a unit means editing `catalog/catalog.yaml` and running
`go generate ./...` — never editing a `*_gen.go` file. Every entry needs a
`source:`; the generator rejects one without.

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

**Kind and quantity are separate fields (D6).** `Kind` is absolute versus
interval. `Quantity` is which quantity a shared dimension is being read as — the
hertz and the becquerel are both T⁻¹. The empty quantity is compatible with
everything, because multiplication and division drop the tag and every computed
magnitude is therefore untagged. Do not merge the two back into one value.

**Every catalogue factor is exact.** A unit whose factor involves π — the degree
of arc, the oersted — does not go into the catalogue with a rounded decimal. It
waits for symbolic factors. See the M4 status note in `CONCEPT.md`.

## Commands

```sh
go build ./...
go vet ./...
go test -race ./...

# coverage gate (D14)
go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go run ./tools/covercheck -profile coverage.out

# once the checker exists (M6)
go vet -vettool=$(go env GOPATH)/bin/unitvet ./...
```

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
- The default precision is deliberately modest. See O2 in `CONCEPT.md` — the
  benchmark data is there; going higher costs measurably and buys nothing for
  physical measurements.
- `unitvet` stays silent on cases it cannot prove (D13). That is the design: a
  dimension checker with false positives gets switched off and then catches
  nothing at all. Do not make it "smarter" by guessing.
