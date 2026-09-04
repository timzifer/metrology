package gum_test

import (
	"fmt"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/gum"
	"github.com/timzifer/metrology/units/interval"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
)

// A budget for a pressure reading: what was read, what the certificate says
// about the instrument, and what the display's last digit hides.
func Example_budget() {
	// Type A: three readings of the same pressure.
	reading, _ := gum.Sample("repeatability", []metrology.Measurement{
		pressure.Bar.Of(2.501), pressure.Bar.Of(2.499), pressure.Bar.Of(2.500),
	})

	// Type B: the calibration certificate gives U = 0.004 bar at k = 2.
	u, _ := gum.FromExpanded(pressure.Bar.Of(0.004), 2)
	calibration, _ := gum.Of(gum.Input{
		Estimate: pressure.Bar.Of(0), Uncertainty: u, Name: "calibration",
	})

	// Type B: the display rounds to the last digit, so the true value lies
	// anywhere within half of it.
	resolution, _ := gum.Rectangular(pressure.Bar.Of(0.0005))
	display, _ := gum.Of(gum.Input{
		Estimate: pressure.Bar.Of(0), Uncertainty: resolution, Name: "resolution",
	})

	corrected, _ := reading.Add(calibration)
	total, _ := corrected.Add(display)

	fmt.Println(total)
	for _, row := range total.Contributions() {
		fmt.Printf("  %-14s %s\n", row.Source.Name(), row.Value)
	}
	freedom, _ := total.EffectiveFreedom()
	fmt.Println("degrees of freedom:", freedom)

	band, _ := total.Expanded(2)
	fmt.Println("at k = 2:", band)

	// Output:
	// 2.5 bar ± 0.002101586702153081922 bar
	//   repeatability  0.00057735026918962576452 bar
	//   calibration    0.002 bar
	//   resolution     0.00028867513459481288225 bar
	// degrees of freedom: 351
	// at k = 2: [2.495796826595693836156, 2.504203173404306163844] bar
}

// The property that separates this package from the interval layer: a
// difference of one quantity with itself has no uncertainty, because the two
// contributions are the same input's and cancel.
func ExampleValue_Sub() {
	length, _ := gum.Standard(length.Metre.Of(2), length.Metre.Of(0.01))

	difference, _ := length.Sub(length)
	fmt.Println(difference)

	// Output: 0 m ± 0 m
}

// An uncertainty is a span along a scale and never a point on it, so a
// temperature carries its uncertainty in kelvin — and a conversion moves the
// estimate as a point and the uncertainty as a span.
func ExampleValue_To() {
	bath, _ := gum.Standard(temperature.Celsius.Of(20), interval.Kelvin.Of(0.3))

	fmt.Println(bath)
	fahrenheit, _ := bath.To(temperature.Fahrenheit)
	fmt.Println(fahrenheit)

	// Output:
	// 20 °C ± 0.3 K
	// 68 °F ± 0.54 °R
}

// Two quantities measured with one instrument move together, and a budget that
// did not know it would report the wrong number for their sum.
func ExampleCorrelated() {
	first := gum.Input{Estimate: length.Metre.Of(1), Uncertainty: length.Metre.Of(0.01), Name: "first"}
	second := gum.Input{Estimate: length.Metre.Of(2), Uncertainty: length.Metre.Of(0.01), Name: "second"}

	independent, _ := gum.Of(first)
	other, _ := gum.Of(second)
	apart, _ := independent.Add(other)

	x, y, _ := gum.Correlated(first, second, "1")
	together, _ := x.Add(y)

	fmt.Println("independent:", apart)
	fmt.Println("correlated: ", together)

	// Output:
	// independent: 3 m ± 0.014142135623730950489 m
	// correlated:  3 m ± 0.02 m
}
