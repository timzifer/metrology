# D11 — Errors are typed and comparable

One package-level set of error types: sentinel values for the class, struct types
for the context, everything usable with `errors.Is` / `errors.As`.

Because dimensional analysis happens at runtime per D1, the error message is what
the user gets instead of a compile error. It must name both dimensions in
readable form — `expected L²M¹T⁻², got L¹M¹T⁻²` — not merely
`dimensions not equal`.

**The dimension stringer uses a fixed axis order,** `L M T I Θ N J`, rather than
sorting by exponent. This message is read by someone comparing two dimension
strings; a fixed order means one differing exponent produces one differing
character, where sorting by exponent would permute `L²M¹T⁻²` against `L¹M¹T⁻²`.

**`ErrNoScale` / `*NoScaleError` is the one class that names no operands,** and
that is the same reasoning applied to a case where naming them says less than
nothing. It reports the zero `Unit` of D1, which renders as the empty string: a
message quoting it would read `expected , got `. It names the operation and the
remedy instead. It is also deliberately *not* a `DimensionError`, although the
zero `Unit` is dimensionless and that is how it reaches the arithmetic at all —
the dimensions match, which is precisely what makes the mistake worth its own
message rather than one about exponents that agree.
