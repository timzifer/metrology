package parse

import (
	"encoding/json"
	"strings"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/internal/decimaltext"
)

// Measurement reads the text form with the units the library ships:
//
//	m, err := parse.Measurement("2.5 bar")
//
// It is [Parser.Measurement] on [Default].
func Measurement(text string) (metrology.Measurement, error) {
	return Default().Measurement(text)
}

// Unit reads a unit symbol or expression with the units the library ships. It
// is [Parser.Unit] on [Default].
func Unit(text string) (metrology.Unit, error) {
	return Default().Unit(text)
}

// Text is a measurement that reads itself back from text, JSON and SQL.
//
// The measurement is embedded, so a Text writes exactly what a
// [metrology.Measurement] writes — "2.5 bar" — and reads what this package can
// read. It exists because the standard decoding interfaces are handed no
// context to resolve a symbol with, and a catalogue is context (D7):
//
//	var p parse.Text
//	err := json.Unmarshal([]byte(`"2.5 bar"`), &p)
//	fmt.Println(p.Measurement)
//
// The zero Text reads with the shipped catalogue. [Parser.Text] returns one
// that reads with a catalogue of your own, and [Text.In] one that reads a bare
// magnitude — a NUMERIC column, a JSON number — on a unit the schema fixes
// rather than the value.
type Text struct {
	metrology.Measurement

	// parser resolves the symbols. The zero Parser means the shipped
	// catalogue, resolved on use so that a zero Text is usable.
	parser Parser

	// bare is the unit a magnitude without one is read on, where a caller has
	// said what the column or the field holds. Nil means a measurement without
	// a unit is an error, which is the safe default: inventing a unit for a
	// number is how a pressure in bar becomes a pressure in pascal.
	bare *metrology.Unit
}

// Text returns a [Text] that resolves symbols with this parser.
//
// This is how a program with its own units decodes JSON or SQL: the parser
// travels in the destination, since the interfaces carry nothing else.
//
//	row := struct{ P parse.Text }{P: parse.New(mine).Text()}
//	err := json.Unmarshal(data, &row)
func (p Parser) Text() Text { return Text{parser: p} }

// In returns a copy of this Text that reads a bare magnitude on u.
//
// It is the two-column layout: the magnitude in a NUMERIC column, the unit in
// the schema. Text that does carry a unit is still read from it, and a unit
// that disagrees with u is not silently converted — it is the unit of the
// result, because what the text says is what the text means.
func (t Text) In(u metrology.Unit) Text {
	t.bare = &u
	return t
}

// UnmarshalText reads the text form, implementing [encoding.TextUnmarshaler].
func (t *Text) UnmarshalText(text []byte) error {
	m, err := t.read(string(text))
	if err != nil {
		return err
	}
	t.Measurement = m
	return nil
}

// UnmarshalJSON reads a measurement from JSON, implementing [json.Unmarshaler].
//
// Three shapes are accepted, because three shapes are what producers emit:
//
//	"2.5 bar"                       the canonical form of D12
//	{"value": "2.5", "unit": "bar"} the object form, for producers that emit it
//	2.5                             a bare number, on the unit given to [Text.In]
//
// Only the first is written back. The object form keeps its magnitude as text
// where the producer wrote it as text; where the producer wrote a JSON number,
// the digits are read from the JSON itself and not through a float64, so
// nothing is lost that the producer did not lose first.
//
// A JSON null leaves the measurement untouched, as the convention for
// [json.Unmarshaler] requires.
func (t *Text) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	switch {
	case trimmed == "null":
		return nil
	case strings.HasPrefix(trimmed, `"`):
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		return t.UnmarshalText([]byte(text))
	case strings.HasPrefix(trimmed, "{"):
		return t.object(data)
	default:
		return t.UnmarshalText([]byte(trimmed))
	}
}

// object reads the {"value": …, "unit": …} form.
func (t *Text) object(data []byte) error {
	var obj struct {
		Value json.RawMessage `json:"value"`
		Unit  string          `json:"unit"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	// A producer writes the magnitude either as a string or as a JSON number.
	// A string is unquoted; a number is taken from the JSON text itself and
	// never through a float64, so its digits arrive as the producer wrote them.
	magnitude := strings.TrimSpace(string(obj.Value))
	var quoted string
	if err := json.Unmarshal(obj.Value, &quoted); err == nil {
		magnitude = quoted
	}
	if obj.Unit == "" {
		// No unit in the object either; the magnitude has to stand on the one
		// the schema fixed, or on nothing.
		return t.UnmarshalText([]byte(magnitude))
	}
	unit, err := t.reader().Unit(obj.Unit)
	if err != nil {
		return err
	}
	m, err := unit.OfString(magnitude)
	if err != nil {
		return err
	}
	t.Measurement = m
	return nil
}

// Scan reads a measurement out of a database value, implementing [sql.Scanner].
//
// A text column holds the whole measurement — "2.5 bar" — and is read as such.
// A numeric column holds a magnitude only, and is read on the unit given to
// [Text.In]; without one it is an error rather than a guess.
//
// A driver that hands over a float64 for a NUMERIC column has already rounded
// the value before this method sees it, and no library can undo that. What
// arrives is taken at the shortest decimal that reads back as the same float,
// which is the most the float can still claim. Store measurements as text, or
// the magnitude in a NUMERIC column that the driver hands over as a string, and
// nothing is lost at all.
//
// A NULL is a [ScanError]: use sql.Null[[Text]] for a nullable column.
func (t *Text) Scan(src any) error {
	switch v := src.(type) {
	case string:
		return t.UnmarshalText([]byte(v))
	case []byte:
		return t.UnmarshalText(v)
	case int64:
		return t.number(v)
	case float64:
		return t.number(v)
	default:
		return &ScanError{Value: src}
	}
}

// number reads a magnitude the driver has already turned into a number, on the
// unit the schema fixed.
func (t *Text) number(v any) error {
	text, err := json.Marshal(v)
	if err != nil {
		return &ScanError{Value: v}
	}
	return t.UnmarshalText(text)
}

// read resolves one piece of text: a measurement, or a bare magnitude on the
// unit this Text was given.
func (t Text) read(s string) (metrology.Measurement, error) {
	trimmed := strings.TrimSpace(s)
	if t.bare != nil && decimaltext.Valid(trimmed) {
		return t.bare.OfString(trimmed)
	}
	return t.reader().Measurement(s)
}

// reader is the parser this Text resolves with, defaulting to the shipped
// catalogue so that the zero value works.
func (t Text) reader() Parser {
	if t.parser.units == nil {
		return Default()
	}
	return t.parser
}
