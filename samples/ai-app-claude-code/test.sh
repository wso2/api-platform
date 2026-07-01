#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
GATEWAY_BASE_URL="http://localhost:8080/reading-list/v1"
GATEWAY_HEALTH_URL="http://localhost:9094/health"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SETTINGS_FILE="${SCRIPT_DIR}/.claude/settings.json"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
PASS=0
FAIL=0

pass() { echo "[PASS]  $*"; PASS=$((PASS + 1)); }
fail() { echo "[FAIL]  $*" >&2; FAIL=$((FAIL + 1)); }
info() { echo "[INFO]  $*"; }
section() { echo ""; echo "--- $* ---"; }

assert_status() {
  local label="$1"
  local expected="$2"
  local actual="$3"
  if [[ "${actual}" == "${expected}" ]]; then
    pass "${label} → HTTP ${actual}"
  else
    fail "${label} → expected HTTP ${expected}, got HTTP ${actual}"
  fi
}

# ---------------------------------------------------------------------------
# Pre-flight: settings.json and gateway health
# ---------------------------------------------------------------------------
section "Pre-flight checks"

if [[ ! -f "${SETTINGS_FILE}" ]]; then
  echo "[ERROR] ${SETTINGS_FILE} not found. Run ./setup.sh first." >&2
  exit 1
fi

API_KEY=$(python3 -c "
import json, sys
d = json.load(open('${SETTINGS_FILE}'))
k = d.get('env', {}).get('API_KEY', '')
if not k:
    sys.exit('API_KEY missing in settings.json')
print(k)
")
info "API key loaded from settings.json."

if ! curl -sf "${GATEWAY_HEALTH_URL}" > /dev/null 2>&1; then
  echo "[ERROR] Gateway is not healthy at ${GATEWAY_HEALTH_URL}. Run ./setup.sh first." >&2
  exit 1
fi
info "Gateway is healthy."

# ---------------------------------------------------------------------------
# 1. Authentication enforcement
# ---------------------------------------------------------------------------
section "Authentication enforcement"

# 1a. No key → 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${GATEWAY_BASE_URL}/books")
assert_status "GET /books (no API key)" "401" "${STATUS}"

# 1b. Wrong key → 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "X-API-Key: invalid-key-00000000" \
  "${GATEWAY_BASE_URL}/books")
assert_status "GET /books (wrong API key)" "401" "${STATUS}"

# ---------------------------------------------------------------------------
# 2. GET /books — list; grab a seeded book id for later
# ---------------------------------------------------------------------------
section "GET /books"

_TMP=$(mktemp)
STATUS=$(curl -s -w "%{http_code}" -o "${_TMP}" \
  -H "X-API-Key: ${API_KEY}" \
  "${GATEWAY_BASE_URL}/books")
LIST_RESPONSE=$(cat "${_TMP}"); rm -f "${_TMP}"
assert_status "GET /books (valid key)" "200" "${STATUS}"

SEEDED_ID=$(echo "${LIST_RESPONSE}" | python3 -c "
import sys, json
books = json.load(sys.stdin).get('books', [])
print(books[0]['id'] if books else '')
" 2>/dev/null || echo "")

# ---------------------------------------------------------------------------
# 3. POST /books — add
# ---------------------------------------------------------------------------
section "POST /books"

_TMP=$(mktemp)
HTTP_STATUS=$(curl -s -w "%{http_code}" -o "${_TMP}" \
  -X POST "${GATEWAY_BASE_URL}/books" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{"title":"Test Book","author":"Test Author","status":"to_read"}')
ADD_RESPONSE=$(cat "${_TMP}"); rm -f "${_TMP}"
assert_status "POST /books" "201" "${HTTP_STATUS}"

BOOK_ID=$(echo "${ADD_RESPONSE}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(d.get('uuid', d.get('id', '')))
" 2>/dev/null || echo "")

if [[ -z "${BOOK_ID}" ]]; then
  fail "POST /books did not return an id"
else
  pass "POST /books returned id: ${BOOK_ID}"
fi

# ---------------------------------------------------------------------------
# 4. GET /books/{id} — fetch a seeded book (newly added books are not persisted)
# ---------------------------------------------------------------------------
section "GET /books/{id}"

if [[ -n "${SEEDED_ID}" ]]; then
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "X-API-Key: ${API_KEY}" \
    "${GATEWAY_BASE_URL}/books/${SEEDED_ID}")
  assert_status "GET /books/${SEEDED_ID}" "200" "${STATUS}"
else
  fail "GET /books/{id} skipped — no seeded book id from GET /books"
fi

# ---------------------------------------------------------------------------
# 5. PUT /books/{id} — update status
# ---------------------------------------------------------------------------
section "PUT /books/{id}"

if [[ -n "${BOOK_ID}" ]]; then
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X PUT "${GATEWAY_BASE_URL}/books/${BOOK_ID}" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: ${API_KEY}" \
    -d '{"status":"reading"}')
  assert_status "PUT /books/${BOOK_ID} (status=reading)" "200" "${STATUS}"
else
  fail "PUT /books/{id} skipped — no book id from POST"
fi

# ---------------------------------------------------------------------------
# 6. DELETE /books/{id}
# ---------------------------------------------------------------------------
section "DELETE /books/{id}"

if [[ -n "${BOOK_ID}" ]]; then
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X DELETE "${GATEWAY_BASE_URL}/books/${BOOK_ID}" \
    -H "X-API-Key: ${API_KEY}")
  assert_status "DELETE /books/${BOOK_ID}" "200" "${STATUS}"
else
  fail "DELETE /books/{id} skipped — no book id from POST"
fi

# ---------------------------------------------------------------------------
# 7. api_client.py smoke test
# ---------------------------------------------------------------------------
section "api_client.py smoke test"

SMOKE_OUTPUT=$(cd "${SCRIPT_DIR}" && python3 api_client.py 2>&1)
if echo "${SMOKE_OUTPUT}" | grep -q "Books in reading list"; then
  pass "api_client.py executed successfully"
else
  fail "api_client.py smoke test failed: ${SMOKE_OUTPUT}"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Results: ${PASS} passed, ${FAIL} failed"
echo "============================================================"

[[ "${FAIL}" -eq 0 ]]
