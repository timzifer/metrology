package uncertainty

import (
	"errors"
	"strconv"
	"strings"
	"unicode"

	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/internal/decimaltext"
	"github.com/timzifer/metrology/parse"
)

// Parser reads the text form of a [Range].
//
// Writing is a method and reading is a parser (D12), for the reason that has
// not changed: resolving a symbol needs a catalogue, a catalogue is context,
// and there is nowhere to keep one without the global state D7 forbids. So a
// Parser is a value holding its units, and a program with units of its own
// builds one over its own [parse.Parser].
//
// The zero Parser reads with the units the library ships, so that a zero [Text]
// is usable.
type Parser struct {
	units parse.Parser

	// set distinguishes a parser built over an explicit catalogue from the
	// zero value. The catalogue itself lives in parse.Parser, whose fields
	// this package cannot see — and would have no business reading.
	set bool
}

// New returns a Parser reading ranges with the units of p.
//
//	r, err := uncertainty.New(parse.New(mine)).Range("[1, 2] widget")
func New(p parse.Parser) Parser { return Parser{units: p, set: true} }

// Default returns a Parser reading ranges with the units the library ships.
func Default() Parser { return Parser{units: parse.Default(), set: true} }

// Parse reads a range with the units the library ships. It is [Parser.Range] on
// [Default].
func Parse(text string) (Range, error) { return Default().Range(text) }

// reader returns the unit parser this one reads with, resolving the zero value.
func (p Parser) reader() parse.Parser {
	if !p.set {
		return parse.Default()
	}
	return p.units
}

// Range reads a range in any of the three accepted forms:
//
//	[3.65, 3.75] cm²/s   the bracket form, which is what a Range writes
//	3.7 ± 0.2 cm         a value and a tolerance, also spelled +/-
//	3.7(2) cm            the same, with the tolerance on the last digits
//
// Only the first is canonical (D12): it states the two magnitudes the range
// holds and every range can be written in it. The other two are what a data
// sheet says, and reading them is the point of accepting them.
//
// On an absolute scale the tolerance is a span along it, so it is read on the
// interval unit the scale declares — 20 ± 0.5 °C is 0.5 K worth of tolerance,
// because the sum of two points on a scale is not a point on it (D6).
func (p Parser) Range(text string) (Range, error) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "[") {
		return p.bracket(text, trimmed)
	}
	return p.tolerance(text, trimmed)
}

// bracket reads "[3.65, 3.75] cm²/s".
func (p Parser) bracket(text, trimmed string) (Range, error) {
	end := strings.IndexByte(trimmed, ']')
	if end < 0 {
		return Range{}, syntaxError(text, "no closing bracket")
	}
	inside := trimmed[1:end]
	comma := strings.IndexByte(inside, ',')
	if comma < 0 {
		return Range{}, syntaxError(text, "a bracketed range needs two bounds separated by a comma")
	}
	unit, err := p.unit(text, trimmed[end+1:])
	if err != nil {
		return Range{}, err
	}
	lo, err := unit.OfString(strings.TrimSpace(inside[:comma]))
	if err != nil {
		return Range{}, err
	}
	hi, err := unit.OfString(strings.TrimSpace(inside[comma+1:]))
	if err != nil {
		return Range{}, err
	}
	return Between(lo, hi)
}

// tolerance reads the two forms that write a value and a tolerance.
func (p Parser) tolerance(text, trimmed string) (Range, error) {
	n := decimaltext.Len(trimmed)
	if n == 0 {
		return Range{}, syntaxError(text, "no magnitude")
	}
	value := trimmed[:n]
	rest := strings.TrimLeftFunc(trimmed[n:], unicode.IsSpace)

	switch {
	case strings.HasPrefix(rest, "±"):
		rest = rest[len("±"):]
	case strings.HasPrefix(rest, "+/-"):
		rest = rest[len("+/-"):]
	case strings.HasPrefix(rest, "("):
		return p.compact(text, value, rest)
	default:
		return Range{}, syntaxError(text,
			"not a range: expected a bracketed interval, a ± tolerance or a parenthesised one")
	}

	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	t := decimaltext.Len(rest)
	if t == 0 {
		return Range{}, syntaxError(text, "no tolerance after the ±")
	}
	return p.symmetric(text, value, rest[:t], rest[t:])
}

// compact reads "3.7(2) cm", where the digits in the parentheses stand on the
// last places of the magnitude: 3.7(2) is 3.7 ± 0.2, and 12.345(12) is
// 12.345 ± 0.012.
func (p Parser) compact(text, value, rest string) (Range, error) {
	end := strings.IndexByte(rest, ')')
	if end < 0 {
		return Range{}, syntaxError(text, "no closing parenthesis")
	}
	digits := rest[1:end]
	if digits == "" || strings.IndexFunc(digits, notDigit) >= 0 {
		return Range{}, syntaxError(text, "the tolerance in parentheses must be digits")
	}
	// The magnitude is a decimal by construction — decimaltext.Len said so —
	// but it may be a NaN or an infinity, and neither has a last place for the
	// parentheses to stand on.
	magnitude, _, err := apd.NewFromString(value)
	if err != nil || magnitude.Form != apd.Finite {
		return Range{}, syntaxError(text, "the compact form needs a finite magnitude to attach the tolerance to")
	}
	return p.symmetric(text, value, digits+"E"+strconv.FormatInt(int64(magnitude.Exponent), 10), rest[end+1:])
}

// symmetric resolves the unit and builds the range from a value and a
// tolerance, both already isolated as text.
func (p Parser) symmetric(text, value, tol, unitText string) (Range, error) {
	unit, err := p.unit(text, unitText)
	if err != nil {
		return Range{}, err
	}
	// Both magnitudes are decimals by construction — the value is what
	// decimaltext.Len measured, and the tolerance is either that or the digits
	// from the parentheses with an exponent this function wrote — so neither
	// reading can fail, and a branch for it would be a branch nothing reaches.
	magnitude, _ := unit.OfString(value)
	span, _ := toleranceUnit(unit).OfString(tol)
	return Symmetric(magnitude, span)
}

// unit resolves the unit part, which this package does not read itself: the
// expression grammar, the prefixes, the superscripts and the catalogue
// substitution that keeps the quantity tag of D6 all live in parse, and a
// second reader of them would be a second answer to the same question.
func (p Parser) unit(text, symbols string) (metrology.Unit, error) {
	trimmed := strings.TrimSpace(symbols)
	if trimmed == "" {
		return metrology.Unit{}, syntaxError(text, "no unit: a bare interval is not a measurement")
	}
	return p.reader().Unit(trimmed)
}

// toleranceUnit returns the scale a tolerance beside a magnitude is read on.
//
// For a span it is the scale itself. For a point it is the interval unit the
// scale declares — K beside a °C — because a tolerance is a distance along a
// scale and never a place on it (D6). A scale that declares none has no such
// unit, and the kind rules report it when the range is built.
func toleranceUnit(u metrology.Unit) metrology.Unit {
	if u.Kind() != metrology.Absolute {
		return u
	}
	if declared, ok := u.IntervalUnit(); ok {
		return declared
	}
	return u
}

func notDigit(r rune) bool { return r < '0' || r > '9' }

// syntaxError reports text this package could not read as a range. It is the
// core's class, so a caller matching metrology.ErrSyntax catches it alongside
// everything else that was not a number.
func syntaxError(input, why string) error {
	return &metrology.SyntaxError{Op: "uncertainty", Input: input, Err: errors.New(why)}
}
