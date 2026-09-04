# 8. Deferred

| Topic | Rationale |
|---|---|
| Fractional exponents | Occur in correlations (e.g. `m·s⁻⁰·⁵`) but require rationals instead of `int8` in the dimension word. The eight reserved bits from D5 keep that door open. |
| ~~Units defined through π~~ | **Built as D20.** The factor carries one π exponent beside the fraction, and the degree, the arcminute, the arcsecond, the gon, the oersted and the parsec are catalogue entries that are still exact and still checkable against their source. What stays deferred is everything a *general* symbolic factor would be: a table of constants, an expression tree, anything that is not one integer exponent of one transcendental. |
| Quantities sharing a dimension *and* a symbol | Thermal diffusivity and a diffusion coefficient print as `m²/s`, like kinematic viscosity. The quantity tag of D6 separates them in code, but the text form of D12 has to read back to one unit. Needs a text form that carries the quantity. |
| ~~Measurement uncertainty — propagation~~ | **Built as D21**, and as a layer on top of the core exactly as this entry always said it would be: `metrology/gum`, first-order propagation over a term list that remembers which independent input each contribution came from, so correlated inputs and `x − x` both come out right. The *interval* half is D15. What remains deferred is Monte Carlo evaluation (JCGM 101), second-order terms, distributions as objects, and the t-table that turns effective degrees of freedom into a coverage factor — see D21 for why each. |
| Non-linear scales | dB, pH, degrees Baumé. They do not fit the factor/offset model of D4 and need their own abstraction. |
| Localised output | Decimal comma, unit names per language. Maintainable only once the catalogue is settled. |
| Vector and tensor quantities | A different subject. A library for scalar quantities stays one. |

## Risks

| Risk | Mitigation |
|---|---|
| The aliasing invariant breaks unnoticed | It is the one rule whose violation causes silent data corruption. Hence the dedicated guard test of D3, using values above 38 digits — below that threshold apd/v3 masks the bug. |
| Decimal arithmetic is too slow | Measured, and the measurement is in the tree: `BenchmarkConvert` puts a conversion three orders of magnitude above `float64` (D9). Irrelevant for design calculations and reporting, not irrelevant for a loop over millions of sensor readings — which is why README.md names the boundary rather than leaving a user to find it, and why `BenchmarkKernel` measures that boundary as faster than any arithmetic swapped in behind it (D17). If it binds, the escape is a fast path for values that fit losslessly in `int64`, which D17 measures rather than proposes: nine times on an accumulation, no allocations, and the same answer as the slow path — not a return to `float64`, which D17 measures as slower than the boundary it would replace and as a different arithmetic besides. |
| A π conversion rounds twice (D20) | The one place the exactness of D4 does not reach. Held down two ways: the exponents cancel in every conversion that stays inside the π units, so only a crossing conversion materialises π at all; and where one does, π enters at the engine's precision plus ten guard digits, with a test recomputing every crossing conversion at sixty digits and requiring the same twenty. The constant itself is checked against a Machin recomputation rather than trusted, and a precision it cannot serve is an error rather than a truncation. |
| Kind semantics proliferate | Every new kind needs a justification in the catalogue. No dimension collision and no affinity, no kind. |
| `unitvet` produces a false positive | The one failure mode that kills the tool, because users disable it and then get nothing. Every rule must be provable before it reports; `analysistest` asserts the silent cases as explicitly as the reported ones. Prefer missing a real bug over inventing one. |
| The dropped-tag rule of D16 becomes a false positive | It is the one diagnostic that predicts no run-time error, so it is the one rule whose noise would be indistinguishable from a bug in the checker. Held down by the two limits D16 states — provenance dies with the dimension, and two disagreeing tags leave none — both asserted in the silent half of the corpus. If it ever reports a deliberate reinterpretation more often than a mistake, the rule goes, not the marker that silences it. |
| `unitvet` drifts from the library | Prevented by construction: its dimension table is generated from the catalogue of D8, in the same `go generate` run. A hand-maintained second table would be the defect waiting to happen. |
| The coverage target degrades into assertion-free tests | The known failure mode of a 100 % rule. Mitigated by keeping the correctness weight in property and golden tests, and by treating a coverage-only test in review as a defect. If the number is ever met by tests that assert nothing, the rule has done harm. |
| Go 1.27 as a minimum deters adopters | Accepted deliberately. The fallback is free functions instead of generic methods — a cost in ergonomics, not in substance. |

## Open questions

**None.** There were three, and all three are decided:

| | |
|---|---|
| O1 — non-SI units: subpackage or separate module? | **D19** — a subpackage, `units/customary`, generated by the same run from a catalogue of its own |
| O2 — a fast mode: swappable arithmetic or an adaptive fast path? | **D17** — no type parameter and no facade, one arithmetic, and an `int64` fast path that is additive |
| O3 — what covers the units the two customary systems disagree about? | **D19** — `units/customary` with `us` and `imperial` below it, and an ambiguous spelling naming no unit at all |

The entries stay rather than being deleted: a reader who remembers the question
should find the answer where the question was. What remains before `v1.0.0` is
the API review of [section 7](status.md), which is a review and not an open question — it has
to *look at* a surface that is already decided.
