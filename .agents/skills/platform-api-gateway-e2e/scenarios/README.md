# Scenarios

A scenario is an executable bash script, `scenarios/<name>.sh <version>...`, that:

1. sources `../scripts/lib.sh`;
2. creates its artifacts once with `ensure_created <collection> <json>` / `ensure_secret <handle> <value>`
   (artifacts are org-scoped and version-independent; 409 means "reuse");
3. for each version: `"$S/deploy.sh" <collection> <handle> "$v"`, then invokes the gateway with
   `invoke_until <secs> <echo-field> <expected> "$v" <METHOD> <path> [body] [curl args]`
   (or `"$S/invoke.sh"` when no field should be waited for);
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
| `secret-rotation` | rotated secret reaches the backend after redeploy (1.2.0: only after a controller reconnect — known gateway cache behaviour); run after `llm-provider-secret` |
| `secret-migrated-row` | a secret row as the v1→v2 migration writes it (NULL `description`/`created_by`/`updated_by`) behind an api-key-secured LLM provider still deploys below 1.2.0; valid key → 200 with the plaintext upstream, no key → 401 |
| `secret-undecryptable` | a secret that exists but cannot be decrypted: deploy below 1.2.0 ends `FAILED`/`SECRET_RESOLUTION_FAILED`, platform-api logs the cause, the gateway log carries the 422 body |

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
