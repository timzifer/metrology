package main

import (
	"fmt"
	"sort"
	"strings"
)

// validate checks the catalogue before a line of Go is written.
//
// M3 requires these checks to happen at generation time and to abort, rather
// than to panic at runtime the way a registry does: a catalogue defect is a
// broken build, not a broken program in somebody else's production (D7).
func validate(c *catalogue) error {
	var problems []string
	report := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	byID := map[string]unitSpec{}
	packages := map[string]string{}
	symbols := map[string]string{}
	canonical := map[string]string{}
	identifiers := map[string]string{}

	for _, u := range c.units() {
		q := u.quantity

		if u.ID == "" || u.Go == "" || q.Package == "" {
			report("a unit of package %q is missing its id, its Go name or its package", q.Package)
			continue
		}
		if first, seen := byID[u.ID]; seen {
			report("duplicate unit id %q: %s and %s", u.ID, first.qualified(), u.qualified())
			continue
		}
		byID[u.ID] = u

		// One dimension per package: the package is the quantity, and a package
		// holding two dimensions would make the autocompletion catalogue lie.
		if previous, seen := packages[q.Package]; seen && previous != q.Dimension.dimensionExpr() {
			report("package %q declares two dimensions", q.Package)
		}
		packages[q.Package] = q.Dimension.dimensionExpr()

		if previous, seen := identifiers[u.qualified()]; seen {
			report("duplicate Go identifier %s, from unit ids %s and %s", u.qualified(), previous, u.ID)
		}
		identifiers[u.qualified()] = u.ID

		// A symbol must resolve to one unit for the text form of D12 to read
		// back. The kind is part of the key: "K" is a temperature and a
		// temperature difference, and the two are different quantities that
		// print the same.
		symbolKey := u.Symbol.text() + "\x00" + u.kindExpr()
		if previous, seen := symbols[symbolKey]; seen {
			report("duplicate symbol %q for %s units: %s and %s",
				u.Symbol.text(), strings.TrimPrefix(u.kindExpr(), "metrology."), previous, u.ID)
		}
		symbols[symbolKey] = u.ID

		if _, err := u.Symbol.symbolExpr(); err != nil {
			report("unit %s: %v", u.ID, err)
		}

		// Exactly one canonical unit per dimension and kind. This is the check
		// that used to be a runtime panic when two packages claimed the same
		// dimension; here it is a failed build.
		if u.Canonical {
			key := q.Dimension.dimensionExpr() + "\x00" + u.kindExpr()
			if previous, seen := canonical[key]; seen {
				report("two canonical units for the same dimension and kind: %s and %s", previous, u.ID)
			}
			canonical[key] = u.ID
		}

		if err := validateFactor(u); err != nil {
			report("unit %s: %v", u.ID, err)
		}
		if u.Source == "" {
			// A conversion factor without a citation cannot be checked, and an
			// uncheckable factor is exactly what D4 was written against.
			report("unit %s has no source", u.ID)
		}
	}

	// Every dimension and kind that occurs must have a canonical unit, or a
	// result computed into that dimension has nothing to be expressed in.
	for _, u := range c.units() {
		key := u.quantity.Dimension.dimensionExpr() + "\x00" + u.kindExpr()
		if _, ok := canonical[key]; !ok {
			report("no canonical unit for the dimension of package %q, kind %s",
				u.quantity.Package, strings.TrimPrefix(u.kindExpr(), "metrology."))
			canonical[key] = u.ID // report once
		}
	}

	// Interval references are resolved last, when every id is known.
	for _, u := range c.units() {
		if u.Interval == "" {
			continue
		}
		target, ok := byID[u.Interval]
		if !ok {
			report("unit %s refers to an unknown interval unit %q", u.ID, u.Interval)
			continue
		}
		if target.isAbsolute() {
			report("unit %s names %s as its interval unit, but that one is absolute", u.ID, target.ID)
		}
		if target.quantity.Dimension != u.quantity.Dimension {
			report("unit %s names %s as its interval unit, but the dimensions differ", u.ID, target.ID)
		}
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("catalogue is invalid:\n  %s", strings.Join(problems, "\n  "))
}

// validateFactor checks what [metrology.NewUnit] would check, before the
// generated code exists to check it at run time.
func validateFactor(u unitSpec) error {
	num, err := decimalOrDefault(u.Factor.Num, 1)
	if err != nil {
		return fmt.Errorf("numerator %q: %w", u.Factor.Num, err)
	}
	den, err := decimalOrDefault(u.Factor.Den, 1)
	if err != nil {
		return fmt.Errorf("denominator %q: %w", u.Factor.Den, err)
	}
	offset, err := decimalOrDefault(u.Offset, 0)
	if err != nil {
		return fmt.Errorf("offset %q: %w", u.Offset, err)
	}
	if num.Sign() == 0 || den.Sign() == 0 {
		return fmt.Errorf("factor %s/%s is zero", num, den)
	}
	if offset.Sign() != 0 && !u.isAbsolute() {
		return fmt.Errorf("offset %s on an interval unit; an offset makes a scale affine", offset)
	}
	if u.Interval != "" && !u.isAbsolute() {
		return fmt.Errorf("an interval unit does not need an interval unit of its own")
	}
	return nil
}
