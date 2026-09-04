# 5. Package layout

| Package | Contents |
|---|---|
| `metrology` | `Measurement`, `Unit`, arithmetic, `Engine`, error types, serialisation |
| `dimension` | the packed word, product, quotient, reciprocal, stringer |
| `symbol` | SI prefixes, product, quotient and the special forms, spellings |
| `parse` | reading the text form, resolving unit expressions, `parse.Text` |
| `uncertainty` | the interval layer of D15 — `Range`, its arithmetic with outward-rounding bounds, its text form, its parser and `uncertainty.Text` |
| `gum` | the propagation layer of D21 — `Value`, the contributions it carries, the Type A and Type B evaluations, the budget and its degrees of freedom |
| `catalog` | the YAML source, the generated index, and the lookups over it |
| `unitvet`, `cmd/unitvet` | the static dimension checker of D13 |
| `internal/pi` | the digits of π, as an enclosure, for the crossing conversion of D20 |
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
