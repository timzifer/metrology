# D10 — Generic methods at the system boundary

The core computes exclusively in `apd.Decimal`. Input and output in arbitrary
numeric types go through generic methods, which Go 1.27 permits on concrete
types.

```go
func (u Unit) Of[N Numeric](v N) Measurement
func (m Measurement) In[N Numeric](u Unit) (N, error)
```

`go.mod` declares `go 1.27` for this — the language version follows from that
line, not from the installed toolchain. It is also the library's minimum version.

**The exact readout is `DecimalIn`, not `In[*apd.Decimal]`.** A pointer type
cannot join a `~float64`-style type set, so the exact path is its own method.

**`Unit.Of` is total.** A NaN or an infinity is carried as the decimal form of
itself rather than rejected at the boundary, so construction never returns an
error a caller has to thread through. Both stay visible: they print as NaN and
Infinity, and asking for one as an integer is a `RangeError`.

**`Measurement.In` refuses rather than truncates.** A fractional magnitude read
into an integer, or one outside its range, is an error and not a silently altered
number — which is the failure mode this library exists to avoid.
