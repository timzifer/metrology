package metrology_test

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

// The mini catalogue of M2: enough units to exercise every rule, and no more.
//
// It lives in the test binary on purpose. The real catalogue is generated from
// YAML (D8) and arrives with M3; a hand-maintained one in the library would be
// a second source of truth for exactly as long as it takes someone to forget
// it exists.
var (
	// Length: a base unit, a scaled one, and a squared one for products.
	Metre     = metrology.MustUnit(metrology.UnitDef{Dimension: dimension.L, Symbol: symbol.SI("m")})
	Kilometre = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.L, Symbol: symbol.Static("km"), Numerator: "1000",
	})
	SquareMetre = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.L.Pow(2), Symbol: symbol.SIPow("m", 2),
	})

	// Force and pressure: the dimensions a division has to produce.
	Newton = metrology.MustUnit(metrology.UnitDef{
		Dimension: forceDimension, Symbol: symbol.SI("N"),
	})
	Pascal = metrology.MustUnit(metrology.UnitDef{
		Dimension: pressureDimension, Symbol: symbol.SI("Pa"),
	})
	Bar = metrology.MustUnit(metrology.UnitDef{
		Dimension: pressureDimension, Symbol: symbol.Static("bar"), Numerator: "100000",
	})
	// D4: the torr is 101325/760 pascal exactly. Stored as a fraction it can
	// be checked against the SI Brochure; stored as 133.32236842105263 it
	// cannot, and it rounds a second time on every conversion.
	Torr = metrology.MustUnit(metrology.UnitDef{
		Dimension: pressureDimension, Symbol: symbol.Static("Torr"),
		Numerator: "101325", Denominator: "760",
	})

	// Temperature: the affine scales D6 exists for.
	Kelvin = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Symbol: symbol.SI("K"),
	})
	KelvinAbsolute = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.SI("K"),
		Interval: &Kelvin,
	})
	Celsius = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.Static("°C"),
		Offset: "273.15", Interval: &Kelvin,
	})
	// Fahrenheit: (v + 459.67) · 5/9 kelvin. The 5/9 is why D4 stores
	// fractions — as a decimal it is 0.5555… and never exact.
	Fahrenheit = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.Static("°F"),
		Numerator: "5", Denominator: "9", Offset: "459.67", Interval: &Rankine,
	})
	// Rankine is the interval scale of Fahrenheit: 1 °R is 5/9 K.
	Rankine = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Symbol: symbol.Static("°R"),
		Numerator: "5", Denominator: "9",
	})
)

var (
	forceDimension    = dimension.New(dimension.Exponents{Length: 1, Mass: 1, Time: -2})
	pressureDimension = dimension.New(dimension.Exponents{Length: -1, Mass: 1, Time: -2})
)
