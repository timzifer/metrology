// Package unitvet is the static dimension checker of D13: a go/analysis pass
// that reports arithmetic and conversions across incompatible dimensions
// without running the code.
//
// Go cannot express dimensional analysis in its type system — there are no
// integer type parameters, so a Q[Length, Time] is not constructible (D1) —
// and the library compensates at run time, with errors that name both
// dimensions. This pass recovers a part of what the type system could not
// give, at the point in the toolchain where Go conventionally puts such a
// check:
//
//	go vet -vettool=$(go env GOPATH)/bin/unitvet ./...
//
//	app/app.go:12:33: Add on incompatible dimensions: L⁻¹M¹T⁻² and Θ¹
//
// # Silence on doubt
//
// The pass reports only what it can prove. Where an operand's unit cannot be
// resolved with certainty it says nothing at all: a unit chosen in an if, one
// arriving as a parameter, one read out of a map. A dimension checker with
// false positives is a dimension checker that gets switched off, and then it
// catches nothing — so false negatives are the acceptable failure, and the
// run-time check of D1 remains the backstop.
//
// What it does resolve is a unit variable from the catalogue —
// pressure.Bar.Of(2.5) — through assignments, through the arithmetic that
// composes units, and across package boundaries by way of the fact mechanism
// of the analysis framework: a function whose result is provably always on one
// scale exports that scale, and the packages importing it read it back.
//
// The table of catalogue units is generated from catalog/catalog.yaml, the
// same source the library itself is generated from (D8), so the checker and
// the run time cannot drift apart.
//
// # The tag a product dropped
//
// One rule reports something the run time accepts, and it is the only one
// (D16). Multiplication and division drop the quantity tag of D6 — a becquerel
// times a metre is not anything the catalogue names — so a becquerel scaled by
// a plain number reaches the next operation as an untagged T⁻¹, and the run
// time, which no longer holds the tag, lets it meet a frequency:
//
//	scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
//	_, _ = scaled.Add(frequency.Hertz.Of(50)) // accepted; reported here
//
// This pass walks the operands backwards anyway, so it keeps the discarded tag
// as provenance and reports the conflict the run time can no longer see. The
// message says as much, because a diagnostic the reader cannot reproduce by
// running the code is a diagnostic the reader stops believing.
//
// The provenance is not a tag. Converting that magnitude into a curie is legal
// and stays silent; only a conflicting tag is reported. It survives only where
// the dimension does — a becquerel times a metre carries nothing, and dividing
// the metre back out recovers nothing — and two surviving tags that disagree
// leave none.
//
// # Silencing a report
//
// A test that asserts an operation fails is an operation the pass is right to
// report and nobody wants reported. A line comment silences the diagnostic on
// its own line or on the line below it:
//
//	//unitvet:ignore the point of this test is that it errors
//	_, err := frequency.Hertz.Of(50).To(activity.Becquerel)
//
// State a reason, as the //coverage:ignore markers of D14 do. The pass does
// not read it, but the next person does.
package unitvet

import (
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
)

// corePath is the import path of the library whose operations this pass has
// rules for. The generated table keys its units by paths under it.
const corePath = "github.com/timzifer/metrology"

// ignoreMarker silences the diagnostics on the line it is written on and on
// the line below it. The spelling matches the //coverage:ignore markers of
// D14, because a repository should have one convention for this, not two.
const ignoreMarker = "//unitvet:ignore"

// Analyzer is the pass. It is registered by cmd/unitvet and can be composed
// into any other analysis driver.
var Analyzer = &analysis.Analyzer{
	Name:      "unitvet",
	Doc:       "report arithmetic and conversions across incompatible dimensions",
	URL:       "https://pkg.go.dev/github.com/timzifer/metrology/unitvet",
	Requires:  []*analysis.Analyzer{buildssa.Analyzer},
	FactTypes: []analysis.Fact{(*resultScale)(nil)},
	Run:       run,
}

// scale is everything the checker knows about a measurement or a unit: which
// dimension it is on, whether it is a point on a scale or a span along it, and
// which quantity it claims to be where the dimension carries more than one
// (D6). It is the generated table's value type.
//
// The dropped tag is the checker's own field and never comes out of the table:
// it is the quantity a product or a quotient discarded while leaving the
// dimension intact, which is the one thing about a computed magnitude the run
// time cannot remember and this pass can (D16).
type scale struct {
	dim      dimension.Dimension
	kind     metrology.Kind
	quantity metrology.Quantity
	dropped  metrology.Quantity
}

// String names a scale the way a diagnostic and a fact do.
func (s scale) String() string {
	text := s.dim.String() + " " + s.kind.String()
	if s.quantity != "" {
		text += " " + string(s.quantity)
	}
	if s.dropped != "" {
		text += " from " + string(s.dropped)
	}
	return text
}

// resultScale is the fact that carries a scale across a package boundary: a
// function whose result is provably always on one scale exports it, and a
// caller in another package reads it back instead of giving up (D13).
//
// The fields are the plain forms of a scale because a fact is gob-encoded.
type resultScale struct {
	Dimension uint64
	Kind      uint8
	Quantity  string
	Dropped   string
}

// AFact marks resultScale as a fact of the analysis framework.
func (*resultScale) AFact() {}

func (f *resultScale) String() string { return "returns " + f.scale().String() }

func (f *resultScale) scale() scale {
	return scale{
		dim:      dimension.Dimension(f.Dimension),
		kind:     metrology.Kind(f.Kind),
		quantity: metrology.Quantity(f.Quantity),
		dropped:  metrology.Quantity(f.Dropped),
	}
}

func factOf(s scale) *resultScale {
	return &resultScale{
		Dimension: uint64(s.dim),
		Kind:      uint8(s.kind),
		Quantity:  string(s.quantity),
		Dropped:   string(s.dropped),
	}
}

// checker carries the state of one pass over one package.
type checker struct {
	pass *analysis.Pass

	// core maps the library's own types, as the package under analysis knows
	// them, to their names. Identity, not import paths: within one pass every
	// package shares one *types.TypeName per type, and comparing pointers
	// cannot be fooled by a vendored copy under a different path.
	core map[*types.TypeName]string

	// resolved memoises the walk. The SSA of a chain of arithmetic is a graph
	// with shared operands, and re-walking it from every use is exponential.
	resolved map[ssa.Value]resolution

	// ignored holds the lines a marker silences, by file.
	ignored map[position]bool
}

type resolution struct {
	scale scale
	known bool
}

type position struct {
	file string
	line int
}

func run(pass *analysis.Pass) (any, error) {
	core := coreTypes(pass.Pkg)
	if len(core) == 0 {
		// The package does not reach the library at all, directly or through
		// a quantity package. There is nothing here this pass has rules for.
		return nil, nil
	}

	c := &checker{
		pass:     pass,
		core:     core,
		resolved: map[ssa.Value]resolution{},
		ignored:  ignoredLines(pass),
	}

	funcs := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA).SrcFuncs
	c.exportFacts(funcs)
	for _, fn := range funcs {
		c.check(fn)
	}
	return nil, nil
}

// coreTypes finds Measurement, Unit and Engine as the package under analysis
// knows them.
//
// It walks the import graph rather than looking at the direct imports: a
// program usually imports a quantity package and never names the core at all,
// and pressure.Bar is still a metrology.Unit.
func coreTypes(pkg *types.Package) map[*types.TypeName]string {
	core := findPackage(pkg, map[*types.Package]bool{})
	if core == nil {
		return nil
	}
	out := map[*types.TypeName]string{}
	for _, name := range []string{"Measurement", "Unit", "Engine"} {
		if tn, isType := core.Scope().Lookup(name).(*types.TypeName); isType {
			out[tn] = name
		}
	}
	return out
}

// findPackage returns the library's package from anywhere in the import graph
// of pkg, or nil where it is not reachable.
func findPackage(pkg *types.Package, seen map[*types.Package]bool) *types.Package {
	if pkg.Path() == corePath {
		return pkg
	}
	if seen[pkg] {
		return nil
	}
	seen[pkg] = true
	for _, imported := range pkg.Imports() {
		if found := findPackage(imported, seen); found != nil {
			return found
		}
	}
	return nil
}

// ignoredLines collects the lines a marker comment silences.
func ignoredLines(pass *analysis.Pass) map[position]bool {
	out := map[position]bool{}
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if !strings.HasPrefix(comment.Text, ignoreMarker) {
					continue
				}
				at := pass.Fset.Position(comment.Pos())
				out[position{file: at.Filename, line: at.Line}] = true
			}
		}
	}
	return out
}

// report emits a diagnostic unless a marker silences it.
//
// The marker counts on the line of the diagnostic — a trailing comment — and
// on the line above it, which is where a comment carrying a reason goes.
func (c *checker) report(pos token.Pos, format string, args ...any) {
	at := c.pass.Fset.Position(pos)
	if c.ignored[position{file: at.Filename, line: at.Line}] ||
		c.ignored[position{file: at.Filename, line: at.Line - 1}] {
		return
	}
	c.pass.Reportf(pos, format, args...)
}
