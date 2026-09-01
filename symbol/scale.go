package symbol

import (
	"strings"

	"github.com/cockroachdb/apd/v3"
)

// Scale returns v expressed with the prefix this symbol wants for that
// magnitude, together with the prefixed text: 250000 with the symbol Pa becomes
// 250 and "kPa".
//
// v is never written to (D3): the result is a fresh Decimal. The scaling is a
// shift of the decimal exponent and therefore exact — no digit of v is lost and
// no power of ten lands one unit in the last place below itself, which is what
// a logarithmic prefix search does to 1000 m. Trailing zeros the shift
// introduces are trimmed (D9), so 1000 m yields 1, not 1.000.
func (s Symbol) Scale(v *apd.Decimal) (*apd.Decimal, string) {
	switch s.form {
	case formProduct:
		if len(s.parts) == 0 {
			return copyOf(v), ""
		}
		scaled, head := s.parts[0].Scale(v)
		texts := s.partStrings()
		texts[0] = head
		return scaled, strings.Join(texts, "·")
	case formQuotient:
		scaled, numerator := s.parts[0].Scale(v)
		return scaled, numerator + "/" + s.denominatorString()
	case formSI:
		return s.prefixed(v, siPrefixes, 0)
	case formGram:
		// Magnitudes are in kilograms, prefixes attach to the gram, so the
		// selection happens three decimal places up.
		return s.prefixed(v, siPrefixes, 3)
	case formLitre:
		return s.prefixed(v, litrePrefixes, 0)
	default: // formStatic
		return copyOf(v), s.text
	}
}

// prefixed picks a prefix from table and applies it.
//
// base shifts the magnitude before the prefix is chosen, for symbols whose
// unprefixed form is not the unit the magnitude is measured in — the kilogram.
func (s Symbol) prefixed(v *apd.Decimal, table []prefix, base int) (*apd.Decimal, string) {
	out := copyOf(v)
	e10, ok := decimalExponent(v)
	if !ok {
		// Zero has no order of magnitude, and neither has an infinity or a NaN.
		// Any prefix would be arbitrary, so none is applied.
		return out, s.text + powerSuffix(s.power)
	}
	p := pick(table, e10+base, s.power)
	// Multiplying by a power of ten is a change of the exponent alone, which is
	// why this cannot round.
	out.Exponent += int32(base - p.exponent*s.power)
	trimFraction(out)
	return out, p.name + s.text + powerSuffix(s.power)
}

// trimFraction removes the trailing zeros the shift moved behind the decimal
// point, so that 1000 m prints as 1 km rather than 1.000 km.
//
// It stops at the decimal point rather than reducing all the way (D9): pulling
// 250 up to 2.5E+2 would be the same number written in a form nobody puts on a
// gauge. Trailing zeros carry no information here — the library does not track
// significant figures (D9).
func trimFraction(d *apd.Decimal) {
	var quotient, remainder apd.BigInt
	for d.Exponent < 0 {
		quotient.QuoRem(&d.Coeff, apd.NewBigInt(10), &remainder)
		if remainder.Sign() != 0 {
			return
		}
		d.Coeff.Set(&quotient)
		d.Exponent++
	}
}

// decimalExponent returns floor(log10(|v|)) exactly, and false when v has no
// order of magnitude: zero, an infinity or a NaN.
func decimalExponent(v *apd.Decimal) (int, bool) {
	if v.Form != apd.Finite || v.Coeff.Sign() == 0 {
		return 0, false
	}
	return int(v.NumDigits()) - 1 + int(v.Exponent), true
}

// copyOf returns a Decimal that shares nothing with v.
//
// D3: apd.Decimal shares its coefficient slice on a plain struct copy, so the
// copy must go through Set.
func copyOf(v *apd.Decimal) *apd.Decimal {
	return new(apd.Decimal).Set(v)
}
