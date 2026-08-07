# API Portal Integration Tests

Integration tests for the API Portal. There are two suites, both run against a
real portal instance in Docker Compose:

- **REST API suite** (`rest-api/`) — Jest + Supertest tests that exercise the
  Admin/Portal REST APIs, webhook delivery, key generation, and database side effects.
- **UI E2E suite** (`ui/`) — Cypress tests that validate portal rendering, authentication
  flows, try-out consoles, theming, and search in a headless browser.

Each suite can run against either **SQLite** (default, no external DB) or **PostgreSQL**.

## Architecture

```
┌──────────────────────────────┐   ┌──────────────────────────────┐
│   REST API suite (Jest)      │   │   UI suite (Cypress)         │
│   rest-api/**/*.spec.js       │   │   ui/cypress/e2e/**/*.cy.js  │
│   (Supertest + DB asserts)    │   │   (headless Electron)        │
└──────────────┬───────────────┘   └──────────────┬───────────────┘
               │                                   │
               ▼                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Docker Compose Environment                     │
│  ┌──────────────┐   ┌──────────────────────┐   ┌────────────────┐   │
│  │ platform-api │◄──│  developer-portal    │──►│  postgres      │   │
│  │  :9243       │   │    :9543 (HTTP)      │   │  :5432         │   │
│  │  (auth/IdP)  │   │    /health           │   │  (postgres     │   │
│  └──────────────┘   └──────────────────────┘   │   profile only)│   │
│                                                 └────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

**Components:**
- **platform-api** — provides file-based auth / IdP so the REST API suite can perform real
  session logins (`admin`/`admin`, `publisher`/`publisher`, `developer`/`developer`).
- **developer-portal** — the pre-built image under test, tagged `:test` by `make ensure-test-tag`.
  Pinned to the single organization `default`, matching platform-api's.
- **developer-portal-other-org** — a second instance of the same image, pinned to `other-org`
  while platform-api still issues tokens for `default`. A portal serves exactly one
  organization and refuses a login carrying any other one's — a check the matched pair above
  can never reach. Started only for the `test-rest-api*` targets (as a `rest-api-tests`
  dependency), and driven by `rest-api/auth/foreign-org-login.spec.js`.
- **Jest + Supertest** — REST API test framework.
- **Cypress** — UI E2E test framework (headless Electron).
- **SQLite / PostgreSQL** — SQLite by default; the `-postgres` targets swap in a Postgres service.

## Authorization modes

`auth.authorization.mode` decides where a request's effective scopes come from, and the
REST suite runs **in full, once per mode**:

| Mode | Effective scopes come from | Grant table |
|---|---|---|
| `scope` | the token's own `scope` claim, as minted by platform-api | `configs/roles-platform-api-it.yaml` |
| `role` (shipped default) | expanding the token's `roles` claim — the scope claim is **ignored** | `configs/portal-roles-it.yaml` |

Both are real deployment configurations, so both must pass the same specs. That works
because the portal-side table mirrors platform-api's exactly, giving each IT account the
same grant either way. Two things keep that honest:

- **`rest-api/auth/grant-table-parity.spec.js`** fails if the two tables drift apart,
  naming the role and the missing scopes — instead of surfacing as a puzzling 403 in
  some unrelated spec. Regenerate the portal table after editing platform-api's.
- **`rest-api/auth/authorization-mode.spec.js`** covers the one deliberate divergence.
  The `narrow` account's roles claim (`dp_narrow_it`) is granted the full developer scope
  set by platform-api but read-only by the portal, so the *same token* creating an
  application succeeds in scope mode and is refused in role mode. Each assertion runs in
  exactly one mode; together they prove the scope claim really is ignored under role mode
  rather than merged — i.e. a caller cannot widen a role's grant by getting extra scopes
  from their issuer. No other spec uses that account.

Mode is selected by `AUTH_MODE`, which the compose fixture feeds to both the portal
(`APIP_AP_AUTH_AUTHORIZATION_MODE`) and the test process (`API_PORTAL_AUTH_MODE`) so they
cannot disagree. Cypress always runs in the default `scope` mode.

## Prerequisites

- Docker and Docker Compose
- A built `developer-portal` image — run `make build` from `portals/api-portal/` first.
  (The Make targets here auto-tag that image as `:test`.)

## Quick Start

```bash
# 1. Build the developer-portal image (from the portal root)
cd portals/api-portal && make build

# 2. Run the tests (from this directory)
cd it
make test-rest-api   # REST API suite (Jest, SQLite)
make test            # UI E2E suite (Cypress, SQLite)
```

## Project Structure

```
portals/api-portal/it/
├── rest-api/                       # REST API suite (Jest + Supertest)
│   ├── <feature>/*.spec.js         # apis, api-keys, applications, subscriptions,
│   │                               #   key-managers, mcp-servers, webhook-subscribers, ...
│   ├── support/                    # client.js, db.js, fixtures.js, webhook-sink.js,
│   │                               #   global-setup.js, global-teardown.js, wait-for.js, ...
│   ├── jest.config.js              # Jest config (jest-junit → reports/rest-api-results.xml)
│   └── package.json
├── ui/                             # UI E2E suite (Cypress)
│   ├── cypress/
│   │   ├── e2e/                    # 000-smoke, auth, rest-apis, graphql-apis,
│   │   │   └── **/*.cy.js          #   mcp-servers, websocket-apis, design-mode, search, ...
│   │   ├── fixtures/               # org.json, users.json
│   │   └── support/                # commands.js, e2e.js, commands/{auth,applications}.js
│   ├── cypress.config.js
│   └── package.json
├── configs/                        # config-platform-api-it.toml (auth/IdP config)
├── reports/                        # Test artifacts (generated at runtime)
├── docker-compose.test.yaml        # Test environment (SQLite)
├── docker-compose.test.postgres.yaml  # Test environment (PostgreSQL)
├── Makefile                        # Developer commands
└── README.md
```

## Make Commands

| Command | Description |
|---------|-------------|
| `make test` | Run the Cypress UI suite headlessly (SQLite, CI-friendly) |
| `make test-postgres` | Run the Cypress UI suite headlessly (PostgreSQL) |
| `make test-rest-api` | Run the Jest REST API suite (SQLite) — **both** authorization modes, sequentially |
| `make test-rest-api-scope` | Same, scope mode only |
| `make test-rest-api-role` | Same, role mode only (the shipped default) |
| `make test-rest-api-postgres` | Run the Jest REST API suite (PostgreSQL) — both modes |
| `make test-rest-api-postgres-scope` | Same, scope mode only |
| `make test-rest-api-postgres-role` | Same, role mode only |
| `make open` | Open the Cypress interactive UI against a locally running portal |
| `make deps` | Install Node dependencies (only needed for `make open`) |
| `make clean` | Remove test containers, volumes, and report artifacts |

> All targets require the `developer-portal` image to be built first (`make build` in
> `portals/api-portal/`). `make ensure-test-tag` (run automatically) tags it as `:test`.

The SQLite targets run under compose project `devportal-it` and the PostgreSQL ones under `devportal-it-postgres` (`-p` in the Makefile), so the two flavours never share containers or volumes.

You can also run both UI suites from the portal root: `make -C portals/api-portal it`
(SQLite) and `make -C portals/api-portal it-postgres`.

## Continuous Integration

Both suites run automatically on pull requests that touch `portals/api-portal/**`,
via [`.github/workflows/devportal-integration-test.yml`](../../../.github/workflows/devportal-integration-test.yml):

- **`rest-api-test`** — builds the image and runs the suite in an
  `sqlite` × `postgres` × `scope` × `role` matrix (four parallel jobs, via the
  per-mode targets, so covering both authorization modes doesn't double wall-clock).
- **`ui-test`** — builds the image and runs `make test` (Cypress, SQLite).

Test reports (`it/reports/`) are uploaded as workflow artifacts on every run. The workflow
can also be triggered manually via **workflow_dispatch**.

> **Note:** `make open` launches Cypress directly in E2E mode (`--e2e`), bypassing the
> Launchpad setup screen and showing the spec list immediately. It runs against a locally
> running portal (start it with `docker compose up` from `portals/api-portal/`).

## Cypress Custom Commands

Defined in `ui/cypress/support/`:

| Command | Description |
|---------|-------------|
| `cy.visitPortal(path)` | Navigate to a path inside the default portal view |
| `cy.portalUrl(path)` | Build a URL under the default view without visiting it |
| `cy.apiRequest(method, path, options)` | `cy.request` wrapper that authenticates with the current session cookie plus the `X-CSRF-Token` header (call `cy.login()` first) |
| `cy.login(username, password)` | Perform a real login flow (see `support/commands/auth.js`) |
| `cy.logout()` | Log the current user out |
| `cy.createApplication(name)` / `cy.deleteApplication(name)` | Create/delete an application (see `support/commands/applications.js`) |

## Example Tests

**UI (Cypress)** — `ui/cypress/e2e/`:

```js
describe('API Portal — API Listing', () => {
    it('GET /portal/organizations/{handle} returns the configured organization', () => {
        cy.apiRequest('GET', `/api/v0.9/organizations/${Cypress.env('ORG_HANDLE')}`).then((resp) => {
            expect(resp.status).to.eq(200);
            expect(resp.body.id).to.eq(Cypress.env('ORG_HANDLE'));
        });
    });

    it('loads the API browse page without errors', () => {
        cy.visitPortal('/apis');
        cy.get('body').should('be.visible');
        cy.get('body').should('not.contain.text', '500');
    });
});
```

**REST API (Jest + Supertest)** — `rest-api/`:

```js
const client = require('../support/client');

describe('Organizations REST API', () => {
    beforeAll(async () => {
        await client.login('admin');       // real session login via platform-api
    });

    // The portal serves one organization, so client.ORG_HANDLE is the only one
    // any spec can address — listing them is a 405 (organizations.spec.js).
    it('retrieves its own organization', async () => {
        const res = await client.as('admin').get(`/organizations/${client.ORG_HANDLE}`);
        expect(res.status).toBe(200);
        expect(res.body.id).toBe(client.ORG_HANDLE);
    });
});
```

## Test Reports

After a run, artifacts are available under `reports/`:

| Report | Location |
|--------|----------|
| REST API JUnit results | `reports/rest-api-results.xml` |
| Cypress screen recordings | `reports/videos/*.mp4` |
| Cypress failure screenshots | `reports/screenshots/` |

## Authentication in Tests

- **REST API suite** performs **real session logins** against `platform-api` using the
  file-based users defined in `configs/config-platform-api-it.toml`
  (`admin`/`admin`, `publisher`/`publisher`, `developer`/`developer`).
- **UI suite** uses real login flows throughout. Seeding and cleanup hooks call
  `cy.login()` and then `cy.apiRequest`, which authenticates with the resulting session
  cookie plus the `X-CSRF-Token` double-submit header. `testIsolation` clears cookies
  between tests, so each `before`/`after` hook needs its own `cy.login()`.

## Adding New Tests

**REST API (Jest):**
1. Add a `*.spec.js` file under the relevant `rest-api/<feature>/` directory.
2. Use the helpers in `rest-api/support/` (`client.js` for authenticated requests,
   `db.js` to assert database side effects, `webhook-sink.js` for webhook delivery, etc.).
3. Run `make test-rest-api` to verify before committing.

**UI (Cypress):**
1. Add a `*.cy.js` file under the relevant `ui/cypress/e2e/<area>/` directory.
2. Use the custom commands in `ui/cypress/support/` and fixtures in `ui/cypress/fixtures/`.
3. Run `make test` to verify before committing.
