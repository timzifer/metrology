# 11. Appendix: verification log

Every claim in this documentation about Go 1.27, apd, and runtime cost was measured,
not estimated. Reproduction steps:

## Go 1.27 language capabilities

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

## apd copy aliasing

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

## Conversion cost by precision

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
in this documentation checkable on the reader's own machine, which is the only sense
in which a quoted nanosecond figure is evidence at all.

## Fast mode: where the time goes (D17)

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

## The rounding mode a bound needs (D15)

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

## SSA and generic methods (D13)

`buildssa` runs with `ssa.BuilderMode(0)`, so generic methods are not instantiated
uniformly and a call to `Of[float64]` reports its name as `Of[float64]`.
Normalise through `(*ssa.Function).Origin()` before comparing method names, or
every generic constructor silently fails to resolve and the pass reports nothing
at all. This costs an hour to rediscover, which is why it is written down.

## Sources

- [Generic Methods in Go 1.27](https://go.dev/blog/generic-methods)
- [golang/go#77273 — spec: generic methods for Go](https://github.com/golang/go/issues/77273)
- SI Brochure, 9th edition (BIPM) — the catalogue's primary source
- NIST SP 811 — the source of the conversion golden tests
