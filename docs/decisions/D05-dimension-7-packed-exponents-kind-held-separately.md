# D5 — Dimension: 7 packed exponents, kind held separately

`Dimension` is a packed integer holding seven `int8` exponents — comparable,
usable as a map key, allocation-free. `Kind` is its own field, outside that word.

| Bits | Field | Symbol |
|---:|---|---|
| 0–7 | time | T |
| 8–15 | length | L |
| 16–23 | mass | M |
| 24–31 | electric current | I |
| 32–39 | temperature | Θ |
| 40–47 | amount of substance | N |
| 48–55 | luminous intensity | J |
| 56–63 | reserved | — |

**Why they are separate.** Sharing one word produces two bugs at once: a
`WithoutKind` that clears four of eight kind bits through an operator precedence
mistake, and a `Product` that discards the kind entirely, so every multiplication
loses the absolute marker. Both are impossible by construction once kind is not a
bitfield in the same word — and kind gains room for more than eight values, which
D6 requires. The same argument later separated `Quantity` from `Kind`.

**Construction names its axes.** `New(Exponents{Time: -2, Length: 1})` allocates
nothing, names every axis at the call site, and has a matching inverse in
`Dimension.Exponents` — construction and destructuring read the same way, which
is what the generator of D8 emits.

**Exponent arithmetic wraps at the `int8` boundary and does not error.** Reaching
it takes 128 multiplications of the same axis. An error return on `Product` would
push a case that cannot occur into every caller of the D6 arithmetic, and D14
would then demand a test for a branch that is unreachable by construction.

The eight reserved bits are held for fractional exponents, which are deferred,
not forgotten ([section 8](../deferred.md)).
