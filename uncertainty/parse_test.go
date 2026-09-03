package uncertainty_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/uncertainty"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{"the bracket form", "[3.65, 3.75] mm", "[3.65, 3.75] mm"},
		{"without the space after the comma", "[3.65,3.75] mm", "[3.65, 3.75] mm"},
		{"without the space before the unit", "[3.65, 3.75]mm", "[3.65, 3.75] mm"},
		{"with spaces everywhere", "  [ 3.65 , 3.75 ]  mm  ", "[3.65, 3.75] mm"},
		{"a unit expression", "[1, 2] J/(kg·K)", "[1, 2] J/(kg·K)"},
		{"a power", "[1, 2] mm²/s", "[1, 2] mm²/s"},
		{"negative bounds", "[-2, 3] m", "[-2, 3] m"},
		{"a zero-width range", "[7, 7] m", "[7, 7] m"},
		{"exponents", "[1e-3, 2e-3] m", "[0.001, 0.002] m"},

		{"the plus-minus form", "3.7 ± 0.2 m", "[3.5, 3.9] m"},
		{"the ASCII plus-minus", "3.7 +/- 0.2 m", "[3.5, 3.9] m"},
		{"without spaces", "3.7±0.2m", "[3.5, 3.9] m"},
		{"on an affine scale, the tolerance is a span", "20 ± 0.5 °C", "[19.5, 20.5] °C"},

		{"the compact form", "3.7(2) m", "[3.5, 3.9] m"},
		{"the compact form with more digits", "12.345(12) m", "[12.333, 12.357] m"},
		{"the compact form on a whole number", "370(20) m", "[350, 390] m"},
		{"the compact form with an exponent", "1.5e3(5) m", "[1000, 2000] m"},
		{"the compact form on an affine scale", "20.0(5) °C", "[19.5, 20.5] °C"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := uncertainty.Parse(tc.text)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want error
	}{
		{"empty", "", metrology.ErrSyntax},
		{"a bare number", "3.7", metrology.ErrSyntax},
		{"a bare interval", "[1, 2]", metrology.ErrSyntax},
		{"no closing bracket", "[1, 2 m", metrology.ErrSyntax},
		{"one bound", "[1] m", metrology.ErrSyntax},
		{"no bounds at all", "[] m", metrology.ErrSyntax},
		{"an empty lower bound", "[, 2] m", metrology.ErrSyntax},
		{"an empty upper bound", "[1, ] m", metrology.ErrSyntax},
		{"a bound that is not a number", "[one, two] m", metrology.ErrSyntax},
		{"the bounds the wrong way round", "[2, 1] m", uncertainty.ErrReversed},
		{"an unknown unit", "[1, 2] widget", parse.ErrUnknownUnit},
		{"a measurement, not a range", "3.7 m", metrology.ErrSyntax},
		{"no tolerance after the sign", "3.7 ± m", metrology.ErrSyntax},
		{"no unit after the tolerance", "3.7 ± 0.2", metrology.ErrSyntax},
		{"no closing parenthesis", "3.7(2 m", metrology.ErrSyntax},
		{"an empty parenthesis", "3.7() m", metrology.ErrSyntax},
		{"a tolerance that is not digits", "3.7(a) m", metrology.ErrSyntax},
		{"a signed tolerance in parentheses", "3.7(-2) m", metrology.ErrSyntax},
		{"the compact form on a NaN", "NaN(2) m", metrology.ErrSyntax},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := uncertainty.Parse(tc.text)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%q gave %s, %v; want %v", tc.text, got, err, tc.want)
			}
		})
	}
}

// The unit part is not read here: it is handed to parse, which knows the
// prefixes, the superscripts, the solidus and the catalogue substitution that
// keeps the quantity tag of D6. A range of hertz is a frequency.
func TestParseKeepsTheQuantityTag(t *testing.T) {
	tagged, err := uncertainty.Parse("[49, 51] Hz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tagged.Quantity(); got != "frequency" {
		t.Errorf("quantity is %q, want frequency", got)
	}
	// The spelling is the statement (D12): s⁻¹ makes no claim, so it carries
	// none.
	untagged, err := uncertainty.Parse("[49, 51] s⁻¹")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := untagged.Quantity(); got != "" {
		t.Errorf("quantity is %q, want untagged", got)
	}
}

// An absolute scale that declares no interval unit has nothing for a tolerance
// to be read on, and the kind rules of D6 report it when the range is built:
// the sum of two points on a scale is not a point on it.
func TestParseAToleranceOnAScaleWithNoIntervalUnit(t *testing.T) {
	lonely := metrology.MustUnit(metrology.UnitDef{
		Symbol:    symbol.Static("°X"),
		Dimension: metrology.MustUnit(metrology.UnitDef{}).Dimension(),
		Kind:      metrology.Absolute,
		Offset:    "100",
	})
	mine := uncertainty.New(parse.New([]metrology.Unit{lonely}))

	// The bracket form needs no tolerance and reads fine.
	if _, err := mine.Range("[19.5, 20.5] °X"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := mine.Range("20 ± 0.5 °X"); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("got %v, want ErrKind", err)
	}
	if _, err := mine.Range("20.0(5) °X"); !errors.Is(err, metrology.ErrKind) {
		t.Errorf("got %v, want ErrKind", err)
	}
}

// A parser is a value holding its units (D7), so a program with units of its
// own reads them without a registry to write into.
func TestParserWithItsOwnCatalogue(t *testing.T) {
	widget := metrology.MustUnit(metrology.UnitDef{
		Symbol:    symbol.Static("widget"),
		Dimension: metrology.MustUnit(metrology.UnitDef{}).Dimension(),
	})
	mine := uncertainty.New(parse.New([]metrology.Unit{widget}))

	got, err := mine.Range("[1, 2] widget")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[1, 2] widget"; got.String() != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := mine.Range("[1, 2] m"); !errors.Is(err, parse.ErrUnknownUnit) {
		t.Errorf("a catalogue of one unit read the metre: %v", err)
	}

	// The zero Parser reads the shipped catalogue.
	if _, err := (uncertainty.Parser{}).Range("[1, 2] m"); err != nil {
		t.Errorf("the zero parser did not read the shipped catalogue: %v", err)
	}
}
