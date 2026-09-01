package catalog_test

import (
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/area"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/force"
	"github.com/timzifer/metrology/interval"
	"github.com/timzifer/metrology/length"
	"github.com/timzifer/metrology/pressure"
	"github.com/timzifer/metrology/temperature"
)

func TestCanonical(t *testing.T) {
	for _, tc := range []struct {
		name string
		dim  dimension.Dimension
		kind metrology.Kind
		want metrology.Unit
	}{
		{"length", length.Metre.Dimension(), metrology.Interval, length.Metre},
		{"area", area.SquareMetre.Dimension(), metrology.Interval, area.SquareMetre},
		{"force", force.Newton.Dimension(), metrology.Interval, force.Newton},
		{"pressure", pressure.Pascal.Dimension(), metrology.Interval, pressure.Pascal},
		// One dimension, two canonical units: a temperature and a temperature
		// difference are different quantities on the same axis (D6).
		{"temperature as a point", temperature.Kelvin.Dimension(), metrology.Absolute, temperature.Kelvin},
		{"temperature as a span", interval.Kelvin.Dimension(), metrology.Interval, interval.Kelvin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := catalog.Canonical(tc.dim, tc.kind)
			if !ok {
				t.Fatalf("no canonical unit for %s", tc.dim)
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A dimension nobody has named has no canonical unit, and that is an answer
// rather than a failure: a length over a mass is a perfectly good quantity.
func TestCanonicalReportsWhatItDoesNotHave(t *testing.T) {
	odd := dimension.New(dimension.Exponents{Length: 1, Mass: -1})
	if _, ok := catalog.Canonical(odd, metrology.Interval); ok {
		t.Errorf("the catalogue claims a unit for %s", odd)
	}
	// The dimension exists, the kind does not: there is no absolute pressure
	// scale in this catalogue.
	if _, ok := catalog.Canonical(pressure.Pascal.Dimension(), metrology.Absolute); ok {
		t.Error("the catalogue claims an absolute pressure unit")
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
	unit, ok := catalog.Canonical(q.Dimension(), q.Kind())
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
