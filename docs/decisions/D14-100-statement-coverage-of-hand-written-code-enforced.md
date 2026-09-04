# D14 — 100 % statement coverage of hand-written code, enforced

CI fails below 100 % statement coverage. The target applies to hand-written
packages; generated files are excluded from both numerator and denominator.

**Why this library and not every library.** Three properties make the target
reachable here rather than aspirational. The core is pure computation with no I/O,
no clock and no network — every branch is reachable from a table-driven test. The
catalogue is generated (D8), so the part that would otherwise dominate the line
count and be tested by copy-paste is excluded by rule. And the failure mode this
library must avoid is the silent wrong number, which is exactly what an untested
branch produces.

**What is excluded, and how.**

| Excluded | Mechanism |
|---|---|
| Generated catalogue code | files carrying the standard `// Code generated … DO NOT EDIT.` line are filtered out of the coverage profile |
| Defensive branches that cannot be triggered | require an explicit `//coverage:ignore` comment stating why, and are listed in `COVERAGE_EXCEPTIONS.md` with a rationale |
| `cmd/` and `tools/` | thin wrappers and build-time tools; the analysis pass itself is covered through `analysistest`, the generator through its own tests |

Anything else claiming an exemption is a design smell, not a testing problem. An
error branch that cannot be reached usually means the error cannot occur and the
check should go, or that the dependency needs to be injectable so it can be made
to fail.

Because the generated files are declarations only (D8), they currently contribute
*zero* statements, so the exclusion changes no number today. It is tested anyway,
against a generated fixture that does contain statements: the day a generated
file grows a function is not the day to discover the filter never worked.

**The trap, named explicitly.** Coverage measures execution, not verification. A
test that calls a function and asserts nothing raises the number and lowers the
value — it converts an untested line into a line everyone believes is tested,
which is worse than where we started. The rule is therefore: **coverage is a
floor, never the goal.** The correctness weight is carried by the property tests
of the dimension algebra, the round-trip tests of the parser, the kind-rule table
of D6, the aliasing guard of D3 and the NIST golden tests of the catalogue.
Coverage only ensures none of them has a blind spot.

**Mechanics.** `go test -covermode=atomic -coverpkg=./... -coverprofile=…` across
all packages, so cross-package calls count; `tools/covercheck` strips generated
files and the declared exceptions, then compares against 100. With `-coverpkg`
every test binary reports every block of every package, so a block covered by one
package's tests also appears with count 0 in the profiles of the others; merging
the repeated records is what makes the number right, and without it the gate
reports covered code as uncovered as soon as a second test package exists. Blocks
with no statements — an empty `AFact` method body — are neither covered nor
uncovered and are not listed. The per-function output is part of the CI log,
because "which function dropped" is the only useful form of a coverage failure.
