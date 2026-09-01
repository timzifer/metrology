package decimaltext_test

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/internal/decimaltext"
)

func TestLen(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"2.5", 3},
		{"-40", 3},
		{"+1", 2},
		{".5", 2},
		{"1.", 2},
		{"0", 1},
		{"1e3", 3},
		{"1E+3", 4},
		{"1e-3", 4},
		{"2.5bar", 3},
		{"-40°C", 3},
		// The e of the electronvolt is not an exponent: there are no digits
		// behind it, and swallowing it would lose the unit.
		{"1eV", 1},
		{"1e", 1},
		{"1e+", 1},
		{"NaN", 3},
		{"nan m", 3},
		{"Infinity", 8},
		{"-Infinity", 9},
		{"Inf m", 3},
		{"sNaN", 4},
		{"", 0},
		{"bar", 0},
		{".", 0},
		{"-", 0},
		{"-.", 0},
		{"e3", 0},
		// apd accepts this one and prints it as "0.-1", which reads back as
		// nothing. It is the reason this package exists.
		{".-1", 0},
		{"--1", 0},
	} {
		if got := decimaltext.Len(tc.in); got != tc.want {
			t.Errorf("Len(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestValid(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"2.5", true},
		{"NaN", true},
		{"", false},
		{"2.5bar", false},
		{".-1", false},
		{"1.2.3", false},
	} {
		if got := decimaltext.Valid(tc.in); got != tc.want {
			t.Errorf("Valid(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// The property that matters: everything this package calls a decimal, apd reads
// as one and prints in a form that is a decimal again. Anything else would make
// the text form of D12 a one-way street.
func TestWhatItAcceptsAPDReadsBack(t *testing.T) {
	for _, s := range []string{
		"2.5", "-40", "+1", ".5", "1.", "0", "1e3", "1E+3", "1e-3", "1e100000",
		"NaN", "sNaN", "Infinity", "-Infinity", "inf",
		"1234567890123456789012345678901234567890.12345678901234567890",
	} {
		if !decimaltext.Valid(s) {
			t.Fatalf("%q is not accepted", s)
		}
		d, _, err := apd.NewFromString(s)
		if err != nil {
			t.Fatalf("apd rejects %q: %v", s, err)
		}
		printed := d.Text('f')
		if !decimaltext.Valid(printed) {
			t.Errorf("%q prints as %q, which is not a decimal", s, printed)
		}
	}
}
