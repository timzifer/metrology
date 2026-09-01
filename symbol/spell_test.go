package symbol_test

import (
	"testing"

	"github.com/timzifer/metrology/symbol"
)

// spellingOf finds what a symbol says one text means, so that a test can ask
// about "kPa" without depending on where in the table it sits.
func spellingOf(t *testing.T, s symbol.Symbol, text string) (int, bool) {
	t.Helper()
	for _, sp := range s.Spellings() {
		if sp.Text == text {
			return sp.Exponent, true
		}
	}
	return 0, false
}

func TestSpellings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		s        symbol.Symbol
		text     string
		exponent int
	}{
		{"the symbol's own form has exponent zero", symbol.SI("Pa"), "Pa", 0},
		{"kilo", symbol.SI("Pa"), "kPa", 3},
		{"micro", symbol.SI("m"), "µm", -6},
		{"quetta", symbol.SI("m"), "Qm", 30},
		// One prefix step on m² is a factor of a million, because a square
		// kilometre is 10⁶ square metres.
		{"a prefix on a squared symbol scales by the power", symbol.SIPow("m", 2), "km²", 6},
		{"and downwards too", symbol.SIPow("m", 2), "mm²", -6},
		{"a negative power reverses the step", symbol.SIPow("s", -1), "ks⁻¹", -3},
		// Magnitudes are in kilograms, so the gram is the prefixed spelling.
		{"the kilogram spells itself with its prefix", symbol.Gram(), "kg", 0},
		{"the gram is a thousandth of it", symbol.Gram(), "g", -3},
		{"and a milligram a millionth", symbol.Gram(), "mg", -6},
		{"the litre takes the prefixes labels use", symbol.Litre(), "cL", -2},
		{"including hecto", symbol.Litre(), "hL", 2},
		{"a product is prefixed on its first multiplicand", symbol.Product(symbol.SI("N"), symbol.SI("m")), "kN·m", 3},
		{"a quotient on its numerator", symbol.Quotient(symbol.SI("m"), symbol.SI("s")), "km/s", 3},
		{"a gram numerator keeps the kilogram convention", symbol.Quotient(symbol.Gram(), symbol.SI("s")), "g/s", -3},
		{"a product denominator stays bracketed", symbol.Quotient(
			symbol.SI("J"), symbol.Product(symbol.Gram(), symbol.SI("K")),
		), "kJ/(kg·K)", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := spellingOf(t, tc.s, tc.text)
			if !ok {
				t.Fatalf("%s has no spelling %q", tc.s, tc.text)
			}
			if got != tc.exponent {
				t.Errorf("%q means 10^%d of %s, want 10^%d", tc.text, got, tc.s, tc.exponent)
			}
		})
	}
}

// TestSpellingsAreStrict is the half that matters more: a symbol must not
// accept a spelling that belongs to another unit. The prefixes a symbol takes
// are the ones its form declares, so a static symbol takes none at all — which
// is what keeps "cd" the candela rather than a centi-day.
func TestSpellingsAreStrict(t *testing.T) {
	for _, tc := range []struct {
		s    symbol.Symbol
		text string
	}{
		{symbol.Static("d"), "cd"}, // the day takes no prefix
		{symbol.Static("Torr"), "mTorr"},
		{symbol.SI("Pa"), "Pascal"},
		{symbol.SI("m"), "m²"}, // the power is part of the symbol
		{symbol.SIPow("m", 2), "m"},
		{symbol.Gram(), "kkg"},
		{symbol.Litre(), "kL"}, // not a prefix the litre carries
		{symbol.Quotient(symbol.SI("m"), symbol.SI("s")), "m/ks"},
		{symbol.Product(symbol.SI("N"), symbol.SI("m")), "N·km"},
	} {
		if _, ok := spellingOf(t, tc.s, tc.text); ok {
			t.Errorf("%s accepts %q", tc.s, tc.text)
		}
	}
}

// TestSpellingsFirstIsTheSymbolsOwn states the ordering a parser relies on to
// resolve a collision: "km" is the kilometre before it is a prefixed metre.
func TestSpellingsFirstIsTheSymbolsOwn(t *testing.T) {
	for _, s := range []symbol.Symbol{
		symbol.Static("bar"), symbol.SI("Pa"), symbol.SIPow("m", 3), symbol.Gram(),
		symbol.Litre(), symbol.Product(symbol.SI("N"), symbol.SI("m")),
		symbol.Quotient(symbol.SI("m"), symbol.SI("s")), symbol.Product(),
	} {
		first := s.Spellings()[0]
		if first.Text != s.String() || first.Exponent != 0 {
			t.Errorf("%s spells itself %q with 10^%d, want %q with 10^0",
				s, first.Text, first.Exponent, s.String())
		}
	}
}

// A symbol whose power is zero has one spelling: every prefix would scale by
// 10⁰ and the letters would mean nothing.
func TestSpellingsOfAZerothPower(t *testing.T) {
	got := symbol.SIPow("m", 0).Spellings()
	if len(got) != 1 || got[0].Text != "m⁰" || got[0].Exponent != 0 {
		t.Errorf("SIPow(m, 0).Spellings() = %v", got)
	}
}

func TestPow(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    symbol.Symbol
		n    int
		want string
	}{
		{"the first power is the symbol itself", symbol.SI("m"), 1, "m"},
		{"squared", symbol.SI("m"), 2, "m²"},
		{"cubed", symbol.SI("m"), 3, "m³"},
		{"reciprocal", symbol.SI("s"), -1, "s⁻¹"},
		{"a power of a power multiplies", symbol.SIPow("m", 2), 3, "m⁶"},
		{"the zeroth power is the dimensionless one", symbol.SI("m"), 0, "1"},
		{"and for a product too", symbol.Product(symbol.SI("N"), symbol.SI("m")), 0, "1"},
		{"a plain static symbol takes the exponent directly", symbol.Static("bar"), 2, "bar²"},
		{"the kilogram is a word too", symbol.Gram(), 2, "kg²"},
		{"the litre likewise", symbol.Litre(), 3, "L³"},
		// Squaring m³/h must square the whole unit, not the hour.
		{"a static symbol with an operator is bracketed", symbol.Static("m³/h"), 2, "(m³/h)²"},
		// An exponent already on the symbol binds to it, so m² cubed is
		// (m²)³ — written flat it would read as m to the eighth.
		{"a static symbol with an exponent is bracketed", symbol.Static("m²"), 3, "(m²)³"},
		{"a product is bracketed", symbol.Product(symbol.SI("N"), symbol.SI("m")), 2, "(N·m)²"},
		{"a quotient is bracketed", symbol.Quotient(symbol.SI("m"), symbol.SI("s")), 2, "(m/s)²"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Pow(tc.n).String(); got != tc.want {
				t.Errorf("%s.Pow(%d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}
