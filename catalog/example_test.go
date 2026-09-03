package catalog_test

import (
	"fmt"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/catalog"
	"github.com/timzifer/metrology/units/area"
	"github.com/timzifer/metrology/units/force"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/temperature"
)

func ExampleCanonical() {
	// A quotient carries the dimension of a pressure but not the name: its
	// unit records where it came from, N/m². The catalogue is what turns that
	// back into a pascal.
	q, err := force.Newton.Of(100).Div(area.SquareMetre.Of(2))
	if err != nil {
		panic(err)
	}

	unit, ok := catalog.Canonical(q.Dimension(), q.Kind(), q.Quantity())
	if !ok {
		panic("no canonical unit")
	}
	named, err := q.To(unit)
	fmt.Println(q, "is", named, err)
	// Output: 50 N/m² is 50 Pa <nil>
}

func ExampleBySymbol() {
	// Reading a unit out of a configuration file, where it is a string.
	unit, ok := catalog.BySymbol("bar", metrology.Interval)
	if !ok {
		panic("unknown unit")
	}

	p := unit.Of(2.5)
	pascals, err := p.In[float64](pressure.Pascal)
	fmt.Println(p, "=", pascals, "Pa", err)
	// Output: 2.5 bar = 250000 Pa <nil>
}

func ExampleBySymbol_kind() {
	// "K" is two units: a temperature and a temperature difference. Which one
	// a text means is not in the text.
	point, _ := catalog.BySymbol("K", metrology.Absolute)
	span, _ := catalog.BySymbol("K", metrology.Interval)

	sum, err := point.Of(293.15).Add(span.Of(5))
	fmt.Println(sum, err)

	// The same two units the other way round is meaningless, and says so.
	_, err = point.Of(293.15).Add(point.Of(5))
	fmt.Println(err)
	// Output:
	// 298.15 K <nil>
	// metrology: Add: absolute and absolute: the sum of two points on a scale is not a point on it
}

func ExampleUnits() {
	fmt.Println(len(catalog.Units()), "units in the catalogue")
	// Output: 82 units in the catalogue
}

func ExampleCanonical_quantity() {
	// T⁻¹ is a frequency and a radioactivity. Which unit the catalogue hands
	// back depends on which of the two you say you mean.
	hz, _ := catalog.Canonical(frequency.Hertz.Dimension(), metrology.Interval, "frequency")
	bq, _ := catalog.Canonical(frequency.Hertz.Dimension(), metrology.Interval, "radioactivity")
	fmt.Println(hz, bq)

	// And a magnitude in one is not a magnitude in the other.
	_, err := frequency.Hertz.Of(50).To(bq)
	fmt.Println(err)
	// Output:
	// Hz Bq
	// metrology: To: frequency and radioactivity share a dimension but are different quantities
}

func Example_quantityPackages() {
	// The everyday path does not touch this package at all: the unit is known
	// at compile time, and autocompletion in the quantity package is the
	// catalogue.
	t, err := temperature.Celsius.Of(20).To(temperature.Fahrenheit)
	fmt.Println(t, err)
	// Output: 68 °F <nil>
}
