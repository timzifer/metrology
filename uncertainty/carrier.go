package uncertainty

import (
	"encoding/json"
	"strings"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/internal/decimaltext"
	"github.com/timzifer/metrology/parse"
)

// Text is a range that reads itself back from text, JSON and SQL.
//
// The range is embedded, so a Text writes exactly what a [Range] writes —
// "[3.65, 3.75] cm" — and reads what [Parser] can read. It exists for the same
// reason [parse.Text] does: the standard decoding interfaces are handed no
// context to resolve a symbol with, and a catalogue is context (D7).
//
//	var r uncertainty.Text
//	err := json.Unmarshal([]byte(`"[1, 2] m"`), &r)
//	fmt.Println(r.Range)
//
// The zero Text reads with the shipped catalogue. [Parser.Text] returns one
// that reads with a catalogue of your own, and [Text.In] one that reads a bare
// magnitude — a NUMERIC column, a JSON number — as a point on a unit the schema
// fixes rather than the value.
type Text struct {
	Range

	// parser resolves the symbols. The zero Parser means the shipped
	// catalogue, resolved on use so that a zero Text is usable.
	parser Parser

	// bare is the unit a magnitude without one is read on. Nil means a number
	// without a unit is an error, which is the safe default: inventing a unit
	// for a number is how a pressure in bar becomes a pressure in pascal.
	bare *metrology.Unit
}

// Text returns a [Text] that resolves symbols with this parser.
func (p Parser) Text() Text { return Text{parser: p} }

// In returns a [Text] that reads a bare magnitude as a point on u.
//
// A range needs two bounds and a number carries one, so what arrives without a
// unit is read as a range of zero width — which is what a NUMERIC column
// holding an exact value means.
func (t Text) In(u metrology.Unit) Text {
	t.bare = &u
	return t
}

// UnmarshalText reads the text form.
func (t *Text) UnmarshalText(text []byte) error {
	r, err := t.read(string(text))
	if err != nil {
		return err
	}
	t.Range = r
	return nil
}

// UnmarshalJSON reads a JSON string, or a bare number where [Text.In] has said
// what unit one is on.
//
// A null leaves the destination untouched, so that a nullable column decodes
// into the zero Range rather than an error.
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
	default:
		return t.UnmarshalText([]byte(trimmed))
	}
}

// Scan reads a range from a SQL driver value.
func (t *Text) Scan(src any) error {
	switch v := src.(type) {
	case string:
		return t.UnmarshalText([]byte(v))
	case []byte:
		return t.UnmarshalText(v)
	default:
		return &parse.ScanError{Value: src}
	}
}

// read resolves one piece of text, taking the bare-magnitude route where the
// destination has named a unit and the text is nothing but a number.
func (t Text) read(s string) (Range, error) {
	trimmed := strings.TrimSpace(s)
	if t.bare != nil && decimaltext.Valid(trimmed) {
		// A number is a range of zero width: the schema fixed the unit and the
		// value carries one magnitude, which is a point and not an interval.
		m, err := t.bare.OfString(trimmed)
		return Of(m), err
	}
	return t.parser.Range(s)
}
