# 4. The API

The shape, in the vocabulary the decisions establish. Runnable examples live in
the godoc of each package.

```go
// --- Core -------------------------------------------------------

type Dimension uint64          // 7 × int8, packed (D5)
type Kind      uint8           // absolute or interval, held separately (D5/D6)
type Quantity  string          // which quantity a shared dimension is read as (D6)

type Unit struct {             // value type, immutable (D1/D3)
    dim      Dimension
    kind     Kind
    quantity Quantity
    sym      Symbol
    num      *apd.Decimal      // exact fraction (D4)
    den      *apd.Decimal
    offset   *apd.Decimal
    interval *Unit             // the unit differences are read on (D6)
}

type Measurement struct {      // copyable; compare with Equal, not ==
    unit Unit
    val  apd.Decimal           // never written in place (D3)
}

// --- Construction and readout -----------------------------------

m  := pressure.Bar.Of(2.5)                     // implicit N = float64
m2, err := pressure.Bar.OfString("2.50000000001")

pa, err := m.In[float64](pressure.Pascal)      // 250000
d,  err := m.DecimalIn(pressure.Pascal)        // exact, not In[*apd.Decimal] (D10)

// --- Arithmetic -------------------------------------------------

t, _ := temperature.Celsius.Of(20).
        Add(interval.Kelvin.Of(5))             // 25 °C          (D6)

_, err = temperature.Celsius.Of(20).
        Add(temperature.Celsius.Of(5))         // ErrAbsoluteSum (D6)

q, _ := force.Newton.Of(100).
        Div(area.SquareMetre.Of(2))            // 50 N/m², kind and tag dropped

u, _ := catalog.Canonical(q.Dimension(), q.Kind(), q.Quantity())
p, _ := q.To(u)                                // 50 Pa, named in a checked step

// --- Composing units --------------------------------------------

perSecond, err := volume.CubicMetre.Per(duration.Second)  // Times, Per, Pow

// --- Precision --------------------------------------------------

e := metrology.NewEngine(34)                   // decimal128; zero Engine is
r, err := e.Mul(m, m)                          // DefaultPrecision = 20 (D9)

// --- Inspecting errors ------------------------------------------

var de *metrology.DimensionError
if errors.As(err, &de) {
    log.Printf("expected %s, got %s", de.Want, de.Got)
}

// --- Text: writing is a method, reading is a parser (D12) -------

text, _ := p.MarshalText()                     // "50 Pa"
data, _ := json.Marshal(p)                     // "50 Pa", quoted
disp    := p.Prefixed()                        // the display form

m3, err := parse.Measurement("250 kPa")        // the shipped catalogue
u2, err := parse.Unit("J/(kg·K)")              // expressions resolve too
mine    := parse.New(myUnits)                  // a catalogue of your own

var field parse.Text                           // and a destination that
err = json.Unmarshal(data, &field)             // carries its parser along

// --- Bounds instead of a point: the interval layer (D15) ------

r, err := uncertainty.Parse("2.55 ± 0.05 bar") // also [2.5, 2.6] bar, 2.55(5) bar
lo      := r.Lo()                              // 2.5 bar
w, err  := r.Width()                           // 0.1 bar
inTorr, err := r.To(pressure.Torr)             // each bound rounds outward
agree, err  := r.Overlaps(specified)           // so a conversion never
                                               // manufactures a disagreement
text, ok := r.PlusMinus()                      // the display form, and whether
                                               // it says exactly what r says

// --- An uncertainty budget: the propagation layer (D21) ------

l, err := gum.Standard(length.Metre.Of(100),   // an estimate and its standard
    length.Metre.Of(0.1))                      // uncertainty, as measurements
area, err := l.Mul(w)                          // ∂(lw)/∂l = w, and the merge
zero, err := l.Sub(l)                          // 0 m ± 0 m: one input, cancelled
u, err    := area.Uncertainty()                // the combination, rounded up
rows      := area.Contributions()              // the budget, one row per input
nu, err   := area.EffectiveFreedom()           // Welch-Satterthwaite (§G.4.1)
band, err := area.Expanded(2)                  // handed back as a Range (D15)
```
