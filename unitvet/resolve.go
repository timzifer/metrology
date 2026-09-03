package unitvet

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/dimension"
)

// resolve reports the scale a measurement or a unit is on, and whether the
// checker can prove it.
//
// Everything not proved is unknown, and an unknown operand makes the pass
// silent (D13). There is no cycle to guard against: the only SSA value that
// can refer to itself is a phi node, and a phi is a unit chosen at run time,
// which is unknown by the first rule of this pass.
func (c *checker) resolve(v ssa.Value) (scale, bool) {
	if r, seen := c.resolved[v]; seen {
		return r.scale, r.known
	}
	s, known := c.resolveValue(v)
	c.resolved[v] = resolution{scale: s, known: known}
	return s, known
}

func (c *checker) resolveValue(v ssa.Value) (scale, bool) {
	switch v := v.(type) {
	case *ssa.UnOp:
		// A package-level unit is read through its address: pressure.Bar is a
		// load of a global. Only a unit in the generated table is trusted, so
		// a global of somebody else's that happens to hold a unit stays
		// unknown — nothing here assumes a variable is never written to.
		if v.Op == token.MUL {
			if g, isGlobal := v.X.(*ssa.Global); isGlobal {
				s, found := catalogue[g.Pkg.Pkg.Path()+"."+g.Name()]
				return s, found
			}
		}
	case *ssa.Extract:
		// Everything that can fail returns (Measurement, error); the
		// measurement is the first component.
		if v.Index == 0 {
			return c.resolve(v.Tuple)
		}
	case *ssa.Call:
		return c.resolveCall(v.Common())
	}
	return scale{}, false
}

// resolveCall reports the scale a call returns.
func (c *checker) resolveCall(call *ssa.CallCommon) (scale, bool) {
	if name, isCore := c.coreCall(call); isCore {
		return c.coreResult(name, call)
	}
	callee := call.StaticCallee()
	if callee == nil {
		// A call through an interface, a func value or a closure. Which
		// function runs is not decided here, so neither is the unit.
		return scale{}, false
	}
	obj := callee.Object()
	if obj == nil {
		// An anonymous function has no declaration a fact can hang off.
		return scale{}, false
	}
	var fact resultScale
	if c.pass.ImportObjectFact(obj, &fact) {
		return fact.scale(), true
	}
	return scale{}, false
}

// coreCall reports which method of the library's own types a call invokes.
//
// It insists on a genuine method call — a callee whose signature still has its
// receiver. A method value binds the receiver into a closure and calls the
// wrapper without it, and the wrapper carries the method's own object, so
// reading the operands by position off such a call would read the wrong ones.
// A receiver bound out of sight is not something this pass can prove anything
// about anyway.
func (c *checker) coreCall(call *ssa.CallCommon) (name string, ok bool) {
	callee := call.StaticCallee()
	if callee == nil {
		return "", false
	}
	if origin := callee.Origin(); origin != nil {
		// buildssa runs with ssa.BuilderMode(0), so an instantiated generic
		// method is a function of its own and names itself In[float64]. The
		// rule is about In, whichever number it reads into (see the D13 entry
		// in the verification log of CONCEPT.md).
		callee = origin
	}
	receiver := callee.Signature.Recv()
	if receiver == nil {
		return "", false
	}
	named, isNamed := types.Unalias(receiver.Type()).(*types.Named)
	if !isNamed {
		// A pointer receiver. The library's own API has none: a Measurement
		// and a Unit are values (D1).
		return "", false
	}
	if _, isCore := c.core[named.Obj()]; !isCore {
		return "", false
	}
	return callee.Name(), true
}

// coreResult reports the scale a call to one of the library's own methods
// returns.
//
// The operand positions rely on the SSA argument list being flat — the
// receiver first, then the parameters — so that the last two entries are the
// two operands whether the method is on a Measurement or on an Engine, which
// takes both explicitly. The method set and this table are generated and
// written together with the API they describe; a change to one is a change to
// the other.
func (c *checker) coreResult(name string, call *ssa.CallCommon) (scale, bool) {
	switch name {
	case "Of", "OfString":
		// A magnitude is read on the unit it is put on.
		return c.resolve(call.Args[0])

	case "To":
		// A conversion lands on its target, whether or not it is legal; an
		// illegal one returns the zero Measurement, which the diagnostic for
		// the conversion itself already reports.
		return c.resolve(last(call))

	case "Times", "Per", "Pow":
		return c.composed(name, call)

	case "Add", "Sub":
		return c.combined(name, call)

	case "Mul", "Div":
		return c.scaled(name, call)
	}
	return scale{}, false
}

// composed reports the scale of a unit built out of other units.
//
// Both drop the kind and the quantity, exactly as the arithmetic of D6 does:
// a product of a force and a length is not a torque until someone says so.
func (c *checker) composed(name string, call *ssa.CallCommon) (scale, bool) {
	left, known := c.resolve(call.Args[0])
	if !known || left.kind == metrology.Absolute {
		// A point on a scale has no product and no power; the operation
		// returns an error and the zero Unit.
		return scale{}, false
	}
	if name == "Pow" {
		exponent, isConst := call.Args[1].(*ssa.Const)
		if !isConst {
			return scale{}, false
		}
		n := exponent.Int64()
		if n < -metrology.MaxPower || n > metrology.MaxPower {
			return scale{}, false
		}
		dim := left.dim.Pow(dimension.Exponent(n))
		return scale{dim: dim, dropped: dropTag(dim, left, scale{})}, true
	}

	right, known := c.resolve(call.Args[1])
	if !known || right.kind == metrology.Absolute {
		return scale{}, false
	}
	if name == "Times" {
		dim := dimension.Product(left.dim, right.dim)
		return scale{dim: dim, dropped: dropTag(dim, left, right)}, true
	}
	dim := dimension.Quotient(left.dim, right.dim)
	return scale{dim: dim, dropped: dropTag(dim, left, right)}, true
}

// combined reports the scale of a sum or a difference.
//
// The result is on the receiver's dimension and carries whichever tag the two
// operands make a claim with. An operation the rules of D6 forbid has no
// result at all: it returns the zero Measurement, and pretending otherwise
// would make one mistake report as several.
func (c *checker) combined(name string, call *ssa.CallCommon) (scale, bool) {
	left, right, known := c.operands(call)
	if !known || left.dim != right.dim || !compatible(left.quantity, right.quantity) {
		return scale{}, false
	}
	quantity := left.quantity
	if quantity == "" {
		quantity = right.quantity
	}
	// A sum neither drops a tag nor restores one, so whatever provenance the
	// operands carried travels with it (D16).
	dropped := dropTag(left.dim, left, right)
	if name == "Add" {
		if left.kind == metrology.Absolute && right.kind == metrology.Absolute {
			return scale{}, false
		}
		if left.kind == metrology.Absolute || right.kind == metrology.Absolute {
			return scale{dim: left.dim, kind: metrology.Absolute, quantity: quantity, dropped: dropped}, true
		}
		return scale{dim: left.dim, quantity: quantity, dropped: dropped}, true
	}
	switch {
	case left.kind == metrology.Absolute && right.kind == metrology.Absolute:
		// The difference of two points is read on the interval unit the
		// receiver's scale declares — K for °C — and which unit that is, is
		// not something this table records. The dimension is settled, the tag
		// is not, so the answer is that there is no answer.
		return scale{}, false
	case right.kind == metrology.Absolute:
		return scale{}, false
	default:
		return scale{dim: left.dim, kind: left.kind, quantity: quantity, dropped: dropped}, true
	}
}

// scaled reports the scale of a product or a quotient of two measurements.
func (c *checker) scaled(name string, call *ssa.CallCommon) (scale, bool) {
	left, right, known := c.operands(call)
	if !known || left.kind == metrology.Absolute || right.kind == metrology.Absolute {
		return scale{}, false
	}
	if name == "Mul" {
		dim := dimension.Product(left.dim, right.dim)
		return scale{dim: dim, dropped: dropTag(dim, left, right)}, true
	}
	dim := dimension.Quotient(left.dim, right.dim)
	return scale{dim: dim, dropped: dropTag(dim, left, right)}, true
}

// operands resolves the two values an operation combines: the last two entries
// of the argument list, which are the receiver and the argument for a method
// on a Measurement and the two explicit operands for the same method on an
// Engine.
func (c *checker) operands(call *ssa.CallCommon) (left, right scale, known bool) {
	args := call.Args
	left, leftKnown := c.resolve(args[len(args)-2])
	right, rightKnown := c.resolve(args[len(args)-1])
	return left, right, leftKnown && rightKnown
}

// last is the final argument of a call: the target of a conversion.
func last(call *ssa.CallCommon) ssa.Value { return call.Args[len(call.Args)-1] }

// compatible mirrors the run-time rule of D6: two tags must agree, and an
// untagged operand goes either way.
func compatible(left, right metrology.Quantity) bool {
	return left == "" || right == "" || left == right
}

// effective is the quantity a scale speaks for: its own tag where it has one,
// and otherwise the tag an earlier product dropped (D16).
//
// The two are not interchangeable at the run time — a dropped tag is gone, and
// the arithmetic treats the magnitude as untagged — which is exactly why the
// checker keeps them apart and only compares them here.
func effective(s scale) metrology.Quantity {
	if s.quantity != "" {
		return s.quantity
	}
	return s.dropped
}

// dropTag reports the quantity a multiplicative result carries as provenance.
//
// A product drops the tag (D6) and the run time cannot get it back. The checker
// can, for the one case where the tag still says something about the result: an
// operand whose dimension the result kept. Scaling a becquerel by a plain
// number leaves a T⁻¹ that is still a radioactivity; multiplying it by a metre
// leaves a T⁻¹L¹ that is not anything the catalogue names, and there the tag is
// gone for good.
//
// Where both operands survive with tags of their own and the tags differ —
// a plane angle times a solid angle, both dimensionless — there is no single
// answer, and no answer is the same as none.
func dropTag(result dimension.Dimension, left, right scale) metrology.Quantity {
	var tag metrology.Quantity
	for _, operand := range [...]scale{left, right} {
		carried := effective(operand)
		if operand.dim != result || carried == "" {
			continue
		}
		if tag != "" && tag != carried {
			return ""
		}
		tag = carried
	}
	return tag
}
