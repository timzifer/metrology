// Package superscript renders integers as Unicode superscript digits.
//
// Both the dimension stringer (L²M¹T⁻²) and the symbol stringer (m²) need it,
// and it is small enough that pulling in a dependency for it would cost more
// than it saves.
package superscript

import "strings"

// digits maps '0'…'9' and '-' onto their superscript code points. The minus is
// U+207B SUPERSCRIPT MINUS, not U+002D, because the ASCII hyphen renders at
// baseline height next to the digits.
var digits = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
	'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'-': '⁻',
}

// Itoa renders n in superscript digits.
func Itoa(n int) string {
	var b strings.Builder
	if n < 0 {
		b.WriteRune(digits['-'])
	}
	var stack [20]rune
	i := len(stack)
	for {
		i--
		d := n % 10
		if d < 0 {
			d = -d
		}
		stack[i] = digits[rune('0'+d)]
		n /= 10
		if n == 0 {
			break
		}
	}
	b.WriteString(string(stack[i:]))
	return b.String()
}

// values maps each superscript rune back onto what it stands for. It is the
// inverse of digits, written out rather than derived at init time: a package
// built from another package-level variable at init is exactly the kind of
// ordering dependency D7 keeps out of this library.
var values = map[rune]int{
	'⁰': 0, '¹': 1, '²': 2, '³': 3, '⁴': 4,
	'⁵': 5, '⁶': 6, '⁷': 7, '⁸': 8, '⁹': 9,
}

// maxAtoi bounds what [Atoi] reads, so that a long run of digits cannot
// overflow the accumulator. Every exponent this library accepts is far below
// it — a dimension exponent is an int8 — so the bound is reached only by input
// that is nonsense anyway.
const maxAtoi = 1 << 20

// Atoi reads an integer written in superscript digits, as [Itoa] writes it.
//
// It reports false for everything else: an empty string, a lone minus, an
// ordinary ASCII digit, or a run so long that no exponent could mean it. The
// parser of the text form reads the exponent of a unit symbol with it, where an
// unreadable exponent has to be an error rather than a zero.
func Atoi(s string) (int, bool) {
	runes := []rune(s)
	negative := len(runes) > 0 && runes[0] == digits['-']
	if negative {
		runes = runes[1:]
	}
	if len(runes) == 0 {
		return 0, false
	}
	n := 0
	for _, r := range runes {
		value, ok := values[r]
		if !ok {
			return 0, false
		}
		n = n*10 + value
		if n > maxAtoi {
			return 0, false
		}
	}
	if negative {
		return -n, true
	}
	return n, true
}

// Is reports whether r is one of the runes [Itoa] writes: a superscript digit
// or the superscript minus.
//
// A lexer needs it to find where an exponent begins, which is a question about
// one rune and not about a whole string.
func Is(r rune) bool {
	_, ok := values[r]
	return ok || r == digits['-']
}
