package superscript_test

import (
	"testing"

	"github.com/timzifer/metrology/internal/superscript"
)

func TestItoa(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{
		{0, "⁰"},
		{1, "¹"},
		{2, "²"},
		{3, "³"},
		{9, "⁹"},
		{10, "¹⁰"},
		{12, "¹²"},
		{-1, "⁻¹"},
		{-12, "⁻¹²"},
		{1234567890, "¹²³⁴⁵⁶⁷⁸⁹⁰"},
		{-1234567890, "⁻¹²³⁴⁵⁶⁷⁸⁹⁰"},
	} {
		if got := superscript.Itoa(tc.in); got != tc.want {
			t.Errorf("Itoa(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The minus must be U+207B, not the ASCII hyphen: next to superscript digits an
// ASCII '-' sits at baseline height and reads as a dash between two symbols.
func TestMinusIsSuperscript(t *testing.T) {
	for _, r := range superscript.Itoa(-2) {
		if r == '-' {
			t.Error("Itoa(-2) contains an ASCII hyphen")
		}
	}
}
