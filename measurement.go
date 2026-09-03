package metrology

import (
	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/internal/decimaltext"
)

// Measurement is a magnitude on a scale: a decimal paired with a [Unit].
//
// It is an ordinary Go value (D1) — copyable, passable, free of identity and of
// hidden behaviour. It is not comparable with ==; use [Measurement.Cmp] or
// [Measurement.Equal], which compare what the two measurements mean rather than
// how they are written.
//
// Nothing inside a Measurement is ever written after construction (D3). Every
// operation allocates its result, and [Measurement.Decimal] hands out a copy,
// because an apd.Decimal shares its coefficient with every struct copy of
// itself: one in-place write would silently corrupt every measurement derived
// from it.
type Measurement struct {
	unit Unit
	val  apd.Decimal
}

// Of returns a measurement of v in this unit (D10).
//
//	p := Bar.Of(2.5)
//	n := Metre.Of(3)
//
// A float is taken at the shortest decimal that reads back as the same float,
// which is the number the caller wrote. Callers with more digits than a float64
// holds use [Unit.OfString] and skip the detour entirely. A NaN or an infinity
// is carried as such and fails at the first operation that has to compute with
// it.
func (u Unit) Of[N Numeric](v N) Measurement {
	return u.measurement(decimalFromNumber(v))
}

// OfString returns a measurement of the magnitude written in s.
//
// This is the exact path: the digits in s are the digits stored, with no float64
// in between. Accepted is what the text form of D12 writes — an optional sign,
// digits with at most one decimal point among them, an exponent after e or E,
// and the words NaN and Infinity.
//
// The shape is checked here rather than left to apd, which accepts ".-1" and
// yields a decimal that prints as "0.-1" — a value whose own text form is not a
// value, which a canonical text form must not be able to hold.
func (u Unit) OfString(s string) (Measurement, error) {
	if !decimaltext.Valid(s) {
		return Measurement{}, &SyntaxError{Op: "OfString", Input: s, Err: errNotDecimal}
	}
	d, _, err := apd.NewFromString(s)
	if err != nil {
		return Measurement{}, &SyntaxError{Op: "OfString", Input: s, Err: err}
	}
	return u.measurement(d), nil
}

// OfDecimal returns a measurement of d in this unit, exactly.
//
// This is the exported counterpart of [Measurement.Decimal]: what that hands
// out, this takes back, with no float64 and no text in between. The decimal is
// copied on the way in (D3), so the caller keeps no reach into the result and
// may go on writing to its own.
//
// It exists for the layer that computes with bare magnitudes and has to label
// them again — the interval type of D15 holds one unit and two decimals, and
// every bound it hands out passes through here.
func (u Unit) OfDecimal(d *apd.Decimal) Measurement {
	return u.measurement(d)
}

// measurement pairs a decimal with this unit, taking a copy of the decimal so
// that the caller cannot reach into the result (D3).
func (u Unit) measurement(d *apd.Decimal) Measurement {
	m := Measurement{unit: u}
	m.val.Set(d)
	// A zero with a positive exponent prints as "00": the fixed-point form
	// expands the exponent into digits, and for a zero coefficient those
	// digits are zeros. It is the same zero written twice and only one of the
	// two forms reads back as itself, so the canonical one is stored (D12).
	// Digits behind the point are left alone — 0.00 is what the caller wrote.
	if m.val.Form == apd.Finite && m.val.Coeff.Sign() == 0 && m.val.Exponent > 0 {
		m.val.Exponent = 0
	}
	return m
}

// Unit returns the scale this measurement is read on.
func (m Measurement) Unit() Unit { return m.unit }

// Dimension returns what is measured.
func (m Measurement) Dimension() dimension.Dimension { return m.unit.dim }

// Kind reports whether this is a point on a scale or a span along it (D6).
func (m Measurement) Kind() Kind { return m.unit.kind }

// Quantity reports what is measured, where the dimension does not say it.
func (m Measurement) Quantity() Quantity { return m.unit.quantity }

// Decimal returns the magnitude in this measurement's own unit, as a copy that
// shares nothing with the measurement (D3).
func (m Measurement) Decimal() *apd.Decimal {
	return copyDecimal(&m.val)
}

// In returns the magnitude expressed in u, as a number (D10).
//
//	pa, err := Bar.Of(2.5).In[float64](Pascal)   // 250000
//
// An integer target rejects a fractional or out-of-range magnitude instead of
// truncating or wrapping it.
func (m Measurement) In[N Numeric](u Unit) (N, error) {
	d, err := m.DecimalIn(u)
	if err != nil {
		var zero N
		return zero, err
	}
	return numberFromDecimal[N]("In", d)
}

// DecimalIn returns the magnitude expressed in u, exactly.
//
// This is [Measurement.In] without the numeric boundary: the decimal is the
// value the library computed, not an approximation of it.
func (m Measurement) DecimalIn(u Unit) (*apd.Decimal, error) {
	converted, err := m.To(u)
	if err != nil {
		return nil, err
	}
	return converted.Decimal(), nil
}

// To expresses this measurement on another scale of the same dimension and
// kind.
//
// The conversion rounds once, at the single division of D4. Converting a
// magnitude to its own unit returns it unchanged rather than passing it through
// that division.
func (m Measurement) To(u Unit) (Measurement, error) {
	return Engine{}.To(m, u)
}

// To is [Measurement.To] at this engine's precision (D9).
func (e Engine) To(m Measurement, u Unit) (Measurement, error) {
	if m.unit.dim != u.dim {
		return Measurement{}, &DimensionError{Op: "To", Want: u.dim, Got: m.unit.dim}
	}
	if m.unit.kind != u.kind {
		return Measurement{}, &KindError{
			Op: "To", Left: m.unit.kind, Right: u.kind,
			Why: "a point on a scale and a span along it are not the same quantity",
		}
	}
	if !m.unit.quantity.compatible(u.quantity) {
		return Measurement{}, &QuantityError{Op: "To", Left: m.unit.quantity, Right: u.quantity}
	}
	d, err := e.convert(&m.val, m.unit, u)
	if err != nil {
		return Measurement{}, err
	}
	return u.measurement(d), nil
}

// String renders the measurement in its own unit: "2.5 bar".
//
// This is the canonical text form of D12 — the unit is the one the measurement
// is held in, not a prefixed rendering of it, so that reading the text back
// yields the same measurement. [Measurement.Prefixed] is the display form.
func (m Measurement) String() string {
	return m.val.Text('f') + " " + m.unit.String()
}

// Prefixed renders the measurement with the SI prefix that fits its magnitude:
// 250000 Pa reads as "250 kPa".
//
// The prefix is chosen in decimal arithmetic, so the value is shifted, never
// rounded (D9).
func (m Measurement) Prefixed() string {
	value, sym := m.unit.sym.Scale(&m.val)
	return value.Text('f') + " " + sym
}
