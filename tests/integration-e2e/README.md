# Combined platform-api + gateway end-to-end integration tests

This stack runs the **real platform-api (control plane)** and the **real gateway
(gateway-controller + gateway-runtime data plane)** against the **same database
engine**, so a single scenario exercises both products integrated end to end:
an API created in platform-api is deployed to a gateway and served by the data
plane.

It complements the per-component cross-database suites:
- `platform-api/it` — platform-api store on SQLite / PostgreSQL / SQL Server.
- `gateway/it` — gateway store on SQLite / PostgreSQL / SQL Server.

## Topology

```
        REST (9243)                 control-plane WS (9243)
client ───────────────► platform-api ◄────────────────── gateway-controller ──xDS──► gateway-runtime ──► sample-backend
                            │                                  │                          (8080 ingress)
                         platform_api DB                   gateway_test DB
                            └──────────── shared engine (postgres/sqlserver) ─────────────┘
```

platform-api and gateway-controller keep **separate databases** (their schemas
share table names like `artifacts`/`gateways`/`subscriptions`). The PostgreSQL
variant uses one server with two databases (`init-db.sql`).

## Integration contract (verified from source)

- gateway-controller dials platform-api at `…/api/internal/v1/ws/gateways/connect`
  with an `api-key` header (the gateway **registration token**); platform-api
  validates it via `GatewayService.VerifyToken` and replies `connection.ack`.
- platform-api pushes `subscription.created` / deployment events down that socket;
  the gateway-controller pulls subscription/plan data from
  `…/api/internal/v1/subscription-plans` and posts its manifest to
  `…/api/internal/v1/gateways/{id}/manifest`.

## Cucumber suite (godog)

The suite is written in Gherkin and run by [godog](https://github.com/cucumber/godog),
consistent with `gateway/it`:

| File | Purpose |
|------|---------|
| `features/api-deployment.feature` | The scenarios (Gherkin). |
| `suite_test.go` | godog runner + `BeforeSuite`/`AfterSuite` (compose orchestration, bootstrap) + platform-api REST helpers. |
| `steps_test.go` | step definitions + ingress polling. |
| `docker-compose*.yaml` | the stack per database engine. |

### How the harness works

`BeforeSuite` brings the whole stack up once and solves the registration-token
chicken-and-egg: it starts the control plane, authenticates (`admin`/`admin`),
creates a project and one (or two) gateway(s) with their registration tokens,
then starts the gateway controllers with those tokens. Each scenario then creates
its own API and deploys it to a pre-registered gateway, so scenarios are
independent. The `I deploy the API to the gateway` step just creates the
deployment via the platform-api REST API: while the controller is connected,
platform-api pushes an `api.deployed` event down the control-plane socket and the
controller deploys that API to the data plane live (`handleAPIDeployedEvent` in
`pkg/controlplane/client.go`) — no restart needed. The connect-time **full** sync
(`c.syncOnce`) still runs only once, on connect; that is the path the restart /
recovery scenarios exercise (a controller that starts with a deployment it has
never seen re-syncs it from the control plane).

## Running

Build the component images once (tagged `it-e2e`), then run the suite:

```bash
cd platform-api && docker build -t platform-api:it-e2e --build-context common=../common .
cd gateway      && make build VERSION=it-e2e   # gateway-controller / gateway-runtime :it-e2e

cd tests/integration-e2e
go test -run TestFeatures -v ./...                                  # PostgreSQL (default)
E2E_DB=sqlite go test -run TestFeatures -v ./...                    # SQLite
MSSQL_IMAGE=mcr.microsoft.com/azure-sql-edge:latest E2E_DB=sqlserver \
  go test -run TestFeatures -v ./...                                # SQL Server (azure-sql-edge on Apple Silicon)
```

Or via make (from `platform-api/`): `make e2e`, `make e2e-all-dbs`.

- `E2E_DB` = `postgres` (default) | `sqlite` | `sqlserver`.
- `E2E_KEEP=1` leaves the stack up after the run for inspection.
- `E2E_TAGS=@smoke` runs a tag subset (`@restart` selects just the restart /
  recovery scenarios). The `@multigateway` and `@restart` scenarios run only on
  the postgres stack (the only one wired with a second gateway, and the only stack
  the store-wipe recovery step supports) and are otherwise skipped automatically.
- `PA_HOST_PORT` / `GW_HTTP_PORT` / `GW2_HTTP_PORT` override the published host
  ports to avoid clashing with other local stacks (defaults 9243 / 18080 / 18081).

### Scenarios

`features/api-deployment.feature` — deployment happy path:

1. **An API deployed to a gateway is served by the data plane** — deploy, then a
   request to the ingress returns 200 via Envoy; a path outside the API context
   returns 404.
2. **Undeploy / redeploy** — undeploying stops the data plane serving the API
   (404); redeploying restores it (200).
3. **Multi-gateway** (`@multigateway`, postgres) — the same API deployed to two
   gateways is served by both (fan-out), and undeploying from one leaves the
   other serving (per-gateway isolation).

`features/gateway-restart.feature` (`@restart`, postgres) — a gateway must keep
serving, and reconcile to the control plane's desired state, across restarts. A
process restart re-runs the connect-time full sync (`c.syncOnce`), so these prove
that recovery path from every angle:

4. **Controller restart** (`@smoke`) — restarting the gateway controller keeps
   the API served; unmapped paths still 404.
5. **Runtime restart** — restarting the gateway runtime (Envoy) re-serves the API
   (it re-pulls its xDS snapshot from the controller on reconnect).
6. **Whole-gateway restart** — restarting controller + runtime recovers all
   served APIs from persisted state.
7. **Deploy while down** — a deployment created while the controller is stopped is
   picked up on its next start (connect-time sync), alongside the already-served
   API.
8. **Undeploy while down** — an undeployment issued while the controller is
   stopped is applied on its next start (the API stops being served, → 404).
9. **Empty-store recovery** (`@recovery`) — a gateway whose local store is wiped
   re-fetches every deployed artifact from the control plane on restart, proving
   the control plane is the source of truth for disaster recovery.
10. **Restart isolation** (`@multigateway`) — restarting one gateway leaves the
    other serving uninterrupted.

`features/secret.feature` (`@secret`, postgres) — a secret created in
platform-api must reach the data plane without ever leaving the control plane in
plaintext. There is no live secret push: the controller syncs secrets on connect,
before deployments, so `{{ secret "…" }}` placeholders resolve
(`c.syncOnce → syncSecrets → syncDeployments`). Both scenarios deploy a REST API
whose `set-headers` policy injects `X-Auth-Token: Bearer {{ secret "…" }}` and
assert (via the sample backend, which echoes request headers) that the resolved
plaintext reaches the upstream:

11. **Secret resolved into an upstream header** (`@smoke`) — after a controller
    restart syncs the secret, the injected header carries the resolved value.
12. **Secret survives a full gateway restart** (`@restart`) — the resolved secret
    is re-synced and re-injected after restarting controller + runtime.
13. **Secret re-fetchable after a control-plane restart** (`@cp-restart`) — after
    platform-api restarts, wiping the gateway store and re-syncing still yields the
    correct value. This requires a stable `PLATFORM_SECRET_ENCRYPTION_KEY` (set in
    the compose files); without it, demo mode mints a new key per restart and the
    stored secret would be undecryptable (the platform-api ephemeral-key bug).

`features/platform-api-restart.feature` (`@restart @cp-restart`, postgres) — a
control-plane restart is distinct from a gateway restart: the data plane must keep
serving while platform-api is down, and the controller must auto-reconnect its
control-plane socket (no gateway restart) when platform-api returns:

13. **Gateway survives a CP restart** — restarting platform-api leaves the gateway
    serving the API (data plane is independent of CP availability), and the control
    plane accepts requests again afterwards (recovered).
14. **Auto-reconnect picks up new work** — after platform-api restarts, a newly
    deployed API is picked up by the auto-reconnected controller (via the
    eventhub's replay-on-reconnect), with no gateway restart.

`features/lifecycle.feature` (`@lifecycle`, postgres) — the full REST deployment
lifecycle, not just first deploy:

15. **Update → redeploy** — mutating an API (a set-headers version marker) and
    redeploying propagates the new version to live traffic (echoed header flips
    v1 → v2).
16. **Delete → cleanup** — deleting the API in platform-api (`api.deleted`) makes
    the gateway stop serving it.
17. **Undeploy → restore** — restoring an UNDEPLOYED deployment
    (`/deployments/{id}/restore`) resumes serving.

`features/api-key-auth.feature` (`@apikey`, postgres) — API-key auth enforced end
to end:

18. **Key enforcement** — a deployed API carrying an `api-key-auth` policy rejects
    an unauthenticated request; a key generated in platform-api
    (`POST /rest-apis/{id}/api-keys`, broadcast via `apikey.created`) is then
    accepted. (API-key auth is an operation **policy**, not a top-level security
    field — that field is LLM-only.)

`features/mcp-proxy.feature` (`@mcp`, postgres) — an **MCP proxy** created in
platform-api is deployed to the gateway and served at `<context>/mcp`; a JSON-RPC
`initialize` through the ingress returns a result. Needs a real MCP server
(`mcp-backend`, image `rakhitharr/mcp-everything:v3`, started on demand) — the
echo backend can't satisfy the MCP handshake.

`features/llm-proxy.feature` (`@llm-proxy`, **quarantined `@wip`**) — an LLM proxy
over a provider. Transitively blocked by the same provider template-apiVersion bug
(a proxy requires its provider + template deployed on the gateway first). Captures
the intended flow; remove `@wip` once the bug is fixed.

`features/llm-provider-secret.feature` (`@llm`, postgres) — **quarantined `@wip`**,
excluded from default runs (`~@wip`); run explicitly with `E2E_TAGS=@llm` to
reproduce. Same secret flow as above but via an **LLM provider** (openai template)
whose `upstream.main.auth.value` is `Bearer {{ secret "…" }}`, invoked at
`<context>/chat/completions`, before and after a full gateway restart. These are
written and correct but currently **fail on an open platform-api bug** (see
below); secret resolution itself works — the failure is the template-apiVersion
gap. Remove `@wip` once the bug is fixed.

## Status — passing on all three databases

The full live-traffic scenarios (`api-deployment.feature`) pass on **SQLite,
PostgreSQL and SQL Server** (verified locally; SQL Server via `azure-sql-edge` on
Apple Silicon). The extended suites — gateway-restart, secret, control-plane
restart, lifecycle, api-key-auth and mcp-proxy — are postgres-only and pass there:
**20/20 scenarios green in the default run** (verified locally; the resolved secret
value and the api-key/MCP paths were also confirmed by hand; the auto-reconnect
scenario was flake-checked across repeated runs). The `@wip` LLM-provider and
LLM-proxy scenarios are excluded from default runs (open bug, below).

Bugs this harness surfaced and that are fixed alongside it:
- platform-api image build (`go.sum` missing `go-mssqldb` → `go mod tidy`).
- platform-api SQL Server `LIMIT` and cascade-path/self-ref schema issues.
- gateway eventhub `INSERT … ON CONFLICT` (invalid on SQL Server) in the
  deployment event-publish path → made dialect-aware
  (`common/eventhub/sqlbackend.go`).

Open bug this harness surfaced (test quarantined `@wip`, not yet fixed):
- **LLM provider deploy from platform-api fails gateway validation** with
  `template: version: Version must be 'gateway.api-platform.wso2.com/v1'`
  (`gateway-controller/pkg/config/llm_validator.go`). platform-api's LLM provider
  templates carry no apiVersion: the default template files
  (`platform-api/.../default-llm-provider-templates/*.yaml`) omit it, the loader
  (`llm_provider_template_loader.go`) reads `apiVersion` but never stores it on the
  model, and the provider deploy YAML sends only the template handle — so the
  template the gateway validates has an empty apiVersion. Secret resolution itself
  works. Fix by propagating the template apiVersion from platform-api to the
  gateway, then drop `@wip` from `features/llm-provider-secret.feature`.
