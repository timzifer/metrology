// The corpus the pass is tested against. It is a module of its own so that
// analysistest loads it in module mode and it can import the library itself:
// the point of the test is that the checker resolves the real catalogue, not a
// stand-in written next to it.
module corpus

go 1.27

require github.com/timzifer/metrology v0.0.0

require github.com/cockroachdb/apd/v3 v3.2.1 // indirect

replace github.com/timzifer/metrology => ../..
