package unitvet

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// exportFacts records, for every exported function whose result is provably
// always on one and the same scale, what that scale is.
//
// This is how a diagnostic crosses a package boundary (D13): the importing
// package has no SSA for the callee, only the fact, and a fact is exactly as
// much as can be said without one.
//
// The loop runs to a fixed point because one such function may be written in
// terms of another, and the second is determinate only once the first is. It
// terminates: there are finitely many functions and a fact, once exported, is
// never withdrawn.
//
// The memo of resolved values is dropped at the start of every round. A value
// that resolved to nothing before a fact existed would otherwise stay resolved
// to nothing after it did, and the fixed point would depend on the order the
// functions happen to be in.
func (c *checker) exportFacts(funcs []*ssa.Function) {
	for changed := true; changed; {
		changed = false
		c.resolved = map[ssa.Value]resolution{}
		for _, fn := range funcs {
			if c.exportFact(fn) {
				changed = true
			}
		}
	}
}

// exportFact examines one function and reports whether it learned something
// new about it.
func (c *checker) exportFact(fn *ssa.Function) bool {
	obj := fn.Object()
	if obj == nil || !obj.Exported() {
		// An anonymous or unexported function. A fact on it would never be
		// read from another package, which is the only thing facts are for.
		return false
	}
	var known resultScale
	if c.pass.ImportObjectFact(obj, &known) {
		return false
	}
	results := fn.Signature.Results()
	if results.Len() == 0 {
		return false
	}
	if _, isCore := c.coreType(results.At(0).Type()); !isCore {
		// Everything else — an error, a number, a string — is not a scale.
		return false
	}

	var found scale
	seen := false
	for _, block := range fn.Blocks {
		ret, isReturn := block.Instrs[len(block.Instrs)-1].(*ssa.Return)
		if !isReturn {
			continue
		}
		s, resolved := c.resolve(ret.Results[0])
		if !resolved || (seen && s != found) {
			// One return the checker cannot resolve, or two that disagree,
			// and the function has no single scale to speak of.
			return false
		}
		found, seen = s, true
	}
	if !seen {
		// A function without a body: an assembly stub, or a declaration this
		// package only compiles against.
		return false
	}
	c.pass.ExportObjectFact(obj, factOf(found))
	return true
}

// coreType reports which of the library's types a type is, if it is one.
//
// A Range counts as much as a Measurement: a function returning one is a
// function whose result is on a scale, and the fact that carries that scale
// across a package boundary says nothing about which of the two it was.
func (c *checker) coreType(t types.Type) (owned, bool) {
	named, isNamed := types.Unalias(t).(*types.Named)
	if !isNamed {
		return owned{}, false
	}
	found, ok := c.core[named.Obj()]
	return found, ok
}
