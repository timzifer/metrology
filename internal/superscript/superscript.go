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
