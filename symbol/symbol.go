// Package symbol renders unit symbols and picks the SI prefix for a magnitude.
//
// A [Symbol] is a concrete value type, not an interface (D1). The forms it
// takes — static, SI-prefixable, product, quotient — differ only in rendering
// and in which prefixes they accept, which is a switch, not a hierarchy: an
// interface would box on every use and buy no extension point, because only
// this package knows how to scale what it renders.
//
// Prefix selection is decimal arithmetic, not math.Log (D9). The exponent of a
// decimal is exact, so 1000 m is 1 km and not 999.9999999 m.
package symbol

import (
	"strings"

	"github.com/timzifer/metrology/internal/superscript"
)

// form distinguishes the shapes a symbol can take. Each shape differs only in
// how it renders and which prefixes it may carry, which is too little to
// justify seven types.
type form uint8

const (
	formStatic   form = iota // no prefix, ever: °C, %, torr
	formSI                   // full prefix range: m, Pa, s
	formGram                 // SI base unit is the kilogram, prefixes attach to the gram
	formLitre                // the historic litre prefixes: µ, m, c, d, h
	formProduct              // N·m
	formQuotient             // m/s
)

// Symbol is the printable form of a unit.
//
// The zero Symbol is the empty static symbol, which is the correct rendering
// for a dimensionless count.
type Symbol struct {
	form  form
	text  string
	power int      // 1 for m, 2 for m², 3 for m³ — scales the prefix step
	parts []Symbol // multiplicands of a product, or {numerator, denominator}
}

// Static returns a symbol that never carries a prefix. Use it for units whose
// symbol is fixed by convention — °C, %, torr, °.
func Static(text string) Symbol {
	return Symbol{form: formStatic, text: text}
}

// SI returns a prefixable symbol: 1000 m renders as 1 km.
func SI(text string) Symbol {
	return Symbol{form: formSI, text: text, power: 1}
}

// SIPow returns a prefixable symbol raised to power, such as m² or m³.
//
// The prefix step scales with the power: one step on m² is a factor of 10⁶,
// because a square kilometre is 10⁶ square metres.
func SIPow(text string, power int) Symbol {
	return Symbol{form: formSI, text: text, power: power}
}

// Gram returns the symbol of the kilogram.
//
// The kilogram is the one SI base unit whose name already carries a prefix.
// Magnitudes are in kilograms, prefixes attach to the gram: 0.001 kg renders as
// 1 g, 1000 kg as 1 Mg.
func Gram() Symbol {
	return Symbol{form: formGram, text: "g", power: 1}
}

// Litre returns the symbol of the litre, which by convention also takes the
// prefixes c, d and h that the SI otherwise discourages.
func Litre() Symbol {
	return Symbol{form: formLitre, text: "L", power: 1}
}

// Product returns the symbol of a product, rendered N·m.
//
// A prefix attaches to the first multiplicand only, because a prefix on each
// factor would multiply: k(N·m) is kN·m, never kN·km.
//
// A product of a product is flattened into one. Multiplication is associative
// and the rendering is flat either way, so keeping the nesting would leave two
// structures for one symbol — and [Symbol.Equal], which promises that two
// symbols rendering alike are alike, would have to answer false for them.
//
// Repeated prefixable multiplicands gather into a power for the same reason one
// level down: m·m and m² are one scale spelled twice, and a spelling that
// differs is a unit that differs — [Symbol.Equal] answers false, the parser's
// substitution misses the catalogue entry, and a magnitude loses the quantity
// tag of D6 to a difference in notation. The gathering makes Times agree with
// Pow, which is what the two ought to have agreed on all along (D12). Only a
// prefixable symbol gathers; see gather for why.
func Product(multiplicands ...Symbol) Symbol {
	parts := make([]Symbol, 0, len(multiplicands))
	for _, m := range multiplicands {
		if m.form == formProduct {
			parts = append(parts, m.parts...)
			continue
		}
		parts = append(parts, m)
	}
	parts = gather(parts)
	switch {
	case len(parts) == 1:
		// One multiplicand is not a product, it is that multiplicand. Wrapping
		// it would be a second structure for one rendering again.
		return parts[0]
	case len(parts) == 0 && len(multiplicands) != 0:
		// Everything cancelled — m·m⁻¹ — and the dimensionless one is spelled
		// the way [Symbol.Pow] spells it.
		return Static("1")
	}
	return Symbol{form: formProduct, parts: parts}
}

// gather folds repeated multiplicands into powers, keeping each base where it
// first appeared: m·m is m², m²·m is m³, and N·m is left alone. A base that
// cancels to the zeroth power drops out rather than rendering as a factor of
// one.
//
// Only a prefixable symbol gathers, and that restriction is the whole of what
// makes this sound. An SI symbol records its power as a number, so a power can
// be added to it and taken off again. Every other form carries its power in its
// text — [Symbol.Pow] of a static torr is the static "torr²" — and a static
// that has been raised cannot be recognised as a power of anything afterwards.
// Gathering those would render 1·1·1 as 1²·1, then as 1²·1², then as (1²)²:
// a spelling that reads back as a different symbol every time it is written.
// The fuzzer of D12 found exactly that, which is why it is stated here.
func gather(parts []Symbol) []Symbol {
	merged := make([]Symbol, 0, len(parts))
	at := map[string]int{}
	for _, p := range parts {
		if p.form != formSI {
			merged = append(merged, p)
			continue
		}
		if i, seen := at[p.text]; seen {
			merged[i] = SIPow(p.text, merged[i].power+p.power)
			continue
		}
		at[p.text] = len(merged)
		merged = append(merged, p)
	}
	out := make([]Symbol, 0, len(merged))
	for _, p := range merged {
		if p.form == formSI && p.power == 0 {
			// m·m⁻¹ cancelled, and a factor of one is not written down.
			continue
		}
		out = append(out, p)
	}
	return out
}

// Quotient returns the symbol of a quotient, rendered m/s. A prefix attaches to
// the numerator.
func Quotient(numerator, denominator Symbol) Symbol {
	return Symbol{form: formQuotient, parts: []Symbol{numerator, denominator}}
}

// String renders the symbol without a prefix: the form in which a unit is
// named, as opposed to the form in which a magnitude is printed.
func (s Symbol) String() string {
	switch s.form {
	case formGram:
		return "kg"
	case formSI:
		return s.text + powerSuffix(s.power)
	case formProduct:
		return strings.Join(s.partStrings(), "·")
	case formQuotient:
		return s.parts[0].String() + "/" + s.denominatorString()
	default: // formStatic, formLitre
		return s.text
	}
}

// powerSuffix renders the exponent of a symbol; the first power is implicit.
func powerSuffix(power int) string {
	if power == 1 {
		return ""
	}
	return superscript.Itoa(power)
}

// partStrings renders every part of a product, bracketing any but the first
// that carries a solidus of its own.
//
// A solidus and a middle dot bind equally and from the left (D12), so a·b/c
// reads back as (a·b)/c — and a product whose second multiplicand is a quotient
// has to say so, or it renders as a unit it is not. The first multiplicand
// needs no brackets: a/b·c already reads as (a/b)·c, which is what it is.
//
// Before repeated multiplicands gathered into powers this went unnoticed,
// because m·m²/s read back as a product of m and m² and rendered the same
// string again. It was two structures for one spelling then and a wrong
// spelling now; the bracket is the fix for both.
func (s Symbol) partStrings() []string {
	out := make([]string, len(s.parts))
	for i, p := range s.parts {
		out[i] = p.String()
		if i > 0 && strings.ContainsRune(out[i], '/') {
			out[i] = "(" + out[i] + ")"
		}
	}
	return out
}

// denominatorString parenthesises a denominator that joins more than one unit,
// so that J/(kg·K) is not read as (J/kg)·K, m/(s/A) not as (m/s)/A, and b/(km/h)
// not as (b/km)/h.
//
// In each case both readings are plausible: a solidus and a middle dot bind
// equally and from the left, which is how the text form of D12 is read back.
// Without the brackets the two would be the same string and one of them would
// come back as a different dimension.
//
// The test is on the rendered text rather than on the form, because a symbol
// that joins two units need not be a product or a quotient: km/h is a static
// symbol and still has to be bracketed. An exponent does not count — m² binds
// to its own symbol and J/m² is unambiguous.
func (s Symbol) denominatorString() string {
	den := s.parts[1].String()
	if composite(den) {
		return "(" + den + ")"
	}
	return den
}

// composite reports whether a rendered symbol joins more than one unit.
func composite(text string) bool {
	return strings.ContainsAny(text, "·/()")
}

// Equal reports whether two symbols render identically in every magnitude.
//
// Symbol is not comparable with == because a product holds a slice, and a
// slice is exactly what a product needs.
func (s Symbol) Equal(other Symbol) bool {
	if s.form != other.form || s.text != other.text || s.power != other.power {
		return false
	}
	if len(s.parts) != len(other.parts) {
		return false
	}
	for i := range s.parts {
		if !s.parts[i].Equal(other.parts[i]) {
			return false
		}
	}
	return true
}
