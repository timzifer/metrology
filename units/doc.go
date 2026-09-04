// Package units is the root of the generated catalogue: one subpackage per
// quantity, each holding the units that measure it.
//
// It declares nothing itself. The units live one level down —
// units/pressure.Bar, units/temperature.Celsius — and are imported from there;
// this package exists so that the forty-four generated packages have a name
// as a group and do not sit beside the seven hand-written ones (D18).
//
// A quantity package is generated from catalog/catalog.yaml and never edited by
// hand (D8). Which unit a magnitude of a given dimension resolves to at run
// time is github.com/timzifer/metrology/catalog; reading a unit out of text is
// github.com/timzifer/metrology/parse.
package units
