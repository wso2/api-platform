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

## Bootstrap (solves the token chicken-and-egg)

The gateway-controller needs a valid registration token at start-up, but the
token is minted by platform-api at run time. The scenario therefore runs in two
phases:

1. **Phase 1** – start `postgres` + `platform-api`. Authenticate, then via the
   platform-api REST API: `POST /organizations` → `POST /projects` →
   create the REST API → `POST /gateways` (CreateGateway) →
   `POST /gateways/{id}/tokens` (RotateToken) to mint the registration token.
2. **Phase 2** – start `gateway-controller` (with
   `GATEWAY_REGISTRATION_TOKEN=<minted token>`) + `gateway-runtime` +
   `sample-backend`. The controller connects to platform-api and receives
   pushed deployments.
3. **Assert** – `POST /apis/{id}/gateways` + `POST /apis/deployments` in
   platform-api, then send a request to the gateway-runtime ingress
   (`http://localhost:8080/...`) and assert the sample-backend response.

## Running

Build the component images once (tagged `it-e2e`), then run `run-e2e.sh`:

```bash
cd platform-api && make build VERSION=it-e2e PLATFORM_API_IMAGE=platform-api:it-e2e   # or: docker build -t platform-api:it-e2e ...
cd gateway      && make build VERSION=it-e2e                                            # gateway-controller / gateway-runtime :it-e2e

cd tests/integration-e2e
./run-e2e.sh                 # PostgreSQL (default)
E2E_DB=sqlite ./run-e2e.sh   # SQLite
MSSQL_IMAGE=mcr.microsoft.com/azure-sql-edge:latest E2E_DB=sqlserver ./run-e2e.sh  # SQL Server (azure-sql-edge on Apple Silicon)
```

Or via make (from `platform-api/`): `make e2e`, `make e2e-all-dbs`.

`E2E_KEEP=1` leaves the stack up for inspection; `./run-e2e.sh down` tears it down.
The gateway ingress is published on host port `18080` (override with `GW_HTTP_PORT`)
to avoid clashing with other local services on 8080.

### What the scenario does

1. **Phase 1** – start the DB + platform-api + sample-backend. Log in (admin/admin),
   then via the platform-api REST API create a project, a REST API (upstream →
   sample-backend), a gateway, a registration token, attach the gateway and
   **deploy** the API. (Deploying before the controller starts means its initial
   sync-on-connect picks up the deployment — no race.)
2. **Phase 2** – start the gateway-controller (with the registration token) +
   gateway-runtime. The controller connects to platform-api, syncs the deployment
   and programs the runtime.
3. **Assert** – a request to the gateway ingress (`:18080/e2e/`) returns the
   sample-backend response (HTTP 200 via Envoy).
4. **Lifecycle** – a path outside the API context returns 404; undeploy → the
   data plane stops serving (404); redeploy → it serves again (200).
5. **Multi-gateway** (PostgreSQL stack only — it is DB-independent) – the same
   API is deployed to a second gateway; both ingresses serve it (fan-out), and
   undeploying from one gateway leaves the other serving (per-gateway isolation).

## Status — passing on all three databases

The full live-traffic scenario passes on **SQLite, PostgreSQL and SQL Server**
(verified locally; SQL Server via `azure-sql-edge` on Apple Silicon).

Bugs this harness surfaced and that are fixed alongside it:
- platform-api image build (`go.sum` missing `go-mssqldb` → `go mod tidy`).
- platform-api SQL Server `LIMIT` and cascade-path/self-ref schema issues.
- gateway eventhub `INSERT … ON CONFLICT` (invalid on SQL Server) in the
  deployment event-publish path → made dialect-aware
  (`common/eventhub/sqlbackend.go`).
