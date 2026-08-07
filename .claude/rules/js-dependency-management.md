# Rule: JavaScript/npm Dependency Addition and Update Standards

## Context & Scope

Apply whenever adding a package to `package.json`/`package-lock.json` in `portals/developer-portal` (and, by the same standard, `portals/ai-workspace` and `portals/management-portal`), or bumping an existing package. JS/npm counterpart to `dependency-management.md` (Go) — same bar (latest maintained version, zero known vulnerabilities, Apache-2.0-compatible license), applied with npm tooling, plus a transitive audit since one `npm install` can pull in dozens of packages.

## Directives

1. **Latest, actively maintained version.** Check `npm view <package> version` and `npm view <package> deprecated` before adding — use the current `latest` dist-tag, not an outdated copied version. Reject deprecated or archived/stale (2+ years no release) packages unless no maintained alternative exists (state why in the PR). Avoid `next`/`beta`/`rc`/`alpha` tags in production dependencies unless explicitly justified.
2. **Zero known vulnerabilities.** Run `npm audit --omit=dev` and `npm audit` after any add/bump, before committing the lockfile. No new `high`/`critical` findings. For `moderate`/`low`, judge reachability rather than ignoring blindly — never reflexively run `npm audit fix --force` (can silently jump a major version). Cross-check `osv.dev` for very recent disclosures.
3. **Apache-2.0 license compatibility.** This repo is Apache-2.0. Allowed: `Apache-2.0`, `MIT`, `BSD-2/3-Clause`, `ISC`, `0BSD`, `CC0-1.0`, `Unlicense`, `MPL-2.0` (file-level use only). Disallowed without legal sign-off: `GPL-*`, `AGPL-3.0`, `LGPL-*`, `SSPL`, `CC-BY-NC-*`, `WTFPL`, or no declared license. Run `npx license-checker --production --failOn 'GPL;AGPL-3.0;LGPL;SSPL;CC-BY-NC'` against the full installed tree — a package's own `license` field can be missing or wrong.
4. **Transitive dependencies get the same bar.** Run `npm ls <pkg> --all` (or diff the lockfile) to see what a single add pulled in. `npm audit`/`license-checker` already scan the whole tree — a finding can come from a transitive package, not the one you typed. Flag an unavoidable failure explicitly in the PR rather than merging past it.
5. **Lockfile discipline and bumps.** Always commit `package-lock.json` — never `--no-save`, never hand-edit it. Check `npm outdated` and read the changelog before bumping; bump one package at a time rather than a blanket `npm update`, so any new issue is attributable. Semver-major bumps need the migration guide read and the full test suite run.
6. **No deferring a violation behind a code comment.** Never resolve a failing check from this rule (unapproved license, known vulnerability, undocumented deprecated/stale package) by adding a `// TODO`/`FIXME`-style comment and merging anyway — a comment is not a remediation. If it genuinely can't be fixed before merging, track it as an issue with an owner and state it explicitly in the PR description, not only as an inline source comment.

## Example Workflow

```bash
npm view some-package version && npm view some-package deprecated
npm install some-package@latest
npm audit --omit=dev && npm audit
npx license-checker --production --failOn 'GPL;AGPL-3.0;LGPL;SSPL;CC-BY-NC'
npm ls some-package --all   # see what transitive packages this pulled in
npm test
git add package.json package-lock.json && git commit -m "deps: add some-package vX.Y.Z (MIT, npm audit clean)"
```

> **Verification Checklist before merging a dependency change:**
> * Latest non-deprecated version used?
> * `npm audit` (prod + full) shows zero `high`/`critical` findings?
> * `license-checker --production` passes against the full installed tree?
> * Transitive packages reviewed (`npm ls <pkg> --all` or lockfile diff) against the same bar?
> * `package-lock.json` committed, no manual edits, no `--no-save`?
> * Semver-major bumps: migration guide read and full test suite run?
> * Is a failing check "resolved" by a `// TODO`/`FIXME`-style comment instead of an actual fix or a tracked, PR-flagged issue?
