// Package dimension packs the seven SI base-quantity exponents into one
// comparable integer.
//
// A [Dimension] is a value: copyable, comparable with ==, usable as a map key
// and free of allocation. It carries no kind — the absolute/interval marker and
// the collision resolution of D6 live next to it in the core, not inside the
// word (D5). Keeping kind out of the word is what makes the arithmetic here
// total: every operation is defined on all seven axes and has no second field
// to preserve, mask or accidentally drop.
package dimension

import "github.com/timzifer/metrology/internal/superscript"

// Exponent is the power of one base quantity. Physical dimensions stay far
// inside the int8 range — L³ is the largest exponent in the SI catalogue.
type Exponent int8

// Dimension is seven int8 exponents packed into one word (D5).
//
// Bits 56–63 are reserved and always zero for a Dimension built by [New]. They
// are held for fractional exponents, which are deferred, not forgotten (D5) —
// see section 9 of CONCEPT.md.
type Dimension uint64

// Bit offsets of the seven axes, in the order of D5.
const (
	shiftTime = 8 * iota
	shiftLength
	shiftMass
	shiftElectricCurrent
	shiftTemperature
	shiftAmountOfSubstance
	shiftLuminousIntensity
)

// One is the dimensionless dimension: every exponent zero.
const One Dimension = 0

// The seven base dimensions. They are constants rather than variables so that
// no package-level state exists to be mutated (D7).
const (
	T Dimension = 1 << shiftTime              // time
	L Dimension = 1 << shiftLength            // length
	M Dimension = 1 << shiftMass              // mass
	I Dimension = 1 << shiftElectricCurrent   // electric current
	Θ Dimension = 1 << shiftTemperature       // thermodynamic temperature
	N Dimension = 1 << shiftAmountOfSubstance // amount of substance
	J Dimension = 1 << shiftLuminousIntensity // luminous intensity
)

// Exponents names the seven axes. It is the argument of [New] and the result of
// [Dimension.Exponents], so that constructing and destructuring a dimension
// read the same way and neither depends on positional order.
type Exponents struct {
	Time              Exponent
	Length            Exponent
	Mass              Exponent
	ElectricCurrent   Exponent
	Temperature       Exponent
	AmountOfSubstance Exponent
	LuminousIntensity Exponent
}

// New packs e into a Dimension.
func New(e Exponents) Dimension {
	// The cast to uint8 before widening is what carries the sign: converting a
	// negative Exponent straight to Dimension would sign-extend across all
	// seven axes.
	return Dimension(uint8(e.Time))<<shiftTime |
		Dimension(uint8(e.Length))<<shiftLength |
		Dimension(uint8(e.Mass))<<shiftMass |
		Dimension(uint8(e.ElectricCurrent))<<shiftElectricCurrent |
		Dimension(uint8(e.Temperature))<<shiftTemperature |
		Dimension(uint8(e.AmountOfSubstance))<<shiftAmountOfSubstance |
		Dimension(uint8(e.LuminousIntensity))<<shiftLuminousIntensity
}

// Time returns the exponent of the time axis.
func (d Dimension) Time() Exponent { return Exponent(uint8(d >> shiftTime)) }

// Length returns the exponent of the length axis.
func (d Dimension) Length() Exponent { return Exponent(uint8(d >> shiftLength)) }

// Mass returns the exponent of the mass axis.
func (d Dimension) Mass() Exponent { return Exponent(uint8(d >> shiftMass)) }

// ElectricCurrent returns the exponent of the electric current axis.
func (d Dimension) ElectricCurrent() Exponent {
	return Exponent(uint8(d >> shiftElectricCurrent))
}

// Temperature returns the exponent of the thermodynamic temperature axis.
func (d Dimension) Temperature() Exponent { return Exponent(uint8(d >> shiftTemperature)) }

// AmountOfSubstance returns the exponent of the amount of substance axis.
func (d Dimension) AmountOfSubstance() Exponent {
	return Exponent(uint8(d >> shiftAmountOfSubstance))
}

// LuminousIntensity returns the exponent of the luminous intensity axis.
func (d Dimension) LuminousIntensity() Exponent {
	return Exponent(uint8(d >> shiftLuminousIntensity))
}

// Exponents unpacks d into its seven axes.
func (d Dimension) Exponents() Exponents {
	return Exponents{
		Time:              d.Time(),
		Length:            d.Length(),
		Mass:              d.Mass(),
		ElectricCurrent:   d.ElectricCurrent(),
		Temperature:       d.Temperature(),
		AmountOfSubstance: d.AmountOfSubstance(),
		LuminousIntensity: d.LuminousIntensity(),
	}
}

// IsOne reports whether d is dimensionless.
func (d Dimension) IsOne() bool { return d == One }

// axes is the display order of D11: L²M¹T⁻² rather than T⁻²L²M¹. It is fixed
// rather than sorted by exponent so that two dimensions differing in one
// exponent produce two strings differing in one place — the error message of
// D11 is read by someone comparing them.
var axes = [...]struct {
	symbol   rune
	exponent func(Dimension) Exponent
}{
	{'L', Dimension.Length},
	{'M', Dimension.Mass},
	{'T', Dimension.Time},
	{'I', Dimension.ElectricCurrent},
	{'Θ', Dimension.Temperature},
	{'N', Dimension.AmountOfSubstance},
	{'J', Dimension.LuminousIntensity},
}

// String renders d in the form L²M¹T⁻², the notation D11 requires of dimension
// errors. The dimensionless dimension renders as "1".
func (d Dimension) String() string {
	if d.IsOne() {
		return "1"
	}
	var out []rune
	for _, axis := range axes {
		e := axis.exponent(d)
		if e == 0 {
			continue
		}
		out = append(out, axis.symbol)
		out = append(out, []rune(superscript.Itoa(int(e)))...)
	}
	return string(out)
}
