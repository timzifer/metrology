// Package metrology models physical quantities as immutable values with exact
// decimal arithmetic and runtime dimensional analysis.
//
// The core type is [Measurement]: a decimal magnitude paired with a [Unit].
// Measurements are ordinary Go values — copyable, comparable and free of hidden
// state. Arithmetic that would violate physics returns an error rather than
// panicking, and conversions round exactly once, by a documented rule.
//
// That promise covers the zero value too: the zero [Unit] is not a scale — no
// constructor produced it and it holds no factor — so an operation handed one
// returns [ErrNoScale] instead of dereferencing what is not there. It is what a
// caller who ignored an error and kept computing arrives with, since every
// failed operation returns the zero value.
//
// Units are not constructed by hand. Each quantity lives in its own package, so
// that autocompletion doubles as a catalogue:
//
//	p := pressure.Bar.Of(2.5)
//	pa, err := p.In[float64](pressure.Pascal)
//
// A measurement writes itself as "2.5 bar" — the canonical text form of D12 —
// through [Measurement.MarshalText], [Measurement.MarshalJSON] and
// [Measurement.Value]. Reading it back needs a catalogue of units, which this
// package deliberately has no place to keep (D7): that is the job of
// github.com/timzifer/metrology/parse, where a parser is a value holding the
// units it knows.
//
// See CONCEPT.md in the repository root for the architecture and the reasoning
// behind it. Design decisions are referenced throughout the code as D1 … D14.
package metrology

// The quantity packages and the catalogue index are generated from the YAML
// catalogue (D8). Never edit a file carrying the "Code generated" line: change
// catalog/catalog.yaml and run this.
//
//go:generate go run ./tools/catgen
