package main

import (
	"fmt"
	"sort"
	"strings"
)

// validate checks the catalogue before a line of Go is written.
//
// D8 requires these checks to happen at generation time and to abort, rather
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

		// One dimension per package, unless the package is a provenance rather
		// than a quantity (D19): the package is the quantity, and a package
		// holding two dimensions would make the autocompletion catalogue lie.
		if !q.isProvenance() {
			if previous, seen := packages[q.Package]; seen && previous != u.dimension().dimensionExpr() {
				report("package %q declares two dimensions", q.Package)
			}
			packages[q.Package] = u.dimension().dimensionExpr()
		}

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

		// Exactly one canonical unit per dimension, kind and quantity. This is
		// the check that used to be a runtime panic when two packages claimed
		// the same dimension; here it is a failed build. The quantity is part
		// of the key because the hertz and the becquerel legitimately claim
		// the same dimension and are not the same unit (D6).
		if u.Canonical {
			if previous, seen := canonical[canonicalKey(u)]; seen {
				report("two canonical units for the same dimension, kind and quantity: %s and %s",
					previous, u.ID)
			}
			canonical[canonicalKey(u)] = u.ID
		}

		if err := validateFactor(u); err != nil {
			report("unit %s: %v", u.ID, err)
		}
		if u.Source == "" {
			// A conversion factor without a citation cannot be checked, and an
			// uncheckable factor is exactly what D4 was written against.
			report("unit %s has no source", u.ID)
		}

		// Where the dimension is written decides what the package is, so the
		// two have to agree (D19). A quantity package states it once; a
		// provenance package states it per unit and cannot state it twice.
		switch {
		case q.isProvenance() && u.Dimension == nil:
			report("unit %s is in the provenance package %q and declares no dimension of its own", u.ID, q.Package)
		case !q.isProvenance() && u.Dimension != nil:
			report("unit %s declares a dimension, but package %q is a quantity and declares one for it", u.ID, q.Package)
		}
	}

	// A group is a quantity or a provenance and nothing else. A misspelling
	// would otherwise read as the default and take the checks above with it.
	for _, q := range c.Quantities {
		if q.Group != "" && q.Group != groupProvenance {
			report("package %q declares an unknown group %q", q.Package, q.Group)
		}
		if q.isProvenance() && q.Quantity != "" {
			report("package %q is a provenance and cannot carry one quantity tag", q.Package)
		}
	}

	// Every dimension, kind and quantity that occurs must have a canonical
	// unit, or a result computed into that dimension has nothing to be
	// expressed in.
	for _, u := range c.units() {
		if _, ok := canonical[canonicalKey(u)]; !ok {
			report("no canonical unit for the dimension %s, kind %s",
				u.dimension().dimensionExpr(), strings.TrimPrefix(u.kindExpr(), "metrology."))
			canonical[canonicalKey(u)] = u.ID // report once
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

// canonicalKey identifies the slot a canonical unit fills: one per dimension,
// kind and quantity.
func canonicalKey(u unitSpec) string {
	return u.dimension().dimensionExpr() + "\x00" + u.kindExpr() + "\x00" + u.quantityTag()
}

// maxPiExponent is the largest π exponent the catalogue admits. The square
// degree is π², the oersted is π⁻¹, and nothing in the standards reaches
// further; a catalogue entry past this is a mistake, not a unit.
const maxPiExponent = 2

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
	// D20: the exponent is stored in an int8 and the run time refuses more.
	// No definition needs a third power of π, so the bound here is the one
	// the catalogue can justify rather than the one the field can hold.
	if u.Factor.Pi < -maxPiExponent || u.Factor.Pi > maxPiExponent {
		return fmt.Errorf("π exponent %d is outside ±%d", u.Factor.Pi, maxPiExponent)
	}
	if offset.Sign() != 0 && !u.isAbsolute() {
		return fmt.Errorf("offset %s on an interval unit; an offset makes a scale affine", offset)
	}
	if u.Interval != "" && !u.isAbsolute() {
		return fmt.Errorf("an interval unit does not need an interval unit of its own")
	}
	return nil
}
