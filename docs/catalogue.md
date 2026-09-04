# 6. The catalogue

The catalogue is two files. `catalog/catalog.yaml` holds **90 units across 44
quantity packages**: all seven SI base units, all twenty-two named derived
units, and the non-SI units of NIST SP 811 that process engineering uses.
`catalog/customary.yaml` holds **14 customary units in three packages** grouped
by provenance rather than by quantity (D19): eight the two systems agree on, and
three each where they do not. Both are read by one `catgen` run and
validated as one document, because a symbol resolving to two units is a defect
whichever file each half is written in.

| Block | Contents |
|---|---|
| SI base | s, m, kg, A, K, mol, cd |
| Named derived SI | rad, sr, Hz, N, Pa, J, W, C, V, F, Ω, S, Wb, T, H, lm, lx, Bq, Gy, Sv, kat, °C |
| Mechanics, heat, material data | area, volume, velocity, acceleration, density, concentration, mass flow, volume flow, viscosity and kinematic viscosity, surface tension, thermal conductivity, specific heat |
| Process-engineering non-SI | bar, torr, mmHg, mmH₂O, atm, l and l/min, m³/h, kWh, ppm and ppb, °F, t, min, h, d |
| CGS and other legacy units | dyne, erg, poise, stokes, gauss, maxwell, curie, rem, calorie, electronvolt, ångström, barn, are, hectare |
| Dimensionless | ratio, plane angle, solid angle — separated by the quantity tag of D6 |
| Defined through π (D20) | degree, arcminute, arcsecond, gon; the oersted with the ampere per metre; the parsec with the astronomical unit |
| Customary (`units/customary`, D19) | in, ft, yd, mi, lb, oz, lbf, psi — every one exact by the 1959 agreement |
| Customary, by system | `us` and `imperial`: galUS/galImp, flozUS/flozImp, tonUS/tonImp — the bare `gal` names nothing |

A golden test compares every non-SI unit against the conversion factors printed
in NIST SP 811. It compares to **eighteen significant digits, not to the last
one**: factors such as one 3600th have no finite decimal form, so the conversion
rounds once by D9 and the return trip cannot undo that rounding. Eighteen digits
is two below the engine default and far past where a pre-divided factor fails — a
torr stored as 133.32236842105263 goes wrong in the seventeenth.

**What the catalogue deliberately does not contain.**

- **The square degree and the gilbert**, which are the two π units D20 named and
  the catalogue does not have. The square degree would need a second spelling of
  the degree — it is written `deg²` and never `°²` — and a catalogue with two
  spellings for one unit is the ambiguity D12 refuses; `angle.Degree.Pow(2)`
  builds the scale without naming it. The gilbert would need a
  magnetomotive-force group whose canonical unit is the ampere again, which is a
  modelling question about D6 and not a missing factor. Both wait, and neither
  waits for arithmetic.
- **The absorbed-dose rad.** Its symbol is `rad`, which is the radian. The
  collision is real, it is in the standards, and the generator is right to refuse
  it. The rem is present; the CGS dose unit waits for a symbol namespace.
- **Thermal diffusivity and diffusion coefficients.** They are m²/s, like
  kinematic viscosity, and the quantity tag separates them in code but not in the
  text form of D12, where `5 m²/s` has to read back to exactly one unit. The
  catalogue carries kinematic viscosity and says so.

Dimension collisions cluster precisely in process engineering, which is why D6 is
not a footnote but the rule that makes this catalogue consistent in the first
place.

**Adding a unit** means editing the YAML and running `go generate ./...`; every
entry needs a `source:`, and no `*_gen.go` file is ever edited by hand.
