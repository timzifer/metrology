package metrology_test

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology"
)

// The benchmarks behind the numbers in D9 and in the risk table of
// docs/deferred.md. Those documents quote a cost per conversion and a cost per
// precision step; this file is what produces them, so the claim can be rechecked
// on the machine of whoever doubts it rather than believed:
//
//	go test -run '^$' -bench . -benchmem ./...
//
// The conversion benchmarked is bar → torr, the same one D9 quotes: one offset,
// one exact multiplication by 100000·760, one division by 101325 (D4). It is the
// full shape of a conversion, not its cheapest case.
//
// Nothing here asserts. A benchmark that fails is not a defect in the library;
// the D14 correctness weight is in the property, golden and guard tests, and
// pretending otherwise by adding assertions here would only make the numbers
// measure the assertions too.

// The sinks exist so that the compiler cannot delete the work being measured.
// They are package-level because a local variable assigned and never read is
// exactly what a Go compiler is entitled to remove.
var (
	sinkMeasurement metrology.Measurement
	sinkFloat       float64
	sinkString      string
	sinkBytes       []byte
	sinkInt         int
	sinkDecimal     *apd.Decimal
	sinkErr         error
)

// benchPrecisions is the ladder of D9: the default, decimal128, and one step
// past it to show the cost is not a cliff but a slope.
var benchPrecisions = []struct {
	name      string
	precision uint32
}{
	{"20digits", 20},
	{"34digits", 34},
	{"50digits", 50},
}

func BenchmarkConvert(b *testing.B) {
	from := Bar.Of(2.5)

	for _, p := range benchPrecisions {
		engine := metrology.NewEngine(p.precision)
		b.Run(p.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sinkMeasurement, sinkErr = engine.To(from, Torr)
			}
		})
	}

	// The reference D9 measures the decimal cost against: the same conversion
	// in float64, factor pre-divided the way a library without D4 would store
	// it. It is here to keep the comparison honest and reproducible, not
	// because the library offers this path.
	b.Run("float64", func(b *testing.B) {
		b.ReportAllocs()
		v := 2.5
		for b.Loop() {
			sinkFloat = v * 76000000 / 101325
		}
	})
}

// BenchmarkConvertAffine is the conversion D6 exists for: °F → °C carries an
// offset on both ends, so it costs an addition, a scaling and a subtraction
// where the linear case costs one scaling.
func BenchmarkConvertAffine(b *testing.B) {
	from := Fahrenheit.Of(68)

	b.ReportAllocs()
	for b.Loop() {
		sinkMeasurement, sinkErr = from.To(Celsius)
	}
}

// BenchmarkConvertIdentity measures the floor: converting a measurement to the
// unit it is already in still walks the same path.
func BenchmarkConvertIdentity(b *testing.B) {
	from := Bar.Of(2.5)

	b.ReportAllocs()
	for b.Loop() {
		sinkMeasurement, sinkErr = from.To(Bar)
	}
}

func BenchmarkArithmetic(b *testing.B) {
	length := Metre.Of(3.5)
	width := Metre.Of(2.25)
	area := SquareMetre.Of(7.875)
	force := Newton.Of(1250)
	warm := Celsius.Of(20)
	step := Kelvin.Of(5)

	// D9: addition and subtraction of two decimals are exact and never round,
	// which is why they cost visibly less than a product or a quotient.
	b.Run("Add", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = length.Add(width)
		}
	})
	// The affine rule of D6: absolute + interval, the one addition that has to
	// consult a kind before it may proceed.
	b.Run("AddAffine", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = warm.Add(step)
		}
	})
	b.Run("Sub", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = length.Sub(width)
		}
	})
	b.Run("Mul", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = length.Mul(width)
		}
	})
	b.Run("Div", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = force.Div(area)
		}
	})
	b.Run("Cmp", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkInt, sinkErr = length.Cmp(width)
		}
	})
}

// BenchmarkBoundary measures the two crossings a program actually pays for:
// getting a number into a Measurement and getting one back out. Section
// "Working with float64" of README.md quotes these to say where the boundary
// belongs — outside the loop.
func BenchmarkBoundary(b *testing.B) {
	m := Bar.Of(2.5)

	b.Run("OfFloat64", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement = Bar.Of(2.5)
		}
	})
	b.Run("OfInt64", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement = Bar.Of(int64(2))
		}
	})
	// OfString is the exact constructor: no float64 ever holds the value, so
	// 0.1 bar is 0.1 bar and not 0.1000000000000000055511151231257827.
	b.Run("OfString", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = Bar.OfString("2.5")
		}
	})
	// In converts and leaves the exact domain in one call, which is the cost
	// of the conversion plus the cost of the crossing.
	b.Run("InFloat64", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkFloat, sinkErr = m.In[float64](Pascal)
		}
	})
	// Decimal returns a copy taken via Set (D3). The copy is the point: it is
	// what keeps a caller from mutating the interior of a live value.
	b.Run("Decimal", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkDecimal = m.Decimal()
		}
	})
}

// BenchmarkText measures the writing half of D12. Reading is benchmarked in the
// parse package, where the parser lives.
func BenchmarkText(b *testing.B) {
	m := Bar.Of(2.5)
	pascals, err := m.To(Pascal)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkString = m.String()
		}
	})
	// Prefixed picks a prefix by exact decimal arithmetic rather than by a
	// logarithm (D10), so it costs more than String and buys 250 kPa.
	b.Run("Prefixed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkString = pascals.Prefixed()
		}
	})
	b.Run("MarshalText", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBytes, sinkErr = m.MarshalText()
		}
	})
	b.Run("MarshalJSON", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBytes, sinkErr = m.MarshalJSON()
		}
	})
}

// BenchmarkCompose measures unit algebra, which happens once per derived unit
// rather than once per value — the reason it is allowed to be the expensive
// part of the library.
func BenchmarkCompose(b *testing.B) {
	var sinkUnit metrology.Unit

	b.Run("Times", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkUnit, sinkErr = Metre.Times(Metre)
		}
	})
	b.Run("Per", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkUnit, sinkErr = Newton.Per(SquareMetre)
		}
	})
	b.Run("Pow", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkUnit, sinkErr = Metre.Pow(3)
		}
	})
	_ = sinkUnit
}

// BenchmarkKernel is the loop O2 turns on: a window of readings multiplied and
// summed, once with every intermediate a [metrology.Measurement] and once with
// the units left at the boundary.
//
// The two are the same physics and differ by two orders of magnitude, which is
// the whole of the argument in section 10 against an arithmetic passed in from
// outside. A swappable backend would make the magnitudes cheap and leave the
// per-operation unit algebra of Exact standing; Boundary removes both by
// crossing twice instead of 2·window times, and it needs nothing from the
// library that D10 does not already provide.
func BenchmarkKernel(b *testing.B) {
	// 64 readings is a window, not a limit: the ratio between the two is set
	// by the crossings, so it grows with the window rather than levelling off.
	const window = 64

	pressure := Bar.Of(2.5)
	area := SquareMetre.Of(1.25)
	product, err := Bar.Times(SquareMetre)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Exact", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			total := product.Of(0)
			for range window {
				var force metrology.Measurement
				force, sinkErr = pressure.Mul(area)
				total, sinkErr = total.Add(force)
			}
			sinkMeasurement = total
		}
	})

	// The exact domain is left once and re-entered once. Everything between is
	// float64 and carries no unit — which is the trade, and why the boundary
	// belongs where the loop is not.
	b.Run("Boundary", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			p, _ := pressure.In[float64](Bar)
			a, _ := area.In[float64](SquareMetre)
			var total float64
			for range window {
				total += p * a
			}
			sinkMeasurement = product.Of(total)
		}
	})
}
