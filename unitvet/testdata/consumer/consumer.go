// Package consumer reads the facts exported by package facts. It has no SSA
// for those functions — only what the fact says about them, which is exactly
// as much as can be said across a package boundary (D13).
package consumer

import (
	"github.com/timzifer/metrology/activity"
	"github.com/timzifer/metrology/length"

	"corpus/facts"
)

func acrossAPackageBoundary() {
	_, _ = facts.Ambient().Add(facts.Inlet())   // want `Add on incompatible dimensions: Θ¹ and L⁻¹M¹T⁻²`
	_, _ = facts.Inlet().To(facts.Scale())      // want `To on incompatible dimensions: L⁻¹M¹T⁻² and L¹`
	_, _ = facts.Mains().To(activity.Becquerel) // want `To on incompatible quantities: frequency and radioactivity`
	_, _ = facts.Warmer().Add(facts.Ambient())  // want `Add on incompatible kinds: absolute and absolute; the sum of two points on a scale is not a point on it`
}

// A tag dropped in another package is still a tag this one may not contradict
// (D16). The fact is what carries it here.
func aDroppedTagAcrossAPackageBoundary() {
	_, _ = facts.Decay().Cmp(facts.Mains()) // want `Cmp on incompatible quantities: a magnitude computed from radioactivity and frequency; Mul and Div drop the tag \(D6\), so the run time no longer sees the conflict`
}

// Nothing is known about a function without a fact, and nothing is said.
func withoutAFact() {
	_, _ = facts.Undecided(true).Add(facts.Inlet())
	_, _ = facts.FromAParameter(length.Metre).Add(facts.Inlet())
	_, _ = facts.Panics().Add(facts.Inlet())
}
