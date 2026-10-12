# Scenarios

A scenario is an executable bash script, `scenarios/<name>.sh <version>...`, that:

1. sources `../scripts/lib.sh`;
2. creates its artifacts once with `ensure_created <collection> <json>` / `ensure_secret <handle> <value>`
   (artifacts are org-scoped and version-independent; 409 means "reuse");
3. for each version: `"$S/deploy.sh" <collection> <handle> "$v"`, then invokes the gateway with
   `invoke_until <secs> <echo-field> <expected> "$v" <METHOD> <path> [body] [curl args]`
   (or `invoke_until_code <secs> <http-code> ...` to wait for a status such as 401, which carries no
   echo; or `"$S/invoke.sh"` when nothing should be waited for). Extra curl args only follow the
   body, so a GET with a header passes `""` as the body. API keys: `api_key_create <collection>
   <handle> [displayName] [value]` prints `id apiKey`, `api_key_revoke <collection> <handle> <id>`
   prints the HTTP code, `gw_api_key_count <ver> <collection> <handle>` counts the keys the gateway
   itself lists and `gw_api_key_present <ver> <collection> <handle> <id>` says whether one is listed
   (the gateway names a key by its platform-api id and drops it when revoked), `echo_has_header
   <name>` says whether a header reached the backend;
4. records every assertion with `check <scenario> "$v" <check-name> <expected> <actual>`, which
   prints a PASS/FAIL line and appends it to `$WORK/results.tsv` (read by `report.sh`).

Use stable handles prefixed `e2e-` so reruns reuse artifacts. Built-in scenarios:

| Scenario | What it proves |
|---|---|
| `rest-api-basic` | REST API routes to the upstream with context stripped and query kept |
| `llm-provider-secret` | LLM provider upstream auth `{{ secret }}` reaches the backend as plaintext on every version; stored deployment keeps the placeholder |
| `mcp-proxy-upstream-path` | MCP upstream ending in `/mcp` is called at exactly that path (old gateways get it stripped) |
| `kind-gating` | Agent proxy deploy is refused with 400 `DEPLOYMENT_KIND_UNSUPPORTED_BY_GATEWAY` on 1.x, nothing stored |
| `llm-policies-flatten` | LLM provider `globalPolicies` + `operationPolicies` (set-headers) both apply on every version (flattened below 1.2.0) |
| `mcp-spec-versions-fold` | MCP `mcpSpecVersions` list reaches every LTS gateway as one `specVersion` (newest supported) |
| `upstream-auth-types` | MCP upstream auth `none` deploys on every LTS gateway with no auth block; `other` deploys with a platform-api warning and is applied like api-key |
| `llm-proxy-additional-providers` | LLM proxy `additionalProviders` stripped with a warning below 1.2.0, kept on 1.2.0; the proxy routes to its primary provider (run after `llm-provider-secret`) |
| `rest-api-api-key` | REST API with the `api-key-auth` policy: keys created in platform-api (generated and caller-supplied) → 200 with the key header removed upstream; none/wrong/revoked → 401 `Valid API key required`; a key created before the deploy arrives by backfill, one created after it by the live event; a revoked key is not backfilled to the next gateway; the gateway lists the keys |
| `llm-provider-api-key` | the same through an LLM provider's `security.apiKey` (api-key-auth global policy, flattened below 1.2.0) |
| `llm-proxy-api-key` | LLM proxy over an api-key-secured provider: the provider key is stored as a secret and injected on the loopback via `provider.auth` `{{ secret }}`; the proxy key gates the proxy; the backend sees the provider's upstream secret and neither key header; keys are bound per artifact (provider key refused at the proxy and vice versa); stored proxy deployment keeps the placeholder |
| `secret-rotation` | rotated secret reaches the backend after redeploy (1.2.0: only after a controller reconnect — known gateway cache behaviour); run after `llm-provider-secret` |
| `secret-migrated-row` | a secret row as the v1→v2 migration writes it (NULL `description`/`created_by`/`updated_by`) behind an api-key-secured LLM provider still deploys below 1.2.0; valid key → 200 with the plaintext upstream, no key → 401 |
| `secret-undecryptable` | a secret that exists but cannot be decrypted: deploy below 1.2.0 ends `FAILED`/`SECRET_RESOLUTION_FAILED`, platform-api logs the cause, the gateway log carries the 422 body (1.2.0+ sync secrets themselves and are skipped) |

## Writing a custom scenario

When the user describes a scenario in prose, write it to a temp file
(`mktemp "${TMPDIR:-/tmp}/scenario-XXXXXX.sh"`) from the template below and pass its path to `run.sh --scenarios`. Take request bodies from
`platform-api/resources/openapi.yaml` (the REST contract) and, for gateway-side shapes, from the
service tests under `platform-api/internal/service/*_test.go`. Collections with deployments:
`rest-apis`, `llm-providers`, `llm-proxies`, `mcp-proxies`, `agent-proxies`.

Upstreams must point at `$BACKEND_URL_FROM_GW` (request-info as seen from the containers). Its echo
gives `Request.Method`, `Request.Header.<Name>.0`, `Request.URL.Path`, `Request.URL.RawQuery`,
`Request.Body` — read them with `echo_get`. request-info is not an MCP or OpenAI server: assert on
what the gateway sent, not on protocol replies. Useful extra evidence:

- gateway view of a deployed artifact: `curl -u admin:admin "$(gw_mgmt_base <ver>)/<kind>"` (e.g. `mcp-proxies`, `llm-providers`; the base path differs per release and `gw_mgmt_base` finds it);
- stored content: `sqlite3 $WORK/platform-api/api_platform.db "select content from deployments where uuid='<id>'"`;
- translator warnings: `grep 'adapted for older gateway' $WORK/logs/platform-api.log`.

API keys (`rest-api-api-key`, `llm-provider-api-key`, `llm-proxy-api-key`): a key belongs to one
artifact, not to a gateway, and is created with `POST /<collection>/<handle>/api-keys`
(`{"displayName"}`; `rest-apis` also accept a caller-supplied `"apiKey"`). The plaintext is returned
once, so display names must be unique per run. The gateway only ever receives the sha256 hash: as the
websocket event `apikey.created`/`apikey.revoked` to the gateways the artifact is deployed on, as a
backfill of every active key at each deploy, and as a bulk pull when a gateway reconnects — so a key
created before the artifact is deployed on a gateway arrives with the deploy, one created after it a
couple of seconds later (`invoke_until_code`). Revoking before the first deploy answers 503. The
`api-key-auth` policy (same `@v1` module in 1.0.0/1.1.0/1.2.0) removes the key header before
forwarding and answers every missing, wrong or revoked key with 401
`{"error":"Unauthorized","message":"Valid API key required"}`. `security.apiKey.valuePrefix` exists
only in the policy shipped with 1.2.0 and is deliberately not used. Gateway-side evidence:
`gw_api_key_count` / `gw_api_key_present` or
`curl -u admin:admin "$(gw_mgmt_base <ver>)/<collection>/<handle>/api-keys"` — the gateway lists each
key under its platform-api id (`name`, `source: external`, masked value) and removes it on revoke.

```bash
#!/usr/bin/env bash
# Scenario <name>: <one-line intent and expected behaviour per version>
set -uo pipefail
S="${SKILL_SCRIPTS:-$(git rev-parse --show-toplevel)/.agents/skills/platform-api-gateway-e2e/scripts}"; . "$S/lib.sh"
N=<name>
ensure_created rest-apis "{\"id\":\"e2e-x\",\"displayName\":\"e2e-x\",\"context\":\"/x\",\"version\":\"v1\",\"projectId\":\"default\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/x\"}}}"
for v in "$@"; do
  "$S/deploy.sh" rest-apis e2e-x "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until 20 URL.Path /x/ping "$v" GET /x/ping)
  check $N "$v" http-status 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /x/ping "$(printf '%s\n' "$out" | echo_get URL.Path)"
done
```
