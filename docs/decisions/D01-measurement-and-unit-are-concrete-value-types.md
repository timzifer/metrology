# D1 — Measurement and Unit are concrete value types

`Measurement` is a struct of a decimal value and a unit, not an interface. `Unit`
likewise. There are no `Quantity`, `BaseUnit` or `DerivedUnit` abstractions
behind them, and `Symbol` is a tagged value type rather than a hierarchy: static,
SI-prefixable, gram, litre, product and quotient differ only in how they render
and which prefixes they accept, which is a switch, not a set of implementations.

**Why.** Two reasons converge. Generic methods are forbidden in interfaces, so
D10's `Of[N]` and `In[N]` cannot exist on one. And every interface value boxes
and allocates, which is measurable for a type that may exist in the millions.

**Cost.** Third-party extension happens through data — a catalogue of your own,
passed as a value — not through implementing types.

**The zero value is the other cost, and it is not free.** A concrete struct has
a zero value no constructor produced. The zero `Unit` holds no factor and no
offset — three nil decimals — so it is *not a scale*, and nothing can be read on
it. That value is not exotic: every failed operation returns it (D13 depends on
that, so the static walk stops at a forbidden operation rather than propagating
a scale it would not have had), which means a caller who ignores one error and
keeps computing arrives at the arithmetic holding it.

So every operation that would have to read a scale checks for its absence and
returns `ErrNoScale`. The check lives at four choke points — `convert`,
`intervalUnits`, `Unit.Pow`, and `sameScale` — chosen so that none of them is
unreachable, because an unreachable branch is what D14 refuses. Two consequences
are worth stating because they look inconsistent and are not:

- `Unit.Equal` answers `true` for two zero units. It reports that two units are
  the same scale, not that either is a usable one, and the interval layer of D15
  relies on it: `uncertainty.Between` compares the bounds' units, and a zero
  `Range` must still be constructible.
- `Unit.Factor` and `Unit.Offset` report the identity — 1/1 and 0, what
  `NewUnit(UnitDef{})` would have built — rather than an error. An accessor has
  no error channel, and a nil decimal would only move the dereference one frame
  out, into the caller. The arithmetic is where a caller finds out.

The rejected alternative was to make the zero `Unit` *behave* as the
dimensionless identity, substituting 1, 1 and 0 wherever the decimals are read.
It needs no new error class and fewer branches. It is wrong because it answers a
question nobody asked correctly: a zero `Measurement` that quietly participates
in arithmetic as `0` on a dimensionless scale is a wrong number delivered with
confidence, which is the failure this library exists to prevent. A crash was a
worse answer than an error; a plausible number is worse than either.
