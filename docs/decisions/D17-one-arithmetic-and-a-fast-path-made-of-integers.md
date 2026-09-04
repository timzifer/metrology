# D17 — One arithmetic, and a fast path made of integers

This was O2, and it had to be answered before `v1.0.0` because only half of it
is additive: parameterising `Measurement` later renames every type in the API,
while the other half is invisible in the API and can land at any time.

Two proposals travel under one name and they do not have the same answer. One
hands the arithmetic in from outside; the other keeps one arithmetic and lets a
magnitude be a machine integer when it can be. The first is refused below on a
measurement, the second is not.

## Reading 1 — the arithmetic passed in from outside

`apd.Decimal` by default and, where a simulation wants speed over exactness, a
float-backed implementation passed in its place — as an interface, or as a type
parameter of `Measurement`.

Both readings were measured, on `bench_test.go` and on prototypes of each shape;
the numbers are in [section 11](../verification.md). `BenchmarkKernel` was added for this question and
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
(verified, [section 11](../verification.md)). The interface spelling of the same idea — a magnitude
behind a `Value` interface — costs 18 ns and one allocation per operation, which
is D1's boxing argument in this exact setting.

**What the type parameter costs the rest of the design.** `Measurement[V, B]`
does not stay in `measurement.go`:

- **The catalogue instantiates.** The units of [section 6](../catalogue.md) are package-level
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
  normalisation. [Section 11](../verification.md) already records an hour lost to exactly that class
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

## Reading 2 — a magnitude that is sometimes a machine integer

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

## The prerequisite both readings kept running into — done

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
