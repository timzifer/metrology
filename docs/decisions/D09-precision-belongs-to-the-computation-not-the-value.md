# D9 — Precision belongs to the computation, not the value

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
deliberate — measurement uncertainty is a topic of its own ([section 8](../deferred.md)), not a
by-product of number representation. D15 does not change this: the rule that
reads `[3.65, 3.75]` out of the literal `3.7` lives in a parser, at the
boundary, and never in a magnitude.
