// Command covercheck enforces the coverage policy of D14: 100 % statement
// coverage of hand-written code.
//
// It reads a Go coverage profile, removes everything the policy excludes, and
// reports what is left uncovered. Exit status 1 means the gate failed.
//
// Excluded from both numerator and denominator:
//
//   - files carrying the standard "// Code generated ... DO NOT EDIT." line
//   - packages under the configured -exclude prefixes (cmd/, tools/ by default)
//   - statements marked //coverage:ignore, which must state a reason and be
//     listed in COVERAGE_EXCEPTIONS.md
//
// It is a Go program rather than a shell script so that it behaves identically
// on Windows and in CI.
//
// Usage:
//
//	go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
//	go run ./tools/covercheck -profile coverage.out
//
// With -badge it also writes a shields.io endpoint document, which is what the
// coverage badge in the README reads.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// block is one coverage record: a statement range and how often it ran.
type block struct {
	file       string
	startLine  int
	startCol   int
	endLine    int
	endCol     int
	statements int
	count      int
}

var profileLine = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

// generatedRE matches the convention from https://go.dev/s/generatedcode.
var generatedRE = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

const ignoreMarker = "//coverage:ignore"

func main() {
	var (
		profilePath = flag.String("profile", "coverage.out", "coverage profile to read")
		modulePath  = flag.String("module", "", "module path to strip from file names (default: read from go.mod)")
		excludes    = flag.String("exclude", "cmd/,tools/", "comma-separated package prefixes to exclude")
		minimum     = flag.Float64("min", 100, "required coverage percentage")
		verbose     = flag.Bool("v", false, "list every uncovered statement, not just a summary")
		badgePath   = flag.String("badge", "", "write a shields.io endpoint JSON with the measured percentage to this file")
	)
	flag.Parse()

	mod := *modulePath
	if mod == "" {
		m, err := moduleFromGoMod("go.mod")
		if err != nil {
			fatal("cannot determine module path: %v (pass -module)", err)
		}
		mod = m
	}

	blocks, err := parseProfile(*profilePath)
	if err != nil {
		fatal("%v", err)
	}

	prefixes := splitPrefixes(*excludes)
	kept, skippedGenerated, skippedExcluded, skippedIgnored := filter(blocks, mod, prefixes)

	total, covered, uncovered := summarise(kept)

	pct := 100.0
	if total > 0 {
		pct = float64(covered) / float64(total) * 100
	}

	fmt.Printf("covercheck: %d/%d statements covered (%.2f%%)\n", covered, total, pct)
	fmt.Printf("  excluded: %d generated, %d by prefix %v, %d marked %s\n",
		skippedGenerated, skippedExcluded, prefixes, skippedIgnored, ignoreMarker)

	// The badge is written from the same number the gate compares against, so
	// that a green badge and a passing gate cannot disagree. It is written
	// before the gate decides, because a badge reading 97 % is exactly what a
	// failing run should publish.
	if *badgePath != "" {
		if err := writeBadge(*badgePath, pct, *minimum); err != nil {
			fatal("cannot write badge: %v", err)
		}
	}

	if len(uncovered) > 0 {
		files := make([]string, 0, len(uncovered))
		for f := range uncovered {
			files = append(files, f)
		}
		sort.Strings(files)

		fmt.Fprintf(os.Stderr, "\nuncovered statements:\n")
		for _, f := range files {
			bs := uncovered[f]
			sort.Slice(bs, func(i, j int) bool { return bs[i].startLine < bs[j].startLine })
			n := 0
			for _, b := range bs {
				n += b.statements
			}
			fmt.Fprintf(os.Stderr, "  %s — %d statement(s) in %d block(s)\n", f, n, len(bs))
			if *verbose {
				for _, b := range bs {
					fmt.Fprintf(os.Stderr, "      %s:%d-%d\n", f, b.startLine, b.endLine)
				}
			} else if len(bs) > 0 {
				lines := make([]string, 0, len(bs))
				for _, b := range bs {
					lines = append(lines, strconv.Itoa(b.startLine))
				}
				fmt.Fprintf(os.Stderr, "      lines: %s\n", strings.Join(lines, ", "))
			}
		}
	}

	if pct+1e-9 < *minimum {
		fmt.Fprintf(os.Stderr, "\nFAIL: coverage %.2f%% is below the required %.2f%% (D14)\n", pct, *minimum)
		fmt.Fprintf(os.Stderr, "Either test the statements above, or — if a branch is genuinely\n")
		fmt.Fprintf(os.Stderr, "unreachable — mark it %s <reason> and record it in\n", ignoreMarker)
		fmt.Fprintf(os.Stderr, "COVERAGE_EXCEPTIONS.md. An unreachable error branch usually means\n")
		fmt.Fprintf(os.Stderr, "the error cannot occur and the check should go.\n")
		os.Exit(1)
	}
	fmt.Println("covercheck: OK")
}

// summarise counts the statements of the blocks that count and collects the
// uncovered ones by file.
//
// A block with no statements in it — the body of a marker method such as an
// analysis fact's AFact — can be neither covered nor uncovered, and reporting
// it would point the reader at a file with nothing wrong in it.
func summarise(kept []block) (total, covered int, uncovered map[string][]block) {
	uncovered = map[string][]block{}
	for _, b := range kept {
		total += b.statements
		switch {
		case b.count > 0:
			covered += b.statements
		case b.statements == 0:
		default:
			uncovered[b.file] = append(uncovered[b.file], b)
		}
	}
	return total, covered, uncovered
}

// writeBadge writes a shields.io endpoint document. Shields renders it through
// https://img.shields.io/endpoint?url=… , so the repository publishes the
// number itself and no third-party service ever sees the coverage data.
func writeBadge(path string, pct, minimum float64) error {
	colour := "brightgreen"
	switch {
	case pct+1e-9 < minimum:
		colour = "red"
	case pct < 100:
		// Below 100 % but above the configured floor: the gate passes, and the
		// badge says the target is not met either way.
		colour = "yellow"
	}
	badge := struct {
		SchemaVersion int    `json:"schemaVersion"`
		Label         string `json:"label"`
		Message       string `json:"message"`
		Color         string `json:"color"`
	}{
		SchemaVersion: 1,
		Label:         "coverage",
		Message:       fmt.Sprintf("%.1f%%", pct),
		Color:         colour,
	}
	body, err := json.Marshal(badge)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "covercheck: "+format+"\n", args...)
	os.Exit(2)
}

func splitPrefixes(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func moduleFromGoMod(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest), sc.Err()
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no module directive in %s", path)
}

// merge folds the repeated records of a multi-package profile into one record
// per statement block.
//
// With -coverpkg=./... every test binary reports every block of every package,
// so a block covered by one package's tests also appears with count 0 in the
// profiles of all the others. Counting those separately reports covered code as
// uncovered; go tool cover sums them, and so must this.
func merge(blocks []block) []block {
	type key struct {
		file                                 string
		startLine, startCol, endLine, endCol int
	}
	index := map[key]int{}
	out := make([]block, 0, len(blocks))
	for _, b := range blocks {
		k := key{b.file, b.startLine, b.startCol, b.endLine, b.endCol}
		if i, seen := index[k]; seen {
			out[i].count += b.count
			continue
		}
		index[k] = len(out)
		out = append(out, b)
	}
	return out
}

func parseProfile(path string) ([]block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read coverage profile: %w", err)
	}
	defer f.Close()

	var blocks []block
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		m := profileLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("malformed profile line: %q", line)
		}
		b := block{file: m[1]}
		for i, dst := range []*int{&b.startLine, &b.startCol, &b.endLine, &b.endCol, &b.statements, &b.count} {
			v, err := strconv.Atoi(m[i+2])
			if err != nil {
				return nil, fmt.Errorf("malformed number in %q: %w", line, err)
			}
			*dst = v
		}
		blocks = append(blocks, b)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return merge(blocks), nil
}

// filter applies the D14 exclusions and returns the blocks that count.
func filter(blocks []block, module string, prefixes []string) (kept []block, generated, excluded, ignored int) {
	// Cache per-file decisions; a profile has many blocks per file.
	type fileInfo struct {
		skip        bool
		generated   bool
		ignoreLines map[int]bool
	}
	cache := map[string]fileInfo{}

	for _, b := range blocks {
		rel := strings.TrimPrefix(b.file, module+"/")

		info, seen := cache[b.file]
		if !seen {
			info = fileInfo{}
			for _, p := range prefixes {
				if strings.HasPrefix(rel, p) {
					info.skip = true
					break
				}
			}
			if !info.skip {
				gen, ign := inspectSource(rel)
				info.generated = gen
				info.ignoreLines = ign
			}
			cache[b.file] = info
		}

		switch {
		case info.skip:
			excluded += b.statements
		case info.generated:
			generated += b.statements
		case info.ignoreLines[b.startLine] || info.ignoreLines[b.startLine-1]:
			ignored += b.statements
		default:
			kept = append(kept, block{
				file: rel, startLine: b.startLine, startCol: b.startCol,
				endLine: b.endLine, endCol: b.endCol,
				statements: b.statements, count: b.count,
			})
		}
	}
	return kept, generated, excluded, ignored
}

// inspectSource reports whether a file is generated and which lines carry an
// ignore marker. A file that cannot be read is treated as ordinary source: the
// gate must never pass because a path lookup failed.
func inspectSource(rel string) (generated bool, ignoreLines map[int]bool) {
	ignoreLines = map[int]bool{}

	f, err := os.Open(filepath.FromSlash(rel))
	if err != nil {
		return false, ignoreLines
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		// The convention allows the marker anywhere in the first comment block;
		// scanning the whole file is cheap and avoids false negatives.
		if generatedRE.MatchString(line) {
			generated = true
		}
		if strings.Contains(line, ignoreMarker) {
			ignoreLines[n] = true
		}
	}
	return generated, ignoreLines
}
