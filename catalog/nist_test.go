package catalog_test

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/units/absorbeddose"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/area"
	"github.com/timzifer/metrology/units/customary"
	"github.com/timzifer/metrology/units/customary/imperial"
	"github.com/timzifer/metrology/units/customary/us"
	"github.com/timzifer/metrology/units/density"
	"github.com/timzifer/metrology/units/dose"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/energy"
	"github.com/timzifer/metrology/units/fluxdensity"
	"github.com/timzifer/metrology/units/force"
	"github.com/timzifer/metrology/units/kinematicviscosity"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/magneticflux"
	"github.com/timzifer/metrology/units/mass"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/ratio"
	"github.com/timzifer/metrology/units/velocity"
	"github.com/timzifer/metrology/units/viscosity"
	"github.com/timzifer/metrology/units/volume"
	"github.com/timzifer/metrology/units/volumeflow"
)

// The golden test of the catalogue: one of each non-SI unit, in the SI unit it
// is defined against, compared to the factor printed in NIST SP 811.
//
// The expected values are written out in full rather than computed, because a
// test that computes what it checks proves only that the code agrees with
// itself. These digits come from the standard; if a catalogue entry is edited
// into a plausible-looking wrong number, this table is what notices.
//
// Every factor in the catalogue is exact. The comparison is to eighteen
// significant digits, two below the default precision of the engine, because a
// factor such as one 3600th has no finite decimal form and the conversion
// therefore rounds once (D4, D9). Eighteen digits is far past the point where a
// pre-divided factor would already have failed: 133.32236842105263 as a stored
// torr goes wrong in the seventeenth.
func TestNISTConversionFactors(t *testing.T) {
	for _, tc := range []struct {
		name string
		from metrology.Measurement
		to   metrology.Unit
		want string
	}{
		// --- SI Brochure Table 8, non-SI units accepted for use with the SI --
		{"minute", duration.Minute.Of(1), duration.Second, "60"},
		{"hour", duration.Hour.Of(1), duration.Second, "3600"},
		{"day", duration.Day.Of(1), duration.Second, "86400"},
		{"tonne", mass.Tonne.Of(1), mass.Kilogram, "1000"},
		{"litre", volume.Litre.Of(1), volume.CubicMetre, "0.001"},
		{"hectare", area.Hectare.Of(1), area.SquareMetre, "10000"},
		{"bar", pressure.Bar.Of(1), pressure.Pascal, "100000"},
		{"electronvolt", energy.Electronvolt.Of(1), energy.Joule, "0.0000000000000000001602176634"},

		// --- NIST SP 811 Appendix B.8, alphabetically as printed there -------
		{"ångström", length.Angstrom.Of(1), length.Metre, "0.0000000001"},
		{"are", area.Are.Of(1), area.SquareMetre, "100"},
		{"barn", area.Barn.Of(1), area.SquareMetre,
			"0.0000000000000000000000000001"},
		{"calorie (IT)", energy.Calorie.Of(1), energy.Joule, "4.1868"},
		{"curie", activity.Curie.Of(1), activity.Becquerel, "37000000000"},
		{"dyne", force.Dyne.Of(1), force.Newton, "0.00001"},
		{"erg", energy.Erg.Of(1), energy.Joule, "0.0000001"},
		{"gauss", fluxdensity.Gauss.Of(1), fluxdensity.Tesla, "0.0001"},
		{"kilometre per hour", velocity.KilometrePerHour.Of(1), velocity.MetrePerSecond,
			"0.2777777777777777777777777778"},
		{"kilowatt hour", energy.KilowattHour.Of(1), energy.Joule, "3600000"},
		{"maxwell", magneticflux.Maxwell.Of(1), magneticflux.Weber, "0.00000001"},
		{"millimetre of mercury", pressure.MillimetreOfMercury.Of(1), pressure.Pascal,
			"133.322387415"},
		{"millimetre of water", pressure.MillimetreOfWater.Of(1), pressure.Pascal, "9.80665"},
		{"poise", viscosity.Poise.Of(1), viscosity.PascalSecond, "0.1"},
		{"rem", dose.Rem.Of(1), dose.Sievert, "0.01"},
		{"standard atmosphere", pressure.Atmosphere.Of(1), pressure.Pascal, "101325"},
		{"stokes", kinematicviscosity.Stokes.Of(1), kinematicviscosity.SquareMetrePerSecond,
			"0.0001"},
		{"torr", pressure.Torr.Of(1), pressure.Pascal, "133.3223684210526315789"},

		// --- NIST SP 811 Appendix B.9, units of the process industries -------
		{"cubic metre per hour", volumeflow.CubicMetrePerHour.Of(1), volumeflow.CubicMetrePerSecond,
			"0.0002777777777777777777778"},
		{"litre per minute", volumeflow.LitrePerMinute.Of(1), volumeflow.CubicMetrePerSecond,
			"0.00001666666666666666666667"},
		{"gram per litre", density.GramPerLitre.Of(1), density.KilogramPerCubicMetre, "1"},
		{"percent", ratio.Percent.Of(1), ratio.One, "0.01"},
		{"parts per million", ratio.PartsPerMillion.Of(1), ratio.One, "0.000001"},

		// --- NIST SP 811 Appendix B.6, the customary units (D19) ------------
		//
		// All of these are exact: the international yard and pound agreement of
		// 1959 fixes the inch and the pound, and the rest follow by exact
		// multiplication. The pound per square inch is the one whose quotient
		// is not a finite decimal, which is why it is stored as the fraction it
		// is defined by (D4) and compared here to more digits than a rounded
		// factor could carry.
		{"inch", customary.Inch.Of(1), length.Metre, "0.0254"},
		{"foot", customary.Foot.Of(1), length.Metre, "0.3048"},
		{"yard", customary.Yard.Of(1), length.Metre, "0.9144"},
		{"mile", customary.Mile.Of(1), length.Metre, "1609.344"},
		{"pound", customary.Pound.Of(1), mass.Kilogram, "0.45359237"},
		{"ounce", customary.Ounce.Of(1), mass.Kilogram, "0.028349523125"},
		{"pound-force", customary.PoundForce.Of(1), force.Newton, "4.4482216152605"},
		{"pound per square inch", customary.PoundPerSquareInch.Of(1), pressure.Pascal,
			"6894.757293168361336722673445"},

		// The two systems disagree about these, which is why they are in two
		// packages and neither spells itself "gal" (O3).
		{"US gallon", us.Gallon.Of(1), volume.CubicMetre, "0.003785411784"},
		{"imperial gallon", imperial.Gallon.Of(1), volume.CubicMetre, "0.00454609"},
		{"US fluid ounce", us.FluidOunce.Of(1), volume.CubicMetre, "0.0000295735295625"},
		{"imperial fluid ounce", imperial.FluidOunce.Of(1), volume.CubicMetre, "0.0000284130625"},
		{"short ton", us.Ton.Of(1), mass.Kilogram, "907.18474"},
		{"long ton", imperial.Ton.Of(1), mass.Kilogram, "1016.0469088"},

		// --- The exact ones, in the direction that exposes a rounded factor --
		{"760 torr is one atmosphere", pressure.Torr.Of(760), pressure.Pascal, "101325"},
		{"and back", pressure.Atmosphere.Of(1), pressure.Torr, "760"},
		{"a gray is a joule per kilogram", absorbeddose.Gray.Of(1), absorbeddose.Gray, "1"},
		{"twelve inches are a foot", customary.Inch.Of(12), customary.Foot, "1"},
		{"sixteen ounces are a pound", customary.Ounce.Of(16), customary.Pound, "1"},
		{"1760 yards are a mile", customary.Yard.Of(1760), customary.Mile, "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.from.To(tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want, err := tc.to.OfString(tc.want)
			if err != nil {
				t.Fatalf("the expected value is not a number: %v", err)
			}
			assertAgrees(t, got, want, 18)
		})
	}
}

// Both directions, for every unit in the catalogue: converting to the canonical
// unit of the dimension and back reproduces the magnitude.
//
// This is the property the golden table above only samples, and it is the one
// that catches a factor whose numerator and denominator do not belong together:
// such a unit survives one direction and comes back somewhere else.
//
// It is a round trip to eighteen digits, not to the last one. Where a factor
// has no finite decimal form — one 60000th of a cubic metre per second, for the
// litre per minute — the conversion rounds by D9 and the return trip cannot
// undo that rounding. Demanding exactness here would be demanding that the
// engine store an infinite decimal.
func TestEveryUnitRoundTripsThroughItsCanonicalUnit(t *testing.T) {
	magnitudes := []string{"1", "2.5", "760", "0.0001", "-40", "123456789.123456789"}

	for _, unit := range catalog.Units() {
		canonical, ok := catalog.Canonical(unit.Dimension(), unit.Kind(), unit.Quantity())
		if !ok {
			t.Errorf("%s has no canonical unit", unit)
			continue
		}
		for _, magnitude := range magnitudes {
			t.Run(unit.String()+"/"+magnitude, func(t *testing.T) {
				start, err := unit.OfString(magnitude)
				if err != nil {
					t.Fatalf("OfString: %v", err)
				}
				there, err := start.To(canonical)
				if err != nil {
					t.Fatalf("to %s: %v", canonical, err)
				}
				back, err := there.To(unit)
				if err != nil {
					t.Fatalf("back to %s: %v", unit, err)
				}
				assertAgrees(t, back, start, 18)
			})
		}
	}
}

// assertAgrees compares two measurements to the given number of significant
// digits.
//
// The rounding happens here rather than in the library: a Measurement holds
// what it was given, and D9 puts the precision in the computation. A test that
// asks whether two conversions agree has to say how far it is looking.
func assertAgrees(t *testing.T, got, want metrology.Measurement, digits uint32) {
	t.Helper()

	ctx := apd.BaseContext
	ctx.Precision = digits

	var roundedGot, roundedWant apd.Decimal
	if _, err := ctx.Round(&roundedGot, got.Decimal()); err != nil {
		t.Fatalf("rounding %s: %v", got, err)
	}
	if _, err := ctx.Round(&roundedWant, want.Decimal()); err != nil {
		t.Fatalf("rounding %s: %v", want, err)
	}
	if roundedGot.Cmp(&roundedWant) != 0 {
		t.Errorf("got %s, want %s (to %d digits: %s against %s)",
			got, want, digits, &roundedGot, &roundedWant)
	}
}
