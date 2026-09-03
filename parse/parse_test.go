package parse_test

import (
	"errors"
	"testing"
	"time"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/energy"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/mass"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
	"github.com/timzifer/metrology/units/volume"
)

// TestMeasurement reads the forms a program actually meets: what the library
// prints, what a data sheet writes, and what a configuration file holds.
func TestMeasurement(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want string // the canonical form of what was read
	}{
		{"the canonical form", "2.5 bar", "2.5 bar"},
		{"a negative magnitude on an affine scale", "-40 °C", "-40 °C"},
		{"every digit survives", "2.500000000000000000000000000001 bar", "2.500000000000000000000000000001 bar"},
		{"an exponent", "1e3 Pa", "1000 Pa"},
		{"a prefixed symbol", "250 kPa", "250 kPa"},
		{"a prefix below the unit", "1000 mg", "1000 mg"},
		{"the gram, which is the prefixed spelling of the kilogram", "1 g", "1 g"},
		{"a litre prefix labels use", "0.5 dL", "0.5 dL"},
		{"no space is needed", "2.5bar", "2.5 bar"},
		{"nor before a degree sign", "-40°C", "-40 °C"},
		{"an e that is a unit and not an exponent", "1eV", "1 eV"},
		{"a unit that spells itself with a solidus", "3 m³/h", "3 m³/h"},
		{"a computed unit", "50 N/m²", "50 N/m²"},
		{"a product", "1 N·m", "1 N·m"},
		{"an asterisk for the middle dot", "1 N*m", "1 N·m"},
		{"a bracketed denominator", "1 J/(kg·K)", "1 J/(kg·K)"},
		{"a bracketed denominator that is itself a quotient", "1 m/(s/A)", "1 m/(s/A)"},
		{"an ASCII exponent", "1 m^3", "1 m³"},
		{"a negative exponent", "1 s⁻¹", "1 s⁻¹"},
		{"an ASCII negative exponent", "1 s^-1", "1 s⁻¹"},
		{"a bracketed expression raised to a power", "1 (m/s)²", "1 (m/s)²"},
		{"blanks around the operators", "1 N · m", "1 N·m"},
		{"the dimensionless unit is written 1", "2.5 1", "2.5 1"},
		{"surrounding space is ignored", "  2.5 bar  ", "2.5 bar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := parse.Measurement(tc.text)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.text, err)
			}
			if got := m.String(); got != tc.want {
				t.Errorf("parse %q = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// A parsed unit is the catalogue's own unit wherever the catalogue has one, so
// that what comes out of a configuration file is what a program compiled
// against.
func TestMeasurementResolvesCatalogueUnits(t *testing.T) {
	for _, tc := range []struct {
		text string
		unit metrology.Unit
	}{
		{"2.5 bar", pressure.Bar},
		{"1 km", length.Kilometre}, // the catalogue entry, not the prefixed metre
		{"1 kg", mass.Kilogram},
		{"1 L", volume.Litre},
		{"1 h", duration.Hour},
		{"1 kWh", energy.KilowattHour},
		{"20 °C", temperature.Celsius},
	} {
		m, err := parse.Measurement(tc.text)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.text, err)
		}
		if !m.Unit().Equal(tc.unit) {
			t.Errorf("parse %q gave unit %s, want %s", tc.text, m.Unit(), tc.unit)
		}
	}
}

// A prefix multiplies the unit exactly, and the value it stands for is what the
// conversion produces — no rounding anywhere in the path.
func TestMeasurementPrefixesAreExact(t *testing.T) {
	for _, tc := range []struct {
		text string
		in   metrology.Unit
		want string
	}{
		{"250 kPa", pressure.Pascal, "250000 Pa"},
		{"1 mm", length.Metre, "0.001 m"},
		{"1 µm", length.Metre, "0.000001 m"},
		{"1 Mg", mass.Kilogram, "1000 kg"},
		{"1 mg", mass.Kilogram, "0.000001 kg"},
		{"1 mL", volume.CubicMetre, "0.000001 m³"},
	} {
		m, err := parse.Measurement(tc.text)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.text, err)
		}
		converted, err := m.To(tc.in)
		if err != nil {
			t.Fatalf("convert %q: %v", tc.text, err)
		}
		if got := converted.String(); got != tc.want {
			t.Errorf("%q in %s = %q, want %q", tc.text, tc.in, got, tc.want)
		}
	}
}

// One prefix step on a squared symbol is a factor of a million, because a
// square kilometre is 10⁶ square metres.
func TestMeasurementPrefixedPower(t *testing.T) {
	m, err := parse.Measurement("1 km²")
	if err != nil {
		t.Fatal(err)
	}
	squareMetre, err := parse.Unit("m²")
	if err != nil {
		t.Fatal(err)
	}
	converted, err := m.To(squareMetre)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := converted.String(), "1000000 m²"; got != want {
		t.Errorf("1 km² = %q, want %q", got, want)
	}
}

func TestUnit(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
	}{
		{"Pa", "Pa"},
		{"kPa", "kPa"},
		{"m/s", "m/s"},
		{"N·m", "N·m"},
		{"m³", "m³"},
		{"kg·m/s²", "kg·m/s²"},
		{" m/s ", "m/s"},
	} {
		u, err := parse.Unit(tc.text)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.text, err)
		}
		if got := u.String(); got != tc.want {
			t.Errorf("parse %q = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// An expression means what the operators say: · and / bind equally and from
// the left, which is what the bracketing of the renderer assumes.
func TestUnitExpressionDimensions(t *testing.T) {
	for _, tc := range []struct {
		text string
		want dimension.Dimension
	}{
		{"kg·m/s²", dimension.New(dimension.Exponents{Mass: 1, Length: 1, Time: -2})},
		{"J/(kg·K)", dimension.New(dimension.Exponents{Length: 2, Time: -2, Temperature: -1})},
		{"m/s/A", dimension.New(dimension.Exponents{Length: 1, Time: -1, ElectricCurrent: -1})},
		{"m/(s/A)", dimension.New(dimension.Exponents{Length: 1, Time: -1, ElectricCurrent: 1})},
		{"m·s⁻¹", dimension.New(dimension.Exponents{Length: 1, Time: -1})},
		{"(m/s)²", dimension.New(dimension.Exponents{Length: 2, Time: -2})},
		{"m⁰", dimension.One},
	} {
		u, err := parse.Unit(tc.text)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.text, err)
		}
		if got := u.Dimension(); got != tc.want {
			t.Errorf("parse %q has dimension %s, want %s", tc.text, got, tc.want)
		}
	}
}

// An expression carries no quantity tag, exactly as a computed magnitude does
// (D6) — so a result computed into m²/s can be named, and one read from a
// symbol the catalogue knows arrives tagged.
func TestQuantityTags(t *testing.T) {
	tagged, err := parse.Measurement("5 m²/s")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tagged.Quantity(), metrology.Quantity("kinematic viscosity"); got != want {
		t.Errorf("quantity of %q = %q, want %q", "5 m²/s", got, want)
	}
	computed, err := parse.Measurement("5 m·m/s")
	if err != nil {
		t.Fatal(err)
	}
	if got := computed.Quantity(); got != "" {
		t.Errorf("quantity of an expression = %q, want the empty one", got)
	}
	// Untagged converts into the tagged unit; that is what the empty tag is for.
	if _, err := computed.To(tagged.Unit()); err != nil {
		t.Errorf("naming a computed magnitude: %v", err)
	}
}

// "K" is a temperature and a temperature difference, and the text does not say
// which. The parser does.
func TestPreferKind(t *testing.T) {
	span, err := parse.Measurement("5 K")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := span.Kind(), metrology.Interval; got != want {
		t.Fatalf("kind of %q = %s, want %s", "5 K", got, want)
	}
	if !span.Unit().Equal(interval.Kelvin) {
		t.Errorf("%q gave %s, want the interval kelvin", "5 K", span.Unit())
	}
	// The reading that composes: 20 °C plus 5 K is 25 °C (D6).
	sum, err := temperature.Celsius.Of(20).Add(span)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sum.String(), "25 °C"; got != want {
		t.Errorf("20 °C + %q = %q, want %q", "5 K", got, want)
	}

	point, err := parse.Default().Prefer(metrology.Absolute).Measurement("5 K")
	if err != nil {
		t.Fatal(err)
	}
	if !point.Unit().Equal(temperature.Kelvin) {
		t.Errorf("preferring absolute gave %s, want the thermodynamic kelvin", point.Unit())
	}
	// A symbol with one reading is unaffected by the preference.
	celsius, err := parse.Default().Prefer(metrology.Interval).Measurement("20 °C")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := celsius.Kind(), metrology.Absolute; got != want {
		t.Errorf("kind of %q = %s, want %s", "20 °C", got, want)
	}
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		text   string
		target error
	}{
		{"no magnitude", "bar", metrology.ErrSyntax},
		{"empty", "", metrology.ErrSyntax},
		{"blank", "   ", metrology.ErrSyntax},
		{"a bare number is not a measurement", "2.5", metrology.ErrSyntax},
		{"two decimal points", "1.2.3 m", metrology.ErrSyntax},
		{"an unknown symbol", "2.5 zzz", parse.ErrUnknownUnit},
		{"an unknown symbol inside an expression", "2.5 N/zzz", parse.ErrUnknownUnit},
		{"a prefix on a symbol that takes none", "2.5 mbar", parse.ErrUnknownUnit},
		{"an unclosed bracket", "1 N/(m", metrology.ErrSyntax},
		{"an unopened bracket", "1 N/m)", metrology.ErrSyntax},
		{"an operator without a right operand", "1 N·", metrology.ErrSyntax},
		{"an operator without a left operand", "1 ·N", metrology.ErrSyntax},
		{"a lone superscript minus", "1 m⁻", metrology.ErrSyntax},
		{"an ASCII exponent without digits", "1 m^", metrology.ErrSyntax},
		{"an ASCII exponent that is only a sign", "1 m^-", metrology.ErrSyntax},
		{"an exponent beyond a dimension", "1 m^999", metrology.ErrRange},
		{"an exponent longer than any number", "1 m^99999999999999999999", metrology.ErrSyntax},
		{"a point on a scale in a product", "1 °C·m", metrology.ErrKind},
		{"a point on a scale in a quotient", "1 °C/s", metrology.ErrKind},
		{"a point on a scale raised to a power", "1 °C²", metrology.ErrKind},
		// The range a magnitude may have belongs to the core: apd holds a
		// decimal exponent up to ±100000 and reports anything beyond it.
		{"a magnitude beyond the exponent range", "1e200000 m", metrology.ErrSyntax},
		{"a magnitude below it", "1e-200000 m", metrology.ErrSyntax},
		{"an empty bracket", "1 ()", metrology.ErrSyntax},
		// Found by the fuzzer: a broken byte used to walk the lexer off the
		// end of its own input, because the replacement rune is three bytes
		// long and the byte it stands for is one.
		{"invalid UTF-8 in the symbol", "0\xe7", parse.ErrUnknownUnit},
		{"invalid UTF-8 after a symbol", "1 m\xff", parse.ErrUnknownUnit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := parse.Measurement(tc.text)
			if err == nil {
				t.Fatalf("parse %q = %q, want an error", tc.text, m)
			}
			if !errors.Is(err, tc.target) {
				t.Errorf("parse %q: error = %v, want one matching %v", tc.text, err, tc.target)
			}
		})
	}
}

// The unknown-unit error names the symbol, because that is what the reader has
// to go and add — and it is not a syntax error, because the text is fine.
func TestUnknownUnitError(t *testing.T) {
	_, err := parse.Measurement("2.5 zzz")
	var ue *parse.UnknownUnitError
	if !errors.As(err, &ue) {
		t.Fatalf("error = %v, want an UnknownUnitError", err)
	}
	if ue.Symbol != "zzz" {
		t.Errorf("Symbol = %q, want %q", ue.Symbol, "zzz")
	}
	if errors.Is(err, metrology.ErrSyntax) {
		t.Error("an unknown unit matches ErrSyntax, and should not")
	}
	if got, want := err.Error(), `metrology: parse: "zzz": "zzz" is not a unit this parser knows`; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestUnitErrors(t *testing.T) {
	for _, text := range []string{"", "  ", "zzz", "N/(m"} {
		if u, err := parse.Unit(text); err == nil {
			t.Errorf("parse.Unit(%q) = %s, want an error", text, u)
		}
	}
}

// The zero Parser knows nothing, which is the honest answer for a parser nobody
// gave any units to.
func TestZeroParser(t *testing.T) {
	var p parse.Parser
	if _, err := p.Measurement("2.5 bar"); !errors.Is(err, parse.ErrUnknownUnit) {
		t.Errorf("error = %v, want ErrUnknownUnit", err)
	}
}

// A program with its own units parses them with the same code as the shipped
// catalogue, which is the reason a parser is a value and not a registry (D7).
func TestCustomUnits(t *testing.T) {
	widget := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.SI("wdg"),
	})
	p := parse.New([]metrology.Unit{widget})

	m, err := p.Measurement("2.5 kwdg")
	if err != nil {
		t.Fatal(err)
	}
	converted, err := m.To(widget)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := converted.String(), "2500 wdg"; got != want {
		t.Errorf("2.5 kwdg = %q, want %q", got, want)
	}
	// The shipped catalogue is not in this parser, and says so.
	if _, err := p.Measurement("2.5 bar"); !errors.Is(err, parse.ErrUnknownUnit) {
		t.Errorf("error = %v, want ErrUnknownUnit", err)
	}
}

// A prefix on an affine scale moves the offset the other way, or 5 m°C would
// come out 273 kelvin too low. No unit in the catalogue is affine and
// prefixable at once — but nothing stops a caller's from being.
func TestPrefixOnAnAffineScale(t *testing.T) {
	kelvin := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Symbol: symbol.SI("K"),
	})
	celsius := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.SI("°C"),
		Offset: "273.15", Interval: &kelvin,
	})
	kelvinPoint := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.Θ, Kind: metrology.Absolute, Symbol: symbol.SI("K"),
		Interval: &kelvin,
	})
	p := parse.New([]metrology.Unit{kelvin, celsius, kelvinPoint}).Prefer(metrology.Absolute)

	m, err := p.Measurement("5 m°C")
	if err != nil {
		t.Fatal(err)
	}
	converted, err := m.To(kelvinPoint)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := converted.String(), "273.155 K"; got != want {
		t.Errorf("5 m°C = %q, want %q", got, want)
	}
	// The interval scale travels with the unit: a difference of two points is
	// a span, and the span is read in kelvin (D6).
	other, err := p.Measurement("1005 m°C")
	if err != nil {
		t.Fatal(err)
	}
	span, err := other.Sub(m)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := span.String(), "1 K"; got != want {
		t.Errorf("1005 m°C − 5 m°C = %q, want %q", got, want)
	}
}

// A parser that prefers points on a scale still reads every unit that has only
// the one reading: preferring absolute does not make the bar a temperature.
func TestPreferFallsBackToTheOtherReading(t *testing.T) {
	m, err := parse.Default().Prefer(metrology.Absolute).Measurement("2.5 bar")
	if err != nil {
		t.Fatal(err)
	}
	if !m.Unit().Equal(pressure.Bar) {
		t.Errorf("read %s, want the bar", m.Unit())
	}
	if got, want := m.Kind(), metrology.Interval; got != want {
		t.Errorf("kind = %s, want %s", got, want)
	}
}

// A power of a power is computed only once its size is known to be one a
// dimension can hold: "(Qm^127)^127" is a factor of half a million digits and
// one bracket more is sixty million, all written in fourteen characters. Found
// by the fuzzer, which stopped making progress on exactly this.
func TestNestedPowersAreBounded(t *testing.T) {
	for _, text := range []string{"1 (Qm^127)^127", "1 ((m^9)^9)^9", "1 (m²)^127"} {
		start := time.Now()
		_, err := parse.Measurement(text)
		if !errors.Is(err, metrology.ErrRange) {
			t.Errorf("parse %q: error = %v, want ErrRange", text, err)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("parse %q took %v, which means it computed the factor first", text, elapsed)
		}
	}
	// What stays inside the range is still read, brackets and all.
	u, err := parse.Unit("((m²)³)⁴")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := u.Dimension(), dimension.L.Pow(24); got != want {
		t.Errorf("dimension = %s, want %s", got, want)
	}
}

// An expression that spells a unit the parser knows is that unit, whitespace or
// no whitespace — otherwise a stray blank would decide whether a magnitude
// carries the quantity tag of D6.
func TestExpressionResolvesToTheNamedUnit(t *testing.T) {
	spaced, err := parse.Measurement("5 m²/ s")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := parse.Measurement("5 m²/s")
	if err != nil {
		t.Fatal(err)
	}
	if !spaced.Unit().Equal(plain.Unit()) {
		t.Errorf("%q read as %s, %q as %s", "5 m²/ s", spaced.Unit(), "5 m²/s", plain.Unit())
	}
	if got, want := spaced.Quantity(), metrology.Quantity("kinematic viscosity"); got != want {
		t.Errorf("quantity = %q, want %q", got, want)
	}
	// A caller's own unit that merely spells itself like a computed one is not
	// substituted for it: the scales have to agree, not just the symbols.
	odd := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.New(dimension.Exponents{Length: 1, Time: -1}),
		Symbol:    symbol.Static("m/s"), Numerator: "5",
	})
	metre := metrology.MustUnit(metrology.UnitDef{Dimension: dimension.L, Symbol: symbol.SI("m")})
	second := metrology.MustUnit(metrology.UnitDef{Dimension: dimension.T, Symbol: symbol.SI("s")})
	p := parse.New([]metrology.Unit{odd, metre, second})

	u, err := p.Unit("m/ s")
	if err != nil {
		t.Fatal(err)
	}
	if u.Equal(odd) {
		t.Error("a unit with the same symbol and a different factor was substituted")
	}
}

// The same guard on the other two ways a symbol can lie about its scale: a unit
// that spells itself like a computed one but measures something else, and one
// that measures a point on a scale rather than a span along it.
func TestExpressionIsNotSubstitutedAcrossScales(t *testing.T) {
	metre := metrology.MustUnit(metrology.UnitDef{Dimension: dimension.L, Symbol: symbol.SI("m")})
	second := metrology.MustUnit(metrology.UnitDef{Dimension: dimension.T, Symbol: symbol.SI("s")})
	for _, tc := range []struct {
		name string
		odd  metrology.Unit
	}{
		{"a different dimension", metrology.MustUnit(metrology.UnitDef{
			Dimension: dimension.L, Symbol: symbol.Static("m/s"),
		})},
		{"a point on a scale", metrology.MustUnit(metrology.UnitDef{
			Dimension: dimension.New(dimension.Exponents{Length: 1, Time: -1}),
			Kind:      metrology.Absolute, Symbol: symbol.Static("m/s"),
		})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := parse.New([]metrology.Unit{tc.odd, metre, second})
			u, err := p.Unit("m/ s")
			if err != nil {
				t.Fatal(err)
			}
			if u.Equal(tc.odd) {
				t.Errorf("%s was substituted for the computed m/s", tc.odd)
			}
			want := dimension.New(dimension.Exponents{Length: 1, Time: -1})
			if got := u.Dimension(); got != want {
				t.Errorf("dimension = %s, want %s", got, want)
			}
		})
	}
}
