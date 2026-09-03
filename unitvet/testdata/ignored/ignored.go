// Package ignored exercises the marker that silences a diagnostic: a test that
// asserts an operation fails is an operation the pass is right to report and
// nobody wants reported.
package ignored

import (
	"github.com/timzifer/metrology/units/activity"
	"github.com/timzifer/metrology/units/duration"
	"github.com/timzifer/metrology/units/frequency"
	"github.com/timzifer/metrology/units/length"
	"github.com/timzifer/metrology/units/pressure"
)

func deliberateMistakes() {
	//unitvet:ignore the point of the assertion is that this fails
	_, _ = frequency.Hertz.Of(50).To(activity.Becquerel)

	_, _ = pressure.Bar.Of(1).Add(length.Metre.Of(1)) //unitvet:ignore and so is this

	// The marker reaches the line it is on and the line below it, and no
	// further: this one is reported.
	_, _ = pressure.Bar.Of(1).Add(duration.Second.Of(1)) // want `Add on incompatible dimensions: L⁻¹M¹T⁻² and T¹`
}
