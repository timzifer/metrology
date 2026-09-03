package uncertainty_test

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/uncertainty"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
)

// The writing half is on the value and the reading half is on a parser (D12),
// and these are the interfaces that split says each side implements.
var (
	_ encoding.TextMarshaler   = uncertainty.Range{}
	_ json.Marshaler           = uncertainty.Range{}
	_ driver.Valuer            = uncertainty.Range{}
	_ encoding.TextUnmarshaler = (*uncertainty.Text)(nil)
	_ json.Unmarshaler         = (*uncertainty.Text)(nil)
	_ sql.Scanner              = (*uncertainty.Text)(nil)
	_ encoding.TextMarshaler   = uncertainty.Text{}
)

func TestString(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    uncertainty.Range
		want string
	}{
		{"a span", span(t, length.Metre, "1", "2"), "[1, 2] m"},
		{"a point", uncertainty.Of(length.Metre.Of(1)), "[1, 1] m"},
		{"an absolute range", span(t, temperature.Celsius, "19.5", "20.5"), "[19.5, 20.5] °C"},
		{"across zero", span(t, length.Metre, "-2", "3"), "[-2, 3] m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.String(); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
			text, err := tc.r.MarshalText()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(text) != tc.want {
				t.Errorf("MarshalText gave %s, want %s", text, tc.want)
			}
			value, err := tc.r.Value()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != driver.Value(tc.want) {
				t.Errorf("Value gave %v, want %s", value, tc.want)
			}
		})
	}
}

// Both magnitudes keep every digit they were given: the text form is the
// exchange format, and an exchange format that rounds is not one.
func TestStringIsExact(t *testing.T) {
	const lo = "1.234567890123456789012345678901"
	const hi = "9.876543210987654321098765432109"
	if got, want := span(t, length.Metre, lo, hi).String(), "["+lo+", "+hi+"] m"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMarshalJSON(t *testing.T) {
	data, err := json.Marshal(struct {
		P uncertainty.Range `json:"p"`
	}{span(t, pressure.Bar, "2.5", "2.6")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `{"p":"[2.5, 2.6] bar"}`; string(data) != want {
		t.Errorf("got %s, want %s", data, want)
	}
}

func TestPlusMinus(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    uncertainty.Range
		want string
	}{
		{"a span", span(t, length.Metre, "3.5", "3.9"), "3.7 ± 0.2 m"},
		{"a point has a zero tolerance", span(t, length.Metre, "7", "7"), "7 ± 0 m"},
		{"across zero", span(t, length.Metre, "-2", "3"), "0.5 ± 2.5 m"},
		{"an absolute range", span(t, temperature.Celsius, "19.5", "20.5"), "20 ± 0.5 °C"},
		{"an asymmetric-looking one still has a centre", span(t, length.Metre, "1", "4"), "2.5 ± 1.5 m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.r.PlusMinus()
			if !ok {
				t.Fatalf("no ± form for %s", tc.r)
			}
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// The midpoint and the half-width need one digit more than the bounds do, so a
// range whose bounds already fill the engine's precision has no ± form that
// reads back as itself — and the second result says so rather than the first
// one rounding.
func TestPlusMinusRefusesToRound(t *testing.T) {
	full := span(t, length.Metre, "1.0000000000000000001", "2.0000000000000000002")
	if got, ok := full.PlusMinus(); ok {
		t.Errorf("got %q, want no ± form: the midpoint needs 21 digits", got)
	}

	// The same range at an engine that can hold the midpoint does have one.
	got, ok := uncertainty.NewEngine(30).PlusMinus(full)
	if !ok {
		t.Fatal("thirty digits are enough for the midpoint and the engine said no")
	}
	if want := "1.50000000000000000015 ± 0.50000000000000000005 m"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// The ± form reads back as the range it was written from, which is what makes
// it a rendering rather than a second exchange format.
func TestPlusMinusReadsBack(t *testing.T) {
	for _, r := range []uncertainty.Range{
		span(t, length.Metre, "3.5", "3.9"),
		span(t, length.Metre, "-2", "3"),
		span(t, temperature.Celsius, "19.5", "20.5"),
		span(t, temperature.Fahrenheit, "67.1", "68.9"),
	} {
		text, ok := r.PlusMinus()
		if !ok {
			t.Fatalf("no ± form for %s", r)
		}
		again, err := uncertainty.Parse(text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		// By value, not by digits: 0.5 − 2.5 is −2.0, and the core keeps the
		// digits a subtraction produces because addition never rounds (D9).
		if !again.Lo().Equal(r.Lo()) || !again.Hi().Equal(r.Hi()) {
			t.Errorf("%s wrote %q and read back as %s", r, text, again)
		}
		if !again.Unit().Equal(r.Unit()) {
			t.Errorf("%q read back on %s, want %s", text, again.Unit(), r.Unit())
		}
	}
}

// The canonical form is a fixed point: written, read and written again, the
// text is the same and so is the range.
func TestRoundTrip(t *testing.T) {
	for _, r := range []uncertainty.Range{
		span(t, length.Metre, "1", "2"),
		span(t, pressure.Torr, "759.5", "760.5"),
		span(t, temperature.Celsius, "-40.25", "-39.75"),
		uncertainty.Of(length.Metre.Of(0)),
		span(t, length.Metre, "1.0000000000000000001", "2.0000000000000000002"),
	} {
		text := r.String()
		again, err := uncertainty.Parse(text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if got := again.String(); got != text {
			t.Errorf("%q read back as %q", text, got)
		}
		if !again.Unit().Equal(r.Unit()) {
			t.Errorf("%q read back on %s, want %s", text, again.Unit(), r.Unit())
		}
	}
}

func TestTextUnmarshalJSON(t *testing.T) {
	var row struct {
		P uncertainty.Text `json:"p"`
		Q uncertainty.Text `json:"q"`
	}
	if err := json.Unmarshal([]byte(`{"p":"[2.5, 2.6] bar","q":null}`), &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := row.P.String(), "[2.5, 2.6] bar"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got := row.Q.String(); got != "[0, 0] " {
		t.Errorf("a null gave %q, want the zero range", got)
	}
}

// A bare number is a point on the unit the schema fixes, and an error where no
// unit was fixed: inventing one for a number is how a pressure in bar becomes a
// pressure in pascal.
func TestTextIn(t *testing.T) {
	field := uncertainty.Default().Text().In(pressure.Bar)
	if err := json.Unmarshal([]byte(`2.5`), &field); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := field.String(), "[2.5, 2.5] bar"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	var without uncertainty.Text
	if err := json.Unmarshal([]byte(`2.5`), &without); err == nil {
		t.Error("a bare number was read without a unit")
	}
}

func TestTextScan(t *testing.T) {
	for _, src := range []any{"[1, 2] m", []byte("[1, 2] m")} {
		var field uncertainty.Text
		if err := field.Scan(src); err != nil {
			t.Fatalf("%T: %v", src, err)
		}
		if got, want := field.String(), "[1, 2] m"; got != want {
			t.Errorf("%T gave %s, want %s", src, got, want)
		}
	}

	var field uncertainty.Text
	if err := field.Scan(42); err == nil {
		t.Error("an integer scanned as a range")
	}
	if err := field.Scan("not a range"); err == nil {
		t.Error("nonsense scanned as a range")
	}
	if err := json.Unmarshal([]byte(`"not a range"`), &field); err == nil {
		t.Error("nonsense unmarshalled as a range")
	}
	if err := json.Unmarshal([]byte(`{`), &field); err == nil {
		t.Error("broken JSON unmarshalled as a range")
	}
}

// A nullable column decodes without a special case on the caller's side.
func TestTextNullable(t *testing.T) {
	var value sql.Null[uncertainty.Text]
	if err := value.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value.Valid {
		t.Error("a NULL scanned as a value")
	}
	if err := value.Scan("[1, 2] m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := value.V.String(), "[1, 2] m"; !value.Valid || got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// A Text built over a catalogue of its own reads that catalogue's units, which
// is the whole reason the parser travels in the destination (D7). The zero
// Text reads the shipped catalogue, which is why it is usable at all.
func TestTextCarriesItsCatalogue(t *testing.T) {
	widget := metrology.MustUnit(metrology.UnitDef{
		Dimension: length.Metre.Dimension(),
		Symbol:    symbol.Static("widget"),
		Numerator: "3",
	})
	mine := uncertainty.New(parse.New([]metrology.Unit{widget})).Text()

	if err := mine.UnmarshalText([]byte("[1, 2] widget")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := mine.String(), "[1, 2] widget"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	// And it reads only its own: the shipped metre is not in this catalogue.
	if err := mine.UnmarshalText([]byte("[1, 2] m")); err == nil {
		t.Error("a catalogue of one unit read a unit it does not have")
	}

	var zero uncertainty.Text
	if err := zero.UnmarshalText([]byte("[1, 2] m")); err != nil {
		t.Fatalf("the zero Text should read the shipped catalogue: %v", err)
	}
	if err := zero.UnmarshalText([]byte("[1, 2] widget")); err == nil {
		t.Error("the shipped catalogue read a unit of somebody else's")
	}
}

// A JSON string that is not one reaches the unmarshaller only when it is called
// directly, which a decoder of a nested value does.
func TestTextUnmarshalJSONBrokenString(t *testing.T) {
	var field uncertainty.Text
	if err := field.UnmarshalJSON([]byte(`"unterminated`)); err == nil {
		t.Error("an unterminated JSON string unmarshalled as a range")
	}
}
