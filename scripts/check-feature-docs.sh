#!/usr/bin/env bash
# Fail a PR that changes a path listed in a feature doc's `sources:` frontmatter
# without also changing that doc. Warn on dead source paths and on docs past
# `stale_after`. Run from the repo root: scripts/check-feature-docs.sh [base-ref]
# Skip the failure (not the warnings) by exporting DOCS_NOT_AFFECTED=1, which CI
# sets when the PR carries the `docs-not-affected` label.
set -euo pipefail
base="${1:-origin/main}"
changed="$(git diff --name-only "$base"...HEAD)"
today="$(date +%F)"
rc=0
while IFS= read -r doc; do
  [ -n "$doc" ] || continue
  srcs="$(awk '/^---$/{f++; next} f==1 && /resource:/{sub(/.*resource: */,""); sub(/#.*/,""); sub(/^\//,""); print}' "$doc")"
  hit=""
  for s in $srcs; do
    [ -e "$s" ] || echo "::warning file=$doc::source path does not exist: $s"
    grep -qx "$s" <<<"$changed" && hit="$hit $s"
  done
  if [ -n "$hit" ] && ! grep -qx "$doc" <<<"$changed"; then
    if [ "${DOCS_NOT_AFFECTED:-0}" = "1" ]; then
      echo "::notice file=$doc::sources changed ($hit); skipped via docs-not-affected"
    else
      echo "::error file=$doc::documented code changed ($hit) but the doc was not updated. Update it or add the 'docs-not-affected' label."
      rc=1
    fi
  fi
  stale="$(awk '/^stale_after:/{print $2}' "$doc")"
  if [ -n "$stale" ] && [[ "$today" > "$stale" ]]; then
    echo "::warning file=$doc::past stale_after ($stale); re-verify against the code and bump it."
  fi
done < <(git ls-files 'kb/*.md' 'kb/**/*.md' | xargs grep -l '^sources:' 2>/dev/null | sort -u)
exit $rc
