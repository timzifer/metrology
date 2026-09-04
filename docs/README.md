# Metrology — Architecture and Design

> A Go library for physical quantities: exact decimal arithmetic, runtime
> dimensional analysis, and one package per quantity.

| | |
|---|---|
| **Module** | `github.com/timzifer/metrology` |
| **Go** | 1.27 (minimum) |
| **State** | complete for the scope of [section 6](catalogue.md); the exported surface is frozen at `v1.0.0` — see [section 7](status.md) |

These pages hold the architecture and the reasoning behind it. The decisions
are numbered D1 … D21, each on a page of its own under [`decisions/`](decisions/README.md),
and referenced from code comments; a change that contradicts one updates the
decision first, in the same pull request, with the reason. Silent divergence
between this documentation and the code is the failure mode it exists to
prevent. The section numbers are kept because code comments and the other
pages cite them.

## Contents

1. [Scope](scope.md)
2. [Guiding principles](principles.md)
3. [Decisions](decisions/README.md) — D1 … D21, one page each
4. [The API](api.md)
5. [Package layout](layout.md)
6. [The catalogue](catalogue.md)
7. [State and the v1.0.0 surface](status.md)
8. [Deferred](deferred.md), with [risks](deferred.md#risks) and [open questions](deferred.md#open-questions)
11. [Appendix: verification log](verification.md)

## Decisions at a glance

- [D1 — Measurement and Unit are concrete value types](decisions/D01-measurement-and-unit-are-concrete-value-types.md)
- [D2 — apd/v3, not v2](decisions/D02-apd-v3-not-v2.md)
- [D3 — Immutability as the load-bearing invariant](decisions/D03-immutability-as-the-load-bearing-invariant.md)
- [D4 — Factors as exact fractions](decisions/D04-factors-as-exact-fractions.md)
- [D5 — Dimension: 7 packed exponents, kind held separately](decisions/D05-dimension-7-packed-exponents-kind-held-separately.md)
- [D6 — Kind and quantity, with explicit arithmetic rules](decisions/D06-kind-and-quantity-with-explicit-arithmetic-rules.md)
- [D7 — No global state, no init side effects](decisions/D07-no-global-state-no-init-side-effects.md)
- [D8 — The catalogue is data; the Go code is generated](decisions/D08-the-catalogue-is-data-the-go-code-is-generated.md)
- [D9 — Precision belongs to the computation, not the value](decisions/D09-precision-belongs-to-the-computation-not-the-value.md)
- [D10 — Generic methods at the system boundary](decisions/D10-generic-methods-at-the-system-boundary.md)
- [D11 — Errors are typed and comparable](decisions/D11-errors-are-typed-and-comparable.md)
- [D12 — Text is the canonical exchange format](decisions/D12-text-is-the-canonical-exchange-format.md)
- [D13 — A `go vet` pass that checks dimensions statically](decisions/D13-a-go-vet-pass-that-checks-dimensions-statically.md)
- [D14 — 100 % statement coverage of hand-written code, enforced](decisions/D14-100-statement-coverage-of-hand-written-code-enforced.md)
- [D15 — Uncertainty as a layer: `metrology/uncertainty`](decisions/D15-uncertainty-as-a-layer-metrology-uncertainty.md)
- [D16 — The quantity tag is identity, and `unitvet` follows the tag a product drops](decisions/D16-the-quantity-tag-is-identity-and-unitvet-follows-the-tag-a-product-drops.md)
- [D17 — One arithmetic, and a fast path made of integers](decisions/D17-one-arithmetic-and-a-fast-path-made-of-integers.md)
- [D18 — The generated packages live under `units/`](decisions/D18-the-generated-packages-live-under-units.md)
- [D19 — Customary units are a subpackage: `units/customary`](decisions/D19-customary-units-are-a-subpackage-units-customary.md)
- [D20 — Symbolic factors: one π exponent beside the fraction](decisions/D20-symbolic-factors-one-exponent-beside-the-fraction.md)
- [D21 — GUM propagation: `metrology/gum`, linear terms with provenance](decisions/D21-gum-propagation-metrology-gum-linear-terms-with-provenance.md)
