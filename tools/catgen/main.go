// Command catgen generates the quantity packages and the catalogue index from
// catalog/catalog.yaml (D8).
//
// Hundreds of units written by hand are the same four lines over and over, with
// four chances for a transposed digit in each. As data they are a table that can
// be checked against the SI Brochure and NIST SP 811 — and the checking happens
// here, before any Go exists: duplicate symbols, two units claiming to be
// canonical for one dimension, a factor that is not a number, a unit without a
// source. All of those abort the generator rather than panicking at run time in
// somebody else's program (D7).
//
// Usage:
//
//	go generate ./...
//
// The output is deterministic: run it twice and the files are byte-identical,
// which is what CI checks by looking for a dirty working tree afterwards.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func main() {
	var (
		source = flag.String("catalog", "catalog/*.yaml", "the catalogue files to read, as a glob")
		root   = flag.String("root", ".", "the module root to write into")
		module = flag.String("module", "github.com/timzifer/metrology", "the module path to import from")
	)
	flag.Parse()

	if err := run(*source, *root, *module); err != nil {
		fmt.Fprintf(os.Stderr, "catgen: %v\n", err)
		os.Exit(1)
	}
}

func run(source, root, module string) error {
	c, err := read(source)
	if err != nil {
		return err
	}

	if err := validate(c); err != nil {
		return err
	}

	byID := map[string]unitSpec{}
	for _, u := range c.units() {
		byID[u.ID] = u
	}

	for _, q := range c.Quantities {
		code, err := emitQuantity(module, q, byID)
		if err != nil {
			return fmt.Errorf("package %s: %w", q.Package, err)
		}
		dir := filepath.Join(root, unitsDir, q.Package)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, q.Package+"_gen.go")
		if err := os.WriteFile(path, code, 0o644); err != nil {
			return err
		}
		fmt.Printf("catgen: %s (%d units)\n", path, len(q.Units))
	}

	index, err := emitIndex(module, c)
	if err != nil {
		return fmt.Errorf("catalogue index: %w", err)
	}
	dir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "units_gen.go")
	if err := os.WriteFile(path, index, 0o644); err != nil {
		return err
	}
	fmt.Printf("catgen: %s (%d units)\n", path, len(c.units()))

	table, err := emitVetTable(module, c)
	if err != nil {
		return fmt.Errorf("unitvet table: %w", err)
	}
	dir = filepath.Join(root, "unitvet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path = filepath.Join(dir, "table_gen.go")
	if err := os.WriteFile(path, table, 0o644); err != nil {
		return err
	}
	fmt.Printf("catgen: %s (%d units)\n", path, len(c.units()))
	return nil
}

// read loads every catalogue file the pattern matches and merges them into one.
//
// The SI catalogue and the customary one are separate files (D19) and one
// document: they are validated together, because a symbol that resolves to two
// units is a defect whichever file each half is written in, and they are
// generated together, because the index and the unitvet table hold all of it.
//
// The files are read in sorted order so that two runs produce the same bytes,
// which is what CI checks by looking for a dirty tree afterwards.
func read(pattern string) (*catalogue, error) {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s: no catalogue file", pattern)
	}
	sort.Strings(paths)

	merged := &catalogue{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var c catalogue
		// KnownFields: a misspelled key is a unit that silently loses its
		// factor, which is the kind of defect a catalogue must not be able to
		// have.
		decoder := yaml.NewDecoder(bytes.NewReader(raw))
		decoder.KnownFields(true)
		if err := decoder.Decode(&c); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		merged.Quantities = append(merged.Quantities, c.Quantities...)
	}
	return merged, nil
}
