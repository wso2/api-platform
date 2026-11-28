#!/usr/bin/env bash
#
# Promote gateway-controller migration steps from next/ into a release
# directory, so the release tag carries the steps it introduced.
#
#   promote-migrations.sh 1.2.0
#
# Accepts "1.2.0", "v1.2.0" or "gateway/v1.2.0" and always creates "1.2.0".
# A release that changed no schema leaves next/ empty and creates nothing.
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-gateway-controller/pkg/storage/migrations}"

raw="${1:-}"
if [ -z "$raw" ]; then
  echo "usage: $(basename "$0") <version>" >&2
  exit 2
fi

# gateway/v1.2.0 -> v1.2.0 -> 1.2.0
release="${raw#gateway/}"
release="${release#ai-gateway/}"
release="${release#v}"

case "$release" in
  ""|*/*|*" "*) echo "error: bad release name '$raw' -> '$release'" >&2; exit 1 ;;
esac

cd "$(dirname "$0")/.."
if [ ! -d "$MIGRATIONS_DIR/next" ]; then
  echo "error: $MIGRATIONS_DIR/next does not exist" >&2
  exit 1
fi

steps=$(find "$MIGRATIONS_DIR/next" -maxdepth 1 -name '*.sql' | sort)
if [ -z "$steps" ]; then
  echo "no migration steps in next/ - $release introduces no schema change"
  exit 0
fi

target="$MIGRATIONS_DIR/$release"
if [ -e "$target" ]; then
  echo "error: $target already exists - $release was already promoted" >&2
  exit 1
fi

in_git=false
git rev-parse --git-dir >/dev/null 2>&1 && in_git=true

mkdir -p "$target"
echo "promoting $(echo "$steps" | wc -l | tr -d ' ') step file(s) to $release:"
while IFS= read -r f; do
  echo "  $(basename "$f")"
  # git mv only works on a tracked file; a step added but not yet committed is
  # not, and the move must still happen.
  if $in_git && git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    git mv "$f" "$target/"
  else
    mv "$f" "$target/"
  fi
done <<< "$steps"

# next/ stays, empty, ready for the following cycle
touch "$MIGRATIONS_DIR/next/.gitkeep"
if $in_git; then
  git add -A "$MIGRATIONS_DIR" || true
fi

echo "done - next/ is empty again"
