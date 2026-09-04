# D2 — apd/v3, not v2

`github.com/cockroachdb/apd/v3` as the single arithmetic dependency, never v2.

> **Measured, not assumed.**
> In **apd/v2**, `Decimal.Coeff` is a `math/big.Int`. An ordinary struct copy
> shares its slice: if either copy is written in place, the other changes
> silently — *at any digit count*, and passing a struct with an embedded
> `apd.Decimal` around by value is enough to trigger it. In **apd/v3** an inline
> optimisation covers values up to 38 digits; beyond that the same behaviour
> returns — which makes the bug rarer and therefore more treacherous.

**Consequence.** v3 does not solve the problem, it only moves the threshold. What
actually solves it is D3.
