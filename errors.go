package metrology

import (
	"errors"
	"fmt"

	"github.com/timzifer/metrology/dimension"
)

// The error classes of this package (D11).
//
// Every error returned here matches exactly one of these with errors.Is, and
// carries a struct with the context through errors.As. Dimensional analysis
// happens at runtime (D1), so these messages are what a user gets in place of a
// compile error: they name both operands, not just the fact that something did
// not match.
var (
	// ErrDimension reports arithmetic or conversion across incompatible
	// dimensions. See [DimensionError] for which two.
	ErrDimension = errors.New("incompatible dimensions")

	// ErrKind reports an operation the affine rules of D6 forbid: adding two
	// absolute values, subtracting an absolute from an interval, or
	// multiplying an absolute at all. See [KindError].
	ErrKind = errors.New("incompatible kinds")

	// ErrQuantity reports an operation between two different quantities that
	// happen to share a dimension — a frequency and a radioactivity, an
	// absorbed dose and a dose equivalent. See [QuantityError].
	ErrQuantity = errors.New("incompatible quantities")

	// ErrSyntax reports a magnitude or a conversion factor that is not a
	// decimal number. See [SyntaxError].
	ErrSyntax = errors.New("not a decimal number")

	// ErrZeroFactor reports a conversion factor with a zero numerator or a
	// zero denominator. Neither describes a unit: one collapses every
	// magnitude to zero, the other converts nothing at all.
	ErrZeroFactor = errors.New("conversion factor is zero")

	// ErrOffsetKind reports an interval unit carrying an offset. An offset is
	// what makes a scale affine, and an affine scale measures points, not
	// spans — so a unit with an offset must be [Absolute] (D6).
	ErrOffsetKind = errors.New("an interval unit cannot have an offset")

	// ErrRange reports a magnitude that does not fit the requested numeric
	// type, or one that a numeric type cannot represent at all. See
	// [RangeError].
	ErrRange = errors.New("value out of range for the target type")
)

// errNotDecimal is what a [SyntaxError] wraps where the text has the wrong
// shape rather than a shape apd rejected for a reason of its own.
var errNotDecimal = errors.New("not a decimal number")

// DimensionError names both dimensions of a failed operation.
//
// D11: at runtime this message replaces a compile error, so it has to carry
// what a type error would have carried — expected L²M¹T⁻², got L¹M¹T⁻².
type DimensionError struct {
	Op   string              // the operation that failed: "Add", "To", …
	Want dimension.Dimension // the dimension the operation required
	Got  dimension.Dimension // the dimension it was given
}

func (e *DimensionError) Error() string {
	return fmt.Sprintf("metrology: %s: expected %s, got %s", e.Op, e.Want, e.Got)
}

// Is reports that every DimensionError matches [ErrDimension].
func (e *DimensionError) Is(target error) bool { return target == ErrDimension }

// KindError names the two kinds that may not meet in this operation, and what
// the affine rules of D6 say about them.
type KindError struct {
	Op    string // the operation that failed: "Add", "Sub", "Mul", …
	Left  Kind   // kind of the left operand
	Right Kind   // kind of the right operand
	Why   string // the rule that was broken, in one clause
}

func (e *KindError) Error() string {
	return fmt.Sprintf("metrology: %s: %s and %s: %s", e.Op, e.Left, e.Right, e.Why)
}

// Is reports that every KindError matches [ErrKind].
func (e *KindError) Is(target error) bool { return target == ErrKind }

// QuantityError names the two quantities that share a dimension but not a
// meaning.
//
// It is a separate class from [DimensionError] because the dimensions match:
// the exponents agree and the numbers would convert, which is exactly what
// makes the mistake worth reporting.
type QuantityError struct {
	Op    string   // the operation that failed: "To", "Add", …
	Left  Quantity // quantity of the left operand
	Right Quantity // quantity of the right operand
}

func (e *QuantityError) Error() string {
	return fmt.Sprintf("metrology: %s: %s and %s share a dimension but are different quantities",
		e.Op, e.Left, e.Right)
}

// Is reports that every QuantityError matches [ErrQuantity].
func (e *QuantityError) Is(target error) bool { return target == ErrQuantity }

// SyntaxError names the text that could not be read as a decimal.
type SyntaxError struct {
	Op    string // where the text came from: "OfString", "NewUnit", …
	Input string // the text itself
	Err   error  // what the decimal parser said
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("metrology: %s: %q: %v", e.Op, e.Input, e.Err)
}

// Is reports that every SyntaxError matches [ErrSyntax].
func (e *SyntaxError) Is(target error) bool { return target == ErrSyntax }

// Unwrap exposes the underlying parser error.
func (e *SyntaxError) Unwrap() error { return e.Err }

// RangeError reports a magnitude that the requested numeric type cannot hold.
//
// It is returned by [Measurement.In] rather than a truncated number: a library
// whose purpose is the exact value has no business handing back a silently
// wrapped one.
type RangeError struct {
	Op    string // the operation that failed: "In", "Of", …
	Value string // the magnitude, in full
	Type  string // the target type
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("metrology: %s: %s does not fit %s", e.Op, e.Value, e.Type)
}

// Is reports that every RangeError matches [ErrRange].
func (e *RangeError) Is(target error) bool { return target == ErrRange }
