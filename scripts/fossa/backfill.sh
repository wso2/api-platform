#!/usr/bin/env bash
# Upload (or, with --dry-run, only write out) the FOSSA analysis of one
# already-released component build. Mirrors .github/workflows/fossa-component-scan.yml,
# which runs on new tags only: configuration comes from the head of the
# support branch, the source from the tag <component>-<build-version>.
#
# Usage: scripts/fossa/backfill.sh <support-branch> <component> <build-version> [--dry-run]
#   e.g. scripts/fossa/backfill.sh support-1.2.0.x-gw-full gateway-controller 1.2.0.16
#
# Needs git, go and fossa on PATH, and FOSSA_API_KEY (a push-only key) in the
# environment unless --dry-run, which passes --output to fossa analyze and
# uploads nothing. The repository is the origin of the clone this is run from,
# or $FOSSA_BACKFILL_REPO. Set KEEP=1 to keep the working directory.
set -euo pipefail

usage() { echo "usage: $0 <support-branch> <component> <build-version> [--dry-run]" >&2; exit 2; }
[ $# -ge 3 ] && [ $# -le 4 ] || usage
branch="$1"; component="$2"; build_version="$3"; dry_run=0
if [ $# -eq 4 ]; then [ "$4" = --dry-run ] || usage; dry_run=1; fi
tag="$component-$build_version"

case "$component" in
  gateway-controller) gomod_dir=gateway/gateway-controller;            version_file=gateway/VERSION ;;
  gateway-runtime)    gomod_dir=gateway/gateway-runtime/policy-engine; version_file=gateway/VERSION ;;
  platform-api)       gomod_dir=platform-api;                          version_file=platform-api/VERSION ;;
  api-portal)         gomod_dir='';                                    version_file=portals/api-portal/VERSION ;;
  ai-workspace)       gomod_dir=portals/ai-workspace/bff;              version_file=portals/ai-workspace/VERSION ;;
  *) echo "backfill: unknown component '$component'" >&2; exit 1 ;;
esac
[[ "$build_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] \
  || { echo "backfill: build version '$build_version' is not x.y.z.N" >&2; exit 1; }

for tool in git fossa; do command -v "$tool" >/dev/null || { echo "backfill: $tool not found on PATH" >&2; exit 1; }; done
if [ -n "$gomod_dir" ]; then command -v go >/dev/null || { echo "backfill: go not found on PATH" >&2; exit 1; }; fi
if [ "$dry_run" -eq 0 ] && [ -z "${FOSSA_API_KEY:-}" ]; then
  echo "backfill: FOSSA_API_KEY is not set (use --dry-run to analyse without uploading)" >&2; exit 1
fi

repo="${FOSSA_BACKFILL_REPO:-$(git remote get-url origin 2>/dev/null || true)}"
[ -n "$repo" ] || { echo "backfill: run inside a clone or set FOSSA_BACKFILL_REPO" >&2; exit 1; }

git ls-remote --exit-code --heads "$repo" "$branch" >/dev/null \
  || { echo "backfill: branch '$branch' not found on $repo" >&2; exit 1; }
git ls-remote --exit-code --tags "$repo" "refs/tags/$tag" >/dev/null \
  || { echo "backfill: tag '$tag' not found on $repo" >&2; exit 1; }

work="$(mktemp -d)"
cleanup() { if [ "${KEEP:-0}" = 1 ]; then echo "backfill: kept $work" >&2; else rm -rf "$work"; fi; }
trap cleanup EXIT

# Configuration: support branch head, only .fossa and scripts/fossa.
git clone --quiet --depth 1 --filter=blob:none --no-checkout --branch "$branch" "$repo" "$work/tools"
git -C "$work/tools" sparse-checkout set --cone .fossa scripts/fossa
git -C "$work/tools" checkout --quiet
[ -f "$work/tools/.fossa/$component.yml" ] \
  || { echo "backfill: .fossa/$component.yml is not on $branch (is the scan configuration merged there?)" >&2; exit 1; }
# Source: the tag.
git clone --quiet --depth 1 --filter=blob:none --branch "$tag" "$repo" "$work/src"

raw="$(tr -d '[:space:]' < "$work/src/$version_file")"
line="${raw%%-*}"
[[ "$line" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "backfill: $version_file holds '$raw', expected x.y.z" >&2; exit 1; }
[[ "$build_version" == "$line".* ]] || { echo "backfill: build version $build_version is not on line $line ($version_file)" >&2; exit 1; }

prepare="$work/tools/scripts/fossa/prepare-$component.sh"
if [ -x "$prepare" ]; then "$prepare" "$work/src" >&2; else echo "backfill: no preparation step for $component" >&2; fi

# Absolute paths: fossa-cli does not accept a --config path that climbs out of
# the project directory.
args=(--config "$work/tools/.fossa/$component.yml")
deps="$work/tools/.fossa/$component-deps.yml"
[ ! -f "$deps" ] || args+=(--fossa-deps-file "$deps")
[ "$dry_run" -eq 0 ] || args+=(--output)

cd "$work/src"
GOWORK=off fossa analyze "${args[@]}" \
  --project "wso2apip-$component-$line" \
  --revision "$build_version" \
  --branch "$branch" \
  --experimental-analyze-path-dependencies
