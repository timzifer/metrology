// Package catalog is the index over every unit the library ships.
//
// The units themselves live in the quantity packages — pressure.Bar,
// temperature.Celsius — where autocompletion doubles as a catalogue. This
// package is for the cases where the unit is not known at compile time: a
// result computed into a dimension that needs a unit to be expressed in, or a
// symbol read out of a configuration file.
//
// The data is generated from catalog/catalog.yaml (D8); the lookups here are
// not. Nothing registers itself at init and nothing can be added at run time
// (D7): what the catalogue contains is decided when it is generated, and a
// defect in it fails the build rather than somebody's program.
package catalog

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
)

// key identifies a canonical unit: a dimension, the kind of quantity read on it
// and — where the dimension is shared — which quantity it is. Temperature has
// two kinds, a point on the scale and a span along it; T⁻¹ has two quantities,
// a frequency and a radioactivity (D6).
type key struct {
	dim      dimension.Dimension
	kind     metrology.Kind
	quantity metrology.Quantity
}

// symbolKey identifies a unit by how it prints. The kind is part of it because
// "K" is both a temperature and a temperature difference.
type symbolKey struct {
	text string
	kind metrology.Kind
}

// Canonical returns the unit a magnitude of this dimension, kind and quantity is
// expressed in — the pascal for a pressure, the metre for a length.
//
// The quantity is usually empty, and empty is a value like any other here: it
// selects the unit for the dimension's ordinary reading. Where a dimension
// carries more than one quantity, the tag chooses between them — T⁻¹ untagged
// has no canonical unit, while T⁻¹ tagged as a frequency is the hertz and as a
// radioactivity the becquerel.
//
// It reports false where the catalogue has no such unit, which is the ordinary
// answer for a dimension nobody has named: a length over a mass is a perfectly
// good quantity with no unit of its own.
func Canonical(dim dimension.Dimension, kind metrology.Kind, quantity metrology.Quantity) (metrology.Unit, bool) {
	u, ok := canonical[key{dim: dim, kind: kind, quantity: quantity}]
	return u, ok
}

// BySymbol returns the unit that prints as text, if the catalogue has one.
//
// The lookup is exact and unprefixed: "Pa" resolves, "kPa" does not. A prefixed
// symbol, an expression such as "J/(kg·K)", and a whole measurement are read by
// github.com/timzifer/metrology/parse, which indexes this catalogue along with
// every spelling each symbol admits.
func BySymbol(text string, kind metrology.Kind) (metrology.Unit, bool) {
	u, ok := bySymbol[symbolKey{text: text, kind: kind}]
	return u, ok
}

// Units returns every unit in the catalogue, ordered by catalogue id.
//
// The slice is a copy: the catalogue is not a variable anyone can rearrange.
func Units() []metrology.Unit {
	out := make([]metrology.Unit, len(all))
	copy(out, all)
	return out
}
