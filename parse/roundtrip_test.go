package parse_test

import (
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/parse"
)

// magnitudes exercise the shapes a magnitude takes: a plain one, a negative one
// on an affine scale, zero, and more digits than a float64 holds.
var magnitudes = []string{"2.5", "-40", "0", "1", "1234567890.123456789012345678901"}

// TestRoundTrip is the property M5 is measured by: everything the library
// prints, the library reads back — as the same unit, the same kind and the same
// digits, across the whole catalogue.
//
// The kind is the one thing the text does not carry ("K" is a temperature and a
// temperature difference), so the parser is asked for the reading the unit was
// written in. Everything else has to come back on its own.
func TestRoundTrip(t *testing.T) {
	for _, unit := range catalog.Units() {
		p := parse.Default().Prefer(unit.Kind())
		for _, magnitude := range magnitudes {
			m, err := unit.OfString(magnitude)
			if err != nil {
				t.Fatal(err)
			}
			text := m.String()
			got, err := p.Measurement(text)
			if err != nil {
				t.Errorf("%q: %v", text, err)
				continue
			}
			if !got.Unit().Equal(unit) {
				t.Errorf("%q came back as %s, want %s", text, got.Unit(), unit)
			}
			if got.Kind() != unit.Kind() || got.Quantity() != unit.Quantity() {
				t.Errorf("%q came back as %s %q, want %s %q",
					text, got.Kind(), got.Quantity(), unit.Kind(), unit.Quantity())
			}
			if again := got.String(); again != text {
				t.Errorf("%q came back as %q", text, again)
			}
		}
	}
}

// The display form of D9 round-trips too, as the same quantity rather than the
// same text: 250 kPa is 250000 Pa, and the prefix a magnitude is printed with
// has to be one the parser reads back.
func TestRoundTripPrefixed(t *testing.T) {
	for _, unit := range catalog.Units() {
		p := parse.Default().Prefer(unit.Kind())
		for _, magnitude := range []string{"2.5", "0.000001", "1000000"} {
			m, err := unit.OfString(magnitude)
			if err != nil {
				t.Fatal(err)
			}
			text := m.Prefixed()
			got, err := p.Measurement(text)
			if err != nil {
				t.Errorf("%q: %v", text, err)
				continue
			}
			if got.Dimension() != m.Dimension() || got.Kind() != m.Kind() {
				t.Errorf("%q came back as %s %s, want %s %s",
					text, got.Dimension(), got.Kind(), m.Dimension(), m.Kind())
			}
			if !got.Equal(m) {
				t.Errorf("%q came back as %q, which is not %q", text, got, m)
			}
		}
	}
}

// A computed unit is not in any catalogue — its symbol is built from the
// operands — and it has to read back all the same, because it is what a program
// prints after doing arithmetic.
func TestRoundTripComputedUnits(t *testing.T) {
	units := catalog.Units()
	for i, left := range units {
		// Every catalogue unit against a handful of others: the full square is
		// six thousand pairs and buys nothing the diagonal does not.
		right := units[(i*7+3)%len(units)]
		if left.Kind() == metrology.Absolute || right.Kind() == metrology.Absolute {
			continue // a point on a scale has no product (D6)
		}
		for _, tc := range []struct {
			name string
			fn   func() (metrology.Unit, error)
		}{
			{"product", func() (metrology.Unit, error) { return left.Times(right) }},
			{"quotient", func() (metrology.Unit, error) { return left.Per(right) }},
			{"square", func() (metrology.Unit, error) { return left.Pow(2) }},
			{"reciprocal", func() (metrology.Unit, error) { return left.Pow(-1) }},
		} {
			unit, err := tc.fn()
			if err != nil {
				t.Fatalf("%s of %s and %s: %v", tc.name, left, right, err)
			}
			text := unit.Of(2.5).String()
			got, err := parse.Measurement(text)
			if err != nil {
				t.Errorf("%q: %v", text, err)
				continue
			}
			if got.Dimension() != unit.Dimension() {
				t.Errorf("%q came back as %s, want %s", text, got.Dimension(), unit.Dimension())
			}
			// The factor has to survive too, not only the exponents: both
			// sides expressed in the same unit have to be the same number.
			if !got.Equal(unit.Of(2.5)) {
				t.Errorf("%q came back as %q", text, got)
			}
			if again := got.String(); again != text {
				t.Errorf("%q came back as %q", text, again)
			}
		}
	}
}

// Two units of the same kind must not claim the same spelling: the text form is
// only a canonical form if a symbol names one unit. The kind is the one axis
// where a collision is by design — "K" is both — and the parser resolves that
// one with [parse.Parser.Prefer].
func TestCatalogueSpellingsAreUnambiguous(t *testing.T) {
	type key struct {
		text string
		kind metrology.Kind
	}
	seen := map[key]metrology.Unit{}
	for _, unit := range catalog.Units() {
		k := key{text: unit.String(), kind: unit.Kind()}
		if other, ok := seen[k]; ok {
			t.Errorf("%q is the symbol of both %s and %s", k.text, other, unit)
		}
		seen[k] = unit
	}
}
