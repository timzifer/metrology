package gum

import (
	"errors"
	"fmt"
)

// ErrInput reports an input this package cannot make a value out of: a negative
// standard uncertainty, negative degrees of freedom, a correlation outside
// [−1, 1], or a sample too small to have a standard deviation. See [InputError].
//
// Everything else a caller can get wrong is a rule of the core and is reported
// by it: a tolerance in the wrong dimension, a point where a span belongs, two
// quantities that share a dimension and are not the same thing (D6, D11).
var ErrInput = errors.New("not a usable input")

// InputError names the input that could not be used and what was wrong with it.
//
// D11: at runtime this message is what replaces a compile error, so it carries
// the name the caller gave the input rather than only the rule it broke — a
// budget with nine inputs needs to say which one.
type InputError struct {
	Op   string // the operation that failed: "Of", "Correlated", "Sample"
	Name string // the caller's name for the input, where it gave one
	Why  string // what was wrong, in words
}

func (e *InputError) Error() string {
	if e.Name == "" {
		return fmt.Sprintf("gum: %s: %s", e.Op, e.Why)
	}
	return fmt.Sprintf("gum: %s: %s: %s", e.Op, e.Name, e.Why)
}

// Is reports that every InputError matches [ErrInput].
func (e *InputError) Is(target error) bool { return target == ErrInput }
