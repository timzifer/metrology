package metrology_test

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"testing"

	"github.com/timzifer/metrology"
)

// The interfaces the text form of D12 is built on, asserted at compile time so
// that a rename cannot quietly drop one.
var (
	_ encoding.TextMarshaler = metrology.Measurement{}
	_ json.Marshaler         = metrology.Measurement{}
	_ driver.Valuer          = metrology.Measurement{}
	_ encoding.TextMarshaler = metrology.Unit{}
)

func TestMeasurementMarshalText(t *testing.T) {
	text, err := Bar.Of(2.5).MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(text), "2.5 bar"; got != want {
		t.Errorf("MarshalText = %q, want %q", got, want)
	}
}

// The text form keeps the digits it was given. That is the whole argument of
// D12: a JSON object with a number in it would have lost them here.
func TestMeasurementMarshalTextIsExact(t *testing.T) {
	m, err := Bar.OfString("2.500000000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	text, err := m.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(text), "2.500000000000000000000000000001 bar"; got != want {
		t.Errorf("MarshalText = %q, want %q", got, want)
	}
}

func TestMeasurementMarshalJSON(t *testing.T) {
	data, err := json.Marshal(struct {
		P metrology.Measurement `json:"p"`
		T metrology.Measurement `json:"t"`
	}{P: Bar.Of(2.5), T: Celsius.Of(-40)})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"p":"2.5 bar","t":"-40 °C"}`; got != want {
		t.Errorf("json.Marshal = %s, want %s", got, want)
	}
}

func TestMeasurementValue(t *testing.T) {
	v, err := Bar.Of(2.5).Value()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v, driver.Value("2.5 bar"); got != want {
		t.Errorf("Value = %v, want %v", got, want)
	}
}

func TestUnitMarshalText(t *testing.T) {
	for _, tc := range []struct {
		unit metrology.Unit
		want string
	}{
		{Bar, "bar"},
		{Celsius, "°C"},
		{SquareMetre, "m²"},
	} {
		text, err := tc.unit.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(text); got != tc.want {
			t.Errorf("MarshalText = %q, want %q", got, tc.want)
		}
	}
}
