#!/usr/bin/env bash
# Create the test organization (seeds the LLM templates and a "default" project).
# Idempotent: an existing organization is reused.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
need_work

out=$(api POST /organizations "{\"id\":\"${E2E_ORG}\",\"displayName\":\"E2E Org\",\"region\":\"us\"}")
code=$(printf '%s\n' "$out" | http_code)
case "$code" in
  201) log "organization '$E2E_ORG' created" ;;
  409) log "organization '$E2E_ORG' already exists — reusing" ;;
  *) printf '%s\n' "$out" >&2; die "create organization failed (HTTP $code)" ;;
esac

# The "default" project is created with the organization.
out=$(api GET /projects/default)
[ "$(printf '%s\n' "$out" | http_code)" = 200 ] || { printf '%s\n' "$out" >&2; die "default project missing"; }
state_set E2E_ORG "$E2E_ORG"
state_set E2E_PROJECT default
log "token (unsigned, internal_token mode): ${TOKEN:0:24}...  base: $BASE"
