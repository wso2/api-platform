# Combined platform-api + gateway end-to-end integration tests

This stack runs the **real platform-api (control plane)** and the **real gateway
(gateway-controller + gateway-runtime data plane)** against the **same database
engine**, so a single scenario exercises both products integrated end to end:
an API created in platform-api is deployed to a gateway and served by the data
plane.

On the postgres stack it additionally runs the **real developer portal**, so the
`@devportal` scenario exercises all three planes together: a credential created
in the portal reaches the gateway via the signed webhook to platform-api.

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
independent. The `I deploy the API to the gateway` step bounces that gateway's
controller, because the controller runs its full deployment sync only once on
connect (`c.syncOnce` in `pkg/controlplane/client.go`); the restart re-runs that
sync so the new deployment is picked up.

## Running

Build the component images once (tagged `it-e2e`), then run the suite:

```bash
cd platform-api && docker build -t platform-api:it-e2e \
  --build-context common=../common --build-context httpkit=../httpkit .
cd gateway      && make build VERSION=it-e2e   # gateway-controller / gateway-runtime :it-e2e
cd portals/developer-portal && docker build -t developer-portal:it-e2e .  # only needed for @devportal

cd tests/integration-e2e
go test -run TestFeatures -v ./...                                  # PostgreSQL (default)
E2E_DB=sqlite go test -run TestFeatures -v ./...                    # SQLite
MSSQL_IMAGE=mcr.microsoft.com/azure-sql-edge:latest E2E_DB=sqlserver \
  go test -run TestFeatures -v ./...                                # SQL Server (azure-sql-edge on Apple Silicon)
```

Or via make (from `platform-api/`): `make e2e`, `make e2e-all-dbs`.

- `E2E_DB` = `postgres` (default) | `sqlite` | `sqlserver`.
- `E2E_KEEP=1` leaves the stack up after the run for inspection.
- `E2E_TAGS=@smoke` runs a tag subset (other tags: `@secured`, `@multigateway`,
  `@devportal`, and `@lifecycle` for the credential-lifecycle scenario — run it alone
  with `E2E_TAGS="@devportal && @lifecycle"`). The `@multigateway` and `@devportal`
  scenarios run only on the postgres stack (the only one wired with a second gateway
  and the developer portal) and are otherwise skipped automatically.
- `PA_HOST_PORT` / `GW_HTTP_PORT` / `GW2_HTTP_PORT` / `DP_HOST_PORT` override the
  published host ports to avoid clashing with other local stacks (defaults 9243 /
  18080 / 18081 / 3000).
- `DEVPORTAL_IMAGE` overrides the developer-portal image (default
  `developer-portal:it-e2e`). `PA_WEBHOOK_KEY` is set automatically by the suite
  (a container-readable copy of the webhook private key) — you don't normally set it.
- `PA_API_BASE` / `DP_API_BASE` override the REST resource-API base path for
  platform-api and the developer portal respectively (default `/api/v0.9` each) —
  set these when either product moves to a new API version, independently of the
  other. `PA_PORTAL_BASE` (login, default `/api/portal/v0.9`) and `PA_WEBHOOK_BASE`
  (webhook receiver, default `/api/internal/v0.9`) cover platform-api's other prefixes.

### Scenarios

1. **An API deployed to a gateway is served by the data plane** — deploy, then a
   request to the ingress returns 200 via Envoy; a path outside the API context
   returns 404.
2. **Undeploy / redeploy** — undeploying stops the data plane serving the API
   (404); redeploying restores it (200).
3. **Multi-gateway** (`@multigateway`, postgres) — the same API deployed to two
   gateways is served by both (fan-out), and undeploying from one leaves the
   other serving (per-gateway isolation).
4. **Secured API** (`@secured`, `features/secured-api-invocation.feature`) — a
   **PUBLISHED** REST API guarded by the `api-key-auth` and
   `subscription-validation` policies is deployed, then invoked through the data
   plane. This exercises the full control-plane → data-plane credential chain:
   create a subscription plan → create + publish the secured API offering it →
   deploy → create an application → subscribe it under the plan (minting a
   `Subscription-Key` token) → issue an `API-Key` → invoke the ingress. A request
   with **both** valid headers returns 200; an unauthenticated request is
   rejected (401/403). Runs on all databases (single gateway).

   Two properties this scenario relies on (verified from source):
   - **Ordering** — the API is deployed *before* the subscription/key are
     created, because platform-api only broadcasts `subscription.created` /
     `apikey.created` to gateways where the artifact is already deployed (and
     `POST …/api-keys` returns 503 when no gateway is connected).
   - **No restart** — unlike deployments (which need the controller bounce
     described above), subscriptions and API keys are pushed live over the
     control-plane WebSocket and applied immediately, so the scenario just polls
     the ingress until they propagate.
5. **Full suite via developer portal** (`@devportal`, postgres,
   `features/devportal-webhook.feature`) — the same secured API, but the
   subscription and API key are created in the **developer portal**, which fires
   signed webhooks to platform-api; platform-api decrypts them, persists the
   credentials, and propagates them to the gateway. The API is then invoked
   through the gateway with those **portal-issued** credentials → 200 (and a
   credential-less request is still rejected). This is the three-plane
   (platform-api + gateway + devportal) round-trip.

   How the trust/transport is wired (all verified end to end):
   - The devportal is added to the postgres compose stack with its **own database
     on the shared postgres** (`devportal`, created by `init-db.sql`). Postgres,
     not the devportal's default SQLite, because the org-update path needs
     `UPDATE … RETURNING`, which SQLite doesn't provide; its schema is pre-loaded
     from `database/schema.postgres.sql` (the devportal does not auto-migrate on
     an external DB).
   - Auth: the devportal accepts the platform-api admin JWT directly (shared
     `DP_PLATFORMAPI_JWTSECRET`, org from the token's `org_handle` claim). The
     admin must carry `dp:*` scopes, which platform-api's built-in admin lacks —
     so the suite injects an admin (ap:* **and** dp:*) via the
     `AUTH_FILE_BASED_USERS` env var (a mounted config's users are ignored; only
     that env override wins). Bearer auth (not API-key mode) is used because the
     write paths need a resolved user for `created_by`.
   - `BeforeSuite` links the portal org (`cpRefId = "default"`, the platform-api
     org handle) and registers a webhook subscriber pointing at
     `…/api/internal/v0.9/webhook/events` with the shared HMAC secret and the RSA
     **public** key `devportal-webhook.pub`. platform-api decrypts with the paired
     `devportal-webhook.pem` (mounted in; the suite copies it to a 0644 file under
     the compose dir — the container runs as uid 10001 and the source is 0600, and
     `/tmp` isn't shared into the container VM).
   - The delivery worker POSTs over raw https with the default agent, so the
     devportal container sets `NODE_TLS_REJECT_UNAUTHORIZED=0` to accept
     platform-api's self-signed cert.
   - Webhooks are signed `t=<unix>,v1=<hmac>` over `"<t>.<body>"` and the key /
     token fields are hybrid-encrypted (RSA-OAEP-SHA256 + AES-256-GCM). platform-api
     re-encrypts the subscription token at rest, so
     `DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY` must be 32 bytes (64 hex chars).
   - platform-api resolves the event's **org, API and plan by handle**, so the
     devportal org's `cpRefId`, the published API's `referenceId`, and the synced
     plan's `refId` are each set to the corresponding platform-api handle.
   - Delivery is fire-once on a ~2s poll; the scenario polls the ingress until the
     credentials propagate.
6. **Developer-portal credential lifecycle** (`@devportal @lifecycle`, postgres,
   same feature file) — after the same publish/deploy/subscribe/key setup, it drives
   every credential-lifecycle change in the developer portal and verifies each via
   the webhook propagation:
   - **Change API key expiry** — set a past expiry; the gateway must reject the now
     expired key (401), then serve again after the expiry is restored. (Verified at
     the gateway, not via a control-plane GET: platform-api exposes **no** REST that
     returns a webhook-created key's expiry — `/me/api-keys` is filtered to the
     caller's own keys and webhook keys have no owner, and `/applications/{id}/api-keys`
     needs a key→app mapping that neither the `apikey.application_updated` webhook nor
     the direct `AddApplicationAPIKeys` REST can establish. This is a real product gap
     worth flagging.)
   - **Change subscription plan** — switch plans in the portal; platform-api's
     `GET /subscriptions` then reports the new `subscriptionPlanName`.
   - **Regenerate subscription token** — the new token works at the gateway (200) and
     the old token is rejected.
   - **Pause** the subscription (status `INACTIVE`) → gateway 403; **resume** → 200.
   - **Revoke** the API key → gateway 401.
   - **Remove** the subscription → gateway 403.
   Isolation: revoked/expired key → **401** (api-key-auth), inactive/deleted
   subscription → **403** (subscription-validation); each check leaves exactly one
   credential invalid so the code identifies the cause (the key is re-issued before
   the subscription is removed).

   Note: the scenarios are heavy (controller restarts + Envoy propagation). Each
   plan/API uses a **unique** display name and handle — the gateway keys plans by
   `(gateway_id, plan_name)`, so reusing a display name across scenarios collides and
   the later plan (with its subscription) silently fails to sync. With that, the full
   six-scenario run passes in one process (~3 min); it is still resource-intensive on
   constrained hosts, where running per-tag (`E2E_TAGS=@devportal`, etc.) is lighter.

## Status — passing on all three databases

The full live-traffic scenario passes on **SQLite, PostgreSQL and SQL Server**
(verified locally; SQL Server via `azure-sql-edge` on Apple Silicon).

Bugs this harness surfaced and that are fixed alongside it:
- platform-api image build (`go.sum` missing `go-mssqldb` → `go mod tidy`).
- platform-api SQL Server `LIMIT` and cascade-path/self-ref schema issues.
- gateway eventhub `INSERT … ON CONFLICT` (invalid on SQL Server) in the
  deployment event-publish path → made dialect-aware
  (`common/eventhub/sqlbackend.go`).
