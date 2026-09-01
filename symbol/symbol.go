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
func Product(multiplicands ...Symbol) Symbol {
	return Symbol{form: formProduct, parts: append([]Symbol(nil), multiplicands...)}
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

// partStrings renders every part of a product.
func (s Symbol) partStrings() []string {
	out := make([]string, len(s.parts))
	for i, p := range s.parts {
		out[i] = p.String()
	}
	return out
}

// denominatorString parenthesises a product denominator, so that J/(kg·K) is
// not read as (J/kg)·K.
func (s Symbol) denominatorString() string {
	den := s.parts[1]
	if den.form == formProduct {
		return "(" + den.String() + ")"
	}
	return den.String()
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
