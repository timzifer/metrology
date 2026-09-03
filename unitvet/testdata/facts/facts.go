// Package facts holds functions whose result is on one and the same scale
// whatever they are called with, and functions that are not.
//
// A function of the first sort exports a fact, which is how a diagnostic
// reaches across a package boundary (D13); the consumer package reads them
// back. Warmer is written before Ambient on purpose: it is determinate only
// once Ambient is, which is what the fixed point in exportFacts is for.
package facts

import (
	"errors"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
	"github.com/timzifer/metrology/units/ratio"
	"github.com/timzifer/metrology/units/temperature"
)

// Warmer is Ambient by another name.
func Warmer() metrology.Measurement { return Ambient() } // want Warmer:"returns Θ¹ absolute"

// Ambient is the temperature of the room.
func Ambient() metrology.Measurement { return temperature.Celsius.Of(20) } // want Ambient:"returns Θ¹ absolute"

// Inlet is the pressure at the inlet.
func Inlet() metrology.Measurement { return pressure.Bar.Of(2.5) } // want Inlet:"returns L⁻¹M¹T⁻² interval"

// Mains is the frequency of the supply.
func Mains() metrology.Measurement { return frequency.Hertz.Of(50) } // want Mains:"returns T⁻¹ interval frequency"

// Decay is a radioactivity scaled by a plain number. The product drops the tag
// (D6), so what the fact carries is the provenance the checker keeps (D16) —
// and it crosses the package boundary with it.
func Decay() metrology.Measurement { // want Decay:"returns T⁻¹ interval from radioactivity"
	scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
	return scaled
}

// Scale is the unit lengths are reported on.
func Scale() metrology.Unit { return length.Metre } // want Scale:"returns L¹ interval"

// Undecided returns one of two scales, so it has none of its own and no fact.
func Undecided(fine bool) metrology.Measurement {
	if fine {
		return pressure.Pascal.Of(1000)
	}
	return length.Metre.Of(1)
}

// Panics never returns at all, which is not the same as returning one scale.
func Panics() metrology.Measurement { panic("no measurement here") }

// FromAParameter is on whichever scale it is handed.
func FromAParameter(u metrology.Unit) metrology.Measurement { return u.Of(1) }

// Count returns a number, Fail an error and Nothing nothing: none of them is a
// scale, and the pass stops before it looks at the body.
func Count() int { return 1 }

func Fail() error { return errors.New("no") }

func Nothing() {}
