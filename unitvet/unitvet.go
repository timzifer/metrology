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
// # A catalogue unit that is assigned
//
// A unit is a package-level variable — pressure.Bar, dose.Sievert — and this
// pass resolves one by name, from the table generated alongside the catalogue.
// Go lets an importer assign to an exported variable of another package, and a
// program that does makes the table untrue: the name still resolves, to a unit
// the variable no longer holds.
//
// The assumption is checked rather than assumed. A direct store to a catalogue
// unit is reported where it happens:
//
//	app/app.go:9:2: dose.Sievert is assigned; the generated table no longer
//	                describes it, and every unit resolved through it is unproven
//
// A write through a pointer taken elsewhere is out of reach, as is one in a
// dependency the vet run does not cover.
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
// # The interval layer
//
// The pass has rules for two packages, not one. A [uncertainty.Range] is a unit
// and two magnitudes, so its dimension, its kind and its quantity are those of
// its bounds — and every rule above therefore applies to it unchanged, without
// a clause of its own:
//
//	_, _ = uncertainty.Of(pressure.Bar.Of(2.5)).
//		Add(uncertainty.Of(temperature.Celsius.Of(20))) // reported
//
// This is not a convenience. D15 moved a second arithmetic surface out of the
// core, and a checker that went blind exactly where the arithmetic went would
// be worse than no checker, because nobody would notice. What it also means is
// that the three constructors — Of, Between and Symmetric — are resolved even
// though they are package-level functions rather than methods: without them no
// range resolves at all, and the pass would be silent on every one of them.
//
// The one thing it will not say is which interval unit the width of an absolute
// range is read on. That is declared by the scale — K for a °C — and the
// generated table does not record it, which is the same silence a difference of
// two points already gets.
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

// rangePath is the interval layer of D15. It is a second package with a second
// receiver type, and the pass has to know it: a range added to a range of
// another dimension is exactly the provable class this checker exists for, and
// a checker that went blind where the arithmetic moved would be worse than no
// checker at all, because nobody would notice.
const rangePath = corePath + "/uncertainty"

// gumPath is the propagation layer of D21. It is the third receiver type, and
// it costs the pass what D15 predicted the second would: nothing but the
// registration. A gum.Value's dimension, kind and quantity are its estimate's,
// so every rule below applies to it unchanged.
const gumPath = corePath + "/gum"

// corePackages names, for each package this pass has rules for, the types whose
// methods those rules are about.
//
// The rules themselves do not distinguish the two: the dimension, the kind and
// the quantity of a range are those of its bounds, so Add on a Range is Add on
// a Measurement as far as anything provable here is concerned. What the owner
// is needed for is telling apart two methods that share a name — Unit.Of puts a
// magnitude on a unit, uncertainty.Of puts a range around a measurement.
var corePackages = map[string][]string{
	corePath:  {"Measurement", "Unit", "Engine"},
	rangePath: {"Range", "Engine"},
	gumPath:   {"Value", "Engine"},
}

// layerConstructors are the package-level functions of the layers above the
// core that build one of their types out of measurements.
//
// They are functions and not methods, so the receiver rule of coreCall does not
// reach them — and without them nothing in either layer is resolvable at all,
// since every range and most values enter through one of these.
//
// What is not here is not an oversight. gum.Of takes a struct, gum.Sample a
// slice and gum.Correlated a pair of structs, and a field of a composite
// literal is exactly the container read this pass does not follow (D13). A
// value built by one of them resolves to nothing, and the pass says nothing
// about it — which is the silence of doubt and not a hole in the rules.
var layerConstructors = map[string]map[string]bool{
	rangePath: {"Of": true, "Between": true, "Symmetric": true},
	gumPath:   {"Exactly": true, "Standard": true, "Apply": true},
}

// layerTypes names the type each layer's constructors build.
var layerTypes = map[string]string{rangePath: "Range", gumPath: "Value"}

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
	// them, to their names and the package each belongs to. Identity, not
	// import paths: within one pass every package shares one *types.TypeName
	// per type, and comparing pointers cannot be fooled by a vendored copy
	// under a different path.
	core map[*types.TypeName]owned

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

// coreTypes finds the library's own types as the package under analysis knows
// them, mapped to the package that owns each.
//
// It walks the import graph rather than looking at the direct imports: a
// program usually imports a quantity package and never names the core at all,
// and pressure.Bar is still a metrology.Unit. A program that reaches only one
// of the two packages gets the rules for that one; a program that reaches
// neither gets none, and the pass returns immediately.
func coreTypes(pkg *types.Package) map[*types.TypeName]owned {
	out := map[*types.TypeName]owned{}
	for path, names := range corePackages {
		found := findPackage(pkg, path, map[*types.Package]bool{})
		if found == nil {
			continue
		}
		for _, name := range names {
			if tn, isType := found.Scope().Lookup(name).(*types.TypeName); isType {
				out[tn] = owned{owner: path, name: name}
			}
		}
	}
	return out
}

// owned names one of the library's types and the package it lives in.
type owned struct {
	owner string // the import path: corePath, rangePath or gumPath
	name  string // Measurement, Unit, Engine, Range, Value
}

// findPackage returns the package with the given import path from anywhere in
// the import graph of pkg, or nil where it is not reachable.
func findPackage(pkg *types.Package, path string, seen map[*types.Package]bool) *types.Package {
	if pkg.Path() == path {
		return pkg
	}
	if seen[pkg] {
		return nil
	}
	seen[pkg] = true
	for _, imported := range pkg.Imports() {
		if found := findPackage(imported, path, seen); found != nil {
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
