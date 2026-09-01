package parse

import (
	"errors"
	"fmt"

	"github.com/timzifer/metrology"
)

// The error classes this package adds to those of D11.
//
// Text that is not a measurement is a [metrology.SyntaxError] and matches
// metrology.ErrSyntax, because the failure is the same one the core reports for
// a magnitude that is not a number. What is new here is a symbol that reads
// perfectly well and names no unit, and a database value that is not text at
// all.
var (
	// ErrUnknownUnit reports a symbol this parser has no unit for. See
	// [UnknownUnitError] for which symbol.
	ErrUnknownUnit = errors.New("unknown unit")

	// ErrNotText reports a value handed to [Text.Scan] that a measurement
	// cannot be read from — a NULL, or a column of a type that carries no
	// measurement. See [ScanError].
	ErrNotText = errors.New("not a measurement")
)

// UnknownUnitError names the symbol that resolved to no unit.
//
// It is deliberately not a syntax error: the text is well formed, and the
// mistake is that this parser was built without the unit — which is a different
// thing to look for and, for a program with its own catalogue, a different
// thing to fix.
type UnknownUnitError struct {
	Input  string // the text the unit was read from
	Symbol string // the part of it that named no unit
}

func (e *UnknownUnitError) Error() string {
	return fmt.Sprintf("metrology: parse: %q: %q is not a unit this parser knows", e.Input, e.Symbol)
}

// Is reports that every UnknownUnitError matches [ErrUnknownUnit].
func (e *UnknownUnitError) Is(target error) bool { return target == ErrUnknownUnit }

// ScanError names a database value a measurement cannot be read from.
//
// A NULL is one of them: the zero measurement is a dimensionless zero and not
// "no measurement", so scanning a NULL into one would invent a reading that the
// row does not have. A nullable column is read into an sql.Null[[Text]].
type ScanError struct {
	Value any // the value the driver handed over
}

func (e *ScanError) Error() string {
	return fmt.Sprintf("metrology: parse: cannot read a measurement from %T", e.Value)
}

// Is reports that every ScanError matches [ErrNotText].
func (e *ScanError) Is(target error) bool { return target == ErrNotText }

// syntaxError reports text this package could not read, in the error class the
// core uses for the same failure (D11).
func syntaxError(input, why string) error {
	return &metrology.SyntaxError{Op: "parse", Input: input, Err: errors.New(why)}
}
