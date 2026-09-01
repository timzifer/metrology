package dimension

// Product multiplies dimensions by adding their exponents axis by axis.
//
// Product(nothing) is [One], the identity the empty product should have. All
// seven axes are summed, and nothing else travels with a dimension that could
// be lost here (D5).
//
// Exponents wrap at the int8 boundary. Reaching it takes 128 multiplications of
// the same axis; no physical quantity comes near, and the alternative — an
// error return on every product — would push a case that cannot occur into
// every caller of D6 arithmetic.
func Product(ds ...Dimension) Dimension {
	var e Exponents
	for _, d := range ds {
		e.Time += d.Time()
		e.Length += d.Length()
		e.Mass += d.Mass()
		e.ElectricCurrent += d.ElectricCurrent()
		e.Temperature += d.Temperature()
		e.AmountOfSubstance += d.AmountOfSubstance()
		e.LuminousIntensity += d.LuminousIntensity()
	}
	return New(e)
}

// Quotient divides numerator by denominator.
func Quotient(numerator, denominator Dimension) Dimension {
	return Product(numerator, denominator.Reciprocal())
}

// Reciprocal negates every exponent, so that Product(d, d.Reciprocal()) is
// [One] for every d whose exponents lie in [-127, 127] — that is, for every d
// that [New] can produce from realistic input. Only the single value -128
// negates to itself, by the same int8 wrap Product documents.
func (d Dimension) Reciprocal() Dimension { return d.Pow(-1) }

// Pow raises d to the n-th power by scaling every exponent.
func (d Dimension) Pow(n Exponent) Dimension {
	e := d.Exponents()
	return New(Exponents{
		Time:              e.Time * n,
		Length:            e.Length * n,
		Mass:              e.Mass * n,
		ElectricCurrent:   e.ElectricCurrent * n,
		Temperature:       e.Temperature * n,
		AmountOfSubstance: e.AmountOfSubstance * n,
		LuminousIntensity: e.LuminousIntensity * n,
	})
}
