package metrology_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/timzifer/metrology"
)

func ExampleUnit_Of() {
	p := Bar.Of(2.5)

	pascals, err := p.In[float64](Pascal)
	fmt.Println(p, "=", pascals, "Pa", err)
	// Output: 2.5 bar = 250000 Pa <nil>
}

func ExampleUnit_OfString() {
	// The exact path: more digits than a float64 holds, none of them lost.
	p, err := Bar.OfString("2.500000000000000000000000000001")
	if err != nil {
		panic(err)
	}
	fmt.Println(p)
	// Output: 2.500000000000000000000000000001 bar
}

func ExampleMeasurement_Add() {
	// 20 °C is a point on a scale, 5 K is a distance along it, and the sum is
	// a point (D6).
	t, err := Celsius.Of(20).Add(Kelvin.Of(5))
	fmt.Println(t, err)

	// The sum of two points is not defined, and saying so is the whole reason
	// the kind exists.
	_, err = Celsius.Of(20).Add(Celsius.Of(5))
	fmt.Println(errors.Is(err, metrology.ErrKind))
	// Output:
	// 25 °C <nil>
	// true
}

func ExampleMeasurement_Sub() {
	// The difference of two points is a span, and it is read on the interval
	// scale: kelvin, not degrees Celsius.
	d, err := Celsius.Of(25).Sub(Celsius.Of(20))
	fmt.Println(d, err)
	// Output: 5 K <nil>
}

func ExampleMeasurement_Div() {
	// A force over an area is a pressure. The result carries no kind and its
	// unit records both operands; converting it into a named unit is a
	// separate, checked step.
	q, err := Newton.Of(100).Div(SquareMetre.Of(2))
	if err != nil {
		panic(err)
	}
	pascals, err := q.In[float64](Pascal)
	fmt.Println(q, "=", pascals, "Pa", err)
	// Output: 50 N/m² = 50 Pa <nil>
}

func ExampleMeasurement_To() {
	// 760 torr is one atmosphere exactly, because the factor is stored as
	// 101325/760 rather than as a rounded decimal (D4).
	atmosphere, err := Torr.Of(760).To(Pascal)
	fmt.Println(atmosphere, err)
	// Output: 101325 Pa <nil>
}

func ExampleDimensionError() {
	_, err := Bar.Of(2.5).Add(Metre.Of(1))

	var de *metrology.DimensionError
	if errors.As(err, &de) {
		fmt.Printf("%s: expected %s, got %s\n", de.Op, de.Want, de.Got)
	}
	// Output: Add: expected L⁻¹M¹T⁻², got L¹
}

func ExampleEngine() {
	// Precision belongs to the computation, not to the value (D9): the same
	// measurement, carried to two different lengths.
	torr := Bar.Of(1)

	rough, _ := metrology.NewEngine(6).To(torr, Torr)
	fine, _ := metrology.NewEngine(30).To(torr, Torr)
	fmt.Println(rough)
	fmt.Println(fine)
	// Output:
	// 750.062 Torr
	// 750.061682704169750801875154207 Torr
}

func ExampleMeasurement_Prefixed() {
	// The canonical form keeps the unit the measurement is held in; the
	// display form picks the prefix that fits.
	p := Pascal.Of(250000)
	fmt.Println(p)
	fmt.Println(p.Prefixed())
	// Output:
	// 250000 Pa
	// 250 kPa
}

func ExampleMeasurement_MarshalText() {
	// The canonical form of D12: the magnitude with the unit it is held in,
	// and every digit the measurement carries.
	m, err := Bar.OfString("2.500000000000000000000000000001")
	if err != nil {
		panic(err)
	}
	text, err := m.MarshalText()
	fmt.Println(string(text), err)
	// Output: 2.500000000000000000000000000001 bar <nil>
}

func ExampleMeasurement_MarshalJSON() {
	// A measurement travels as a string, not as {"value": 2.5, "unit": "bar"}:
	// an object with a number in it puts every consumer back on float64 and
	// loses exactly what this library keeps.
	//
	// Reading the form back needs a catalogue of units, which the core does
	// not have (D7) — that is what the parse package is for.
	data, err := json.Marshal(struct {
		Inlet   metrology.Measurement `json:"inlet"`
		Ambient metrology.Measurement `json:"ambient"`
	}{Inlet: Bar.Of(2.5), Ambient: Celsius.Of(20)})
	fmt.Println(string(data), err)
	// Output: {"inlet":"2.5 bar","ambient":"20 °C"} <nil>
}
