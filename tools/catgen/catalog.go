package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/apd/v3"
)

// catalogue is the YAML document: the source of truth for every unit the
// library ships (D8).
type catalogue struct {
	Quantities []quantity `yaml:"quantities"`
}

// quantity is one generated package: a dimension and the units measuring it.
//
// Or, for the one package that is a provenance rather than a quantity, several
// dimensions and the units that share where they come from — see Group and D19.
type quantity struct {
	Package   string     `yaml:"package"`
	Doc       string     `yaml:"doc"`
	Dimension exponents  `yaml:"dimension"`
	Units     []unitSpec `yaml:"units"`

	// Group is what the units of this package have in common: "quantity", the
	// default and the case for every SI package, or "provenance" for
	// units/imperial, where the common thread is where the units come from and
	// the dimensions differ per unit (D19).
	//
	// It is spelled out rather than inferred from whether a dimension was
	// given, because a package holding two dimensions by accident is a defect
	// the generator has always caught and should keep catching.
	Group string `yaml:"group"`

	// Quantity names what these units measure, where the dimension is shared
	// by more than one quantity: frequency and radioactivity are both T⁻¹.
	// Empty where the dimension belongs to one quantity only, which is the
	// common case. It sits on the group because every unit of a package
	// measures the same thing — that is what makes it a package.
	Quantity string `yaml:"quantity"`
}

// exponents is the dimension of a quantity, one field per SI base quantity.
// Omitted axes are zero, so a length is written {length: 1} rather than spelled
// out seven times.
type exponents struct {
	Time              int `yaml:"time"`
	Length            int `yaml:"length"`
	Mass              int `yaml:"mass"`
	ElectricCurrent   int `yaml:"electric_current"`
	Temperature       int `yaml:"temperature"`
	AmountOfSubstance int `yaml:"amount_of_substance"`
	LuminousIntensity int `yaml:"luminous_intensity"`
}

// unitSpec is one catalogue entry.
type unitSpec struct {
	ID        string     `yaml:"id"`
	Go        string     `yaml:"go"`
	Doc       string     `yaml:"doc"`
	Kind      string     `yaml:"kind"`
	Canonical bool       `yaml:"canonical"`
	Symbol    symbolSpec `yaml:"symbol"`
	Factor    factorSpec `yaml:"factor"`
	Offset    string     `yaml:"offset"`
	Interval  string     `yaml:"interval"`
	Source    string     `yaml:"source"`

	// Dimension is what this unit measures, for a provenance group where the
	// package does not have one of its own. Nil in a quantity group, where the
	// dimension is the package's (D19).
	Dimension *exponents `yaml:"dimension"`

	// quantity is filled in during validation; the YAML does not repeat it.
	quantity *quantity
}

// groupProvenance is the Group of the one package whose units share where they
// come from rather than what they measure (D19).
const groupProvenance = "provenance"

// isProvenance reports whether this package groups its units by where they come
// from. Everything else groups by quantity, which is the default and the case
// for every package generated from the SI catalogue.
func (q quantity) isProvenance() bool { return q.Group == groupProvenance }

// dimension is what a unit measures: its own where the package has none, and
// the package's otherwise.
func (u unitSpec) dimension() exponents {
	if u.Dimension != nil {
		return *u.Dimension
	}
	return u.quantity.Dimension
}

// factorSpec is the exact fraction relating a unit to the base unit (D4). Both
// parts default to one, so a base unit omits the field entirely.
type factorSpec struct {
	Num string `yaml:"num"`
	Den string `yaml:"den"`
}

// symbolSpec describes how a unit prints. The forms mirror the constructors of
// the symbol package; product and quotient nest through Of.
type symbolSpec struct {
	Form  string       `yaml:"form"`
	Text  string       `yaml:"text"`
	Power int          `yaml:"power"`
	Of    []symbolSpec `yaml:"of"`
}

// qualified is the Go expression referring to a unit from outside its package.
func (u unitSpec) qualified() string { return u.quantity.name() + "." + u.Go }

// name is the Go package name: the last element of the package path.
//
// The two differ only where a package sits below another — units/customary/us
// is imported from that path and written us.Gallon (D19). Everywhere else the
// path is one element and the two are the same string.
func (q quantity) name() string {
	if i := strings.LastIndex(q.Package, "/"); i >= 0 {
		return q.Package[i+1:]
	}
	return q.Package
}

// unitsDir is the directory the generated quantity packages live in, below the
// module root: github.com/timzifer/metrology/units/pressure (D18). It is data,
// and it is kept out of the module's own namespace so that the seven
// hand-written packages are not lost among forty-three generated ones.
const unitsDir = "units"

// importPath is where a generated package is imported from.
func (q quantity) importPath(module string) string {
	return module + "/" + unitsDir + "/" + q.Package
}

// quantityTag is the quantity every unit of this package measures.
func (u unitSpec) quantityTag() string { return u.quantity.Quantity }

// kindExpr is the metrology.Kind the generated definition carries.
func (u unitSpec) kindExpr() string {
	if u.Kind == "absolute" {
		return "metrology.Absolute"
	}
	return "metrology.Interval"
}

// isAbsolute reports whether this unit measures points on a scale (D6).
func (u unitSpec) isAbsolute() bool { return u.Kind == "absolute" }

// dimensionExpr renders the dimension as the Go literal that builds it.
func (e exponents) dimensionExpr() string {
	var fields []string
	for _, axis := range []struct {
		name  string
		value int
	}{
		{"Time", e.Time},
		{"Length", e.Length},
		{"Mass", e.Mass},
		{"ElectricCurrent", e.ElectricCurrent},
		{"Temperature", e.Temperature},
		{"AmountOfSubstance", e.AmountOfSubstance},
		{"LuminousIntensity", e.LuminousIntensity},
	} {
		if axis.value != 0 {
			fields = append(fields, fmt.Sprintf("%s: %d", axis.name, axis.value))
		}
	}
	return "dimension.New(dimension.Exponents{" + strings.Join(fields, ", ") + "})"
}

// isZero reports a dimension with every exponent zero — a dimensionless
// quantity, which is legitimate but must be declared rather than forgotten.
func (e exponents) isZero() bool {
	return e == exponents{}
}

// symbolExpr renders a symbol spec as the Go expression that builds it.
func (s symbolSpec) symbolExpr() (string, error) {
	switch s.Form {
	case "static":
		return fmt.Sprintf("symbol.Static(%q)", s.Text), nil
	case "si":
		return fmt.Sprintf("symbol.SI(%q)", s.Text), nil
	case "si_pow":
		return fmt.Sprintf("symbol.SIPow(%q, %d)", s.Text, s.Power), nil
	case "gram":
		return "symbol.Gram()", nil
	case "litre":
		return "symbol.Litre()", nil
	case "product", "quotient":
		parts := make([]string, len(s.Of))
		for i, part := range s.Of {
			expr, err := part.symbolExpr()
			if err != nil {
				return "", err
			}
			parts[i] = expr
		}
		if s.Form == "product" {
			return "symbol.Product(" + strings.Join(parts, ", ") + ")", nil
		}
		return "symbol.Quotient(" + strings.Join(parts, ", ") + ")", nil
	default:
		return "", fmt.Errorf("unknown symbol form %q", s.Form)
	}
}

// text renders the symbol the way the symbol package will, which is what the
// duplicate check compares. It deliberately reimplements the rendering rather
// than importing it: the check has to see the symbol as a reader does, and
// building a symbol here would drag the whole core into the generator.
func (s symbolSpec) text() string {
	switch s.Form {
	case "gram":
		// The kilogram names itself with its prefix; the symbol package prints
		// kg for the unit and attaches prefixes to the gram.
		return "kg"
	case "litre":
		return "L"
	case "si_pow":
		return s.Text + superscript(s.Power)
	case "product":
		parts := make([]string, len(s.Of))
		for i, part := range s.Of {
			parts[i] = part.text()
		}
		return strings.Join(parts, "·")
	case "quotient":
		if len(s.Of) != 2 {
			return ""
		}
		denominator := s.Of[1].text()
		if s.Of[1].Form == "product" {
			denominator = "(" + denominator + ")"
		}
		return s.Of[0].text() + "/" + denominator
	default:
		return s.Text
	}
}

// superscript renders an exponent the way the symbol package does.
func superscript(n int) string {
	if n == 1 {
		return ""
	}
	digits := map[rune]rune{
		'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
		'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹', '-': '⁻',
	}
	var out []rune
	for _, r := range fmt.Sprintf("%d", n) {
		out = append(out, digits[r])
	}
	return string(out)
}

// decimalOrDefault parses a catalogue decimal, treating an empty field as the
// given default.
func decimalOrDefault(text string, fallback int64) (*apd.Decimal, error) {
	if text == "" {
		return apd.New(fallback, 0), nil
	}
	d, _, err := apd.NewFromString(text)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// units returns every unit of the catalogue, sorted by id.
//
// Sorted, because the generated files must be byte-identical from one run to
// the next: CI compares the working tree after go generate, and a map iteration
// somewhere in here would make that check fail at random.
func (c *catalogue) units() []unitSpec {
	var all []unitSpec
	for i := range c.Quantities {
		q := &c.Quantities[i]
		for _, u := range q.Units {
			u.quantity = q
			all = append(all, u)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all
}
