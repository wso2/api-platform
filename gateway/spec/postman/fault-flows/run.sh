#!/usr/bin/env bash
# Builds and runs the fault-flows Postman kit.
#
#   gateway/spec/postman/fault-flows/run.sh [all|build|up|test|down|clean]
#
#   build  build gateway images from upstream main + this branch's changes + the sample policies
#   up     start the stack (gateway-controller, gateway-runtime, mock LLM backend)
#   test   run the Postman collection with newman
#   down   stop the stack and drop its volumes
#   clean  remove the build worktree
#   all    build, up, test (default)
#
# Why a worktree: LLM fault policies (globalFaultPolicies / operationFaultPolicies) need the
# controller half of the fault flow (wso2/api-platform#3714), which is on upstream main but not on
# this branch's base. The build applies this branch's diff — committed and uncommitted — onto
# BASE_REF in a separate worktree, so neither this checkout nor its build.yaml is touched.
#
# Environment:
#   BASE_REF    ref to build on                 (default: upstream/main, else origin/main)
#   BUILD_DIR   worktree location               (default: $TMPDIR/apip-fault-flows-build)
#   TAG         image tag                       (default: fault-flows)
set -euo pipefail

KIT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(git -C "$KIT_DIR" rev-parse --show-toplevel)"
BUILD_DIR="${BUILD_DIR:-${TMPDIR:-/tmp}/apip-fault-flows-build}"
BUILD_DIR="${BUILD_DIR%/}"
TAG="${TAG:-fault-flows}"
RUNTIME_IMAGE="local/fault-flows-gateway-runtime:$TAG"
CONTROLLER_IMAGE="local/fault-flows-gateway-controller:$TAG"
COMPOSE=(docker compose -f "$KIT_DIR/docker-compose.yaml")
export RUNTIME_IMAGE CONTROLLER_IMAGE

# Files this branch changes that are NOT applied: local build files, the kit itself, the
# integration-test wiring, and the Envoy code_details hunks that upstream main now carries.
EXCLUDES=(
  ':(exclude).gitignore'
  ':(exclude)gateway/build.yaml'
  ':(exclude)gateway/build-manifest.yaml'
  ':(exclude)gateway/spec'
  ':(exclude)tests'
  ':(exclude)gateway/gateway-controller/pkg/constants/constants.go'
  ':(exclude)gateway/gateway-controller/pkg/xds/translator.go'
)

log() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }

base_ref() {
  if [ -n "${BASE_REF:-}" ]; then echo "$BASE_REF"; return; fi
  if git -C "$ROOT" remote | grep -qx upstream; then echo upstream/main; else echo origin/main; fi
}

# The policy-engine tests link cgo; the macOS 27 SDK breaks that link, so prefer the 26.5 SDK.
macos_sdk() {
  if [ "$(uname)" = Darwin ] && [ -z "${SDKROOT:-}" ]; then
    local sdk=/Library/Developer/CommandLineTools/SDKs/MacOSX26.5.sdk
    [ -d "$sdk" ] && export SDKROOT="$sdk"
  fi
  return 0
}

cmd_build() {
  local base remote merge_base
  base="$(base_ref)"
  remote="${base%%/*}"
  log "Fetching $base"
  git -C "$ROOT" fetch -q "$remote" "${base#*/}"

  merge_base="$(git -C "$ROOT" merge-base HEAD "$base")"
  log "Preparing worktree $BUILD_DIR on $base"
  if [ -d "$BUILD_DIR" ]; then
    git -C "$ROOT" worktree remove --force "$BUILD_DIR"
  fi
  git -C "$ROOT" worktree add -q --detach "$BUILD_DIR" "$base"

  log "Applying this branch's changes since $(git -C "$ROOT" rev-parse --short "$merge_base")"
  git -C "$ROOT" diff --binary "$merge_base" -- . "${EXCLUDES[@]}" > "$BUILD_DIR/.branch.patch"
  (cd "$BUILD_DIR" && git apply --3way --whitespace=nowarn .branch.patch)
  rm -f "$BUILD_DIR/.branch.patch"

  log "Adding the sample policies"
  for p in "$KIT_DIR"/policies/*/; do
    name="$(basename "$p")"
    rm -rf "$BUILD_DIR/gateway/dev-policies/$name"
    cp -R "$p" "$BUILD_DIR/gateway/dev-policies/$name"
    printf '  - name: %s\n    filePath: ./dev-policies/%s\n' "$name" "$name" >> "$BUILD_DIR/gateway/build.yaml"
  done
  find "$BUILD_DIR/gateway" -name .DS_Store -delete

  macos_sdk

  log "Building $RUNTIME_IMAGE"
  make -C "$BUILD_DIR/gateway/gateway-runtime" build VERSION="$TAG" \
    IMAGE_NAME=local/fault-flows-gateway-runtime
  log "Building $CONTROLLER_IMAGE"
  make -C "$BUILD_DIR/gateway/gateway-controller" build VERSION="$TAG" \
    IMAGE_NAME=local/fault-flows-gateway-controller
}

cmd_up() {
  log "Starting the stack ($RUNTIME_IMAGE, $CONTROLLER_IMAGE)"
  "${COMPOSE[@]}" up -d --force-recreate
  log "Waiting for the management API"
  for _ in $(seq 1 60); do
    if curl -sf -o /dev/null -u admin:admin http://localhost:29090/api/management/v1/llm-providers; then
      echo "  ready: router http://localhost:28080, management http://localhost:29090, mock http://localhost:28009"
      return 0
    fi
    sleep 2
  done
  echo "management API did not come up; see: ${COMPOSE[*]} logs" >&2
  return 1
}

cmd_test() {
  local newman=(newman)
  command -v newman >/dev/null || newman=(npx --yes newman)
  log "Running the collection"
  "${newman[@]}" run "$KIT_DIR/fault-flows.postman_collection.json" --delay-request 20
}

cmd_down() {
  log "Stopping the stack"
  "${COMPOSE[@]}" down -v
}

cmd_clean() {
  log "Removing $BUILD_DIR"
  if [ -d "$BUILD_DIR" ]; then git -C "$ROOT" worktree remove --force "$BUILD_DIR"; fi
  git -C "$ROOT" worktree prune
}

case "${1:-all}" in
  build) cmd_build ;;
  up) cmd_up ;;
  test) cmd_test ;;
  down) cmd_down ;;
  clean) cmd_clean ;;
  all) cmd_build; cmd_up; cmd_test ;;
  *) sed -n '2,19p' "$0"; exit 2 ;;
esac
