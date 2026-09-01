package metrology_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
)

func TestOf(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  metrology.Measurement
		want string
	}{
		{"int", Metre.Of(3), "3 m"},
		{"negative int", Metre.Of(-3), "-3 m"},
		{"int8", Metre.Of(int8(-128)), "-128 m"},
		{"uint64 beyond int64", Metre.Of(uint64(math.MaxUint64)), "18446744073709551615 m"},
		// A float enters at the shortest decimal that reads back as the same
		// float — the number the caller wrote, not its binary expansion.
		{"float64", Metre.Of(2.5), "2.5 m"},
		{"float64 with a long expansion", Metre.Of(0.1), "0.1 m"},
		{"float32", Metre.Of(float32(0.1)), "0.1 m"},
		{"a named numeric type", Metre.Of(centimetres(7)), "7 m"},
		{"NaN is carried, not rejected", Metre.Of(math.NaN()), "NaN m"},
		{"infinity likewise", Metre.Of(math.Inf(1)), "Infinity m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got.String() != tc.want {
				t.Errorf("got %s, want %s", tc.got, tc.want)
			}
		})
	}
}

type centimetres int32

// The exact path: digits given as text are the digits stored, with no float64
// in between to lose them.
func TestOfString(t *testing.T) {
	const digits = "2.500000000000000000000000000000000001"
	m, err := Bar.OfString(digits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.String(); got != digits+" bar" {
		t.Errorf("got %s, want %s bar", got, digits)
	}

	if _, err := Bar.OfString("two and a half"); !errors.Is(err, metrology.ErrSyntax) {
		t.Errorf("error = %v, want ErrSyntax", err)
	}
	var se *metrology.SyntaxError
	_, err = Bar.OfString("½")
	if !errors.As(err, &se) || se.Input != "½" || se.Unwrap() == nil {
		t.Errorf("error = %v, want a *SyntaxError naming its input", err)
	}
}

func TestIn(t *testing.T) {
	t.Run("float", func(t *testing.T) {
		got, err := Bar.Of(2.5).In[float64](Pascal)
		if err != nil || got != 250000 {
			t.Errorf("got %v, %v; want 250000", got, err)
		}
	})
	t.Run("integer", func(t *testing.T) {
		got, err := Bar.Of(2).In[int](Pascal)
		if err != nil || got != 200000 {
			t.Errorf("got %v, %v; want 200000", got, err)
		}
	})
	t.Run("a fractional magnitude is not truncated to an integer", func(t *testing.T) {
		_, err := Metre.Of(2.5).In[int](Metre)
		if !errors.Is(err, metrology.ErrRange) {
			t.Errorf("error = %v, want ErrRange", err)
		}
	})
	t.Run("an out-of-range magnitude is not wrapped", func(t *testing.T) {
		_, err := Metre.Of(300).In[int8](Metre)
		var re *metrology.RangeError
		if !errors.As(err, &re) {
			t.Fatalf("error = %v, want a *RangeError", err)
		}
		if re.Type != "int8" || re.Value != "300" {
			t.Errorf("error = %+v, want it to name the value and the type", re)
		}
	})
	t.Run("a negative magnitude does not become a large unsigned one", func(t *testing.T) {
		_, err := Metre.Of(-1).In[uint32](Metre)
		if !errors.Is(err, metrology.ErrRange) {
			t.Errorf("error = %v, want ErrRange", err)
		}
	})
	t.Run("beyond int64", func(t *testing.T) {
		big := mustOf(Metre, "1e30")
		if _, err := big.In[int64](Metre); !errors.Is(err, metrology.ErrRange) {
			t.Errorf("error = %v, want ErrRange", err)
		}
		got, err := big.In[float64](Metre)
		if err != nil || got != 1e30 {
			t.Errorf("got %v, %v; want 1e30", got, err)
		}
	})
	t.Run("the dimension is checked first", func(t *testing.T) {
		if _, err := Bar.Of(1).In[float64](Metre); !errors.Is(err, metrology.ErrDimension) {
			t.Errorf("error = %v, want ErrDimension", err)
		}
	})
}

func TestDecimalIn(t *testing.T) {
	got, err := Torr.Of(760).DecimalIn(Pascal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "101325" {
		t.Errorf("got %s, want 101325", got)
	}
	if _, err := Torr.Of(760).DecimalIn(Metre); !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("error = %v, want ErrDimension", err)
	}
}

func TestAccessors(t *testing.T) {
	m := Celsius.Of(20)
	if !m.Unit().Equal(Celsius) {
		t.Error("Unit() is not the unit it was built with")
	}
	if m.Dimension() != Celsius.Dimension() {
		t.Error("Dimension() disagrees with the unit")
	}
	if m.Kind() != metrology.Absolute {
		t.Errorf("Kind() = %s, want absolute", m.Kind())
	}
	if got := m.Decimal().String(); got != "20" {
		t.Errorf("Decimal() = %s, want 20", got)
	}
}

func TestPrefixed(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    metrology.Measurement
		want string
	}{
		{"pascal takes a prefix", Pascal.Of(250000), "250 kPa"},
		{"and so does the base unit", Metre.Of(1500), "1.5 km"},
		{"a static symbol never does", Bar.Of(2500), "2500 bar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.Prefixed(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// D3, the invariant everything else rests on. apd.Decimal shares its
// coefficient slice with every struct copy of itself, so a single in-place
// write reaches every measurement derived from the same value.
//
// 200 digits on purpose: apd/v3 stores coefficients up to 38 digits inline,
// which hides the aliasing at the sizes a test would otherwise use. This is the
// bug that passes tests and fails in production, and shortening these values is
// how it comes back.
func TestNoAliasing(t *testing.T) {
	digits := strings.Repeat("1234567890", 20)
	original := mustOf(Metre, digits)
	before := original.String()

	t.Run("Decimal hands out a copy", func(t *testing.T) {
		d := original.Decimal()
		d.Coeff.SetInt64(1)
		d.Exponent = 0
		if original.String() != before {
			t.Fatalf("writing to the result of Decimal() changed the measurement:\n%s", original)
		}
	})

	t.Run("a copied measurement is independent", func(t *testing.T) {
		copied := original
		d := copied.Decimal()
		d.Coeff.SetInt64(7)
		if original.String() != before || copied.String() != before {
			t.Fatal("a copy shares storage with its original")
		}
	})

	t.Run("arithmetic does not write to its operands", func(t *testing.T) {
		other := mustOf(Metre, digits)
		if _, err := original.Add(other); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.Sub(other); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.Mul(other); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := original.To(Kilometre); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if original.String() != before || other.String() != before {
			t.Fatalf("an operand was modified:\n%s\n%s", original, other)
		}
	})

	t.Run("a unit does not hand out its own factor", func(t *testing.T) {
		num, den := Torr.Factor()
		num.Coeff.SetInt64(1)
		den.Coeff.SetInt64(1)
		offset := Celsius.Offset()
		offset.Coeff.SetInt64(0)

		got, err := Torr.Of(760).To(Pascal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "101325 Pa" {
			t.Errorf("the torr changed to %s after its factor was written to", got)
		}
		if c, err := Celsius.Of(0).To(KelvinAbsolute); err != nil || c.String() != "273.15 K" {
			t.Errorf("the celsius offset changed: %s, %v", c, err)
		}
	})

	t.Run("the decimal handed out is not the one stored", func(t *testing.T) {
		// The guard above proves the copy is deep. This proves it is a copy at
		// all rather than the same pointer, which is the cheaper mistake.
		first := original.Decimal()
		second := original.Decimal()
		if first == second {
			t.Fatal("Decimal() returned the same pointer twice")
		}
		if first.Cmp(second) != 0 {
			t.Fatal("two copies of the same magnitude differ")
		}
	})
}

// Reading a magnitude out and putting it back must not change it, whatever
// route it takes.
func TestDecimalRoundTrip(t *testing.T) {
	const digits = "123456789012345678901234567890.123456789"
	m := mustOf(Pascal, digits)
	d := m.Decimal()

	again, err := Pascal.OfString(d.Text('f'))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := again.String(); got != digits+" Pa" {
		t.Errorf("round trip gave %s", got)
	}

	var raw apd.Decimal
	raw.Set(d)
	if raw.Cmp(m.Decimal()) != 0 {
		t.Error("the decimal changed on the way out")
	}
}

// OfString takes the shape of a number seriously, because apd does not: it
// accepts ".-1" and yields a decimal that prints as "0.-1", and a value whose
// own text form is not a value would break the round trip D12 rests on.
func TestOfStringRejectsWhatIsNotANumber(t *testing.T) {
	for _, s := range []string{"", ".-1", "bar", "1.2.3", "2.5 bar", "--1", "1,5", "0x10"} {
		m, err := Bar.OfString(s)
		if err == nil {
			t.Errorf("OfString(%q) = %q, want an error", s, m)
			continue
		}
		if !errors.Is(err, metrology.ErrSyntax) {
			t.Errorf("OfString(%q) error = %v, want ErrSyntax", s, err)
		}
	}
}

// What it does accept is everything the library can print, the two magnitudes
// that are not numbers included.
func TestOfStringAcceptsWhatItPrints(t *testing.T) {
	for _, s := range []string{"2.5", "-40", ".5", "1e3", "NaN", "Infinity", "-Infinity"} {
		m, err := Bar.OfString(s)
		if err != nil {
			t.Errorf("OfString(%q): %v", s, err)
			continue
		}
		if _, err := Bar.OfString(m.Decimal().Text('f')); err != nil {
			t.Errorf("OfString(%q) printed as %q, which does not read back: %v", s, m, err)
		}
	}
}

// An exponent apd cannot hold is rejected by apd itself, and the error says so
// rather than being swallowed into a shape complaint.
func TestOfStringExponentRange(t *testing.T) {
	if _, err := Bar.OfString("1e200000"); !errors.Is(err, metrology.ErrSyntax) {
		t.Errorf("error = %v, want ErrSyntax", err)
	}
}

// A zero carries no order of magnitude, and apd prints one with a positive
// exponent by expanding it into digits: 0E+1 comes out as "00", which reads
// back as "0" and breaks the fixpoint the text form of D12 promises. Found by
// the parser's fuzz test.
func TestZeroPrintsAsOneZero(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0E1", "0 bar"},
		{"0E100", "0 bar"},
		{"-0E2", "-0 bar"},
		{"0", "0 bar"},
		{"0.00", "0.00 bar"}, // digits behind the point are the caller's
	} {
		m, err := Bar.OfString(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := m.String(); got != tc.want {
			t.Errorf("OfString(%q) prints as %q, want %q", tc.in, got, tc.want)
		}
	}
}
