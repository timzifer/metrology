package parse_test

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/pressure"
	"github.com/timzifer/metrology/symbol"
)

// The interfaces the standard library decodes through. A Text implements the
// reading half; the writing half is the measurement it embeds.
var (
	_ encoding.TextUnmarshaler = (*parse.Text)(nil)
	_ json.Unmarshaler         = (*parse.Text)(nil)
	_ sql.Scanner              = (*parse.Text)(nil)
	_ encoding.TextMarshaler   = parse.Text{}
	_ json.Marshaler           = parse.Text{}
	_ driver.Valuer            = parse.Text{}
)

func TestTextUnmarshalText(t *testing.T) {
	var got parse.Text
	if err := got.UnmarshalText([]byte("2.5 bar")); err != nil {
		t.Fatal(err)
	}
	if !got.Equal(pressure.Bar.Of(2.5)) {
		t.Errorf("read %q, want 2.5 bar", got.Measurement)
	}
	if err := got.UnmarshalText([]byte("2.5 zzz")); !errors.Is(err, parse.ErrUnknownUnit) {
		t.Errorf("error = %v, want ErrUnknownUnit", err)
	}
}

// A Text writes what a measurement writes: the round trip through a struct is
// the whole point of the type.
func TestTextJSONRoundTrip(t *testing.T) {
	type config struct {
		Setpoint   parse.Text `json:"setpoint"`
		Hysteresis parse.Text `json:"hysteresis"`
	}
	var cfg config
	const in = `{"setpoint":"20 °C","hysteresis":"2 K"}`
	if err := json.Unmarshal([]byte(in), &cfg); err != nil {
		t.Fatal(err)
	}
	// 20 °C plus 2 K is 25 °C — the two readings compose because the parser
	// reads a bare "K" as a span (D6).
	sum, err := cfg.Setpoint.Add(cfg.Hysteresis.Measurement)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sum.String(), "22 °C"; got != want {
		t.Errorf("setpoint + hysteresis = %q, want %q", got, want)
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != in {
		t.Errorf("round trip = %s, want %s", got, in)
	}
}

func TestTextUnmarshalJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{"the canonical string form", `"2.5 bar"`, "2.5 bar"},
		{"an object with a text magnitude", `{"value":"2.5","unit":"bar"}`, "2.5 bar"},
		{"an object with a JSON number", `{"value":2.5,"unit":"bar"}`, "2.5 bar"},
		{"an object keeps digits a float64 would lose", `{"value":2.500000000000000000001,"unit":"bar"}`, "2.500000000000000000001 bar"},
		{"an object with a whole measurement in its value", `{"value":"2.5 bar"}`, "2.5 bar"},
		{"whitespace around a string", ` "2.5 bar" `, "2.5 bar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got parse.Text
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatal(err)
			}
			if s := got.String(); s != tc.want {
				t.Errorf("read %s as %q, want %q", tc.json, s, tc.want)
			}
		})
	}
}

// A null leaves the destination as it was, which is what json.Unmarshaler
// requires of every implementation.
func TestTextUnmarshalJSONNull(t *testing.T) {
	got := parse.Text{Measurement: pressure.Bar.Of(1)}
	if err := json.Unmarshal([]byte("null"), &got); err != nil {
		t.Fatal(err)
	}
	if s := got.String(); s != "1 bar" {
		t.Errorf("null overwrote the measurement with %q", s)
	}
}

func TestTextUnmarshalJSONErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
	}{
		{"a bare number without a unit", `2.5`},
		{"a string that is not a measurement", `"2.5 zzz"`},
		{"a broken string", `"2.5`},
		{"a broken object", `{"value":`},
		{"an object with a magnitude that is not a number", `{"value":"two","unit":"bar"}`},
		{"an object with an unknown unit", `{"value":"2.5","unit":"zzz"}`},
		{"an object with a value of the wrong shape", `{"value":{},"unit":"bar"}`},
		{"a JSON array", `[]`},
		{"a JSON boolean", `true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got parse.Text
			if err := json.Unmarshal([]byte(tc.json), &got); err == nil {
				t.Errorf("read %s as %q, want an error", tc.json, got.String())
			}
		})
	}
}

// The two-column layout: the magnitude in a NUMERIC column, the unit in the
// schema. The unit is given to the destination, since the interfaces carry
// nothing else.
func TestTextIn(t *testing.T) {
	for _, src := range []any{
		"2.5",         // a driver that hands over the column as text
		[]byte("2.5"), // and one that hands over bytes
		float64(2.5),  // and one that has already made a float of it
		"2.5 bar",     // text that carries its unit is still read from it
	} {
		got := parse.Default().Text().In(pressure.Bar)
		if err := got.Scan(src); err != nil {
			t.Fatalf("Scan(%#v): %v", src, err)
		}
		if !got.Equal(pressure.Bar.Of(2.5)) {
			t.Errorf("Scan(%#v) = %q, want 2.5 bar", src, got.Measurement)
		}
	}
	// An integer column too, exactly.
	got := parse.Text{}.In(pressure.Bar)
	if err := got.Scan(int64(3)); err != nil {
		t.Fatal(err)
	}
	if s := got.String(); s != "3 bar" {
		t.Errorf("Scan(int64(3)) = %q, want %q", s, "3 bar")
	}
	// A JSON number needs the same fixed unit, and gets it.
	var field = parse.Text{}.In(pressure.Bar)
	if err := json.Unmarshal([]byte("2.5"), &field); err != nil {
		t.Fatal(err)
	}
	if s := field.String(); s != "2.5 bar" {
		t.Errorf("2.5 on a fixed unit = %q, want %q", s, "2.5 bar")
	}
}

func TestTextScan(t *testing.T) {
	var got parse.Text
	if err := got.Scan("2.5 bar"); err != nil {
		t.Fatal(err)
	}
	if !got.Equal(pressure.Bar.Of(2.5)) {
		t.Errorf("Scan = %q, want 2.5 bar", got.Measurement)
	}
	if err := got.Scan([]byte("250 kPa")); err != nil {
		t.Fatal(err)
	}
	if s := got.String(); s != "250 kPa" {
		t.Errorf("Scan = %q, want %q", s, "250 kPa")
	}
	// Written back, a Text is the text form again.
	v, err := got.Value()
	if err != nil {
		t.Fatal(err)
	}
	if v != driver.Value("250 kPa") {
		t.Errorf("Value = %v, want %q", v, "250 kPa")
	}
}

func TestTextScanErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		src    any
		target error
	}{
		{"a NULL is not a zero measurement", nil, parse.ErrNotText},
		{"nor is a boolean column", true, parse.ErrNotText},
		{"a number without a unit is not one either", float64(2.5), metrology.ErrSyntax},
		{"and neither is an integer", int64(2), metrology.ErrSyntax},
		{"a NaN is no magnitude at all", nan(), parse.ErrNotText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got parse.Text
			err := got.Scan(tc.src)
			if !errors.Is(err, tc.target) {
				t.Errorf("Scan(%#v) error = %v, want one matching %v", tc.src, err, tc.target)
			}
		})
	}
}

// nan is the one float64 a driver can hand over that names no magnitude.
func nan() float64 { return math.NaN() }

// A nullable column is read into an sql.Null, which is what the standard
// library offers for exactly this: the zero measurement is a dimensionless
// zero, not "no measurement".
func TestTextNullable(t *testing.T) {
	var got sql.Null[parse.Text]
	if err := got.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if got.Valid {
		t.Errorf("a NULL scanned as valid: %v", got.V)
	}
	if err := got.Scan("2.5 bar"); err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.V.String() != "2.5 bar" {
		t.Errorf("Scan = %v, %v, want 2.5 bar", got.V, got.Valid)
	}
}

// A program with its own units decodes with its own parser: the reader travels
// in the destination, because the interfaces carry no context (D7).
func TestTextReader(t *testing.T) {
	widget := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.SI("wdg"),
	})
	row := struct {
		Count parse.Text `json:"count"`
	}{Count: parse.New([]metrology.Unit{widget}).Text()}

	if err := json.Unmarshal([]byte(`{"count":"3 kwdg"}`), &row); err != nil {
		t.Fatal(err)
	}
	converted, err := row.Count.To(widget)
	if err != nil {
		t.Fatal(err)
	}
	if s := converted.String(); s != "3000 wdg" {
		t.Errorf("3 kwdg = %q, want %q", s, "3000 wdg")
	}
	// The same field with the shipped catalogue does not know the unit.
	var plain parse.Text
	if err := plain.UnmarshalText([]byte("3 kwdg")); !errors.Is(err, parse.ErrUnknownUnit) {
		t.Errorf("error = %v, want ErrUnknownUnit", err)
	}
}

// The JSON decoder rejects a malformed document before any UnmarshalJSON is
// called, so the errors that method reports on its own are reached by calling
// it the way a hand-written decoder would.
func TestTextUnmarshalJSONDirect(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
	}{
		{"a string that never ends", `"2.5 bar`},
		{"an object that never ends", `{"value":`},
		{"a number that is not one", `2.5.7`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got parse.Text
			if err := got.UnmarshalJSON([]byte(tc.json)); err == nil {
				t.Errorf("read %s as %q, want an error", tc.json, got.String())
			}
		})
	}
}

// The scan error names the type the driver handed over, because that is what
// tells a reader which column is wrong.
func TestScanErrorMessage(t *testing.T) {
	var got parse.Text
	err := got.Scan(true)
	const want = "metrology: parse: cannot read a measurement from bool"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}
