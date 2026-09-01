# Coverage exceptions

Statements excluded from the D14 coverage target by an explicit
`//coverage:ignore` marker. Every entry needs a reason that survives review.

Automatic exclusions — generated files, `cmd/`, `tools/` — are not listed here;
`covercheck` handles those by rule and reports the counts on every run.

| File | Line | Reason |
|---|---|---|
| _(none)_ | | |

Before adding an entry, consider deleting the code instead: an error branch that
cannot be reached usually means the error cannot occur, and the check is noise.
