#!/usr/bin/env bash
# Sync dev-policies/ from a sibling gateway-controllers checkout.
#
# Policies under dev-policies/ referenced by a `filePath:` entry in build.yaml are compiled
# into the image from THIS directory, not from the source repo. When the two drift, the
# gateway silently runs an older policy: no build error, no warning, just behaviour that
# does not match the code you are reading. That has already cost one long misdiagnosis of a
# product defect that was really a stale copy — run this before every image build.
set -euo pipefail

SRC="${GATEWAY_CONTROLLERS_DIR:-$(cd "$(dirname "$0")/../.." && pwd)/gateway-controllers}"
DEST="$(cd "$(dirname "$0")" && pwd)/dev-policies"

if [ ! -d "$SRC/policies" ]; then
  echo "no gateway-controllers checkout at $SRC" >&2
  echo "set GATEWAY_CONTROLLERS_DIR to point at one" >&2
  exit 1
fi

# Only policies that already exist here — this mirrors an existing dev copy, it does not
# decide which policies are under local development.
status=0
for dir in "$DEST"/*/; do
  name="$(basename "$dir")"
  [ -d "$SRC/policies/$name" ] || continue

  if diff -rq --exclude=.git "$SRC/policies/$name" "$dir" >/dev/null 2>&1; then
    echo "  up to date   $name"
    continue
  fi
  if [ "${1:-}" = "--check" ]; then
    echo "  STALE        $name" >&2
    status=1
    continue
  fi
  rsync -a --delete --exclude '.git' "$SRC/policies/$name/" "$dir"
  echo "  synced       $name"
done

if [ "${1:-}" = "--check" ] && [ "$status" -ne 0 ]; then
  echo >&2
  echo "dev-policies is behind $SRC — run $0 before building, or the image will" >&2
  echo "contain an older policy than the source you are editing." >&2
fi
exit $status
