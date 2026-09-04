# D4 — Factors as exact fractions

A derived unit carries numerator and denominator separately, plus an offset as an
exact decimal. Conversion to the base unit is `(v + offset) · num / den`,
performed as an exact multiplication followed by *one* division.

**Why.** The domain's most important factors are not finite in decimal:
Fahrenheit is 5/9, Torr is 101325/760. Stored as a pre-rounded decimal, every
conversion rounds twice. Stored as a fraction, it rounds once — and the catalogue
stays exactly what the SI Brochure says rather than an approximation of it.

**Auditability.** A catalogue entry can be compared to its source character by
character. `factor: 101325/760` is checkable; `133.32236842105263` is not. Every
entry therefore carries a `source:` citation and the generator refuses one that
does not: a conversion factor is a claim about the world, and a claim without a
citation cannot be checked.

**Exactness is a precondition, not a preference.** A unit whose factor is a
rational multiple of π — the degree of arc, the gon, the oersted — has no finite
decimal fraction, and shipping a rounded one silently because the unit is popular
is how a catalogue stops being auditable. Such a unit is not left out and not
rounded in: D20 gives the factor a π exponent beside the fraction, so
`{den: "180", pi: 1}` is the degree and can be checked against the SI Brochure
character by character. The exponents subtract on conversion, so the exactness
above is unchanged for every conversion that stays inside the π units, and only
one that crosses out of them puts digits in place of π — the single amendment
this decision takes, recorded in D20 rather than hidden here.
