# D16 — The quantity tag is identity, and `unitvet` follows the tag a product drops

D6 gave `Quantity` three behaviours that were each defensible on their own and
never reconciled: `Unit.Equal` compares the tag, `Add`, `Sub`, `Cmp` and `To`
refuse a conflict, `Mul` and `Div` throw the tag away, and `parse` puts one back
by looking the expression's scale up in a catalogue. Before the type can be
frozen ([section 7](../status.md)) it has to mean one thing.

**It means identity.** A hertz and a becquerel are both T⁻¹ and are not the same
measurement; treating one as the other is a wrong number delivered with
confidence, which is the failure this library exists to prevent. So the tag is
part of what a unit *is*, not a label on it, and the run time already enforces
that: `sameQuantity` guards every additive operation and `Engine.To` guards
every conversion. Nothing in the core changes.

**The untagged wildcard is not an exception to it, it is what makes it usable.**
Multiplication and division drop the tag — they must, because a becquerel times
a metre is not anything the catalogue names — so every computed magnitude is
untagged, and a rule that refused to let an untagged magnitude meet a tagged one
would make each computation a dead end. The wildcard is the price of the drop,
and it is worth paying at the run time, where the alternative is a library in
which no quotient can ever be named.

**But it opens a hole, and the hole is static.** The tag can be laundered:

```go
scaled, _ := activity.Becquerel.Of(5).Mul(ratio.One.Of(2))
_, _ = scaled.Add(frequency.Hertz.Of(50))          // accepted at run time
```

The product left the dimension untouched and dropped the tag, so what reaches
the sum is an untagged T⁻¹ and the wildcard lets it through. The magnitude is
still a radioactivity. The run time cannot know that — it holds a value, a unit
and a dimension, and the tag it discarded is gone — and giving it a memory would
mean putting the tag back into `Mul` and `Div`, which is exactly the guessing D6
forbids and would make the product of a torque and an angle a torque.

**`unitvet` has the information the run time discarded.** It walks the operands
backwards anyway (D13), so it can carry the dropped tag as *provenance*: the
quantity a multiplicative operation discarded while leaving the dimension
intact. That provenance conflicts with a real tag exactly where the discarded
one would have:

```
app/app.go:12:9: Add on incompatible quantities: a magnitude computed from
                 radioactivity and frequency; Mul and Div drop the tag (D6),
                 so the run time no longer sees the conflict
```

**Three rules keep it provable rather than clever.**

- **Provenance survives only where the dimension does.** A becquerel scaled by a
  plain number is still a radioactivity; a becquerel times a metre is a T⁻¹L¹
  that names no quantity, and dividing the metre back out does not recover one.
  Where the result dimension differs from the operand's, the tag is gone for
  good and the checker forgets it too.
- **Two surviving tags that disagree are no answer.** A plane angle times a
  solid angle is dimensionless and is neither, so the product carries no
  provenance rather than one of the two.
- **It is provenance, never a tag.** The checker never claims the value *is* a
  radioactivity — it is not, the arithmetic untagged it, and `To(activity.Curie)`
  on it is legal and stays silent. The two fields are separate for the same
  reason `Kind` and `Quantity` are.

**This is the one diagnostic that predicts no run-time error**, and it is the
reason this is a decision rather than a rule inside D13. Everywhere else the
pass and the run time agree by construction, and a reader who runs the code can
confirm the diagnostic. Here the code runs and produces a number, and the
message says so in its own text, because a checker that reports something the
reader cannot reproduce is a checker the reader stops believing. The escape is
the one D13 already ships: `//unitvet:ignore` on the line, with a reason, for
the case where the reinterpretation is deliberate.

**The namespace follows from it.** If the tag is identity then a string literal
at a call site is a second spelling of an identity, with nothing to keep the two
in step — so every tagged package declares the constant and `catgen` writes the
unit definitions from it:

```go
const Quantity metrology.Quantity = "frequency"   // frequency/frequency_gen.go

u, ok := catalog.Canonical(dim, kind, frequency.Quantity)
```

It goes in the quantity package rather than in `catalog`, because that is the
package a caller already imports for the units, and reaching a tag through
`catalog` would pull all forty-four quantity packages in behind it. The tags
are therefore *reserved names of this catalogue*, not of the type: `Quantity`
stays a `string` and a caller's own catalogue declares its own constants for its
own tags. Two catalogues in one program that spell one dimension differently are
still two namespaces, and D6's compatibility rule still compares spellings
across them — but each side now has one place where its spelling is written
down, which is what makes a collision something a person can look up.

**What it leaves to D12.** The third question [section 7](../status.md) asked — what untagged
means crossing the text boundary — is answered there, and answered by leaving it
alone: `50 Hz` reads back tagged and `50 s⁻¹` untagged for one scale, because
the spelling is the statement and a text that made no claim should not have one
invented for it. D16 says what the tag is and who owns the spelling; D12 says
what a text carrying none of it means.
