package symbol_test

import (
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/symbol"
)

func decimal(t *testing.T, s string) *apd.Decimal {
	t.Helper()
	d, _, err := apd.NewFromString(s)
	if err != nil {
		t.Fatalf("NewFromString(%q): %v", s, err)
	}
	return d
}

func TestScale(t *testing.T) {
	for _, tc := range []struct {
		name      string
		s         symbol.Symbol
		in        string
		wantValue string
		wantText  string
	}{
		{"pascal keeps its magnitude", symbol.SI("Pa"), "250", "250", "Pa"},
		{"pascal takes kilo", symbol.SI("Pa"), "250000", "250", "kPa"},
		// The exact case a logarithmic prefix search gets wrong: it yields
		// 999.9999999 m for exactly 1000 m.
		{"exactly one thousand steps up", symbol.SI("m"), "1000", "1", "km"},
		{"one below the step stays", symbol.SI("m"), "999.9", "999.9", "m"},
		{"trailing zeros are reduced", symbol.SI("m"), "250000.0000", "250", "km"},
		{"digits are preserved exactly", symbol.SI("m"),
			"1234.56789012345678901234567890", "1.2345678901234567890123456789", "km"},
		{"milli", symbol.SI("m"), "0.005", "5", "mm"},
		{"micro", symbol.SI("s"), "0.0000012", "1.2", "µs"},
		{"negative values keep their sign", symbol.SI("Pa"), "-250000", "-250", "kPa"},
		{"zero gets no prefix", symbol.SI("Pa"), "0", "0", "Pa"},
		{"below the smallest prefix clamps", symbol.SI("s"), "1E-40", "1E-10", "qs"},
		{"above the largest prefix clamps", symbol.SI("s"), "1E+40", "1E+10", "Qs"},

		// Squared and cubed symbols: one prefix step is 10⁶ resp. 10⁹.
		{"square metre below one step", symbol.SIPow("m", 2), "999999", "999999", "m²"},
		{"square metre at one step", symbol.SIPow("m", 2), "1000000", "1", "km²"},
		{"cubic metre at one step", symbol.SIPow("m", 3), "1000000000", "1", "km³"},
		{"square millimetre", symbol.SIPow("m", 2), "0.000004", "4", "mm²"},
		{"wavenumber scales the other way", symbol.SIPow("m", -1), "0.004", "4", "km⁻¹"},
		{"power zero takes no prefix", symbol.SIPow("m", 0), "1000", "1000", "m⁰"},

		// The kilogram: magnitudes are in kilograms, prefixes on the gram.
		{"one kilogram", symbol.Gram(), "1", "1", "kg"},
		{"one gram", symbol.Gram(), "0.001", "1", "g"},
		{"one milligram", symbol.Gram(), "0.000001", "1", "mg"},
		{"one tonne is a megagram", symbol.Gram(), "1000", "1", "Mg"},

		// The litre carries prefixes the SI otherwise avoids.
		{"one litre", symbol.Litre(), "1", "1", "L"},
		{"deci", symbol.Litre(), "0.5", "5", "dL"},
		{"centi", symbol.Litre(), "0.05", "5", "cL"},
		{"milli", symbol.Litre(), "0.005", "5", "mL"},
		{"hecto", symbol.Litre(), "500", "5", "hL"},
		{"micro clamps the bottom", symbol.Litre(), "0.0000000005", "0.0005", "µL"},

		// Compounds: the prefix attaches once, to the head.
		{"static never takes a prefix", symbol.Static("°C"), "1000", "1000", "°C"},
		{"empty product", symbol.Product(), "1000", "1000", ""},
		{"product prefixes its first factor only",
			symbol.Product(symbol.SI("N"), symbol.SI("m")), "1000", "1", "kN·m"},
		{"quotient prefixes its numerator",
			symbol.Quotient(symbol.SI("m"), symbol.SI("s")), "1000", "1", "km/s"},
		{"quotient with a product denominator", symbol.Quotient(
			symbol.SI("J"), symbol.Product(symbol.Gram(), symbol.SI("K"))),
			"1000", "1", "kJ/(kg·K)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, text := tc.s.Scale(decimal(t, tc.in))
			if got.String() != tc.wantValue || text != tc.wantText {
				t.Errorf("Scale(%s) = %s %s, want %s %s",
					tc.in, got.String(), text, tc.wantValue, tc.wantText)
			}
		})
	}
}

// Scaling must not change the quantity, only how it is written.
func TestScaleIsValuePreserving(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    symbol.Symbol
		in   string
		// factor is what the printed magnitude must be multiplied by to get
		// back to the input, expressed as a power of ten.
		exponent int32
	}{
		{"kilo", symbol.SI("m"), "1234", 3},
		{"micro", symbol.SI("s"), "0.0000012", -6},
		{"square kilometre", symbol.SIPow("m", 2), "2500000", 6},
		{"gram", symbol.Gram(), "0.001", -3},
		{"hectolitre", symbol.Litre(), "500", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := decimal(t, tc.in)
			scaled, _ := tc.s.Scale(in)
			back := new(apd.Decimal).Set(scaled)
			back.Exponent += tc.exponent
			if back.Cmp(in) != 0 {
				t.Errorf("scaled %s back by 1e%d gives %s, want %s",
					scaled, tc.exponent, back, in)
			}
		})
	}
}

// D3: the argument must be untouched, and the result must share nothing with
// it. 200 digits, because apd stores coefficients up to 38 digits inline and a
// shorter value would hide the aliasing.
func TestScaleDoesNotAliasItsArgument(t *testing.T) {
	digits := strings.Repeat("1234567890", 20)
	in := decimal(t, digits)
	before := in.String()

	scaled, _ := symbol.SI("m").Scale(in)
	if in.String() != before {
		t.Fatalf("Scale mutated its argument: %s became %s", before, in)
	}

	// Writing to the result must not reach the argument.
	scaled.Coeff.SetInt64(1)
	scaled.Exponent = 0
	if in.String() != before {
		t.Fatalf("result aliases the argument: %s became %s", before, in)
	}
}

// A non-finite magnitude has no order of magnitude and therefore no prefix. It
// still has to come back out with its symbol.
func TestScaleOfNonFiniteValues(t *testing.T) {
	for _, in := range []string{"Infinity", "-Infinity", "NaN"} {
		t.Run(in, func(t *testing.T) {
			got, text := symbol.SI("m").Scale(decimal(t, in))
			if text != "m" {
				t.Errorf("text = %q, want %q", text, "m")
			}
			if got.String() != decimal(t, in).String() {
				t.Errorf("value = %s, want %s", got, in)
			}
		})
	}
}
