package unitvet

import (
	"golang.org/x/tools/go/ssa"

	"github.com/timzifer/metrology"
)

// check reports every conflict it can prove in one function.
func (c *checker) check(fn *ssa.Function) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, isCall := instr.(*ssa.Call)
			if !isCall {
				continue
			}
			name, isCore := c.coreCall(call.Common())
			if !isCore {
				continue
			}
			switch name {
			case "Add", "Sub", "Cmp":
				c.checkAdditive(call, name)
			case "Mul", "Div", "Times", "Per":
				c.checkScaling(call, name)
			case "To", "In", "DecimalIn":
				c.checkConversion(call, name)
			case "Pow":
				c.checkPower(call)
			}
		}
	}
}

// checkAdditive reports a sum, a difference or a comparison the run time would
// refuse, in the order the run time refuses it: the dimension first, because
// it is the coarser mistake, then the quantity, then the kind.
//
// The one rule that outlives the run time sits between the second and the
// third: a tag a product dropped still conflicts, and the run time has no way
// left to see it (D16).
func (c *checker) checkAdditive(call *ssa.Call, op string) {
	left, right, known := c.operands(call.Common())
	if !known {
		return
	}
	switch {
	case left.dim != right.dim:
		c.report(call.Pos(), "%s on incompatible dimensions: %s and %s", op, left.dim, right.dim)
	case !compatible(left.quantity, right.quantity):
		c.report(call.Pos(), "%s on incompatible quantities: %s and %s", op, left.quantity, right.quantity)
	case !compatible(effective(left), effective(right)):
		c.report(call.Pos(), "%s on incompatible quantities: %s and %s; %s", op, describe(left), describe(right), tagWasDropped)
	default:
		if why, forbidden := affineRule(op, left.kind, right.kind); forbidden {
			c.report(call.Pos(), "%s on incompatible kinds: %s and %s; %s", op, left.kind, right.kind, why)
		}
	}
}

// affineRule applies the kind rules of D6 to an operation of two operands and
// reports the rule broken, if any.
func affineRule(op string, left, right metrology.Kind) (why string, forbidden bool) {
	switch op {
	case "Add":
		if left == metrology.Absolute && right == metrology.Absolute {
			return "the sum of two points on a scale is not a point on it", true
		}
	case "Sub":
		if left == metrology.Interval && right == metrology.Absolute {
			return "a point on a scale cannot be subtracted from a span along it", true
		}
	default: // Cmp
		if left != right {
			return "a point on a scale and a span along it are not comparable", true
		}
	}
	return "", false
}

// checkScaling reports a product or a quotient of a point on a scale, whether
// of two measurements or of the units themselves. Dimensions never conflict
// here: every pair of dimensions has a product.
func (c *checker) checkScaling(call *ssa.Call, op string) {
	left, right, known := c.operands(call.Common())
	if !known {
		return
	}
	if left.kind == metrology.Absolute || right.kind == metrology.Absolute {
		c.report(call.Pos(), "%s on incompatible kinds: %s and %s; a point on a scale has no product",
			op, left.kind, right.kind)
	}
}

// checkConversion reports a conversion onto a scale that cannot hold the
// magnitude, in the order Engine.To refuses it.
func (c *checker) checkConversion(call *ssa.Call, op string) {
	from, to, known := c.operands(call.Common())
	if !known {
		return
	}
	switch {
	case from.dim != to.dim:
		c.report(call.Pos(), "%s on incompatible dimensions: %s and %s", op, from.dim, to.dim)
	case from.kind != to.kind:
		c.report(call.Pos(), "%s on incompatible kinds: %s and %s; a point on a scale and a span along it are not the same quantity",
			op, from.kind, to.kind)
	case !compatible(from.quantity, to.quantity):
		c.report(call.Pos(), "%s on incompatible quantities: %s and %s", op, from.quantity, to.quantity)
	case !compatible(effective(from), effective(to)):
		c.report(call.Pos(), "%s on incompatible quantities: %s and %s; %s", op, describe(from), describe(to), tagWasDropped)
	}
}

// tagWasDropped closes the one diagnostic that predicts no run-time error. It
// says so, because a reader who runs the code and sees it pass would otherwise
// conclude the checker is wrong (D16).
const tagWasDropped = "Mul and Div drop the tag (D6), so the run time no longer sees the conflict"

// describe names the quantity a scale speaks for, saying where the tag is
// provenance rather than a claim the value still makes.
func describe(s scale) string {
	if s.quantity != "" {
		return string(s.quantity)
	}
	return "a magnitude computed from " + string(s.dropped)
}

// checkPower reports a power of a point on a scale. It stands apart from the
// other rules because it has one operand: the square of 20 °C is not 400 of
// anything.
func (c *checker) checkPower(call *ssa.Call) {
	unit, known := c.resolve(call.Common().Args[0])
	if !known {
		return
	}
	if unit.kind == metrology.Absolute {
		c.report(call.Pos(), "Pow on an absolute unit; a point on a scale has no power")
	}
}
