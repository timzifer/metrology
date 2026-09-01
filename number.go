package metrology

import (
	"math"
	"reflect"
	"strconv"

	"github.com/cockroachdb/apd/v3"
)

// Numeric is the set of types a magnitude can enter and leave the library as
// (D10).
//
// The core computes exclusively in apd.Decimal; these types exist at the
// boundary, where a caller has a float64 from a sensor or wants an int for a
// loop. Go 1.27 permits generic methods on concrete types, so the conversion
// lives on [Unit.Of] and [Measurement.In] rather than in free functions.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// decimalFromNumber converts a number to a decimal without going through
// float64 unless the number already is one.
//
// A float is rendered with the shortest text that reads back as the same float,
// which is what the caller wrote: 2.5 becomes 2.5, not 2.5000000000000004.
// There is no exact-binary-expansion option here on purpose — a caller who
// needs the digits they typed passes them as text through [Unit.OfString].
//
// A NaN or an infinity becomes the matching decimal form rather than an error,
// so that [Unit.Of] is total. Both then propagate the way they do everywhere
// else in arithmetic, and are visible as such: a NaN prints as NaN, and asking
// for one as an integer is a [RangeError] rather than a zero.
func decimalFromNumber[N Numeric](v N) *apd.Decimal {
	switch kind := reflect.ValueOf(v).Kind(); kind {
	case reflect.Float32, reflect.Float64:
		f := float64(v)
		switch {
		case math.IsNaN(f):
			return &apd.Decimal{Form: apd.NaN}
		case math.IsInf(f, 1):
			return &apd.Decimal{Form: apd.Infinite}
		case math.IsInf(f, -1):
			return &apd.Decimal{Form: apd.Infinite, Negative: true}
		}
		bits := 64
		if kind == reflect.Float32 {
			bits = 32
		}
		// A shortest-round-trip rendering of a finite float always parses.
		d, _, _ := apd.NewFromString(strconv.FormatFloat(f, 'g', -1, bits))
		return d
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// A uint64 above math.MaxInt64 does not fit apd.New, so it takes the
		// text route — exact either way, and only at the boundary.
		d, _, _ := apd.NewFromString(strconv.FormatUint(uint64(v), 10))
		return d
	default:
		return apd.New(int64(v), 0)
	}
}

// numberFromDecimal converts a decimal back to a number, or reports that the
// target type cannot hold it. The magnitude in the error is written plainly —
// a reader chasing "does not fit int8" is not helped by 3E+2.
//
// An integer target rejects a fractional magnitude rather than truncating it,
// and rejects a magnitude outside its range rather than wrapping. A library
// built to deliver the exact value has no business handing back a silently
// altered one.
func numberFromDecimal[N Numeric](op string, d *apd.Decimal) (N, error) {
	var zero N
	switch reflect.ValueOf(zero).Kind() {
	case reflect.Float32, reflect.Float64:
		f, err := d.Float64()
		if err != nil {
			return zero, &RangeError{Op: op, Value: d.Text('f'), Type: reflect.TypeOf(zero).String()}
		}
		return N(f), nil
	default:
		i, err := d.Int64()
		if err != nil {
			return zero, &RangeError{Op: op, Value: d.Text('f'), Type: reflect.TypeOf(zero).String()}
		}
		// Round-trip through the target type: anything that does not survive
		// it was truncated or wrapped, which is the case this rejects.
		n := N(i)
		if int64(n) != i {
			return zero, &RangeError{Op: op, Value: d.Text('f'), Type: reflect.TypeOf(zero).String()}
		}
		return n, nil
	}
}
