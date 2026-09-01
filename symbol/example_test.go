package symbol_test

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/timzifer/metrology/symbol"
)

func ExampleSymbol_Scale() {
	pressure := symbol.SI("Pa")
	value, _, _ := apd.NewFromString("250000")

	scaled, text := pressure.Scale(value)
	fmt.Printf("%s %s\n", scaled, text)
	// Output: 250 kPa
}

func ExampleQuotient() {
	speed := symbol.Quotient(symbol.SI("m"), symbol.SI("s"))
	value, _, _ := apd.NewFromString("1500")

	scaled, text := speed.Scale(value)
	fmt.Printf("%s — %s %s\n", speed, scaled, text)
	// Output: m/s — 1.5 km/s
}

func ExampleGram() {
	mass := symbol.Gram()
	value, _, _ := apd.NewFromString("0.0025")

	scaled, text := mass.Scale(value)
	fmt.Printf("%s — %s %s\n", mass, scaled, text)
	// Output: kg — 2.5 g
}

func ExampleStatic() {
	// A static symbol never takes a prefix: 1000 °C is not 1 k°C.
	celsius := symbol.Static("°C")
	value, _, _ := apd.NewFromString("1000")

	scaled, text := celsius.Scale(value)
	fmt.Printf("%s %s\n", scaled, text)
	// Output: 1000 °C
}
