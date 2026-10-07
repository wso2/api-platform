---
name: platform-api-gateway-e2e
description: End-to-end test of the WSO2 API Platform `platform-api` (built from the current working tree) against real gateway releases (default 1.0.0, 1.1.0, 1.2.0, downloaded from GitHub). Boots platform-api locally without Docker (SQLite, internal_token auth with no JWT signature validation), creates an organization, registers one gateway per version and starts each release in Docker with its registration token, uses request-info (go install from GitHub) as an echo backend, deploys REST APIs / LLM providers / MCP proxies / agent proxies, invokes the gateways and reports PASS/FAIL per scenario and gateway version. Use when the user asks to test platform-api with gateways, verify gateway version translation, or run deployment scenarios across gateway versions.
compatibility: Requires docker (with compose plugin), Go, python3, openssl, curl, unzip and network access to GitHub releases.
allowed-tools: Bash Read Write AskUserQuestion
---

# Platform API ↔ gateway end-to-end

Everything is scripted under this skill's directory, `.agents/skills/platform-api-gateway-e2e` in the repository (also reachable as `.claude/skills/...`). Paths below are relative to the repository root:

| Script | Does |
|---|---|
| `scripts/run.sh` | the whole flow: boot → gateways → scenarios → report → teardown |
| `scripts/start-platform-api.sh` | creates a fresh temp work dir, builds `platform-api/` of the current repo, writes a config, starts it (HTTP `9380` for curl, HTTPS `9343` self-signed for gateways) |
| `scripts/bootstrap.sh` | creates organization `e2e-org` (seeds LLM templates + `default` project) |
| `scripts/start-backend.sh` | `go install`s [request-info](https://github.com/renuka-fernando/request-info) from GitHub into the work dir, runs it on `127.0.0.1:8090`, checks containers reach it |
| `scripts/start-gateway.sh <ver>` | downloads the release zip from GitHub, registers `gw-<ver>`, mints its token, starts it in Docker, waits until connected |
| `scripts/deploy.sh <collection> <handle> <ver>` | deploys and waits for `DEPLOYED` (exit 2 = refused by platform-api) |
| `scripts/invoke.sh <ver> <METHOD> <path> [body]` | calls that gateway's router; prints the echo + `HTTP <code>` |
| `scripts/report.sh` / `scripts/teardown.sh [--purge]` | results table / stop everything |
| `scenarios/*.sh` | built-in scenarios; `scenarios/README.md` = format + custom-scenario template |

Read `scenarios/README.md` before writing a custom scenario.

## 1. Inputs

Arguments may name versions and scenarios, e.g. `versions=1.0.0,1.2.0 scenarios=llm-provider-secret keep`.
Always confirm the gateway versions with the user unless the arguments name them. Ask everything
missing with **one** `AskUserQuestion` call:

- **Gateway versions** (multiSelect): `1.0.0`, `1.1.0`, `1.2.0` (the default set; all three
  recommended). "Other" = any other published release. At most 4 per run. Platform API only
  compares LTS (semver) releases; a date-named STS release is treated as a current build and gets
  no adaptation (wso2/api-platform#3681), so the built-in expectations hold for LTS releases only.
  Releases come from
  `https://github.com/wso2/api-platform/releases/download/gateway/v{GATEWAY_VERSION}/wso2apip-api-gateway-{GATEWAY_VERSION}.zip`.
- **Scenarios** (multiSelect, max 4 options per question): offer "All built-in scenarios
  (Recommended)", "Gateway translation only" (`mcp-proxy-upstream-path`, `kind-gating`,
  `llm-policies-flatten`, `mcp-spec-versions-fold`, `upstream-auth-types`,
  `llm-proxy-additional-providers`), "Secrets only" (`llm-provider-secret`, `secret-rotation`);
  "Other" = named scenarios or a custom scenario described in prose. The full list with what each
  proves is in `scenarios/README.md`.
- **After the run**: tear down (Recommended) / keep the stack running for manual checks.

For a custom scenario, write it to a temp file (`mktemp "${TMPDIR:-/tmp}/scenario-XXXXXX.sh"`) from
the template in `scenarios/README.md`, show the user its assertions in one line, and pass its path
in `--scenarios`.

## 2. Preflight

Run from the repository root that contains `platform-api/` (the tree under test; override with
`PAPI_REPO`). In one Bash call:

```bash
go version && docker compose version && python3 --version && openssl version | head -1 && command -v unzip
curl -sfIL -o /dev/null https://github.com/wso2/api-platform/releases && echo "github ✓"
docker info >/dev/null 2>&1 && echo "docker ✓" || echo "docker DOWN"
for p in 9380 9343 8090 18080 28080 38080; do lsof -nP -iTCP:$p -sTCP:LISTEN | tail -n +2; done
```

- Docker down and the runtime is colima: `colima start` (stop it again at the end only if you
  started it). Docker Desktop: ask the user to start it.
- A busy port: ask the user before stopping anything. The ports are overridable
  (`PAPI_HTTP_PORT`, `PAPI_HTTPS_PORT`, `BACKEND_PORT`); gateway host ports are
  `index*10000 + default`, index = order in which the versions are started (first → 18080/19090/19094…,
  second → 28080…, third → 38080…, fourth → 48080…).
- Nothing outside a temp work dir is written. `start-platform-api.sh` creates it with `mktemp -d`
  in `$TMPDIR` and records its path in `$TMPDIR/papi-gw-e2e.workdir` for the other scripts. If Docker
  cannot bind-mount from `$TMPDIR` (colima shares only `$HOME`), the work dir is a `mktemp -d` under
  `~/.cache` instead. Override with `PAPI_GW_E2E_DIR`; other overrides: `GW_DOWNLOAD_URL`
  (template with `{GATEWAY_VERSION}`), `REQUEST_INFO_MODULE`.

## 3. Run

```bash
.agents/skills/platform-api-gateway-e2e/scripts/run.sh \
  --versions 1.0.0,1.1.0,1.2.0 \
  [--scenarios rest-api-basic,llm-provider-secret,...] [--keep]   # default: all built-ins, in a safe order
```

Use a timeout of 600000 ms (first run pulls the gateway images from ghcr.io). The steps can also be
run one by one with the individual scripts above, e.g. to rerun one scenario against a kept stack:
`scenarios/llm-provider-secret.sh 1.0.0 1.2.0` (results append to `$WORK/results.tsv`, then
`scripts/report.sh`).

## 4. Report

Relay `report.sh`'s table (scenario × version), every failed check with expected/actual, and the
translator warnings. For a failure, gather evidence before concluding:
`$WORK/logs/platform-api.log` (`$WORK` = the path in `$TMPDIR/papi-gw-e2e.workdir`), `docker compose -p papi-gw-<ver-dashed> logs gateway-controller`
(run in `$WORK/gw-<ver>`), the gateway's own view (`curl -u admin:admin
http://localhost:<index>9090/api/management/v1/...`), and the stored deployment content in
`$WORK/platform-api/api_platform.db`. Say plainly whether a failure is in platform-api, in the
gateway release, or in the scenario itself.

## 5. Teardown

`run.sh` tears down unless `--keep`. Otherwise: `scripts/teardown.sh` (keeps the work dir with logs, DB and
gateway copies) or `scripts/teardown.sh --purge` (also deletes the work dir). It never touches containers or processes it did
not start (it checks the process answers like platform-api/request-info before killing it).

## Facts the scripts rely on

- platform-api config: `auth.mode="internal_token"` + `auth.internal_token.skip_validation=true`,
  so the scripts send an unsigned `alg: none` JWT whose `organization` claim is fixed
  (`E2E_ORG_REF`) and whose `scope` lists the `ap_admin` scopes. Local testing only.
- The organization is not seeded in this mode; `POST /organizations` creates it (and its `default`
  project and LLM templates).
- Gateways speak only `wss://` / `https://` to the control plane (host:port, no scheme), hence the
  self-signed HTTPS listener; the releases ship with `insecure_skip_verify = true`.
- Control-plane wiring: releases without `scripts/setup.sh` (1.0.0/1.1.0) read `GATEWAY_CONTROLPLANE_HOST` / `GATEWAY_REGISTRATION_TOKEN`
  from the compose `.env`; releases with it (1.2.0+) need `scripts/setup.sh` (admin user, AES key) and
  `APIP_GW_CONTROLLER_CONTROLPLANE_HOST` / `_TOKEN` in `api-platform.env`.
- A gateway is registered as `1.0`/`1.1`/`1.2`; its manifest then stores the exact version
  (1.0.0 sends none → stored as `1.0.0`). platform-api shapes deployments for that version.
- The router serves an update a few seconds after the `DEPLOYED` ack: scenarios poll with
  `invoke_until` rather than asserting on the first call.
- Gateway 1.2.0 caches each secret handle after its first fetch and refreshes on reconnect only,
  so a rotated secret reaches it after a controller restart, not after a redeploy
  (`secret-rotation` demonstrates this).
- request-info ([github.com/renuka-fernando/request-info](https://github.com/renuka-fernando/request-info)) is installed with `go install github.com/renuka-fernando/request-info@latest` (no local checkout) and runs with `-read-envs=false` (its default echoes the whole environment) and on
  `127.0.0.1` (it has a `/command` shell endpoint); containers still reach it via
  `host.docker.internal`.

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| controller log `read /etc/gateway-controller/config.toml: is a directory` | work dir not shared with the Docker VM; set `PAPI_GW_E2E_DIR` to a dir under `$HOME` |
| `start-backend.sh`: containers cannot reach the backend | runtime without host loopback forwarding; run request-info on `0.0.0.0` manually, firewall permitting |
| `download failed` | the version is not a published gateway release, or GitHub is unreachable (set `GW_DOWNLOAD_URL` for a mirror) |
| gateway never `isActive` | controller log: TLS/host errors mean the HTTPS listener or `host.docker.internal` mapping; 401 means a stale token — rerun `start-gateway.sh <ver>` (rotates it) |
| deployment stays `DEPLOYING` then `FAILED` after 60 s | the gateway rejected or never received the artifact; check the controller log for the deployment event |
| port 9243 busy | another platform-api (e.g. an ai-workspace stack) — the skill uses 9380/9343 by default for that reason |
