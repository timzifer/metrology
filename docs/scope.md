# 1. Scope

The library models a physical quantity as a decimal magnitude, a unit and a
dimension, and it does so in **concrete value types**: generic methods are
forbidden in interfaces, and an interface value boxes and allocates for a type
that may exist in the millions. That single constraint shapes the rest of this
document.

## Dimensional analysis is a runtime concern

Go's type system cannot carry it. Measured against the compiler ([section 11](verification.md)):

| Question | Answer |
|---|---|
| Generic methods on concrete types | **supported** |
| Generic methods in interfaces | **forbidden** — `interface method must have no type parameters` |
| Heterogeneous storage of instantiated generic types | **impossible** |
| Integer type parameters for exponents (const generics) | **do not exist** |

A `Q[Length, Time]` with exponents checked by the type system cannot be built in
Go without code generation, and generating over the cross-product of all
dimensions is not a viable path. The library compensates with precise errors
instead of compile errors — and makes those errors a first-class part of its API
(D11).

It also compensates outside the type system: D13 ships a `go vet` pass that
proves what *can* be proven statically and stays silent about the rest. That
recovers a useful share of the safety const generics would have given, at the
point in the toolchain where Go conventionally puts such checks.

## Name and module path

`github.com/timzifer/metrology`. The reasoning, recorded because it will be asked
again:

- **not `units`** — semantically perfect, practically the worst option. At least
  six published Go modules already declare `package units` (alecthomas, docker,
  bcicen, ganehag, woweh, go.pitz.tech). No path conflict, but anyone importing
  ours alongside any of those needs an alias forever.
- **not `quantity`** — the closest contender, and the ISO 80000 term for exactly
  what is modelled. Rejected because *quantity* is already load-bearing vocabulary
  **inside** the library: the catalogue maps dimension to quantity, D6 speaks of
  kind-bearing quantities, and the YAML field is called `quantity`. A module of
  the same name makes the word do duty at two levels and forces every doc sentence
  to disambiguate.
- **not `si`** — the catalogue contains bar, torr, °C and kWh, and the planned
  sister package is non-SI by definition. A name that misdescribes its own
  contents from day one.
- **`metrology`** names the field, not a concept in the model. That leaves
  *quantity*, *measurement*, *unit*, *dimension* and *kind* free as internal
  vocabulary — which matters for a library whose entire purpose is precise
  terminology.

The one honest cost: metrology as a field also covers calibration, traceability
and uncertainty, and uncertainty propagation is deferred ([section 8](deferred.md)). The name
promises slightly more than v1 delivers. `metrology/uncertainty` was named here
as the natural home when that changes; D15 made it the decided one for the
interval half of the topic and it is now built, and [section 8](deferred.md) says which half
stays deferred.

No `go-` prefix: that convention marks a Go binding to something else
(`go-redis`, `go-sql-driver`), and the last path element becomes the default
package name, which `go-metrology` could not be.

The same argument settles package names inside the module. `duration` is not
called `time`, because a program that measures durations usually imports the
standard library package of that name as well, and a library that forces an
alias on every consumer has picked the wrong name.
