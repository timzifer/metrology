package pi

import (
	"math/big"
	"strings"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

// TestDigitsAgreeWithMachin is what makes the constant in pi.go a citation
// rather than a paste. It recomputes every digit from Machin's formula,
//
//	π/4 = 4·arccot(5) − arccot(239)
//
// in integer arithmetic, and compares character by character. A transcription
// error anywhere in the thousand digits fails here, which is the same standard
// D4 holds a catalogue factor to: a number nobody can check is a number nobody
// should trust.
func TestDigitsAgreeWithMachin(t *testing.T) {
	t.Parallel()

	want := machin(fractionDigits)
	if digits != want {
		for i := range min(len(digits), len(want)) {
			if digits[i] != want[i] {
				t.Fatalf("digits differ at index %d: have %q, want %q", i, digits[i], want[i])
			}
		}
		t.Fatalf("digits have length %d, want %d", len(digits), len(want))
	}
}

// TestDigitsCoverMaxPrecision guards the relation between the constant and the
// limit Power enforces: the digits have to outlast the largest request, or the
// error would be the only thing keeping the last place honest.
func TestDigitsCoverMaxPrecision(t *testing.T) {
	t.Parallel()

	significant := len(strings.ReplaceAll(digits, ".", ""))
	if significant <= MaxPrecision {
		t.Fatalf("π has %d significant digits, which does not exceed MaxPrecision %d",
			significant, MaxPrecision)
	}
}

// TestPowerEncloses is the property the interval layer rests on (D15): a
// directed mode gives a bound and not an approximation, at every exponent.
func TestPowerEncloses(t *testing.T) {
	t.Parallel()

	const precision = 25
	for n := 1; n <= 4; n++ {
		reference, err := Power(n, MaxPrecision/2, apd.RoundHalfUp)
		if err != nil {
			t.Fatalf("reference π^%d: %v", n, err)
		}
		lo, err := Power(n, precision, apd.RoundFloor)
		if err != nil {
			t.Fatalf("floor π^%d: %v", n, err)
		}
		hi, err := Power(n, precision, apd.RoundCeiling)
		if err != nil {
			t.Fatalf("ceiling π^%d: %v", n, err)
		}
		if lo.Cmp(reference) > 0 {
			t.Errorf("π^%d rounded down is %s, above the reference %s", n, lo, reference)
		}
		if hi.Cmp(reference) < 0 {
			t.Errorf("π^%d rounded up is %s, below the reference %s", n, hi, reference)
		}
		if lo.Cmp(hi) >= 0 {
			t.Errorf("π^%d: the two bounds %s and %s do not straddle it", n, lo, hi)
		}
	}
}

// TestPowerIsTheConstant checks the first power against the digits themselves,
// so that a Power that computed something else entirely — the right shape, the
// wrong number — cannot pass the enclosure test by being consistently wrong.
func TestPowerIsTheConstant(t *testing.T) {
	t.Parallel()

	got, err := Power(1, 20, apd.RoundFloor)
	if err != nil {
		t.Fatalf("Power: %v", err)
	}
	const want = "3.1415926535897932384"
	if got.Text('f') != want {
		t.Errorf("π to 20 digits is %s, want %s", got.Text('f'), want)
	}
}

func TestPowerRejects(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		n    int
		prec uint32
		want error
	}{
		{"an exponent of zero", 0, 20, ErrExponent},
		{"a negative exponent", -1, 20, ErrExponent},
		{"an exponent past the widest two int8 can differ by", MaxExponent + 1, 20, ErrExponent},
		{"a precision past the digits that exist", 1, MaxPrecision + 1, ErrPrecision},
	} {
		if _, err := Power(c.n, c.prec, apd.RoundHalfUp); err != c.want {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// fractionDigits is how many digits the constant holds after the point.
const fractionDigits = 1010

// machin computes π to n places after the decimal point, as text.
//
// Twenty guard digits absorb the truncation of every term, and the series is
// alternating with terms falling by a factor of 25 and of 57121, so the tail
// past the guard cannot reach the last digit returned.
func machin(n int) string {
	const guard = 20
	unity := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n+guard)), nil)

	pi := new(big.Int).Mul(big.NewInt(4), arccot(5, unity))
	pi.Sub(pi, arccot(239, unity))
	pi.Mul(pi, big.NewInt(4))
	pi.Quo(pi, new(big.Int).Exp(big.NewInt(10), big.NewInt(guard), nil))

	s := pi.String()
	return s[:1] + "." + s[1:]
}

// arccot returns arccot(x) scaled by unity, from the Gregory series
// 1/x − 1/3x³ + 1/5x⁵ − …
func arccot(x int64, unity *big.Int) *big.Int {
	bx := big.NewInt(x)
	square := new(big.Int).Mul(bx, bx)

	term := new(big.Int).Quo(unity, bx)
	total := new(big.Int).Set(term)
	for k := int64(1); ; k++ {
		term.Quo(term, square)
		if term.Sign() == 0 {
			return total
		}
		part := new(big.Int).Quo(term, big.NewInt(2*k+1))
		if k%2 == 1 {
			total.Sub(total, part)
		} else {
			total.Add(total, part)
		}
	}
}
