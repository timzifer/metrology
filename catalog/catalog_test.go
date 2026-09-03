package catalog_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/units/absorbeddose"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/angle"
	"github.com/timzifer/metrology/units/area"
	"github.com/timzifer/metrology/units/dose"
	"github.com/timzifer/metrology/units/energy"
	"github.com/timzifer/metrology/units/force"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/kinematicviscosity"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/luminosity"
	"github.com/timzifer/metrology/units/luminousflux"
	"github.com/timzifer/metrology/units/mass"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/ratio"
	"github.com/timzifer/metrology/units/solidangle"
	"github.com/timzifer/metrology/units/specificheat"
	"github.com/timzifer/metrology/units/temperature"
	"github.com/timzifer/metrology/units/volumeflow"
)

func TestCanonical(t *testing.T) {
	for _, tc := range []struct {
		name string
		unit metrology.Unit
	}{
		{"length", length.Metre},
		{"area", area.SquareMetre},
		{"force", force.Newton},
		{"pressure", pressure.Pascal},
		{"energy", energy.Joule},
		{"volume flow", volumeflow.CubicMetrePerSecond},
		// One dimension, two kinds: a temperature and a temperature difference
		// are read on the same axis and are not interchangeable (D6).
		{"temperature as a point", temperature.Kelvin},
		{"temperature as a span", interval.Kelvin},
		// One dimension, two quantities. Both are canonical, and which one you
		// get depends on what you say you are measuring.
		{"frequency", frequency.Hertz},
		{"radioactivity", activity.Becquerel},
		{"absorbed dose", absorbeddose.Gray},
		{"dose equivalent", dose.Sievert},
		{"luminous intensity", luminosity.Candela},
		{"luminous flux", luminousflux.Lumen},
		{"a plane angle", angle.Radian},
		{"a solid angle", solidangle.Steradian},
		{"a bare ratio", ratio.One},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := catalog.Canonical(tc.unit.Dimension(), tc.unit.Kind(), tc.unit.Quantity())
			if !ok {
				t.Fatalf("no canonical unit for %s as %s", tc.unit.Dimension(), tc.unit.Quantity())
			}
			if !got.Equal(tc.unit) {
				t.Errorf("got %s, want %s", got, tc.unit)
			}
		})
	}
}

// The tag is what keeps two quantities on one dimension apart — and asking for
// the wrong one is an error rather than a plausible number.
func TestQuantitiesSharingADimension(t *testing.T) {
	if frequency.Hertz.Dimension() != activity.Becquerel.Dimension() {
		t.Fatal("the hertz and the becquerel no longer share a dimension")
	}

	//unitvet:ignore the assertion is that this conversion fails
	if _, err := frequency.Hertz.Of(50).To(activity.Becquerel); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("50 Hz converted to becquerel: %v", err)
	}
	//unitvet:ignore the assertion is that this addition fails
	if _, err := absorbeddose.Gray.Of(1).Add(dose.Sievert.Of(1)); !errors.Is(err, metrology.ErrQuantity) {
		t.Errorf("a gray was added to a sievert: %v", err)
	}
	if _, err := angle.Radian.Of(1).Add(ratio.One.Of(1)); err != nil {
		t.Errorf("an untagged ratio would not join a plane angle: %v", err)
	}

	// A computed magnitude carries no tag (D6), so it can still be named — that
	// is what makes the untagged case the useful one rather than a hole.
	q, err := energy.Joule.Of(10).Div(mass.Kilogram.Of(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Quantity() != "" {
		t.Errorf("a quotient carries the tag %q", q.Quantity())
	}
	if _, err := q.To(absorbeddose.Gray); err != nil {
		t.Errorf("an untagged J/kg would not become a gray: %v", err)
	}
	if _, err := q.To(dose.Sievert); err != nil {
		t.Errorf("an untagged J/kg would not become a sievert: %v", err)
	}
}

// Every tag this catalogue reserves is declared as a constant in the package
// that owns it, and the unit definitions are generated from that same constant
// (D16). What the test adds is the half a caller depends on: the constant is
// the tag, so a program never has to spell "kinematic viscosity" itself and a
// change to the catalogue reaches it as a compile error rather than as a
// lookup that quietly stops matching.
func TestQuantityConstants(t *testing.T) {
	for _, tc := range []struct {
		unit metrology.Unit
		tag  metrology.Quantity
	}{
		{frequency.Hertz, frequency.Quantity},
		{activity.Becquerel, activity.Quantity},
		{absorbeddose.Gray, absorbeddose.Quantity},
		{dose.Sievert, dose.Quantity},
		{angle.Radian, angle.Quantity},
		{solidangle.Steradian, solidangle.Quantity},
		{luminosity.Candela, luminosity.Quantity},
		{luminousflux.Lumen, luminousflux.Quantity},
		{kinematicviscosity.SquareMetrePerSecond, kinematicviscosity.Quantity},
	} {
		if got := tc.unit.Quantity(); got != tc.tag {
			t.Errorf("%s carries the tag %q, its package declares %q", tc.unit, got, tc.tag)
		}
		canonical, ok := catalog.Canonical(tc.unit.Dimension(), tc.unit.Kind(), tc.tag)
		if !ok {
			t.Errorf("the catalogue has no canonical unit for %q", tc.tag)
			continue
		}
		if !canonical.Equal(tc.unit) {
			t.Errorf("%q resolves to %s, not to %s", tc.tag, canonical, tc.unit)
		}
	}
}

// A dimension nobody has named has no canonical unit, and that is an answer
// rather than a failure: a length over a mass is a perfectly good quantity.
func TestCanonicalReportsWhatItDoesNotHave(t *testing.T) {
	odd := dimension.New(dimension.Exponents{Length: 1, Mass: -1})
	if _, ok := catalog.Canonical(odd, metrology.Interval, ""); ok {
		t.Errorf("the catalogue claims a unit for %s", odd)
	}
	// The dimension exists, the kind does not: there is no absolute pressure
	// scale in this catalogue.
	if _, ok := catalog.Canonical(pressure.Pascal.Dimension(), metrology.Absolute, ""); ok {
		t.Error("the catalogue claims an absolute pressure unit")
	}
	// The dimension and kind exist, the quantity does not: T⁻¹ is a frequency
	// or a radioactivity, and untagged it is neither.
	if _, ok := catalog.Canonical(frequency.Hertz.Dimension(), metrology.Interval, ""); ok {
		t.Error("the catalogue claims an untagged unit for T⁻¹")
	}
}

func TestBySymbol(t *testing.T) {
	for _, tc := range []struct {
		text string
		kind metrology.Kind
		want metrology.Unit
	}{
		{"Pa", metrology.Interval, pressure.Pascal},
		{"bar", metrology.Interval, pressure.Bar},
		{"Torr", metrology.Interval, pressure.Torr},
		{"m²", metrology.Interval, area.SquareMetre},
		{"kg", metrology.Interval, mass.Kilogram},
		{"J/(kg·K)", metrology.Interval, specificheat.JoulePerKilogramKelvin},
		{"m³/h", metrology.Interval, volumeflow.CubicMetrePerHour},
		{"Hz", metrology.Interval, frequency.Hertz},
		{"Bq", metrology.Interval, activity.Becquerel},
		{"°C", metrology.Absolute, temperature.Celsius},
		// The same text, two kinds, two units. This is why the kind is part of
		// the key rather than a detail of the value.
		{"K", metrology.Absolute, temperature.Kelvin},
		{"K", metrology.Interval, interval.Kelvin},
	} {
		t.Run(tc.text+"/"+tc.kind.String(), func(t *testing.T) {
			got, ok := catalog.BySymbol(tc.text, tc.kind)
			if !ok {
				t.Fatalf("no unit for %q", tc.text)
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}

	// The lookup is exact and unprefixed; reading "kPa" is the parser's job,
	// because the prefix moves the magnitude as well as the symbol.
	if _, ok := catalog.BySymbol("kPa", metrology.Interval); ok {
		t.Error("BySymbol resolved a prefixed symbol")
	}
	if _, ok := catalog.BySymbol("furlong", metrology.Interval); ok {
		t.Error("BySymbol resolved a unit the catalogue does not have")
	}
}

func TestUnits(t *testing.T) {
	units := catalog.Units()
	if len(units) == 0 {
		t.Fatal("the catalogue is empty")
	}

	// Every unit is reachable through the symbol index, and the two agree.
	for _, u := range units {
		got, ok := catalog.BySymbol(u.String(), u.Kind())
		if !ok {
			t.Errorf("%s is in the catalogue but not in the symbol index", u)
			continue
		}
		if !got.Equal(u) {
			t.Errorf("%s resolves to %s", u, got)
		}
	}

	// The slice is a copy: the catalogue is not something a caller can
	// rearrange (D7).
	first := units[0].String()
	units[0] = length.Metre
	if got := catalog.Units()[0].String(); got != first {
		t.Errorf("writing to the returned slice changed the catalogue: %s became %s", first, got)
	}
}

// The generated definitions carry the factors of the catalogue, which is the
// only claim that matters: the numbers a user gets are the numbers the YAML
// declares, with no rounding introduced on the way through the generator.
func TestGeneratedUnitsConvertExactly(t *testing.T) {
	for _, tc := range []struct {
		name string
		from metrology.Measurement
		to   metrology.Unit
		want string
	}{
		{"bar to pascal", pressure.Bar.Of(2.5), pressure.Pascal, "250000 Pa"},
		{"the torr is exact", pressure.Torr.Of(760), pressure.Pascal, "101325 Pa"},
		{"kilometre to metre", length.Kilometre.Of(1.5), length.Metre, "1500 m"},
		{"celsius to kelvin", temperature.Celsius.Of(20), temperature.Kelvin, "293.15 K"},
		{"fahrenheit to celsius", temperature.Fahrenheit.Of(212), temperature.Celsius, "100 °C"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.from.To(tc.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// The interval declaration survives generation, across package boundaries:
// temperature.Celsius names interval.Kelvin, and a difference lands there.
func TestGeneratedIntervalUnits(t *testing.T) {
	diff, err := temperature.Celsius.Of(25).Sub(temperature.Celsius.Of(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.String() != "5 K" {
		t.Errorf("got %s, want 5 K", diff)
	}
	if !diff.Unit().Equal(interval.Kelvin) {
		t.Errorf("difference is in %s, want interval.Kelvin", diff.Unit())
	}

	fahrenheit, err := temperature.Fahrenheit.Of(212).Sub(temperature.Fahrenheit.Of(32))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fahrenheit.Unit().Equal(interval.Rankine) {
		t.Errorf("difference is in %s, want interval.Rankine", fahrenheit.Unit())
	}
	kelvins, err := fahrenheit.In[float64](interval.Kelvin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kelvins != 100 {
		t.Errorf("180 °R = %v K, want 100", kelvins)
	}
}

// A quotient computed from catalogue units lands on a dimension the catalogue
// knows, which is what makes Canonical useful: the result can be named.
func TestQuotientResolvesThroughTheCatalogue(t *testing.T) {
	q, err := force.Newton.Of(100).Div(area.SquareMetre.Of(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	unit, ok := catalog.Canonical(q.Dimension(), q.Kind(), q.Quantity())
	if !ok {
		t.Fatalf("no canonical unit for %s", q.Dimension())
	}
	named, err := q.To(unit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if named.String() != "50 Pa" {
		t.Errorf("got %s, want 50 Pa", named)
	}
}
