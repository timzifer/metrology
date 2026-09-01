// Command unitvet is the static dimension checker of D13, packaged as a vet
// tool.
//
// It reports additions, subtractions, comparisons and conversions across
// incompatible dimensions — and the affine mistakes of D6, such as adding two
// temperatures — without running the code:
//
//	go install github.com/timzifer/metrology/cmd/unitvet@latest
//	go vet -vettool=$(go env GOPATH)/bin/unitvet ./...
//
//	app/app.go:12:33: Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹
//
// It reports only what it can prove and stays silent otherwise, so it composes
// with an existing CI setup instead of producing noise. The rules, what is
// decidable and what is not, and how to silence a report are documented in
// github.com/timzifer/metrology/unitvet.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/timzifer/metrology/unitvet"
)

func main() { singlechecker.Main(unitvet.Analyzer) }
