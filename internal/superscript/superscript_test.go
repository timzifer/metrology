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

func TestAtoi(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"⁰", 0, true},
		{"²", 2, true},
		{"¹⁰", 10, true},
		{"⁻¹", -1, true},
		{"⁻¹²", -12, true},
		{"⁰⁰⁵", 5, true},
		{"", 0, false},
		{"⁻", 0, false},
		{"2", 0, false},        // an ordinary digit is not a superscript one
		{"m²", 0, false},       // and neither is a symbol carrying one
		{"¹⁻", 0, false},       // the minus only leads
		{"⁻⁻¹", 0, false},      // and only once
		{"¹²³⁴⁵⁶⁷⁸", 0, false}, // beyond any exponent this library accepts
	} {
		got, ok := superscript.Atoi(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Atoi(%q) = %d, %v, want %d, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// TestAtoiRoundTrip is the property the two functions owe each other: whatever
// Itoa writes, Atoi reads back.
func TestAtoiRoundTrip(t *testing.T) {
	for n := -300; n <= 300; n++ {
		got, ok := superscript.Atoi(superscript.Itoa(n))
		if !ok || got != n {
			t.Fatalf("Atoi(Itoa(%d)) = %d, %v", n, got, ok)
		}
	}
}

func TestIs(t *testing.T) {
	for _, r := range []rune{'⁰', '⁹', '⁻'} {
		if !superscript.Is(r) {
			t.Errorf("Is(%q) = false, want true", r)
		}
	}
	for _, r := range []rune{'0', '9', '-', 'm', '₂'} {
		if superscript.Is(r) {
			t.Errorf("Is(%q) = true, want false", r)
		}
	}
}
