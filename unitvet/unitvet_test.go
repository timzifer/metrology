package unitvet_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/timzifer/metrology/unitvet"
)

// TestAnalyzer runs the pass over the corpus of D13: one package of patterns
// it must report, one of patterns it must stay silent about, and the fact
// mechanism that carries a scale across a package boundary.
//
// Both directions are the test. The reported half is what the pass is for; the
// silent half is what keeps it usable, and a false positive there is the
// failure that gets a checker switched off for good.
func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), unitvet.Analyzer,
		"corpus/provable",
		"corpus/silent",
		"corpus/facts",
		"corpus/consumer",
		"corpus/ignored",
		"corpus/unrelated",
	)
}
