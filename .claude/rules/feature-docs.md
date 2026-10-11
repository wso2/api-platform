# Rule: Feature Docs Stay in Sync with Code

## Context & Scope

Applies to every `kb/**/*.md` whose YAML frontmatter carries a `sources:` list (Open Knowledge Format header). Such a file is the living description of a feature: use cases, decisions, scope, limitations and entry points. The `sources:` list names the code it is derived from; a change to a listed path must update the doc in the same PR.

## Directives

1. **Check before editing.** Before changing a Go, SQL, YAML or config file, run `grep -rlE "resource: /(<path>|<parent-dir>/)$" kb`, or simply `grep -rl "resource: /<parent-dir>" kb`. If a feature doc lists the file, re-read its Use cases, Decisions, Scope, Current limitations and Entry points sections and update whatever the change invalidates, in the same PR.
2. **New surface goes in `sources:`.** A new file or directory owned by a documented feature is added to that doc's `sources:` list and its Entry points table, unless a directory entry already covers it. Shared files, tests and generated code are not added.
3. **Decisions and limitations are rows, not prose.** A product decision taken in a PR becomes a Decisions row citing the PR. A limitation that is fixed has its row deleted. Never leave a stale row.
4. **`verified[].at` means verified.** Bump it only after actually comparing the doc against the code.
5. **No deferring behind a comment.** Never ship `<!-- TODO: update docs -->`. If the change genuinely does not affect the doc, say so in the PR description.

> **Verification Checklist before outputting code:**
> * Did any changed file appear in a feature doc's `sources:`? If so, is the doc updated in this change?
> * Does a new file for a documented feature appear in `sources:` and Entry points?
> * Was `verified[].at` bumped without an actual doc-vs-code comparison?
