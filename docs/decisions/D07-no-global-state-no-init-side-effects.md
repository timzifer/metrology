# D7 — No global state, no init side effects

The catalogue is generated Go code: package-level `var`s and a map from dimension
to canonical unit, without a mutex and without runtime registration. There is no
`Register` and no `Lookup`.

**Why.** Runtime registration means every import of a quantity package creates
global state, and two packages claiming the same dimension panic at process
start, in import order, at the user's site, where nothing can be done about it.
For a published library that is the worst available failure mode. Generated code
does not have this failure class — collisions surface at generation time,
in-house, as a failed build.

**Package-level `var`s are not the state this forbids.** `pressure.Bar` reads the
way callers write it and a function would rebuild its decimals on every call.
Nothing writes to these after init, nothing registers itself, and there is no
runtime `Register` to race with. What D7 forbids is state that an *import*
creates. For the same reason the seven base dimensions are `const`, not `var`: a
constant is not state at all.

**For user-defined units.** A catalogue value the caller constructs and passes —
to `parse.New`, for instance. A value, not a global.
