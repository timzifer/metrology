# D12 — Text is the canonical exchange format

`MarshalText` / `UnmarshalText` as the foundation, with `json.Marshaler`,
`sql.Scanner` and `driver.Valuer` layered on top. A measurement serialises as
`"2.5 bar"` and round-trips losslessly.

**Why text and not number plus unit.** An object `{"value": 2.5, "unit": "bar"}`
forces every consumer through `float64` and thereby loses exactly what D2 through
D4 were built for. The text form preserves the decimal digits. The object form
stays available as an option but is not the default.

**Writing belongs to the measurement, reading does not.** Writing needs nothing
but the value: `Measurement` implements `MarshalText`, `MarshalJSON` and
`driver.Valuer` itself. Reading needs a catalogue — `"bar"` is a unit only
because something says so — and the core has no catalogue and no registry to put
one in (D7, D8). The standard decoding interfaces are handed no context, so a
`Measurement.UnmarshalText` could resolve symbols only out of a package-level
registry, and every program with units of its own would be locked out of exactly
the interfaces it needs most.

Reading therefore lives in `parse`, where a parser is a *value* holding the units
it knows — `parse.Measurement` over the shipped catalogue, `parse.New(mine)` over
any other — and `parse.Text` is the destination type that carries its parser into
`json.Unmarshal` and `sql.Scan`. The asymmetry is the honest shape of the
problem: a symbol table is context, and Go's decoding interfaces pass none.

**What the text does not carry.** It carries a magnitude and a symbol, and
neither the kind of D6 nor the quantity tag. `"5 K"` is a temperature and a
temperature difference written the same way; `"5 m²/s"` says nothing about which
of the quantities on that dimension was meant. A parser resolves both from what
it has: the kind from `Parser.Prefer` — defaulting to the interval reading,
because that is the one that composes with an absolute one — and the quantity
from the catalogue entry the symbol resolves to. An expression such as `"50 N/m²"`
resolves to no catalogue entry and is therefore untagged, which is what a
computed magnitude is too (D6), so it converts into any unit of its dimension.

**The spelling is the statement**, and that is the answer to the last of the
three questions [section 7](../status.md) asked about `Quantity` (D16). `"50 Hz"` reads back
tagged and `"50 s⁻¹"` untagged, for one scale, and the asymmetry is honest
rather than accidental: someone who writes `Hz` means a frequency, and someone
who writes `s⁻¹` has written a rate and said nothing about which quantity it is.
Untagged is not a failure to determine the tag — it is the correct reading of a
text that made no claim.

The alternatives were worse. Resolving by scale rather than by spelling would
name `kg·m/s²` a newton, and D6 is explicit that a product of a force and a
length is not a torque until someone says so. Carrying the tag in the text needs
a notation that no gauge, no standard and no other program writes, and [section 8](../deferred.md)
is where that waits. An asymmetry that can be stated in one sentence beats
either.

**A reader with an expectation states it, and the API already has one way to.**

```go
m, err := parse.Measurement(text)     // whatever the text said
hz, err := m.To(frequency.Hertz)      // a frequency in hertz, or an error
```

`To` is the checked step D6 requires everywhere else: an untagged magnitude goes
in and a tagged one comes out, a conflicting tag is an error rather than a
silent reinterpretation, and a wrong dimension is an error too. A program that
expects hertz gets hertz or gets told, whichever spelling arrived. There is
deliberately no second name for it — an API about to be frozen ([section 7](../status.md)) is
the worst place to grow a synonym for an operation it already has.

**A symbol's spellings are enumerated, never guessed.** `Symbol.Spellings`
reports every way a symbol may be written and the parser indexes exactly those. A
static symbol admits no prefix at all, and a prefix is only ever read in front of
a symbol whose form declares one. That is what keeps `cd` the candela rather than
a centi-day, and `mmHg` a millimetre of mercury rather than a
milli-metre-of-mercury. A longest-prefix matcher over the alphabet would have to
guess, and would guess wrong on exactly these. Where a spelling collides, the
catalogue entry wins: `km` is the kilometre with the source citation, not the
prefixed metre — the same scale either way.

**Rendering has to be unambiguous before parsing can be exact.** A solidus and a
middle dot bind equally and from the left, so `m/(s/A)` and `b/(km/h)` need their
brackets — without them they read as `(m/s)/A` and `(b/km)/h`, which are
different dimensions. The bracketing rule lives on the rendered text rather than
on the symbol form, because a static symbol can join two units too. For the same
reason a product of a product is flattened on construction: it already rendered
flat, so keeping the nesting left two structures for one symbol and made
`Symbol.Equal` answer false for two symbols that print alike.

The same rule reaches one place it had missed: a **quotient in any but the first
place of a product brackets itself**. `N·m/s` reads back as `(N·m)/s`, so a
product of a newton and a metre-per-second has to render `N·(m/s)` or it renders
a unit it is not. The first multiplicand needs none — `m/s·N` already reads as
`(m/s)·N`, which is what it is. This went unnoticed while `m·m²/s` read back as
a product of `m` and `m²` and rendered the same string again: two structures for
one spelling, which is the defect the flattening rule above describes, hiding
the wrong spelling underneath it.

**Repeated prefixable factors gather into a power.** `Times` used to spell
`m·m` where `Pow` spelled `m²`, and the two are one scale. That is not a
cosmetic difference: `Unit.Equal` compares symbols, so `m.Times(m)` was *not*
the square metre; the substitution below looks the rendered symbol up in the
catalogue, so `m·m/s` missed `m²/s` and a magnitude lost the quantity tag of D6
to a notation. `Symbol.Product` now adds the powers of repeated multiplicands —
`m·m` is `m²`, `m²·m` is `m³`, `N·m` is untouched — and a base that cancels to
the zeroth power drops out.

**Only a prefixable symbol gathers**, and the restriction is what makes the rule
sound rather than merely nice. An SI symbol records its power as a number, so a
power can be added to it and taken off again. Every other form carries its power
in its text — `Pow` of a static torr is the static `"torr²"` — and a static that
has been raised cannot be recognised as a power of anything afterwards.
Gathering those renders `1·1·1` as `1²·1`, then `1²·1²`, then `(1²)²`: a
spelling that reads back as a different symbol every time it is written. The
fuzzer found that within thirty seconds of the rule being written without the
restriction, and the input is in the corpus.

**Where the gathering stops.** It normalises a *spelling*, never a *name*. Two
limits follow and both are deliberate. A quotient does not cancel: `mm/m` is a
strain and `1` is not what an engineer asked to see, so `m/m` stays `m/m`. And
`m²·s⁻¹` is the same scale as `m²/s` and still reads back untagged, because the
substitution is a lookup of the rendered symbol and `m²·s⁻¹` is not a spelling
the catalogue holds. Widening it to *any* known unit of the same scale would
turn `kg·m/s²` into `N`, and D6 is explicit that a product of a force and a
length is not a torque until someone says so.

**An expression that spells a unit the parser knows *is* that unit.** `m²/ s` and
`m²/s` differ by a blank, and without this substitution only the second would
carry the quantity tag of D6. The substitution checks the scale and not only the
spelling: a caller's catalogue may spell something `m/s` that is not a metre per
second, and naming that would change the factor instead of the tag.

**A power of a power is bounded before it is computed.** `(Qm^127)^127` is a
factor of half a million digits, and one bracket more is sixty million — all in
fourteen characters of input. The parser multiplies the exponents of nested
brackets and refuses anything beyond `MaxPower`, the same bound a single power
has. Judging the result afterwards would be too late.

**The core validates the shape of a number itself,** through
`internal/decimaltext`, the one scanner both the core and the parser use: apd
accepts `".-1"` and renders it as `"0.-1"`, a value whose own text form is not a
value, and a zero with a positive exponent prints as `"00"`. Reading untrusted
text is what makes such defects reachable, but they are core defects and the
check belongs in `Unit.OfString`. The *range* of a magnitude stays apd's
business: it rejects an absurd exponent first, so a second check in the parser
would be dead code.

**Prefix selection is exact decimal arithmetic** (D9), not a logarithmic search:
`floor(log10 |v|)` comes from the digit count and the exponent of the
`apd.Decimal`, and applying the prefix shifts that exponent, so no digit is lost
and 1000 m is exactly 1 km where `math.Log` yields 999.9999999 m. The result is
trimmed of the trailing zeros the shift introduces but not reduced past the
decimal point: 250 kPa stays `250`, not `2.5E+2`. One prefix step on `m²` is a
factor of 10⁶ and on `m³` a factor of 10⁹, so the step scales with the power and
handles negative powers such as the wavenumber m⁻¹. The kilogram is a *gram*
symbol: magnitudes are in kilograms, prefixes attach to the gram, and the
unprefixed rendering is `kg` — the symbol always names the unit the magnitude is
in.

**`String` is the canonical form, `Prefixed` the display form.** The canonical
text keeps the unit the measurement is held in, because the text has to read back
as the same measurement. Prefix selection is a rendering choice and lives in its
own method.
