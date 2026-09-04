# 2. Guiding principles

Every decision is measured against these seven.

1. **A measurement is a value, not an object.** Copyable, comparable, no
   identity, no hidden behaviour.
2. **Nothing is mutated after the fact.** Every operation returns a new value; no
   existing value is ever written in place.
3. **Accuracy before speed** — but only in the core. Callers who need `float64`
   get it at the boundary, not in the middle.
4. **Conversion factors are exact.** Rounding happens once, at the end, by a
   documented rule — never in the catalogue.
5. **No state created by an import.** The catalogue is generated code, not
   runtime registration.
6. **Wrong physics is an error value, not a panic.** Panics exist only in
   explicitly named `Must` variants.
7. **Every hand-written line is covered.** 100 % statement coverage is enforced
   in CI — see D14 for what that does and does not mean.
