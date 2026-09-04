package parse_test

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/parse"
)

// FuzzMeasurement asserts the two things a parser of untrusted text owes its
// caller: it never panics, and what it accepts it accepts stably.
//
// The second half is the interesting one. A parser that reads "1 kPa" as
// something whose own printed form it then rejects — or reads back as a
// different quantity — has no canonical form, and D12 rests on there being one.
// Every accepted input is therefore printed and read again, and the two have to
// agree on unit, kind, quantity and value.
func FuzzMeasurement(f *testing.F) {
	for _, unit := range catalog.Units() {
		f.Add(unit.Of(2.5).String())
		f.Add(unit.Of(0.000001).Prefixed())
	}
	for _, seed := range []string{
		"", " ", "2.5", "bar", "2.5 bar", "-40°C", "1e3 Pa", "250 kPa", "1 J/(kg·K)",
		"50 N/m²", "1 m^-3", "1 (m/s)²", "1 m/(s/A)", "1 N·m·s⁻¹", "2.5 1", "NaN bar",
		"1 m⁻", "1 m^", "1 ()", "1 m^99999999999999999999", "1e-9999999 m", "1 %",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		m, err := parse.Measurement(text)
		if err != nil {
			return
		}
		// A NaN is not equal to itself, so it cannot be compared with itself
		// either. That it parsed and printed without panicking is the whole
		// claim this case supports.
		if m.Decimal().Form != apd.Finite {
			return
		}
		printed := m.String()
		again, err := parse.Measurement(printed)
		if err != nil {
			t.Fatalf("%q printed as %q, which does not parse: %v", text, printed, err)
		}
		// The unit has to come back as the same scale, compared by what it
		// says rather than by how the expression that produced it was
		// bracketed: "A·(m/s)" and "A·m/s" are one unit written twice, and the
		// text form of D12 keeps the unit, not the brackets.
		if again.Unit().String() != m.Unit().String() {
			t.Fatalf("%q printed as %q, which is a %s and not a %s",
				text, printed, again.Unit(), m.Unit())
		}
		if again.Dimension() != m.Dimension() {
			t.Fatalf("%q printed as %q, which is %s and not %s",
				text, printed, again.Dimension(), m.Dimension())
		}
		if again.Kind() != m.Kind() || again.Quantity() != m.Quantity() {
			t.Fatalf("%q printed as %q, which is a %s %q and not a %s %q",
				text, printed, again.Kind(), again.Quantity(), m.Kind(), m.Quantity())
		}
		if !sameScale(again.Unit(), m.Unit()) {
			t.Fatalf("%q printed as %q, which is read on a different scale", text, printed)
		}
		// Compared digit for digit rather than with Equal, which converts —
		// and a conversion between two scales that are numerically the same
		// but not literally the same unit still rounds to the engine's
		// precision (D9). The property here is about the text, so the
		// comparison has to be the exact one.
		if again.Decimal().Cmp(m.Decimal()) != 0 {
			t.Fatalf("%q printed as %q, which is %q", text, printed, again)
		}
		if third := again.String(); third != printed {
			t.Fatalf("%q printed as %q and then as %q", text, printed, third)
		}
	})
}

// sameScale reports whether two units read a magnitude the same way: the same
// exact factor and offset, and the same symbol on the page.
//
// It is not [metrology.Unit.Equal], which compares the symbol as a structure —
// "A·(m/s)" and "A·m/s" are one scale written twice, and the text form keeps the
// scale rather than the brackets.
func sameScale(a, b metrology.Unit) bool {
	if a.String() != b.String() || a.Offset().Cmp(b.Offset()) != 0 {
		return false
	}
	af, bf := a.Factor(), b.Factor()
	if af.Pi != bf.Pi {
		return false
	}
	aNum, aDen := af.Num, af.Den
	bNum, bDen := bf.Num, bf.Den
	var left, right apd.Decimal
	ctx := apd.BaseContext
	ctx.Precision = 0
	// An exact multiplication of two finite decimals cannot fail, and cross
	// multiplication compares the two fractions without a division that could
	// round.
	_, _ = ctx.Mul(&left, aNum, bDen)
	_, _ = ctx.Mul(&right, bNum, aDen)
	return left.Cmp(&right) == 0
}
