package parse

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/internal/superscript"
)

// Unit expressions are read by recursive descent over a grammar small enough to
// state in full:
//
//	expression := factor { ("·" | "*" | "/") factor }
//	factor     := atom [ exponent ]
//	atom       := symbol | "(" expression ")"
//	exponent   := superscript digits | "^" [sign] digits
//
// The two operators bind equally and from the left, which is how a unit symbol
// is read on a gauge and what the renderer assumes when it brackets J/(kg·K):
// without the brackets that string is J/kg·K, and J/kg·K is (J/kg)·K.
//
// There is no operator for a power because there is no need for one: m³ and m^3
// are both written directly on the symbol.
//
// Every step also reports the power its parts have been raised to, and the
// exponents of nested brackets multiply into it. A unit raised to the 127th and
// that raised to the 127th again is a factor of sixty million digits written in
// fourteen characters, and a parser of untrusted text that computes it first and
// judges it afterwards has already lost. [metrology.MaxPower] is the bound, the
// same one a single power has, and for the same reason: nothing a dimension can
// hold lies beyond it.

// expression reads a unit built out of several symbols.
func (p Parser) expression(text string) (metrology.Unit, error) {
	e := &exprParser{parser: p, input: text, rest: text}
	u, _, err := e.expression()
	if err != nil {
		return metrology.Unit{}, err
	}
	e.space()
	if e.rest != "" {
		return metrology.Unit{}, syntaxError(text, "unexpected "+strconv.Quote(e.rest))
	}
	return u, nil
}

// exprParser is the state of one expression: what this parser knows, the text
// it was given for error messages, and what is left to read.
type exprParser struct {
	parser Parser
	input  string
	rest   string
}

// expression reads a sequence of factors joined by · and /.
func (e *exprParser) expression() (metrology.Unit, int, error) {
	left, power, err := e.factor()
	if err != nil {
		return metrology.Unit{}, 0, err
	}
	for {
		e.space()
		op, size := utf8.DecodeRuneInString(e.rest)
		if op != '·' && op != '*' && op != '/' {
			return left, power, nil
		}
		e.rest = e.rest[size:]
		e.space()
		right, rightPower, err := e.factor()
		if err != nil {
			return metrology.Unit{}, 0, err
		}
		// Operands do not raise one another, so the power of a product is the
		// larger of the two rather than their product.
		power = max(power, rightPower)
		// A point on a scale has no product and no quotient (D6), and the
		// error saying so is the one the core writes.
		if op == '/' {
			left, err = left.Per(right)
		} else {
			left, err = left.Times(right)
		}
		if err != nil {
			return metrology.Unit{}, 0, err
		}
	}
}

// factor reads one atom and the exponent it may carry.
func (e *exprParser) factor() (metrology.Unit, int, error) {
	u, power, err := e.atom()
	if err != nil {
		return metrology.Unit{}, 0, err
	}
	n, ok, err := e.exponent()
	if err != nil {
		return metrology.Unit{}, 0, err
	}
	if !ok {
		return u, power, nil
	}
	total := power * abs(n)
	if total > metrology.MaxPower {
		return metrology.Unit{}, 0, &metrology.RangeError{
			Op: "parse", Value: strconv.Itoa(total), Type: "a dimension exponent",
		}
	}
	u, err = u.Pow(n)
	return u, total, err
}

// atom reads a symbol or a bracketed expression.
func (e *exprParser) atom() (metrology.Unit, int, error) {
	if strings.HasPrefix(e.rest, "(") {
		e.rest = e.rest[1:]
		u, power, err := e.expression()
		if err != nil {
			return metrology.Unit{}, 0, err
		}
		e.space()
		if !strings.HasPrefix(e.rest, ")") {
			return metrology.Unit{}, 0, syntaxError(e.input, "a bracket is not closed")
		}
		e.rest = e.rest[1:]
		return u, power, nil
	}
	text := e.symbol()
	if text == "" {
		return metrology.Unit{}, 0, syntaxError(e.input, "expected a unit symbol")
	}
	u, ok := e.parser.lookup(text)
	if !ok {
		return metrology.Unit{}, 0, &UnknownUnitError{Input: e.input, Symbol: text}
	}
	// A symbol on its own has been raised to the first power.
	return u, 1, nil
}

// abs is the magnitude of an exponent: m³ and m⁻³ cost the same to compute.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// symbol reads the run of runes that spell one symbol: everything up to an
// operator, a bracket, a space or an exponent.
//
// The run is measured with the size the decoder reports and not with the length
// of the rune it decoded. On invalid UTF-8 those two differ — the replacement
// rune is three bytes long and the byte it stands for is one — and taking the
// second is how a lexer walks off the end of its own input. A symbol with a
// broken byte in it resolves to no unit, which is the answer it deserves.
func (e *exprParser) symbol() string {
	end := 0
	for end < len(e.rest) {
		r, size := utf8.DecodeRuneInString(e.rest[end:])
		if isOperator(r) || superscript.Is(r) || unicode.IsSpace(r) {
			break
		}
		end += size
	}
	text := e.rest[:end]
	e.rest = e.rest[end:]
	return text
}

// exponent reads the power a symbol is raised to, in either spelling, and
// reports whether there was one.
func (e *exprParser) exponent() (int, bool, error) {
	if end := superscriptRunLen(e.rest); end > 0 {
		text := e.rest[:end]
		e.rest = e.rest[end:]
		n, ok := superscript.Atoi(text)
		if !ok {
			return 0, false, syntaxError(e.input, "not an exponent: "+strconv.Quote(text))
		}
		return n, true, nil
	}
	if !strings.HasPrefix(e.rest, "^") {
		return 0, false, nil
	}
	rest := e.rest[1:]
	end := 0
	if strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "+") {
		end++
	}
	for end < len(rest) && '0' <= rest[end] && rest[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0, false, syntaxError(e.input, "not an exponent: "+strconv.Quote(rest[:end]))
	}
	e.rest = rest[end:]
	return n, true, nil
}

// superscriptRunLen returns the length in bytes of the superscript digits at
// the start of s.
func superscriptRunLen(s string) int {
	end := 0
	for end < len(s) {
		r, size := utf8.DecodeRuneInString(s[end:])
		if !superscript.Is(r) {
			break
		}
		end += size
	}
	return end
}

// space skips the blanks between the parts of an expression. The canonical form
// has none, but a unit copied off a data sheet does.
func (e *exprParser) space() {
	e.rest = strings.TrimLeftFunc(e.rest, unicode.IsSpace)
}

// isOperator reports whether r joins two units rather than spelling one.
func isOperator(r rune) bool {
	return r == '·' || r == '*' || r == '/' || r == '(' || r == ')' || r == '^'
}
