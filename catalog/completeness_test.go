package catalog_test

import (
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
)

// The catalogue holds all seven SI base units and all twenty-two named derived
// units. This is that list, taken from the SI Brochure, and it is a list rather
// than a count so that a missing unit says which one.
func TestSIIsComplete(t *testing.T) {
	t.Run("the seven base units", func(t *testing.T) {
		for _, tc := range []struct {
			symbol   string
			kind     metrology.Kind
			quantity metrology.Quantity
		}{
			{"s", metrology.Interval, ""},
			{"m", metrology.Interval, ""},
			{"kg", metrology.Interval, ""},
			{"A", metrology.Interval, ""},
			// The kelvin is in the catalogue twice, as the point on the scale
			// and as the span along it (D6). The base unit is the point.
			{"K", metrology.Absolute, ""},
			{"mol", metrology.Interval, ""},
			{"cd", metrology.Interval, "luminous intensity"},
		} {
			assertPresent(t, tc.symbol, tc.kind, tc.quantity)
		}
	})

	t.Run("the twenty-two named derived units", func(t *testing.T) {
		for _, tc := range []struct {
			symbol   string
			kind     metrology.Kind
			quantity metrology.Quantity
		}{
			{"rad", metrology.Interval, "plane angle"},
			{"sr", metrology.Interval, "solid angle"},
			{"Hz", metrology.Interval, "frequency"},
			{"N", metrology.Interval, ""},
			{"Pa", metrology.Interval, ""},
			{"J", metrology.Interval, ""},
			{"W", metrology.Interval, ""},
			{"C", metrology.Interval, ""},
			{"V", metrology.Interval, ""},
			{"F", metrology.Interval, ""},
			{"Ω", metrology.Interval, ""},
			{"S", metrology.Interval, ""},
			{"Wb", metrology.Interval, ""},
			{"T", metrology.Interval, ""},
			{"H", metrology.Interval, ""},
			{"°C", metrology.Absolute, ""},
			{"lm", metrology.Interval, "luminous flux"},
			{"lx", metrology.Interval, ""},
			{"Bq", metrology.Interval, "radioactivity"},
			{"Gy", metrology.Interval, "absorbed dose"},
			{"Sv", metrology.Interval, "dose equivalent"},
			{"kat", metrology.Interval, ""},
		} {
			assertPresent(t, tc.symbol, tc.kind, tc.quantity)
		}
	})
}

// D8 makes the catalogue auditable, which only holds if every entry says where
// it comes from. The generator enforces it; this checks the generator is the
// only thing standing between the catalogue and an unsourced factor.
func TestEveryUnitIsReachableAndDistinct(t *testing.T) {
	seen := map[string]metrology.Unit{}
	for _, u := range catalog.Units() {
		key := u.String() + "\x00" + u.Kind().String()
		if previous, duplicate := seen[key]; duplicate {
			t.Errorf("%s and %s print the same and share a kind", previous, u)
		}
		seen[key] = u

		if _, ok := catalog.BySymbol(u.String(), u.Kind()); !ok {
			t.Errorf("%s is not reachable by its symbol", u)
		}
	}
}

func assertPresent(t *testing.T, symbol string, kind metrology.Kind, quantity metrology.Quantity) {
	t.Helper()

	unit, ok := catalog.BySymbol(symbol, kind)
	if !ok {
		t.Errorf("%s is missing from the catalogue", symbol)
		return
	}
	if unit.Quantity() != quantity {
		t.Errorf("%s is tagged %q, want %q", symbol, unit.Quantity(), quantity)
	}
	// Every dimension in the catalogue resolves to a unit a computed result can
	// be expressed in. It is not always this one: the degree Celsius is a named
	// SI unit, and the kelvin is what a computed temperature comes back as.
	if _, ok := catalog.Canonical(unit.Dimension(), unit.Kind(), unit.Quantity()); !ok {
		t.Errorf("%s has no canonical unit for its own dimension", symbol)
	}
}
