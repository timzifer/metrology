package metrology

import (
	"database/sql/driver"
	"encoding/json"
)

// MarshalText writes the canonical text form of D12: "2.5 bar".
//
// The magnitude keeps every digit it was given and the unit is the one the
// measurement is held in, not a prefixed rendering of it — that is what makes
// the form round-trip. [Measurement.Prefixed] is for display and does not.
//
// Reading the form back is the job of the parse package, not of this type:
// resolving "bar" needs a catalogue of units, and the core has none (D7, D8).
// A parser is a value holding the units it knows, so a program with its own
// units reads them with the same code as the shipped catalogue.
func (m Measurement) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

// MarshalJSON writes the measurement as a JSON string: "2.5 bar".
//
// It is the text form quoted, and deliberately not {"value": 2.5, "unit":
// "bar"} (D12): an object with a number in it forces every consumer through a
// float64 and loses exactly the digits this library exists to keep. The object
// form is accepted on input by the parse package, for producers that emit it.
func (m Measurement) MarshalJSON() ([]byte, error) {
	// Marshalling a Go string is total — the error return of json.Marshal
	// cannot fire here, and a branch that cannot fire is a branch that lies.
	text, _ := json.Marshal(m.String())
	return text, nil
}

// Value implements [driver.Valuer], storing the measurement as its text form.
//
// A text column keeps the measurement whole: the magnitude and the unit travel
// together and no digit is lost on the way in. Storing the magnitude in a
// NUMERIC column and the unit beside it works too, and is what parse.Text reads
// back when the schema fixes the unit rather than the value.
func (m Measurement) Value() (driver.Value, error) {
	return m.String(), nil
}

// MarshalText writes the unit's symbol: "bar", "m/s", "°C".
//
// It is what the unit column of a two-column layout holds, and what the parse
// package resolves back into a unit.
func (u Unit) MarshalText() ([]byte, error) {
	return []byte(u.String()), nil
}
