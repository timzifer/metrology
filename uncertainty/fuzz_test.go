package uncertainty_test

import (
	"testing"

	"github.com/cockroachdb/apd/v3"

	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/uncertainty"
)

// FuzzRange asserts the property the text form of D12 exists for: whatever this
// package accepts, it writes in a form it reads back as the same range, and
// writing that form again changes nothing.
//
// The seeds are the whole catalogue in all three accepted spellings, plus the
// pathologies a hand-written table would not think of. A crasher found here is
// checked in under testdata/fuzz/FuzzRange and becomes a regression case.
func FuzzRange(f *testing.F) {
	for _, unit := range catalog.Units() {
		r := uncertainty.Of(unit.Of(2.5))
		f.Add(r.String())
		if text, ok := r.PlusMinus(); ok {
			f.Add(text)
		}
	}
	for _, seed := range []string{
		"", " ", "[", "[]", "[,]", "[1]", "[1,2]", "[1, 2] m", "[2, 1] m",
		"[1, 2]m", "[ 1 , 2 ] m", "[1e3, 2e3] Pa", "[-2, 3] m", "[0, 0] m",
		"3.7 ± 0.2 m", "3.7±0.2m", "3.7 +/- 0.2 m", "3.7 +/-0.2 m", "3.7 + 0.2 m",
		"3.7(2) m", "3.7() m", "3.7(-2) m", "3.7(2 m", "370(20) m", "1e3(1) m",
		"NaN(2) m", "NaN ± 1 m", "[NaN, NaN] m", "[Infinity, Infinity] m",
		"20 ± 0.5 °C", "[19.5, 20.5] °C", "[1, 2] J/(kg·K)", "[1, 2] m^99999999999999999999",
		"[1, 2] ", "[1, 2] 1", "[1, 2] %",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		r, err := uncertainty.Parse(text)
		if err != nil {
			return
		}
		// A NaN bound compares equal to nothing, itself included, so the
		// round-trip property is not about it: what is asserted for one is that
		// reading it did not panic and writing it is stable.
		printed := r.String()
		again, err := uncertainty.Parse(printed)
		if err != nil {
			t.Fatalf("%q printed as %q, which does not read back: %v", text, printed, err)
		}
		if third := again.String(); third != printed {
			t.Fatalf("%q printed as %q and then as %q", text, printed, third)
		}
		if !again.Unit().Equal(r.Unit()) {
			t.Fatalf("%q read back on %s, want %s", printed, again.Unit(), r.Unit())
		}
		if again.Kind() != r.Kind() || again.Quantity() != r.Quantity() {
			t.Fatalf("%q read back as %s %s, want %s %s",
				printed, again.Kind(), again.Quantity(), r.Kind(), r.Quantity())
		}
		if r.Lo().Decimal().Form != apd.Finite || r.Hi().Decimal().Form != apd.Finite {
			return
		}
		// Exact digits, not equal quantities: the text form is what it is
		// because it carries the value it was given.
		if cmp, err := again.Lo().Cmp(r.Lo()); err != nil || cmp != 0 {
			t.Fatalf("%q read back with a lower bound of %s, want %s (%v)", printed, again.Lo(), r.Lo(), err)
		}
		if cmp, err := again.Hi().Cmp(r.Hi()); err != nil || cmp != 0 {
			t.Fatalf("%q read back with an upper bound of %s, want %s (%v)", printed, again.Hi(), r.Hi(), err)
		}
	})
}
