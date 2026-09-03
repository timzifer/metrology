package parse_test

import (
	"encoding/json"
	"testing"

	"github.com/timzifer/metrology"
	"github.com/timzifer/metrology/parse"
	"github.com/timzifer/metrology/units/pressure"
)

// The reading half of D12, measured. The writing half is benchmarked in the
// root package. Run both with:
//
//	go test -run '^$' -bench . -benchmem ./...
//
// The number that matters here is not the parse itself but the parser: [New]
// indexes every spelling of every symbol it is given (D12), so a program that
// builds one per message pays the whole catalogue per message. The benchmarks
// are laid out to make that visible rather than to be quoted in isolation.

var (
	sinkMeasurement metrology.Measurement
	sinkUnit        metrology.Unit
	sinkParser      parse.Parser
	sinkErr         error
)

func BenchmarkParse(b *testing.B) {
	parser := parse.Default()

	// The plain case: a magnitude and a static symbol.
	b.Run("Measurement", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = parser.Measurement("2.5 bar")
		}
	})
	// A prefixed SI symbol, which costs the prefix split on top.
	b.Run("MeasurementPrefixed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = parser.Measurement("250 kPa")
		}
	})
	// A composite symbol the catalogue spells itself: "m/s²" is looked up
	// whole and never reaches the expression grammar, which is exactly what
	// [Parser.Unit] promises and worth measuring separately from the case
	// that does reach it.
	b.Run("MeasurementComposite", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = parser.Measurement("9.81 m/s²")
		}
	})
	// The expression grammar proper: no catalogue entry spells this, so it is
	// read by recursive descent and the unit is built. This is the benchmark
	// to watch when syntax is added.
	b.Run("MeasurementExpression", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkMeasurement, sinkErr = parser.Measurement("1250 kg·m²/s³")
		}
	})
	b.Run("Unit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkUnit, sinkErr = parser.Unit("kg/m³")
		}
	})
}

// BenchmarkParserNew is the one cost that has to stay outside a loop: indexing
// the spellings of a catalogue. [Default] is built once and shared, so it is
// measured against a parser built per call to show the difference a program
// makes for itself by keeping one.
func BenchmarkParserNew(b *testing.B) {
	units := []metrology.Unit{pressure.Pascal, pressure.Bar, pressure.Torr}

	b.Run("Default", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkParser = parse.Default()
		}
	})
	b.Run("New", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkParser = parse.New(units)
		}
	})
}

// BenchmarkDecode measures the boundary a program actually crosses: JSON and
// SQL through [Text], the destination that carries its parser (D12).
func BenchmarkDecode(b *testing.B) {
	data := []byte(`{"p":"2.5 bar"}`)
	bare := []byte(`{"p":250000}`)

	b.Run("JSON", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var row struct {
				P parse.Text `json:"p"`
			}
			sinkErr = json.Unmarshal(data, &row)
			sinkMeasurement = row.P.Measurement
		}
	})
	// The two-column layout: a NUMERIC column and the unit in the schema.
	b.Run("JSONBareMagnitude", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			row := struct {
				P parse.Text `json:"p"`
			}{P: parse.Default().Text().In(pressure.Pascal)}
			sinkErr = json.Unmarshal(bare, &row)
			sinkMeasurement = row.P.Measurement
		}
	})
	b.Run("SQLScan", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var dst parse.Text
			sinkErr = dst.Scan("2.5 bar")
			sinkMeasurement = dst.Measurement
		}
	})
}
