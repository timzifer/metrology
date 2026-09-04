# D6 — Kind and quantity, with explicit arithmetic rules

Two facts travel with a unit, and they are two fields for the same reason D5 took
the kind out of the dimension word: they are independent, and packing independent
facts into one value is how a `WithoutKind` ends up clearing four of eight bits.

**`Kind` — the affine distinction, *absolute vs. interval*.** 20 °C is a point
on a scale, 5 K is a distance along one. Two values, and the rules below.

**`Quantity` — which quantity a shared dimension is being read as.** The hertz
and the becquerel are both T⁻¹; the gray and the sievert are both L²T⁻²; a plane
angle, a solid angle and a bare ratio are all dimensionless; the candela and the
lumen are both J. A string tag rather than an enum, because the catalogue is
data (D8) and the set of quantities is open — the whole SI needs nine tags, and
none of them touches a line of the core.

The tag is a `string` and the type is open, but the *spellings this catalogue
uses* are not left to a string literal at the call site: every tagged package
declares its own — `frequency.Quantity`, `activity.Quantity` — and the generator
writes the unit definitions in terms of that same constant, so a package has one
spelling of the fact rather than two. A caller with a catalogue of its own
declares its own constants; the type stays open, the *names* are owned by
whoever generated them (D16).

The zero `Quantity` is untagged, and untagged is compatible with everything.
That is not laxity, it is the only workable rule: multiplication and division
drop the tag, so *every computed magnitude is untagged*, and a rule that refused
to name them would make each computation a dead end. The check fires only where
both sides make a claim and the claims differ — 50 Hz asked for in becquerel.

**Rules for addition and subtraction:**

| Operation | Result |
|---|---|
| absolute + interval | absolute — `20 °C + 5 K = 25 °C` |
| absolute − absolute | interval — `25 °C − 20 °C = 5 K` |
| interval ± interval | interval |
| absolute + absolute | error |
| interval − absolute | error |

A sum takes the tag of whichever operand carries one: an untagged T⁻¹ added to a
frequency is a frequency, and there is nothing else it could be.

**Rule for multiplication and division.** The result carries *neither* kind nor
quantity. A product of a torque and an angle is no longer a torque, and a system
that tries to guess will guess wrong. Naming the result is an explicit, checked
conversion — `q.To(pressure.Pascal)`, or `catalog.Canonical` for the unit that
dimension resolves to. Absolute values may not be multiplied at all: 20 °C times
2 is physically meaningless and returns an error.

The drop is unconditional and stays that way: the tag a becquerel loses to a
product is a tag the run time cannot get back, and putting it back would be the
guessing this rule exists to forbid. What *can* remember it is the static
checker, which walks the operands anyway — D16 has the case and the three rules
that keep it provable.

**An interval unit may not carry an offset.** An offset is what makes a scale
affine, and an affine scale measures points. Rejecting the combination at
construction removes the case from every later operation: a unit that reaches the
arithmetic as an interval is linear, so a product never has to ask.

**An absolute unit declares the interval unit its differences are read on** — K
for °C, °R for °F. Without it `25 °C − 20 °C` would have to be 5 °C, which reads
like a temperature and is not one. The difference is *converted* onto the
declared unit, not merely labelled with it, so a scale declaring a counterpart
with a different factor still yields the right number. The scale the difference
is *computed* on is a third unit — the receiver's own factor without the offset —
and conflating the two is a live trap; a test with a Celsius scale declaring
degrees Rankine holds them apart.

**What the tag does not solve.** Two quantities on one dimension that also share
a *symbol* remain indistinguishable in the text form of D12 — `5 m²/s` is
kinematic viscosity and thermal diffusivity and a diffusion coefficient, and no
tag in the world makes that string read back to one of them. The catalogue
therefore carries one of the three and says so; the others wait for a text form
that carries the quantity as well ([section 8](../deferred.md)).
