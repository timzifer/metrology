// Package parse reads the text form of D12: "2.5 bar" back into a measurement.
//
// Writing needs nothing but the measurement, so [metrology.Measurement] writes
// itself. Reading needs a catalogue — "bar" is a unit only because something
// says so — and the core has no catalogue and no registry to put one in (D7,
// D8). A [Parser] is therefore a value holding the units it knows:
//
//	m, err := parse.Measurement("2.5 bar")            // the shipped catalogue
//	m, err := parse.New(mine).Measurement("2.5 foo")  // units of your own
//
// which is why the standard decoding interfaces live on [Text] rather than on
// Measurement itself: encoding.TextUnmarshaler, json.Unmarshaler and sql.Scanner
// are handed no context, so a Measurement implementing them could only resolve
// symbols out of a global registry — and a program with its own units would be
// locked out of exactly the interfaces it needs most.
//
// # What the text carries, and what it does not
//
// The text form carries a magnitude and a symbol. It does not carry the kind of
// D6, and it does not carry the quantity tag: "5 K" is a temperature and a
// temperature difference written the same way, and "5 m²/s" says nothing about
// whether a kinematic viscosity or a diffusion coefficient was meant.
//
// A parser resolves both from what it has. A symbol in its catalogue brings that
// unit's quantity tag with it — "5 m²/s" reads as a kinematic viscosity, because
// that is the unit the catalogue has for the symbol. A symbol built out of an
// expression — "50 N/m²" — carries no tag at all, which is the same thing a
// computed magnitude carries (D6) and converts into any unit of its dimension.
// Where one symbol has both a point and a span reading, [Parser.Prefer] says
// which one this parser means; the default is the span, the zero
// [metrology.Kind].
package parse

import (
	"strings"
	"sync"
	"unicode"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/internal/decimaltext"
	"github.com/timzifer/metrology/symbol"
)

// spelling is one way a unit may be written, together with the reading it is
// written under: "K" is a temperature and a temperature difference, and which
// of them a text means is not in the text.
type spelling struct {
	text string
	kind metrology.Kind
}

// entry is what a spelling resolves to: the unit, the power of ten the prefix
// in the spelling stands for, and whether this is the unit's own spelling
// rather than a prefixed one.
type entry struct {
	unit      metrology.Unit
	exponent  int
	canonical bool
}

// Parser resolves unit symbols against a fixed set of units.
//
// It is a value, built once and read many times. Nothing in it is written after
// construction, so a Parser is safe for concurrent use and copying one costs a
// map header rather than the table behind it.
//
// The zero Parser knows no units at all. Use [New] or [Default].
type Parser struct {
	units map[spelling]entry
	kind  metrology.Kind
}

// New returns a parser over the given units.
//
// Every spelling each unit admits is indexed, which is what makes the
// resolution strict rather than clever: the prefixes a symbol takes are the
// ones its form declares (see [symbol.Symbol.Spellings]), so "cd" is the
// candela and never a centi-day, and "mmHg" is a millimetre of mercury and
// never a milli-metre-of-mercury.
//
// Where two units claim the same spelling, the one that spells itself that way
// wins over the one that reaches it through a prefix: "km" is the kilometre in
// the catalogue, not the prefixed metre — the same unit either way, and the
// catalogue entry is the one with the source citation.
func New(units []metrology.Unit) Parser {
	p := Parser{units: make(map[spelling]entry, len(units)*8)}
	for _, u := range units {
		for i, sp := range u.Symbol().Spellings() {
			p.add(spelling{text: sp.Text, kind: u.Kind()}, entry{
				unit:      u,
				exponent:  sp.Exponent,
				canonical: i == 0,
			})
		}
	}
	return p
}

// add indexes one spelling, keeping the better claim on a collision.
func (p Parser) add(key spelling, e entry) {
	if old, ok := p.units[key]; ok && (old.canonical || !e.canonical) {
		return
	}
	p.units[key] = e
}

// defaultParser is built on first use rather than at init: a program that never
// reads text should not pay for the table, and a package-level variable filled
// by an init function is the global state D7 keeps out of this library.
var defaultParser = sync.OnceValue(func() Parser { return New(catalog.Units()) })

// Default returns the parser over every unit the library ships (D8).
func Default() Parser { return defaultParser() }

// Prefer returns a copy of this parser that reads a symbol carrying two
// readings as the given kind.
//
// Only one symbol in the shipped catalogue has two: "K" is the kelvin, a point
// on the thermodynamic scale, and the kelvin, a difference of two temperatures.
// The default is the difference — the zero [metrology.Kind], and the reading
// that composes: 20 °C plus 5 K is 25 °C, while a parser that read "5 K" as a
// point would make the sum an error. A symbol with one reading only is
// unaffected: "°C" is a point whatever this parser prefers.
func (p Parser) Prefer(kind metrology.Kind) Parser {
	p.kind = kind
	return p
}

// Measurement reads the text form: a magnitude, then a unit.
//
//	2.5 bar
//	-40 °C
//	50 N/m²
//	250 kPa
//
// The magnitude is read exactly, digit for digit, with no float64 anywhere in
// the path. A bare number is not a measurement and is reported as such: the
// dimensionless unit is written "1", so "2.5 1" is how a ratio is spelled.
func (p Parser) Measurement(text string) (metrology.Measurement, error) {
	trimmed := strings.TrimSpace(text)
	magnitude, unitText := split(trimmed)
	switch {
	case magnitude == "":
		return metrology.Measurement{}, syntaxError(text, "no magnitude")
	case unitText == "":
		return metrology.Measurement{}, syntaxError(text, "no unit: a bare number is not a measurement")
	}
	unit, err := p.Unit(unitText)
	if err != nil {
		return metrology.Measurement{}, err
	}
	// The decimal itself is read by the core, which is also where the range a
	// magnitude may have is decided: apd rejects an exponent beyond ±100000,
	// and a second limit here would be a second answer to the same question.
	return unit.OfString(magnitude)
}

// Unit reads a unit symbol, prefixed or not, or an expression built out of
// several: "Pa", "kPa", "m/s", "N·m", "J/(kg·K)", "s⁻¹".
//
// The whole text is looked up first, so a unit that spells itself with an
// operator — "km/h", "m³/h" — resolves to the catalogue entry rather than to an
// expression that merely computes the same factor. Only what no unit spells is
// read as an expression, and an expression carries no quantity tag (D6).
func (p Parser) Unit(text string) (metrology.Unit, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return metrology.Unit{}, syntaxError(text, "no unit")
	}
	if u, ok := p.lookup(trimmed); ok {
		return u, nil
	}
	u, err := p.expression(trimmed)
	if err != nil {
		return metrology.Unit{}, err
	}
	// An expression that spells a unit this parser knows is that unit. "m²/ s"
	// and "m²/s" are one scale written twice, and only the catalogue entry
	// carries the quantity tag of D6 — without this, a stray blank would
	// decide whether a magnitude is a kinematic viscosity or is untagged.
	//
	// The scales have to agree and not merely the spelling: a caller's own
	// catalogue may spell something "m/s" that is not a metre per second, and
	// substituting that would change the factor rather than name it.
	if named, ok := p.lookup(u.String()); ok && sameScale(named, u) {
		return named, nil
	}
	return u, nil
}

// sameScale reports whether two units read a magnitude the same way: same
// dimension, same kind, and the same exact factor.
//
// There is no offset to compare. b is the unit of an expression and therefore
// an interval scale (D6), a has to be one too for the kinds to agree, and an
// interval scale cannot carry an offset — [metrology.NewUnit] refuses one.
//
// The quantity is left out on purpose: telling a tagged unit from an untagged
// one of the same scale is what the caller of this wants to do.
func sameScale(a, b metrology.Unit) bool {
	if a.Dimension() != b.Dimension() || a.Kind() != b.Kind() {
		return false
	}
	aNum, aDen := a.Factor()
	bNum, bDen := b.Factor()
	var left, right apd.Decimal
	ctx := apd.BaseContext
	ctx.Precision = 0
	// Cross multiplication compares two fractions without a division that
	// could round, and an exact multiplication of finite decimals cannot fail.
	_, _ = ctx.Mul(&left, aNum, bDen)
	_, _ = ctx.Mul(&right, bNum, aDen)
	return left.Cmp(&right) == 0
}

// lookup resolves one spelling, in the reading this parser prefers and
// otherwise in the one the catalogue has: "°C" is a point on a scale even for a
// parser that reads "K" as a span.
func (p Parser) lookup(text string) (metrology.Unit, bool) {
	e, ok := p.units[spelling{text: text, kind: p.kind}]
	if !ok {
		e, ok = p.units[spelling{text: text, kind: p.otherKind()}]
	}
	if !ok {
		return metrology.Unit{}, false
	}
	if e.exponent == 0 {
		return e.unit, true
	}
	return prefixedUnit(e.unit, e.exponent, text), true
}

// otherKind is the reading this parser falls back to.
func (p Parser) otherKind() metrology.Kind {
	if p.kind == metrology.Absolute {
		return metrology.Interval
	}
	return metrology.Absolute
}

// prefixedUnit builds the unit a prefixed spelling names: the same scale with
// its factor moved by a power of ten.
//
// The offset moves the other way. A magnitude written in the prefixed unit is
// 10^exponent of the same magnitude in the unprefixed one, so
//
//	(v·10^e + offset)·num/den  =  (v + offset·10^-e)·(num·10^e)/den
//
// and 5 m°C is 273.155 K rather than a temperature 273 kelvin too low. Both
// moves are shifts of a decimal exponent and therefore exact (D4).
//
// The decimals it works on are copies — [metrology.Unit.Factor] and
// [metrology.Unit.Offset] hand out no interior pointers (D3) — so shifting them
// touches no unit anyone else holds. [metrology.MustUnit] cannot panic here:
// every field is taken from a unit that was built successfully, and a shift of
// a decimal exponent changes neither the sign of a factor nor the kind of a
// scale, which are the only things [metrology.NewUnit] rejects.
func prefixedUnit(u metrology.Unit, exponent int, text string) metrology.Unit {
	num, den := u.Factor()
	offset := u.Offset()
	if exponent >= 0 {
		shift(num, exponent)
	} else {
		shift(den, -exponent)
	}
	shift(offset, -exponent)

	def := metrology.UnitDef{
		Dimension:   u.Dimension(),
		Kind:        u.Kind(),
		Quantity:    u.Quantity(),
		Symbol:      symbol.Static(text),
		Numerator:   num.Text('f'),
		Denominator: den.Text('f'),
		Offset:      offset.Text('f'),
	}
	if interval, ok := u.IntervalUnit(); ok {
		def.Interval = &interval
	}
	return metrology.MustUnit(def)
}

// shift multiplies d by 10^exponent, in place and exactly: a power of ten is a
// change of the decimal exponent and of nothing else.
func shift(d *apd.Decimal, exponent int) {
	d.Exponent += int32(exponent)
}

// split separates the magnitude from the unit.
//
// A space separates them where there is one, which is the canonical form. Where
// there is none — "2.5bar", "-40°C" — the magnitude is the longest prefix that
// reads as a decimal, so that "1eV" is one electronvolt and "1e3 Pa" is a
// thousand pascals.
func split(s string) (magnitude, unit string) {
	if i := strings.IndexFunc(s, unicode.IsSpace); i >= 0 {
		return s[:i], strings.TrimSpace(s[i:])
	}
	i := decimaltext.Len(s)
	return s[:i], s[i:]
}
