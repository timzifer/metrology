# D8 — The catalogue is data; the Go code is generated

Quantities, units, symbols, factors and source citations live in
`catalog/catalog.yaml`. `tools/catgen`, run through `go generate ./...`, produces
the quantity packages, `catalog/units_gen.go` and `unitvet/table_gen.go`.

**Why.** At the scope of [section 6](../catalogue.md) this is hundreds of lines that would otherwise
be the same four lines per unit, with four chances for a transposed digit. As
data it is a table that can be checked against the SI Brochure and NIST SP 811 —
and one where each entry is independent.

**The generated files are declarations, nothing else.** A quantity package is a
list of `var X = metrology.MustUnit(…)`; the catalogue index is three composite
literals. There is no generated logic, so there is no generated branch anyone has
to write a test for, and the lookups that do have behaviour are hand-written in
`catalog/catalog.go`, where the coverage gate sees them.

**The generator validates before it writes,** and a rejected catalogue writes no
file. It refuses duplicate ids, duplicate Go identifiers, duplicate symbols
within a kind, two units claiming to be canonical for one dimension and quantity,
a dimension with no canonical unit at all, a factor that is not a number or is
zero, an offset on an interval unit, an interval reference that is missing,
absolute or of another dimension, a package declaring two dimensions, and a unit
without a source. **Unknown YAML keys are an error too:** a misspelled `factorr:`
would otherwise silently produce a unit with a factor of one, which is the one
defect a catalogue must not be able to have.

**The output is ordered, not iterated.** Units are emitted sorted by id, imports
deduplicated and sorted, so two runs are byte-identical and CI can check for a
dirty working tree after `go generate ./...`. A map range anywhere in the emitter
would turn that check into a coin toss.

**Side benefit.** The file doubles as documentation and can be exported as a
machine-readable unit catalogue.
