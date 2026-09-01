package symbol

import "github.com/timzifer/metrology/internal/superscript"

// Spelling is one accepted way of writing a symbol: the text, and the power of
// ten by which that text multiplies the unit the symbol belongs to.
//
// "Pa" spells the pascal with exponent 0, "kPa" spells it with exponent 3, and
// "g" spells the kilogram with exponent −3, because the SI base unit of mass is
// the one whose name already carries a prefix.
type Spelling struct {
	// Text is the spelling itself.
	Text string

	// Exponent is the power of ten the prefix in Text stands for: a magnitude
	// written in Text is 10^Exponent times the same magnitude written in the
	// symbol's own form.
	Exponent int
}

// Spellings returns every way this symbol may be written, the symbol's own form
// first and the prefixed forms after it.
//
// It is what a parser reads the text form with (D12): the set is enumerated
// rather than guessed, so "cd" is the candela and never centi-day — a static
// symbol takes no prefix at all, and a prefix is only ever recognised in front
// of a symbol that admits one. The unprefixed spelling comes first so that a
// parser resolving a collision — "km" is both the kilometre in the catalogue
// and the prefixed metre — can prefer the unit that spells itself that way.
//
// A prefix attaches where [Symbol.Scale] puts it: to the first multiplicand of
// a product and to the numerator of a quotient, never to both sides.
func (s Symbol) Spellings() []Spelling {
	switch s.form {
	case formSI:
		if s.power == 0 {
			// Every prefix would scale by 10⁰ and the spellings would differ
			// only in a letter that means nothing. There is one form.
			return []Spelling{{Text: s.String()}}
		}
		return prefixed(s.text+powerSuffix(s.power), siPrefixes, s.power, 0)
	case formGram:
		// Magnitudes are in kilograms, so the gram itself is the prefixed
		// spelling and "kg" is the one with exponent zero.
		return prefixed("g", siPrefixes, 1, -3)
	case formLitre:
		return prefixed("L", litrePrefixes, 1, 0)
	case formProduct:
		if len(s.parts) == 0 {
			return []Spelling{{Text: ""}}
		}
		tail := ""
		for _, p := range s.parts[1:] {
			tail += "·" + p.String()
		}
		return suffixed(s.parts[0].Spellings(), tail)
	case formQuotient:
		return suffixed(s.parts[0].Spellings(), "/"+s.denominatorString())
	default: // formStatic
		return []Spelling{{Text: s.text}}
	}
}

// prefixed builds the spellings of a symbol that takes the prefixes in table.
//
// power scales the prefix step, because one step on m² is a factor of 10⁶, and
// shift is what the symbol's own form already carries: −3 for the gram, whose
// unprefixed spelling "g" is a thousandth of the unit magnitudes are read in.
// The spelling with exponent zero is moved to the front, since that is the form
// the symbol prints in.
func prefixed(text string, table []prefix, power, shift int) []Spelling {
	out := make([]Spelling, 0, len(table))
	for _, p := range table {
		out = append(out, Spelling{Text: p.name + text, Exponent: p.exponent*power + shift})
	}
	for i, sp := range out {
		if sp.Exponent == 0 {
			out[0], out[i] = out[i], out[0]
			break
		}
	}
	return out
}

// suffixed appends the unprefixable remainder of a product or a quotient to
// every spelling of its head.
func suffixed(head []Spelling, tail string) []Spelling {
	for i := range head {
		head[i].Text += tail
	}
	return head
}

// Pow returns the symbol of this symbol raised to the n-th power: m becomes m³,
// s becomes s⁻¹, and N·m becomes (N·m)².
//
// A composite is parenthesised, and so is a static symbol that is not a plain
// word — "m³/h" squared is "(m³/h)²", never "m³/h²", which would square the
// hour alone. The zeroth power is the dimensionless symbol, written 1 as the
// catalogue writes it.
func (s Symbol) Pow(n int) Symbol {
	switch {
	case n == 1:
		return s
	case n == 0:
		return Static("1")
	}
	if s.form == formSI {
		return SIPow(s.text, s.power*n)
	}
	text := s.String()
	if needsBrackets(text) {
		text = "(" + text + ")"
	}
	return Static(text + superscript.Itoa(n))
}

// needsBrackets reports whether a rendered symbol has to be parenthesised
// before an exponent is appended to it: anything that already carries an
// operator or an exponent of its own would otherwise bind the new exponent to
// its last part — m³/h squared is (m³/h)², never m³/h².
func needsBrackets(text string) bool {
	if composite(text) {
		return true
	}
	for _, r := range text {
		if superscript.Is(r) {
			return true
		}
	}
	return false
}
