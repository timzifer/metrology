package metrology_test

import (
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

// The mini catalogue: enough units to exercise every rule of the core, and no
// more.
//
// It lives in the test binary on purpose. The shipped catalogue is generated
// from YAML (D8), and the core is tested against units the test defines rather
// than against it: a test that uses the catalogue to test the engine fails twice
// for one defect and tells you neither time which one it was.
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

	// Dimensionless: the only dimension the zero Unit shares, and therefore the
	// only one an addition or a conversion involving it gets far enough to
	// reject for having no scale rather than for a dimension mismatch.
	One = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("1"),
	})
	// Unnamed is dimensionless and unspelled, which makes it the one built unit
	// that agrees with the zero Unit on dimension, kind, quantity and symbol —
	// so it is what drives Unit.Equal past those four onto the comparison of
	// the factors themselves.
	Unnamed = metrology.MustUnit(metrology.UnitDef{Dimension: dimension.One})
	// D20: the factors that carry a power of π, in both directions. The degree
	// is π/180 radians and the arcsecond π/648000, so a conversion between the
	// two cancels the exponent and stays exact; the oersted is 1000/4π amperes
	// per metre, where the exponent is negative and π lands in the denominator
	// of the conversion instead.
	Radian = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("rad"),
	})
	Degree = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("°"),
		Denominator: "180", Pi: 1,
	})
	Arcsecond = metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.Static("″"),
		Denominator: "648000", Pi: 1,
	})
	AmperePerMetre = metrology.MustUnit(metrology.UnitDef{
		Dimension: fieldDimension, Symbol: symbol.Static("A/m"),
	})
	Oersted = metrology.MustUnit(metrology.UnitDef{
		Dimension: fieldDimension, Symbol: symbol.Static("Oe"),
		Numerator: "1000", Denominator: "4", Pi: -1,
	})
)

var (
	forceDimension    = dimension.New(dimension.Exponents{Length: 1, Mass: 1, Time: -2})
	pressureDimension = dimension.New(dimension.Exponents{Length: -1, Mass: 1, Time: -2})
	fieldDimension    = dimension.New(dimension.Exponents{Length: -1, ElectricCurrent: 1})
)
