package metrology_test

import (
	"errors"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
	"github.com/timzifer/metrology/symbol"
)

func TestNewUnitDefaults(t *testing.T) {
	u, err := metrology.NewUnit(metrology.UnitDef{Dimension: dimension.L, Symbol: symbol.SI("m")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	num, den := u.Factor()
	if num.String() != "1" || den.String() != "1" {
		t.Errorf("factor = %s/%s, want 1/1", num, den)
	}
	if got := u.Offset(); got.String() != "0" {
		t.Errorf("offset = %s, want 0", got)
	}
	if u.Kind() != metrology.Interval {
		t.Errorf("kind = %s, want interval", u.Kind())
	}
	if _, ok := u.IntervalUnit(); ok {
		t.Error("a unit declared without an interval counterpart reports one")
	}
	if u.String() != "m" || u.Dimension() != dimension.L {
		t.Errorf("unit = %s of %s", u, u.Dimension())
	}
	if !u.Symbol().Equal(symbol.SI("m")) {
		t.Error("Symbol() is not the symbol it was built with")
	}
}

func TestNewUnitRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		def  metrology.UnitDef
		want error
	}{
		{"a numerator that is not a number",
			metrology.UnitDef{Numerator: "one"}, metrology.ErrSyntax},
		{"a denominator that is not a number",
			metrology.UnitDef{Denominator: "760ish"}, metrology.ErrSyntax},
		{"an offset that is not a number",
			metrology.UnitDef{Kind: metrology.Absolute, Offset: "freezing"}, metrology.ErrSyntax},
		// A zero numerator collapses every magnitude to zero, a zero
		// denominator converts nothing at all. Neither is a unit.
		{"a zero numerator", metrology.UnitDef{Numerator: "0"}, metrology.ErrZeroFactor},
		{"a zero denominator", metrology.UnitDef{Denominator: "0"}, metrology.ErrZeroFactor},
		// D6: an offset is what makes a scale affine, and an affine scale
		// measures points, not spans.
		{"an offset on an interval unit",
			metrology.UnitDef{Offset: "273.15"}, metrology.ErrOffsetKind},
		{"an absolute unit as the interval counterpart",
			metrology.UnitDef{Dimension: dimension.Θ, Kind: metrology.Absolute, Interval: &Celsius},
			metrology.ErrOffsetKind},
		{"an interval counterpart of another dimension",
			metrology.UnitDef{Dimension: dimension.Θ, Kind: metrology.Absolute, Interval: &Metre},
			metrology.ErrDimension},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := metrology.NewUnit(tc.def); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMustUnitPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustUnit accepted a zero denominator")
		}
		if msg, ok := r.(string); !ok || msg == "" {
			t.Errorf("panic value = %v, want a message", r)
		}
	}()
	metrology.MustUnit(metrology.UnitDef{Denominator: "0"})
}

func TestUnitEqual(t *testing.T) {
	half := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.L, Symbol: symbol.Static("half"), Numerator: "1", Denominator: "2",
	})
	fiveTenths := metrology.MustUnit(metrology.UnitDef{
		Dimension: dimension.L, Symbol: symbol.Static("half"), Numerator: "5", Denominator: "10",
	})
	for _, tc := range []struct {
		name string
		a, b metrology.Unit
		want bool
	}{
		{"a unit equals itself", Bar, Bar, true},
		// The factor is compared as a number, not digit by digit: 1/2 and 5/10
		// are the same scale, however the catalogue happens to write them.
		{"the same ratio written differently", half, fiveTenths, true},
		{"different factors", Bar, Pascal, false},
		{"different dimensions", Metre, Pascal, false},
		{"different symbols", Bar, metrology.MustUnit(metrology.UnitDef{
			Dimension: Bar.Dimension(), Symbol: symbol.Static("b"), Numerator: "100000",
		}), false},
		{"different kinds", Kelvin, KelvinAbsolute, false},
		{"different offsets", Celsius, metrology.MustUnit(metrology.UnitDef{
			Dimension: Celsius.Dimension(), Kind: metrology.Absolute,
			Symbol: symbol.Static("°C"), Offset: "273.16",
		}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.want {
				t.Errorf("Equal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIntervalUnit(t *testing.T) {
	got, ok := Celsius.IntervalUnit()
	if !ok {
		t.Fatal("Celsius declares an interval counterpart but does not report one")
	}
	if !got.Equal(Kelvin) {
		t.Errorf("interval unit = %s, want K", got)
	}
}
