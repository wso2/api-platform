# Rule: Feature Docs Stay in Sync with Code

## Context & Scope

Applies to every `kb/**/*.md` whose YAML frontmatter carries a `sources:` list (Open Knowledge Format header). Such a file is the living description of a feature: use cases, decisions, scope, limitations and entry points. The `sources:` list names the code it is derived from; `scripts/check-feature-docs.sh` fails a PR that changes a listed path without touching the doc.

## Directives

1. **Check before editing.** Before changing a Go, SQL, YAML or config file, run `grep -rl "resource: /<path>" kb`. If a feature doc lists the file, re-read its Use cases, Decisions, Scope, Current limitations and Entry points sections and update whatever the change invalidates, in the same PR.
2. **New surface goes in `sources:`.** A new handler, service, repository, table, config key or endpoint belonging to a documented feature is added to that doc's `sources:` list and its Entry points table.
3. **Decisions and limitations are rows, not prose.** A product decision taken in a PR becomes a Decisions row citing the PR. A limitation that is fixed has its row deleted. Never leave a stale row.
4. **`verified[].at` means verified.** Bump it only after actually comparing the doc against the code. Bump `stale_after` at the same time.
5. **No deferring behind a comment or label.** Never ship `<!-- TODO: update docs -->`. If the change genuinely does not affect the doc, add the `docs-not-affected` label to the PR with a one-line reason.

> **Verification Checklist before outputting code:**
> * Did any changed file appear in a feature doc's `sources:`? If so, is the doc updated in this change?
> * Does a new file for a documented feature appear in `sources:` and Entry points?
> * Was `verified[].at` bumped without an actual doc-vs-code comparison?
