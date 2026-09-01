package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// load reads a catalogue from disk for a test.
func load(t *testing.T, path string) *catalogue {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return decode(t, string(raw))
}

// decode reads a catalogue from a literal in a test, with the same strictness
// the generator uses.
func decode(t *testing.T, source string) *catalogue {
	t.Helper()
	var c catalogue
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(source)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return &c
}

// index maps unit ids to units, the way run does before emitting.
func index(c *catalogue) map[string]unitSpec {
	byID := map[string]unitSpec{}
	for _, u := range c.units() {
		byID[u.ID] = u
	}
	return byID
}

// write puts a catalogue literal on disk.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// firstLine is for error messages about generated files, where the first line
// is the interesting one.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
