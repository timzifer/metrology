package symbol_test

import (
	"testing"

	"github.com/timzifer/metrology/symbol"
)

func TestString(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    symbol.Symbol
		want string
	}{
		{"zero value renders empty", symbol.Symbol{}, ""},
		{"static", symbol.Static("°C"), "°C"},
		{"si", symbol.SI("Pa"), "Pa"},
		{"si squared", symbol.SIPow("m", 2), "m²"},
		{"si cubed", symbol.SIPow("m", 3), "m³"},
		{"si to the first power has no suffix", symbol.SIPow("m", 1), "m"},
		{"negative power", symbol.SIPow("m", -1), "m⁻¹"},
		// The kilogram names itself with its prefix; the unprefixed form of the
		// SI base unit of mass is kg, not g.
		{"gram", symbol.Gram(), "kg"},
		{"litre", symbol.Litre(), "L"},
		{"product", symbol.Product(symbol.SI("N"), symbol.SI("m")), "N·m"},
		{"product of three", symbol.Product(symbol.SI("N"), symbol.SI("m"), symbol.SI("s")), "N·m·s"},
		{"empty product", symbol.Product(), ""},
		{"quotient", symbol.Quotient(symbol.SI("m"), symbol.SI("s")), "m/s"},
		{"quotient with a squared numerator", symbol.Quotient(symbol.SIPow("m", 2), symbol.SI("s")), "m²/s"},
		// J/(kg·K) must not read as (J/kg)·K.
		{"product denominator is parenthesised", symbol.Quotient(
			symbol.SI("J"), symbol.Product(symbol.Gram(), symbol.SI("K")),
		), "J/(kg·K)"},
		// A solidus binds from the left, so an unbracketed m/s/A would read
		// back as (m/s)/A — a different dimension from the one written.
		{"quotient denominator is parenthesised", symbol.Quotient(
			symbol.SI("m"), symbol.Quotient(symbol.SI("s"), symbol.SI("A")),
		), "m/(s/A)"},
		// The same holds for a static symbol that joins two units itself.
		{"composite static denominator is parenthesised", symbol.Quotient(
			symbol.Static("b"), symbol.Static("km/h"),
		), "b/(km/h)"},
		// An exponent binds to its own symbol, so J/m² needs no brackets.
		{"an exponent in the denominator does not", symbol.Quotient(
			symbol.SI("J"), symbol.SIPow("m", 2),
		), "J/m²"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b symbol.Symbol
		want bool
	}{
		{"same static", symbol.Static("°C"), symbol.Static("°C"), true},
		{"different text", symbol.Static("°C"), symbol.Static("°F"), false},
		// A static "m" prints "m" but never "km"; the forms are not
		// interchangeable even where the text agrees.
		{"different form, same text", symbol.Static("m"), symbol.SI("m"), false},
		{"different power", symbol.SIPow("m", 2), symbol.SIPow("m", 3), false},
		{"same product", symbol.Product(symbol.SI("N"), symbol.SI("m")),
			symbol.Product(symbol.SI("N"), symbol.SI("m")), true},
		{"product order matters", symbol.Product(symbol.SI("N"), symbol.SI("m")),
			symbol.Product(symbol.SI("m"), symbol.SI("N")), false},
		{"different part count", symbol.Product(symbol.SI("N")),
			symbol.Product(symbol.SI("N"), symbol.SI("m")), false},
		{"same quotient", symbol.Quotient(symbol.SI("m"), symbol.SI("s")),
			symbol.Quotient(symbol.SI("m"), symbol.SI("s")), true},
		{"different denominator", symbol.Quotient(symbol.SI("m"), symbol.SI("s")),
			symbol.Quotient(symbol.SI("m"), symbol.SI("A")), false},
		{"gram equals gram", symbol.Gram(), symbol.Gram(), true},
		{"litre equals litre", symbol.Litre(), symbol.Litre(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.want {
				t.Errorf("Equal() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Product copies its argument slice, so that a caller reusing the slice cannot
// change a symbol after the fact (D3).
func TestProductCopiesItsArgument(t *testing.T) {
	parts := []symbol.Symbol{symbol.SI("N"), symbol.SI("m")}
	s := symbol.Product(parts...)
	parts[1] = symbol.SI("s")
	if got := s.String(); got != "N·m" {
		t.Errorf("mutating the argument slice changed the symbol to %q", got)
	}
}
