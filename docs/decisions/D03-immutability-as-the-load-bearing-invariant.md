# D3 — Immutability as the load-bearing invariant

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
