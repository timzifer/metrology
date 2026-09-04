# D18 — The generated packages live under `units/`

Forty-three of the fifty top-level directories were generated quantity packages.
The seven that were not — `dimension`, `symbol`, `parse`, `catalog`, `internal`,
`unitvet`, `tools` — are the library, and at the module root they were the ones
a reader had to search for. The generated packages therefore move one level
down:

```
github.com/timzifer/metrology/units/pressure
github.com/timzifer/metrology/units/temperature
```

**It is a directory move and nothing else.** A package's name is the last
element of its path, so `pressure.Bar` is still `pressure.Bar` and every call
site in every program is untouched; only import lines change. `catgen` writes
the directory (`unitsDir` in `tools/catgen/catalog.go`), so nothing chooses it
by hand, and the same run rewrites the import paths in `catalog/units_gen.go`
and the keys of `unitvet/table_gen.go` — the checker resolves units by import
path, and that is exactly why the table is generated with them rather than
beside them (D13).

**Why now.** An import path is API. After `v1.0.0` this costs a major version or
a set of forwarding packages; before it, it costs one `go generate` and a
`gofmt`. The decision is cheap in exactly one window and it is this one.

**What it buys, stated honestly.** Not collision avoidance: a program that
already has a `duration` package still has to alias one of them, because the
package *name* did not change. What it buys is that the module root now reads as
what it is — a library with a catalogue attached — and that the catalogue has a
name as a group. `units/doc.go` is the only hand-written file under it and
declares nothing; it exists so the group has a doc page saying what it is and
where the lookups live.

**`interval` moves with them.** It is a generated quantity package like the
others — the spans that absolute scales subtract into — and its place beside
`temperature` under `units/` is the one that reads correctly.
