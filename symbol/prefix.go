package symbol

// prefix is one entry of a prefix table: the decimal exponent it stands for and
// the letter it is written with.
type prefix struct {
	exponent int
	name     string
}

// siPrefixes is the full SI prefix range, ascending. The 2022 additions (ronna,
// ronto, quetta, quecto) are included.
var siPrefixes = []prefix{
	{-30, "q"}, {-27, "r"}, {-24, "y"}, {-21, "z"}, {-18, "a"}, {-15, "f"},
	{-12, "p"}, {-9, "n"}, {-6, "µ"}, {-3, "m"}, {0, ""}, {3, "k"},
	{6, "M"}, {9, "G"}, {12, "T"}, {15, "P"}, {18, "E"}, {21, "Z"},
	{24, "Y"}, {27, "R"}, {30, "Q"},
}

// litrePrefixes is the set actually used with the litre. Centi, deci and hecto
// are outside the powers of a thousand the SI otherwise prefers, but "cl" and
// "hl" are what labels say.
var litrePrefixes = []prefix{
	{-6, "µ"}, {-3, "m"}, {-2, "c"}, {-1, "d"}, {0, ""}, {2, "h"},
}

// pick returns the prefix that brings the printed magnitude closest to [1,
// 1000) from below, clamped to what the table offers.
//
// power is the exponent of the symbol: one prefix step on m² moves the decimal
// point by six places, not three. A negative power — m⁻¹ is a wavenumber —
// reverses the order of the scaled exponents, which is why the table is
// searched rather than walked until the first entry that is too large. A power
// of zero has no prefix at all: the zero prefix scales by 10⁰ and prints
// nothing.
func pick(table []prefix, e10, power int) prefix {
	if power == 0 {
		return prefix{}
	}
	// Start from the smallest reachable magnitude, so that a value below the
	// whole table clamps to the smallest prefix instead of falling through.
	best := table[0]
	bestScale := best.exponent * power
	for _, p := range table[1:] {
		if scale := p.exponent * power; scale < bestScale {
			best, bestScale = p, scale
		}
	}
	for _, p := range table {
		if scale := p.exponent * power; scale <= e10 && scale > bestScale {
			best, bestScale = p, scale
		}
	}
	return best
}
