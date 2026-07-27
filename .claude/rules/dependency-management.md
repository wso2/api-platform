# Rule: Go Dependency Addition and Update Standards

## Context & Scope

Apply whenever adding a new module to any `go.mod` in this repo, or bumping an existing module's version. This repo's CI already gates merged PRs on `wso2/engineering-governance`'s approved Go dependency registry (`.github/workflows/dependency-validation.yml`). Treat that as the minimum bar, not the ceiling — satisfy the directives below before opening the PR.

## Directives

1. **Latest, actively maintained version.** Check `go list -m -versions <module>` and use the latest tag. Reject archived repos or modules with no commits/releases in ~12 months unless no maintained alternative exists (state why in the PR). Pinning an older version requires a comment on the `require` line explaining the constraint. Don't add a `replace` pointing at a fork/unmerged branch without a comment and tracking issue.
2. **Zero known vulnerabilities.** Run `govulncheck ./...` after any add/bump, before opening the PR. Also check `osv.dev` for very recent disclosures `govulncheck`'s DB may not have yet. Never add a known-vulnerable version "temporarily" — take the fixed one.
3. **Apache-2.0 license compatibility.** This repo is Apache-2.0. Allowed: `Apache-2.0`, `MIT`, `BSD-2/3-Clause`, `ISC`, `MPL-2.0` (file-level use only), `Unlicense`, `0BSD`. Disallowed without legal sign-off: `GPL-*`, `AGPL-3.0`, `LGPL-*`, `SSPL`, `CC-BY-NC-*`, or no license at all. Verify from the module's actual `LICENSE` file (not a badge) via `go-licenses check ./...`.
4. **Transitive dependencies get the same bar.** Diff `go mod graph` (or `go list -m all`) before/after to see what new indirect modules appeared. `govulncheck`/`go-licenses` already scan the full graph — a finding there can come from a transitive module, not the one you typed. Flag an unavoidable failure explicitly in the PR rather than merging past it.
5. **Bumps get the same scrutiny as new adds.** Read the changelog for security fixes, new transitive deps, and license changes. Bump one module at a time (`go get <module>@<version>`, not a blanket `go get -u ./...`) so any new issue is attributable. Re-run `go mod tidy`, `govulncheck`, `go-licenses`, and the test suite after.

## Example Workflow

```bash
go get github.com/some/pkg@latest && go mod tidy
govulncheck ./...
go-licenses check ./... --disallowed_types=forbidden,restricted
go mod graph > /tmp/after.txt   # diff against a pre-change graph to spot new transitive deps
go test ./...
git add go.mod go.sum && git commit -m "deps: add github.com/some/pkg vX.Y.Z (Apache-2.0, govulncheck clean)"
```

> **Verification Checklist before merging a dependency change:**
> * Latest version used, or an older pin documented with a reason?
> * `govulncheck ./...` clean?
> * `go-licenses check ./...` passes, and the actual LICENSE file confirmed Apache-2.0-compatible?
> * `go mod graph` diffed, and every newly introduced transitive module checked against the same bar?
> * For bumps: changelog read for security fixes / new transitive deps / license changes?
> * No undocumented `replace` directive pointing at a fork?
