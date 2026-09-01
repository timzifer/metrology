# metrology

[![CI](https://github.com/timzifer/metrology/actions/workflows/ci.yml/badge.svg)](https://github.com/timzifer/metrology/actions/workflows/ci.yml)

A Go library for physical quantities: exact decimal arithmetic, runtime
dimensional analysis, and one package per quantity so that autocompletion doubles
as a catalogue.

> **Status: under construction.** The architecture is settled and written down in
> [CONCEPT.md](CONCEPT.md); the implementation is at milestone M1 — the
> `dimension` and `symbol` packages exist, the core of the example below does
> not. Nothing here is importable yet, and the API will change without notice
> until `v1.0.0`.

```go
p := pressure.Bar.Of(2.5)

pa, err := p.In[float64](pressure.Pascal)   // 250000
t, err  := temperature.Celsius.Of(20).
           Add(interval.Kelvin.Of(5))       // 25 °C

_, err = temperature.Celsius.Of(20).
         Add(temperature.Celsius.Of(5))     // error: adding two absolute values
```

## What makes it different

**Exact conversions.** Magnitudes are decimals, not floats, and conversion
factors are stored as exact fractions — Fahrenheit as 5/9, Torr as 101325/760.
A conversion rounds once, by a documented rule, instead of inheriting the error
of a pre-rounded constant. `760 torr` is exactly `101325 Pa`.

**Absolute and relative quantities are distinguished.** 20 °C + 5 K is 25 °C.
25 °C − 20 °C is 5 K, an interval, not a temperature. 20 °C + 5 °C is an error,
because it is meaningless. The rules are explicit rather than special-cased for
temperature.

**Dimensional errors say what went wrong.** Go cannot express dimensional
analysis in its type system — there are no integer type parameters, so a
`Q[Length, Time]` is not constructible. Rather than pretending otherwise, this
library checks at runtime and makes the error message carry its weight, naming
both dimensions.

**A `go vet` pass catches what is statically provable.** `unitvet` parses your
code and reports additions across incompatible dimensions without running it. It
reports only what it can prove and stays silent otherwise, so it composes with
existing CI instead of producing noise. Planned for M6.

## Requirements

Go 1.27 or newer. The library uses generic methods, which are only available from
that release.

## Documentation

- [CONCEPT.md](CONCEPT.md) — architecture, the fourteen design decisions and the
  reasoning behind them, milestones, open questions, and a verification log with
  reproduction steps for every measured claim.
- [CLAUDE.md](CLAUDE.md) — invariants and conventions for anyone (human or agent)
  changing this code.

## Development

```sh
go build ./...
go vet ./...
go test -race ./...

go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go run ./tools/covercheck -profile coverage.out
```

The project targets 100 % statement coverage of hand-written code, enforced in
CI. Generated catalogue files are excluded; see D14 for what the target does and
does not claim.

## Licence

MIT. See [LICENSE](LICENSE).
