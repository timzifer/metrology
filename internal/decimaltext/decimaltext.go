// Package decimaltext measures the decimal number at the start of a string.
//
// Two callers need the same answer. The core reads a magnitude out of text and
// has to reject what is not one; the parser of the text form has to find where
// the magnitude ends and the unit begins, because "1eV" is one electronvolt and
// "1e3 Pa" is a thousand pascals.
//
// It exists because apd is lenient in one place that matters: ".-1" parses, and
// the decimal it yields prints as "0.-1", which reads back as nothing at all. A
// text form that is canonical (D12) cannot contain a value whose own printed
// form is not a value, so both callers measure the number themselves and hand
// apd only what is one.
package decimaltext

import "strings"

// specials are the magnitudes that are not numbers. apd writes NaN and Infinity
// and reads them back in any case, and a measurement that carries one prints it
// — so the text form has to be able to read it too.
var specials = []string{"infinity", "inf", "snan", "nan"}

// Len returns the length in bytes of the decimal at the start of s, and zero
// where s does not start with one.
//
// The grammar is the one apd writes: an optional sign, digits with at most one
// decimal point among them, and an exponent introduced by e or E. The exponent
// is only part of the number when it is one — in "1eV" the e starts the
// electronvolt, and taking it for an exponent would swallow the unit.
func Len(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if n := specialLen(s[i:]); n > 0 {
		return i + n
	}
	digits := 0
	for i < len(s) && isDigit(s[i]) {
		i, digits = i+1, digits+1
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i, digits = i+1, digits+1
		}
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && isDigit(s[j]) {
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			i = j
		}
	}
	return i
}

// Valid reports whether the whole of s is one decimal.
func Valid(s string) bool { return s != "" && Len(s) == len(s) }

// specialLen returns the length of the non-numeric magnitude at the start of s.
func specialLen(s string) int {
	for _, word := range specials {
		if len(s) >= len(word) && strings.EqualFold(s[:len(word)], word) {
			return len(word)
		}
	}
	return 0
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }
