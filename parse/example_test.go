package parse_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/pressure"
	"github.com/timzifer/metrology/symbol"
	"github.com/timzifer/metrology/temperature"
)

func ExampleMeasurement() {
	m, err := parse.Measurement("2.5 bar")
	if err != nil {
		panic(err)
	}
	pascals, err := m.In[float64](pressure.Pascal)
	fmt.Println(m, "=", pascals, "Pa", err)
	// Output: 2.5 bar = 250000 Pa <nil>
}

func ExampleMeasurement_prefix() {
	// A prefix is read off the symbol and applied to the unit exactly: the
	// magnitude keeps its digits and the factor is a power of ten (D4).
	m, err := parse.Measurement("250 kPa")
	if err != nil {
		panic(err)
	}
	pascals, err := m.To(pressure.Pascal)
	fmt.Println(m, "=", pascals, err)
	// Output: 250 kPa = 250000 Pa <nil>
}

func ExampleMeasurement_expression() {
	// A unit nobody named: the symbol is built from its parts, and the result
	// carries no quantity tag — exactly what a computed magnitude carries (D6),
	// so it can still be named.
	m, err := parse.Measurement("50 N/m²")
	if err != nil {
		panic(err)
	}
	named, err := m.To(pressure.Pascal)
	fmt.Println(m, "is", named, err)
	// Output: 50 N/m² is 50 Pa <nil>
}

func ExampleMeasurement_roundTrip() {
	// The text form is canonical (D12): what the library prints, it reads.
	original, err := pressure.Bar.OfString("2.500000000000000000000000000001")
	if err != nil {
		panic(err)
	}
	back, err := parse.Measurement(original.String())
	if err != nil {
		panic(err)
	}
	fmt.Println(back.String() == original.String())
	// Output: true
}

func ExampleUnit() {
	u, err := parse.Unit("J/(kg·K)")
	if err != nil {
		panic(err)
	}
	fmt.Println(u, u.Dimension())
	// Output: J/(kg·K) L²T⁻²Θ⁻¹
}

func ExampleParser_Prefer() {
	// "K" is a temperature and a temperature difference, and the text does not
	// say which. By default it is the difference, which is the reading that
	// composes: 20 °C plus 5 K is 25 °C.
	span, err := parse.Measurement("5 K")
	if err != nil {
		panic(err)
	}
	sum, err := temperature.Celsius.Of(20).Add(span)
	fmt.Println(sum, err)

	// A program reading absolute temperatures says so once, on the parser.
	point, err := parse.Default().Prefer(metrology.Absolute).Measurement("5 K")
	if err != nil {
		panic(err)
	}
	celsius, err := point.To(temperature.Celsius)
	fmt.Println(celsius, err)
	// Output:
	// 25 °C <nil>
	// -268.15 °C <nil>
}

func ExampleNew() {
	// A program with units of its own parses them with the same code as the
	// shipped catalogue: a parser is a value holding what it knows (D7).
	widget := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.One, Symbol: symbol.SI("wdg"),
	})
	p := parse.New([]metrology.Unit{widget})

	m, err := p.Measurement("2.5 kwdg")
	if err != nil {
		panic(err)
	}
	count, err := m.To(widget)
	fmt.Println(m, "=", count, err)
	// Output: 2.5 kwdg = 2500 wdg <nil>
}

func ExampleText() {
	// The standard decoding interfaces are handed no catalogue, so the
	// destination carries one.
	var config struct {
		Setpoint   parse.Text `json:"setpoint"`
		Hysteresis parse.Text `json:"hysteresis"`
	}
	if err := json.Unmarshal([]byte(`{"setpoint":"20 °C","hysteresis":"2 K"}`), &config); err != nil {
		panic(err)
	}
	upper, err := config.Setpoint.Add(config.Hysteresis.Measurement)
	fmt.Println(upper, err)

	out, err := json.Marshal(config)
	fmt.Println(string(out), err)
	// Output:
	// 22 °C <nil>
	// {"setpoint":"20 °C","hysteresis":"2 K"} <nil>
}

func ExampleText_In() {
	// A column that holds the magnitude alone, with the unit in the schema.
	// The driver may hand over a string, a float64 or an int64; what arrives
	// as text keeps every digit.
	column := parse.Text{}.In(pressure.Bar)
	if err := column.Scan("2.5"); err != nil {
		panic(err)
	}
	fmt.Println(column.Measurement)
	// Output: 2.5 bar
}

func ExampleUnknownUnitError() {
	_, err := parse.Measurement("2.5 zzz")

	var ue *parse.UnknownUnitError
	if errors.As(err, &ue) {
		fmt.Printf("%q names no unit\n", ue.Symbol)
	}
	// The text is well formed; what is missing is the unit, not a bracket.
	fmt.Println(errors.Is(err, parse.ErrUnknownUnit), errors.Is(err, metrology.ErrSyntax))
	// Output:
	// "zzz" names no unit
	// true false
}
