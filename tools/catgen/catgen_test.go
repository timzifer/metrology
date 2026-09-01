package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The catalogue that ships must generate. This is the one test here that runs
// against the real file rather than a fixture.
func TestTheShippedCatalogueIsValid(t *testing.T) {
	c := load(t, filepath.Join("..", "..", "catalog", "catalog.yaml"))
	if err := validate(c); err != nil {
		t.Fatalf("catalog/catalog.yaml is invalid:\n%v", err)
	}
}

// Generation is deterministic: CI checks for a dirty working tree after
// go generate, so a map iteration anywhere in the emitter would turn that check
// into a coin toss.
func TestGenerationIsDeterministic(t *testing.T) {
	c := load(t, filepath.Join("..", "..", "catalog", "catalog.yaml"))
	byID := index(c)

	for run := 0; run < 5; run++ {
		for _, q := range c.Quantities {
			first, err := emitQuantity("example.com/m", q, byID)
			if err != nil {
				t.Fatalf("emit %s: %v", q.Package, err)
			}
			again, err := emitQuantity("example.com/m", q, byID)
			if err != nil {
				t.Fatalf("emit %s: %v", q.Package, err)
			}
			if string(first) != string(again) {
				t.Fatalf("package %s differs between two runs", q.Package)
			}
		}
		first, err := emitIndex("example.com/m", c)
		if err != nil {
			t.Fatalf("emit index: %v", err)
		}
		again, err := emitIndex("example.com/m", c)
		if err != nil {
			t.Fatalf("emit index: %v", err)
		}
		if string(first) != string(again) {
			t.Fatal("the catalogue index differs between two runs")
		}
	}
}

// Every generated file carries the line that keeps it out of the coverage
// figure and out of anyone's editor (D14, D8).
func TestGeneratedFilesAreMarked(t *testing.T) {
	c := load(t, filepath.Join("..", "..", "catalog", "catalog.yaml"))
	byID := index(c)

	code, err := emitQuantity("example.com/m", c.Quantities[0], byID)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !strings.HasPrefix(string(code), "// Code generated ") || !strings.Contains(string(code), "DO NOT EDIT.") {
		t.Errorf("generated package does not carry the marker:\n%s", firstLine(string(code)))
	}

	index, err := emitIndex("example.com/m", c)
	if err != nil {
		t.Fatalf("emit index: %v", err)
	}
	if !strings.HasPrefix(string(index), "// Code generated ") {
		t.Errorf("generated index does not carry the marker:\n%s", firstLine(string(index)))
	}
}

// M3: the generator aborts on a defective catalogue instead of producing code
// that panics at run time in somebody else's program.
func TestValidationRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "two units with the same symbol and kind",
			yaml: `
quantities:
  - package: pressure
    dimension: {mass: 1, length: -1, time: -2}
    units:
      - {id: pascal, go: Pascal, canonical: true, symbol: {form: si, text: Pa}, source: s}
      - {id: pascal_again, go: Pascal2, symbol: {form: si, text: Pa}, source: s}
`,
			want: `duplicate symbol "Pa"`,
		},
		{
			name: "two canonical units for one dimension and kind",
			yaml: `
quantities:
  - package: pressure
    dimension: {mass: 1, length: -1, time: -2}
    units:
      - {id: pascal, go: Pascal, canonical: true, symbol: {form: si, text: Pa}, source: s}
      - {id: bar, go: Bar, canonical: true, symbol: {form: static, text: bar}, factor: {num: "100000"}, source: s}
`,
			want: "two canonical units",
		},
		{
			name: "no canonical unit at all",
			yaml: `
quantities:
  - package: pressure
    dimension: {mass: 1, length: -1, time: -2}
    units:
      - {id: bar, go: Bar, symbol: {form: static, text: bar}, factor: {num: "100000"}, source: s}
`,
			want: "no canonical unit",
		},
		{
			name: "a duplicate unit id",
			yaml: `
quantities:
  - package: pressure
    dimension: {mass: 1, length: -1, time: -2}
    units:
      - {id: pascal, go: Pascal, canonical: true, symbol: {form: si, text: Pa}, source: s}
      - {id: pascal, go: Other, symbol: {form: static, text: other}, source: s}
`,
			want: `duplicate unit id "pascal"`,
		},
		{
			name: "a duplicate Go identifier",
			yaml: `
quantities:
  - package: pressure
    dimension: {mass: 1, length: -1, time: -2}
    units:
      - {id: pascal, go: Pascal, canonical: true, symbol: {form: si, text: Pa}, source: s}
      - {id: bar, go: Pascal, symbol: {form: static, text: bar}, source: s}
`,
			want: "duplicate Go identifier",
		},
		{
			name: "one package, two dimensions",
			yaml: `
quantities:
  - package: pressure
    dimension: {mass: 1, length: -1, time: -2}
    units:
      - {id: pascal, go: Pascal, canonical: true, symbol: {form: si, text: Pa}, source: s}
  - package: pressure
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}, source: s}
`,
			want: "declares two dimensions",
		},
		{
			name: "a factor that is not a number",
			yaml: `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}, source: s}
      - {id: odd, go: Odd, symbol: {form: static, text: odd}, factor: {num: "one"}, source: s}
`,
			want: "numerator",
		},
		{
			name: "a factor of zero",
			yaml: `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}, source: s}
      - {id: nothing, go: Nothing, symbol: {form: static, text: nil}, factor: {num: "0"}, source: s}
`,
			want: "is zero",
		},
		{
			name: "an offset on an interval unit",
			yaml: `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}, offset: "1", source: s}
`,
			want: "offset",
		},
		{
			name: "an interval unit that does not exist",
			yaml: `
quantities:
  - package: temperature
    dimension: {temperature: 1}
    units:
      - {id: celsius, go: Celsius, canonical: true, kind: absolute, symbol: {form: static, text: "°C"}, offset: "273.15", interval: nowhere, source: s}
`,
			want: "unknown interval unit",
		},
		{
			name: "an absolute unit named as an interval unit",
			yaml: `
quantities:
  - package: temperature
    dimension: {temperature: 1}
    units:
      - {id: kelvin, go: Kelvin, canonical: true, kind: absolute, symbol: {form: si, text: K}, source: s}
      - {id: celsius, go: Celsius, kind: absolute, symbol: {form: static, text: "°C"}, offset: "273.15", interval: kelvin, source: s}
`,
			want: "but that one is absolute",
		},
		{
			name: "an interval unit of another dimension",
			yaml: `
quantities:
  - package: temperature
    dimension: {temperature: 1}
    units:
      - {id: celsius, go: Celsius, canonical: true, kind: absolute, symbol: {form: static, text: "°C"}, offset: "273.15", interval: metre, source: s}
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}, source: s}
`,
			want: "the dimensions differ",
		},
		{
			name: "a unit without a source",
			yaml: `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}}
`,
			want: "has no source",
		},
		{
			name: "a symbol form that does not exist",
			yaml: `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: runic, text: m}, source: s}
`,
			want: "unknown symbol form",
		},
		{
			name: "a unit without an id",
			yaml: `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {go: Metre, canonical: true, symbol: {form: si, text: m}, source: s}
`,
			want: "missing its id",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := decode(t, tc.yaml)
			err := validate(c)
			if err == nil {
				t.Fatal("the catalogue was accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// A misspelled key would silently drop a factor, which is the one kind of
// defect a catalogue must not be able to have.
func TestUnknownFieldsAreRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	write(t, path, `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, symbol: {form: si, text: m}, factorr: {num: "2"}, source: s}
`)
	err := run(path, dir, "example.com/m")
	if err == nil {
		t.Fatal("a misspelled field was accepted")
	}
	if !strings.Contains(err.Error(), "factorr") {
		t.Errorf("error = %v, want it to name the unknown field", err)
	}
}

// The end-to-end path: read a catalogue, write packages, and refuse to write
// anything at all when the catalogue is defective.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	write(t, path, `
quantities:
  - package: length
    doc: Length.
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, canonical: true, doc: the metre, symbol: {form: si, text: m}, source: SI Brochure}
`)
	if err := run(path, dir, "example.com/m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{
		filepath.Join(dir, "length", "length_gen.go"),
		filepath.Join(dir, "catalog", "units_gen.go"),
	} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s was not written: %v", name, err)
		}
	}

	t.Run("a missing catalogue is an error", func(t *testing.T) {
		if err := run(filepath.Join(dir, "absent.yaml"), dir, "example.com/m"); err == nil {
			t.Error("a missing catalogue was accepted")
		}
	})

	t.Run("a defective catalogue writes nothing", func(t *testing.T) {
		out := t.TempDir()
		bad := filepath.Join(out, "catalog.yaml")
		write(t, bad, `
quantities:
  - package: length
    dimension: {length: 1}
    units:
      - {id: metre, go: Metre, symbol: {form: si, text: m}, source: s}
`)
		if err := run(bad, out, "example.com/m"); err == nil {
			t.Fatal("a catalogue without a canonical unit was accepted")
		}
		if _, err := os.Stat(filepath.Join(out, "length")); err == nil {
			t.Error("the generator wrote a package for a catalogue it rejected")
		}
	})

	t.Run("a catalogue that is not YAML at all", func(t *testing.T) {
		out := t.TempDir()
		broken := filepath.Join(out, "catalog.yaml")
		write(t, broken, "quantities: [")
		if err := run(broken, out, "example.com/m"); err == nil {
			t.Error("unparseable YAML was accepted")
		}
	})
}

// Symbols are rendered here the way the symbol package renders them, because
// the duplicate check has to see what a reader sees.
func TestSymbolText(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec symbolSpec
		want string
	}{
		{"static", symbolSpec{Form: "static", Text: "bar"}, "bar"},
		{"si", symbolSpec{Form: "si", Text: "Pa"}, "Pa"},
		{"squared", symbolSpec{Form: "si_pow", Text: "m", Power: 2}, "m²"},
		{"to the first power", symbolSpec{Form: "si_pow", Text: "m", Power: 1}, "m"},
		{"negative power", symbolSpec{Form: "si_pow", Text: "m", Power: -1}, "m⁻¹"},
		{"the kilogram names itself with its prefix", symbolSpec{Form: "gram"}, "kg"},
		{"litre", symbolSpec{Form: "litre", Text: "L"}, "L"},
		{"product", symbolSpec{Form: "product", Of: []symbolSpec{
			{Form: "si", Text: "N"}, {Form: "si", Text: "m"},
		}}, "N·m"},
		{"quotient", symbolSpec{Form: "quotient", Of: []symbolSpec{
			{Form: "si", Text: "m"}, {Form: "si", Text: "s"},
		}}, "m/s"},
		{"a product denominator is parenthesised", symbolSpec{Form: "quotient", Of: []symbolSpec{
			{Form: "si", Text: "J"},
			{Form: "product", Of: []symbolSpec{{Form: "gram"}, {Form: "si", Text: "K"}}},
		}}, "J/(kg·K)"},
		{"a malformed quotient renders as nothing rather than guessing",
			symbolSpec{Form: "quotient", Of: []symbolSpec{{Form: "si", Text: "m"}}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.spec.text(); got != tc.want {
				t.Errorf("text() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSymbolExpr(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec symbolSpec
		want string
	}{
		{"static", symbolSpec{Form: "static", Text: "bar"}, `symbol.Static("bar")`},
		{"si", symbolSpec{Form: "si", Text: "Pa"}, `symbol.SI("Pa")`},
		{"si_pow", symbolSpec{Form: "si_pow", Text: "m", Power: 3}, `symbol.SIPow("m", 3)`},
		{"gram", symbolSpec{Form: "gram"}, "symbol.Gram()"},
		{"litre", symbolSpec{Form: "litre"}, "symbol.Litre()"},
		{"product", symbolSpec{Form: "product", Of: []symbolSpec{
			{Form: "si", Text: "N"}, {Form: "si", Text: "m"},
		}}, `symbol.Product(symbol.SI("N"), symbol.SI("m"))`},
		{"quotient", symbolSpec{Form: "quotient", Of: []symbolSpec{
			{Form: "si", Text: "m"}, {Form: "si", Text: "s"},
		}}, `symbol.Quotient(symbol.SI("m"), symbol.SI("s"))`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.spec.symbolExpr()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("symbolExpr() = %s, want %s", got, tc.want)
			}
		})
	}

	t.Run("an unknown form inside a product is reported", func(t *testing.T) {
		spec := symbolSpec{Form: "product", Of: []symbolSpec{{Form: "cuneiform", Text: "x"}}}
		if _, err := spec.symbolExpr(); err == nil {
			t.Error("an unknown nested form was accepted")
		}
	})
}

func TestDimensionExpr(t *testing.T) {
	e := exponents{Length: -1, Mass: 1, Time: -2}
	want := "dimension.New(dimension.Exponents{Time: -2, Length: -1, Mass: 1})"
	if got := e.dimensionExpr(); got != want {
		t.Errorf("dimensionExpr() = %s, want %s", got, want)
	}
	if !(exponents{}).isZero() {
		t.Error("the empty dimension does not report itself as dimensionless")
	}
	if e.isZero() {
		t.Error("a pressure reports itself as dimensionless")
	}
}
