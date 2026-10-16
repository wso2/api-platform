#!/usr/bin/env bash
# Prepare gateway/gateway-runtime/policy-engine for a FOSSA source scan so the
# scan sees the dependency graph that ships in the gateway-runtime image.
#
# Usage: scripts/fossa/prepare-gateway-runtime.sh <src-dir>
#   <src-dir> is the checkout of the component build tag (the repository
#   root). The script modifies <src-dir>/gateway/gateway-runtime/policy-engine
#   in place; run it on a throwaway checkout.
#
# Background. The committed policy-engine/go.mod is the development module.
# At image build time gateway/gateway-runtime/Dockerfile (stage
# policy-compiler) runs gateway-builder, which
#   1. resolves every `gomodule:` entry of gateway/build.yaml with
#      `go mod download -json <module>@<major>` and runs `go get` for the
#      resolved module@version inside policy-engine
#      (gateway-builder/internal/policyengine/gomod.go),
#   2. adds a `replace` directive for every `filePath:` entry of
#      gateway/system-policies/system-build-lock.yaml (same file),
#   3. generates Go code that imports each policy's module path
#      (gateway-builder/internal/policyengine/registry.go),
#   4. runs `go mod download`, `go mod tidy`, `go build`
#      (gateway-builder/internal/compilation/compiler.go).
# The builder never touches the existing replace directives for common/ and
# httpkit/; the Dockerfile copies those directories next to policy-engine at
# the same relative paths. The resolved versions are written to
# gateway/build-manifest.yaml.
#
# This script repeats steps 1 to 4 (without the build) in a copy that mirrors
# the Dockerfile's build-context layout, then copies the resulting go.mod,
# go.sum and the generated import file back over policy-engine. The import
# file is needed because FOSSA's gomod strategy runs
# `go list -json -deps all`, which only follows packages that are imported.
#
# POLICY_VERSIONS=manifest pins each policy to the version recorded in
# gateway/build-manifest.yaml instead of resolving the newest release, for a
# backfill run long after the build. The committed manifest is only as fresh
# as its last commit, so the default (resolve, as the builder does) is closer
# to the image when the workflow runs on the tag push.
#
# Out of scope for this version: the Python lock. python-executor's
# requirements.txt has version ranges only; the pinned set exists only inside
# the image, written by the Dockerfile's python-deps stage as
# /app/python-libs/requirements-lock.txt. Producing it requires building that
# stage (or copying the file out of the released image), which the first
# version of the workflow does not do. The source scan therefore reports the
# unpinned requirements.txt for Python.
set -euo pipefail

src="${1:?usage: $0 <src-dir (repository root of the component build tag)>}"
src="$(cd "$src" && pwd)"
pe_rel="gateway/gateway-runtime/policy-engine"
pe="$src/$pe_rel"
build_file="$src/gateway/build.yaml"
system_lock="$src/gateway/system-policies/system-build-lock.yaml"

[ -f "$pe/go.mod" ] || { echo "prepare-gateway-runtime: $pe/go.mod not found" >&2; exit 2; }
[ -f "$build_file" ] || { echo "prepare-gateway-runtime: $build_file not found" >&2; exit 2; }
command -v go >/dev/null || { echo "prepare-gateway-runtime: go is required" >&2; exit 2; }

# The image build has no go.work; the gateway-builder forces GOTOOLCHAIN=auto.
export GOWORK=off
export GOFLAGS="${GOFLAGS:-}"
[ -n "${GOTOOLCHAIN:-}" ] && [ "$GOTOOLCHAIN" != local ] || export GOTOOLCHAIN=auto

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Mirror the Dockerfile's policy-compiler stage: policy-engine, the system
# policies and every local replace target of policy-engine's go.mod (common/,
# httpkit/, and gateway/common/ on branches that have it), at the same
# relative paths, so the relative replace directives resolve. sdk/core is
# copied too, as the Dockerfile does, although go.mod pins it from the proxy.
copy_tree() { # <path relative to src>
  [ -d "$src/$1" ] || return 0
  mkdir -p "$work/$1"
  cp -R "$src/$1/." "$work/$1/"
}
copy_tree "$pe_rel"
copy_tree gateway/system-policies
copy_tree sdk/core
while IFS= read -r target; do
  rel="$(cd "$pe" && cd "$target" && pwd)"; rel="${rel#"$src"/}"
  copy_tree "$rel"
done < <(sed -nE 's#^replace[[:space:]]+[^[:space:]]+[[:space:]]*=>[[:space:]]*(\.\.?/[^[:space:]]+).*#\1#p' "$pe/go.mod")
wpe="$work/$pe_rel"

# Step 1: `go get` each gomodule policy, as gateway-builder does (three
# attempts, like goGetMaxAttempts in gomod.go). pipPackage entries are Python
# and do not touch go.mod.
modules=()
while IFS= read -r m; do modules+=("$m"); done \
  < <(sed -nE 's/^[[:space:]]*gomodule:[[:space:]]*([^[:space:]#]+).*/\1/p' "$build_file")
[ "${#modules[@]}" -gt 0 ] || { echo "prepare-gateway-runtime: no gomodule entries in $build_file" >&2; exit 2; }
echo "prepare-gateway-runtime: ${#modules[@]} Go policy modules in gateway/build.yaml"

manifest="$src/gateway/build-manifest.yaml"
imports=()
for target in "${modules[@]}"; do
  if [ "${POLICY_VERSIONS:-resolve}" = manifest ]; then
    pinned="$(awk -v m="${target%@*}" '$1=="version:"{v=$2} $1=="gomodule:"&&index($2,m"@")==1{print v; exit}' "$manifest")"
    [ -n "$pinned" ] || { echo "prepare-gateway-runtime: ${target%@*} not in $manifest" >&2; exit 1; }
    target="${target%@*}@$pinned"
  fi
  ok=0
  for attempt in 1 2 3; do
    if (cd "$wpe" && go get "$target" 2>"$work/go-get.err"); then ok=1; break; fi
    echo "prepare-gateway-runtime: go get $target failed (attempt $attempt)" >&2
    sleep $((2 * attempt))
  done
  [ "$ok" = 1 ] || { cat "$work/go-get.err" >&2; exit 1; }
  imports+=("${target%@*}")
done

# Step 2: replace directives for filePath system policies.
if [ -f "$system_lock" ]; then
  lock_dir="$work/$(dirname "${system_lock#"$src"/}")"   # the work copy of gateway/system-policies
  while IFS= read -r rel; do
    pdir="$lock_dir/$rel"
    [ -f "$pdir/go.mod" ] || continue     # Python system policies have no go.mod
    mod="$(sed -nE 's/^module[[:space:]]+([^[:space:]]+).*/\1/p' "$pdir/go.mod" | head -1)"
    relpath="$(python3 -c 'import os,sys; print(os.path.relpath(sys.argv[1], sys.argv[2]))' "$pdir" "$wpe" 2>/dev/null \
      || realpath --relative-to="$wpe" "$pdir")"
    (cd "$wpe" && go mod edit -replace "$mod=$relpath")
    imports+=("$mod")
    echo "prepare-gateway-runtime: system policy $mod => $relpath"
  done < <(sed -nE 's/^[[:space:]]*filePath:[[:space:]]*([^[:space:]#]+).*/\1/p' "$system_lock")
fi

# Step 3: generated code importing every policy module (the builder generates
# a registry; a blank-import file has the same effect on the module graph).
gen_rel="internal/fossapolicies/imports.go"
mkdir -p "$wpe/$(dirname "$gen_rel")"
{
  echo "// Code generated by scripts/fossa/prepare-gateway-runtime.sh; DO NOT EDIT."
  echo "// Blank imports of the policy modules gateway-builder links into the shipped"
  echo "// policy-engine binary, so \`go list -deps\` and \`go mod tidy\` keep them."
  echo
  echo "package fossapolicies"
  echo
  echo "import ("
  printf '\t_ "%s"\n' "${imports[@]}"
  echo ")"
} > "$wpe/$gen_rel"

# Step 4: download and tidy, as compilation/compiler.go does before `go build`.
(cd "$wpe" && go mod download && go mod tidy)

# Copy the result back so the FOSSA scan of <src-dir> sees the shipped graph.
cp "$wpe/go.mod" "$pe/go.mod"
cp "$wpe/go.sum" "$pe/go.sum"
mkdir -p "$pe/$(dirname "$gen_rel")"
cp "$wpe/$gen_rel" "$pe/$gen_rel"

echo "prepare-gateway-runtime: resolved policy module versions (compare with gateway/build-manifest.yaml):"
(cd "$pe" && go list -m -f '{{.Path}} {{.Version}}' "${imports[@]}" | sed 's/^/  /')
