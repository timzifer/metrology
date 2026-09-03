package uncertainty

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
)

// String renders the range in the bracket form: "[3.65, 3.75] cm²/s".
//
// This is the canonical text form of D12, and it is the bracket form rather
// than "3.7 ± 0.2" because the brackets state the two magnitudes the range
// actually holds. The ± form states a centre and a tolerance instead, and for a
// range that came out of a product or a quotient there is no centre anybody
// claimed — the arithmetic produced two bounds, and turning them into a centre
// invents a reading of them.
//
// The unit is the one the range is held in, never a prefixed rendering of it,
// so that reading the text back yields the same range. [Range.PlusMinus] is the
// display form.
func (r Range) String() string {
	return "[" + r.lo.Text('f') + ", " + r.hi.Text('f') + "] " + r.unit.String()
}

// PlusMinus renders the range as "3.7 ± 0.2 cm", and reports whether that form
// says exactly what the range says.
//
// It is the second result that carries the honesty. The midpoint and the
// half-width of a range need one digit more than its bounds do, so a range
// whose bounds already fill the engine's precision has no ± form that reads
// back as the same range — and a tolerance rounded to fit is a claim about the
// data that the data does not make. Where that happens there is no rendering
// and the second result is false; [Range.String] always works.
//
// What reads back is the pair of magnitudes, not the digits they are written
// with: 0.5 ± 2.5 m reads back as [-2.0, 3.0] m, which is the range [-2, 3] m
// with the digits a subtraction produced, because addition never rounds and so
// never drops one (D9).
//
// This is the split D12 makes between [metrology.Measurement.String] and
// [metrology.Measurement.Prefixed], with the same rule: the canonical text has
// to read back as the same value, the pleasant form is a rendering choice.
func (r Range) PlusMinus() (string, bool) { return Engine{}.PlusMinus(r) }

// PlusMinus is [Range.PlusMinus] at this engine's precision (D9).
func (e Engine) PlusMinus(r Range) (string, bool) {
	mid, tol, ok := e.centre(r)
	if !ok {
		return "", false
	}
	return mid.Text('f') + " ± " + tol.Text('f') + " " + r.unit.String(), true
}

// centre returns the midpoint and the half-width of a range, and whether both
// are exact at this engine's precision.
//
// Exactness is checked rather than assumed, and it is checked by putting the
// two back together: mid − tol has to be the lower bound and mid + tol the
// upper one, digit for digit. Addition and subtraction never round (D9), so a
// mismatch can only have come from the one division, which is precisely the
// case this reports.
func (e Engine) centre(r Range) (mid, tol *apd.Decimal, ok bool) {
	var s steps
	two := apd.New(2, 0)
	sum := s.do(bare(e.core, metrology.Engine.Add, &r.lo, &r.hi))
	span := s.do(bare(e.core, metrology.Engine.Sub, &r.hi, &r.lo))
	mid = s.do(bare(e.core, metrology.Engine.Div, sum, two))
	tol = s.do(bare(e.core, metrology.Engine.Div, span, two))
	lower := s.do(bare(e.core, metrology.Engine.Sub, mid, tol))
	upper := s.do(bare(e.core, metrology.Engine.Add, mid, tol))
	if s.err != nil || lower.Cmp(&r.lo) != 0 || upper.Cmp(&r.hi) != 0 {
		return nil, nil, false
	}
	return mid, tol, true
}

// MarshalText writes the canonical text form of D12: "[3.65, 3.75] cm²/s".
//
// Both magnitudes keep every digit they were given and the unit is the one the
// range is held in, so the text reads back as the same range.
func (r Range) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// MarshalJSON writes the text form as a JSON string.
//
// A range is one value and it is written as one, not as an object of two
// bounds: the text form is the exchange format (D12), and an object would be a
// second format for every reader to have to know about. [Text] reads it back.
func (r Range) MarshalJSON() ([]byte, error) { return json.Marshal(r.String()) }

// Value writes the text form for a SQL driver.
func (r Range) Value() (driver.Value, error) { return r.String(), nil }
