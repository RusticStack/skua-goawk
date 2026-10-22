# skua-goawk

Public RusticStack fork of [benhoyt/goawk](https://github.com/benhoyt/goawk) (MIT), the `awk`
that Skua Bash bundles (skua-bash `skua/uutils.toml`, card M3-SH5B). Upstream history is kept:

- `master` mirrors upstream `master` (never commit here).
- `skua` is the branch Skua builds: upstream plus the changes below. Merge upstream into it
  periodically: `git fetch upstream && git merge upstream/master` on `skua`, run the tests, push,
  then move the pin in skua-bash `skua/uutils.toml`.

No upstream PRs are opened from here (Skua D-118).

## Changes on `skua`

Goal: behave like Git Bash's gawk for the awk that coding agents and scripts write.

| Change | Where |
| --- | --- |
| Output newlines are written as is (`-N raw` default); upstream wrote CRLF on Windows | `goawk.go` |
| A write to a closed pipe exits 141 with no message (gawk dies of SIGPIPE) | `goawk.go`, `brokenpipe_*.go` |
| Fatal errors exit 2 (gawk); syntax errors stay 1 | `goawk.go` |
| Unicode chars by default unless `LC_ALL`/`LC_CTYPE`/`LANG` names a non-UTF-8 locale; `-b` for bytes | `goawk.go` |
| `/dev/stdin` for `-f`, input operands and `getline <`; `/dev/null` maps to `NUL` | `goawk.go`, `interp/io.go` |
| Strings like `0x1A` are not numbers, `inf`/`nan` only when signed (`+inf`, `-nan`), and print signed: gawk without `--posix` | `interp/value.go` |
| gawk functions: `match(s, re, arr)`, `gensub`, `asort`, `asorti`, `strftime`, `systime` | `lexer`, `parser`, `internal/resolver`, `internal/compiler`, `interp/skua_gawk.go` |

Tests: `go test ./interp -args -awk=` and `go test . ./internal/... ./lexer ./parser` (with
Git for Windows' `usr\bin` on PATH for gawk). CI: `.github/workflows/skua.yml`.
