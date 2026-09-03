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

	"gopkg.in/yaml.v3"
)

func main() {
	var (
		source = flag.String("catalog", "catalog/catalog.yaml", "the catalogue to read")
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
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	var c catalogue
	// KnownFields: a misspelled key is a unit that silently loses its factor,
	// which is the kind of defect a catalogue must not be able to have.
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}

	if err := validate(&c); err != nil {
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

	index, err := emitIndex(module, &c)
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

	table, err := emitVetTable(module, &c)
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
