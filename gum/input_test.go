package gum_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/temperature"
)

// A Type A evaluation, on five observations whose mean and spread are checkable
// by hand: the deviations are 0, ±0.1 and ±0.2, so Σd² is 0.1, the variance of
// the mean is 0.1/(5·4) and the standard uncertainty is its root.
func TestSample(t *testing.T) {
	observations := []metrology.Measurement{
		of(t, length.Metre, "10.0"),
		of(t, length.Metre, "10.2"),
		of(t, length.Metre, "9.8"),
		of(t, length.Metre, "10.1"),
		of(t, length.Metre, "9.9"),
	}

	v, err := gum.Sample("repeatability", observations)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if got, want := v.Estimate().String(), "10 m"; got != want {
		t.Errorf("mean = %s, want %s", got, want)
	}
	if got, want := combined(t, v), "0.070710678118654752441 m"; got != want {
		t.Errorf("u = %s, want %s (√0.005)", got, want)
	}
	if got, want := v.Contributions()[0].Source.Freedom(), 4; got != want {
		t.Errorf("degrees of freedom = %d, want %d", got, want)
	}
	if got, want := v.Contributions()[0].Source.Name(), "repeatability"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}

	// The observations need not be on one unit: the first one names the scale
	// and the rest are converted onto it.
	mixed, err := gum.Sample("mixed", []metrology.Measurement{
		of(t, length.Metre, "1000"),
		of(t, length.Kilometre, "1"),
	})
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if got, want := mixed.Estimate().String(), "1000 m"; got != want {
		t.Errorf("mean = %s, want %s", got, want)
	}
	if got, want := combined(t, mixed), "0 m"; got != want {
		t.Errorf("u = %s, want %s — two spellings of one length do not disagree", got, want)
	}
}

// A mean of temperatures is a temperature: the estimate is taken on the scale's
// own coordinates, because the sum of two points on a scale is not a point on
// it (D6) and an affine map preserves a mean.
func TestSampleOnAnAbsoluteScale(t *testing.T) {
	v, err := gum.Sample("bath", []metrology.Measurement{
		of(t, temperature.Celsius, "19.8"),
		of(t, temperature.Celsius, "20.0"),
		of(t, temperature.Celsius, "20.2"),
	})
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if got, want := v.Estimate().String(), "20 °C"; got != want {
		t.Errorf("mean = %s, want %s", got, want)
	}
	// s = 0.2, so s/√3 = 0.11547…, and it is a span in kelvin.
	if got, want := combined(t, v), "0.11547005383792515291 K"; got != want {
		t.Errorf("u = %s, want %s", got, want)
	}
}

func TestSampleRefuses(t *testing.T) {
	if _, err := gum.Sample("once", []metrology.Measurement{of(t, length.Metre, "1")}); !errors.Is(err, gum.ErrInput) {
		t.Errorf("got %v, want ErrInput", err)
	}
	_, err := gum.Sample("mixed", []metrology.Measurement{
		of(t, length.Metre, "1"), of(t, duration.Second, "1"),
	})
	if !errors.Is(err, metrology.ErrDimension) {
		t.Errorf("got %v, want ErrDimension", err)
	}
}

// The Type B evaluations, each against the divisor the GUM gives for it.
func TestDistributions(t *testing.T) {
	halfWidth := of(t, length.Metre, "0.0005")

	for _, tc := range []struct {
		name string
		got  metrology.Measurement
		err  error
		want string
	}{
		{"rectangular, a/√3", measurementOf(gum.Rectangular(halfWidth)), errorOfMeasurement(gum.Rectangular(halfWidth)),
			"0.00028867513459481288225 m"},
		{"triangular, a/√6", measurementOf(gum.Triangular(halfWidth)), errorOfMeasurement(gum.Triangular(halfWidth)),
			"0.00020412414523193150818 m"},
		{"u-shaped, a/√2", measurementOf(gum.UShaped(halfWidth)), errorOfMeasurement(gum.UShaped(halfWidth)),
			"0.0003535533905932737622 m"},
		{"an expanded uncertainty, U/k", measurementOf(gum.FromExpanded(of(t, length.Metre, "0.002"), 2)), nil,
			"0.001 m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err != nil {
				t.Fatalf("unexpected error: %v", tc.err)
			}
			if got := tc.got.String(); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A half-width is a span along a scale and never a point on it (D6). The core
// enforces that where two measurements meet; here one arrives alone, so this
// layer asks the question itself and reports the core's own error class.
func TestADistributionRefusesAPoint(t *testing.T) {
	point := temperature.Celsius.Of(1)

	for _, err := range []error{
		errorOfMeasurement(gum.Rectangular(point)),
		errorOfMeasurement(gum.Triangular(point)),
		errorOfMeasurement(gum.UShaped(point)),
		errorOfMeasurement(gum.FromExpanded(point, 2)),
	} {
		if !errors.Is(err, metrology.ErrKind) {
			t.Errorf("got %v, want ErrKind", err)
		}
	}
}

// The distributions feed an input, and an input on an absolute scale reads its
// uncertainty on the interval unit the scale declares.
func TestADistributionFeedsAnInput(t *testing.T) {
	u, err := gum.Rectangular(interval.Kelvin.Of(0.5))
	if err != nil {
		t.Fatalf("Rectangular: %v", err)
	}
	v, err := gum.Standard(temperature.Celsius.Of(20), u)
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	if got, want := v.String(), "20 °C ± 0.28867513459481288225 K"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// The engine of this layer is the counterpart of the core's, and it carries the
// same thing: the precision, and nothing else (D9).
func TestEngineCarriesThePrecision(t *testing.T) {
	if got, want := gum.NewEngine(34).Precision(), uint32(34); got != want {
		t.Errorf("precision = %d, want %d", got, want)
	}
	if got, want := (gum.Engine{}).Precision(), uint32(metrology.DefaultPrecision); got != want {
		t.Errorf("the zero engine computes with %d digits, want %d", got, want)
	}

	wide := gum.NewEngine(34)
	x, err := wide.Standard(of(t, length.Metre, "1"), of(t, length.Metre, "1"))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	y, err := wide.Standard(of(t, length.Metre, "1"), of(t, length.Metre, "1"))
	if err != nil {
		t.Fatalf("Standard: %v", err)
	}
	sum, err := wide.Add(x, y)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	u, err := wide.Uncertainty(sum)
	if err != nil {
		t.Fatalf("Uncertainty: %v", err)
	}
	// √2, rounded up at the last of the thirty-four digits: a combined
	// uncertainty never understates itself in the place it reports.
	if got, want := u.String(), "1.414213562373095048801688724209699 m"; got != want {
		t.Errorf("u at 34 digits = %s, want %s", got, want)
	}
}

// The error messages of this package name the input where the caller named it,
// because a budget with nine rows needs to say which one is wrong (D11).
func TestInputErrorMessage(t *testing.T) {
	named := &gum.InputError{Op: "Of", Name: "gauge block", Why: "a negative standard uncertainty"}
	if got, want := named.Error(), "gum: Of: gauge block: a negative standard uncertainty"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	unnamed := &gum.InputError{Op: "FromExpanded", Why: "a coverage factor below one"}
	if got, want := unnamed.Error(), "gum: FromExpanded: a coverage factor below one"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if errors.Is(named, metrology.ErrRange) {
		t.Error("an InputError matches a class it is not")
	}
}

// measurementOf drops the error of a two-result call, for a table that checks
// the value and the error apart.
func measurementOf(m metrology.Measurement, _ error) metrology.Measurement { return m }
