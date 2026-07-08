# Developer Portal Integration Tests

Integration tests for the Developer Portal. There are two suites, both run against a
real portal instance in Docker Compose:

- **Backend REST API suite** (`backend/`) — Jest + Supertest tests that exercise the
  Admin/DevPortal REST APIs, webhook delivery, key generation, and database side effects.
- **UI E2E suite** (`ui/`) — Cypress tests that validate portal rendering, authentication
  flows, try-out consoles, theming, and search in a headless browser.

Each suite can run against either **SQLite** (default, no external DB) or **PostgreSQL**.

## Architecture

```
┌──────────────────────────────┐   ┌──────────────────────────────┐
│   Backend suite (Jest)       │   │   UI suite (Cypress)         │
│   backend/**/*.spec.js        │   │   ui/cypress/e2e/**/*.cy.js  │
│   (Supertest + DB asserts)    │   │   (headless Electron)        │
└──────────────┬───────────────┘   └──────────────┬───────────────┘
               │                                   │
               ▼                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Docker Compose Environment                     │
│  ┌──────────────┐   ┌──────────────────────┐   ┌────────────────┐   │
│  │ platform-api │◄──│  developer-portal    │──►│  postgres      │   │
│  │  :9243       │   │    :3000 (HTTP)      │   │  :5432         │   │
│  │  (auth/IdP)  │   │    /health           │   │  (postgres     │   │
│  └──────────────┘   └──────────────────────┘   │   profile only)│   │
│                                                 └────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

**Components:**
- **platform-api** — provides file-based auth / IdP so the backend suite can perform real
  session logins (`admin`/`admin`, `publisher`/`publisher`, `developer`/`developer`).
- **developer-portal** — the pre-built image under test, tagged `:test` by `make ensure-test-tag`.
- **Jest + Supertest** — backend REST API test framework.
- **Cypress** — UI E2E test framework (headless Electron).
- **SQLite / PostgreSQL** — SQLite by default; the `-postgres` targets swap in a Postgres service.

## Prerequisites

- Docker and Docker Compose
- A built `developer-portal` image — run `make build` from `portals/developer-portal/` first.
  (The Make targets here auto-tag that image as `:test`.)

## Quick Start

```bash
# 1. Build the developer-portal image (from the portal root)
cd portals/developer-portal && make build

# 2. Run the tests (from this directory)
cd it
make test-backend    # backend REST API suite (Jest, SQLite)
make test            # UI E2E suite (Cypress, SQLite)
```

## Project Structure

```
portals/developer-portal/it/
├── backend/                        # Backend REST API suite (Jest + Supertest)
│   ├── <feature>/*.spec.js         # apis, api-keys, applications, subscriptions,
│   │                               #   key-managers, mcp-servers, webhook-subscribers, ...
│   ├── support/                    # client.js, db.js, fixtures.js, webhook-sink.js,
│   │                               #   global-setup.js, global-teardown.js, wait-for.js, ...
│   ├── jest.config.js              # Jest config (jest-junit → reports/backend-results.xml)
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
| `make test-backend` | Run the Jest backend REST API suite (SQLite) |
| `make test-backend-postgres` | Run the Jest backend REST API suite (PostgreSQL) |
| `make open` | Open the Cypress interactive UI against a locally running devportal |
| `make deps` | Install Node dependencies (only needed for `make open`) |
| `make clean` | Remove test containers, volumes, and report artifacts |

> All targets require the `developer-portal` image to be built first (`make build` in
> `portals/developer-portal/`). `make ensure-test-tag` (run automatically) tags it as `:test`.

You can also run both UI suites from the portal root: `make -C portals/developer-portal it`
(SQLite) and `make -C portals/developer-portal it-postgres`.

## Continuous Integration

Both suites run automatically on pull requests that touch `portals/developer-portal/**`,
via [`.github/workflows/devportal-integration-test.yml`](../../../.github/workflows/devportal-integration-test.yml):

- **`backend-integration-test`** — builds the image and runs `make test-backend` /
  `make test-backend-postgres` in an `sqlite` × `postgres` matrix.
- **`ui-test`** — builds the image and runs `make test` (Cypress, SQLite).

Test reports (`it/reports/`) are uploaded as workflow artifacts on every run. The workflow
can also be triggered manually via **workflow_dispatch**.

> **Note:** `make open` launches Cypress directly in E2E mode (`--e2e`), bypassing the
> Launchpad setup screen and showing the spec list immediately. It runs against a locally
> running devportal (start it with `docker compose up` from `portals/developer-portal/`).

## Cypress Custom Commands

Defined in `ui/cypress/support/`:

| Command | Description |
|---------|-------------|
| `cy.visitPortal(path)` | Navigate to a path inside the default portal view |
| `cy.portalUrl(path)` | Build a URL under the default view without visiting it |
| `cy.apiRequest(method, path, options)` | `cy.request` wrapper that injects the IT API key header for admin-protected endpoints |
| `cy.login(username, password)` | Perform a real login flow (see `support/commands/auth.js`) |
| `cy.logout()` | Log the current user out |
| `cy.createApplication(name)` / `cy.deleteApplication(name)` | Create/delete an application (see `support/commands/applications.js`) |

## Example Tests

**UI (Cypress)** — `ui/cypress/e2e/`:

```js
describe('Developer Portal — API Listing', () => {
    it('GET /devportal/organizations returns 200 with an array', () => {
        cy.apiRequest('GET', '/devportal/organizations').then((resp) => {
            expect(resp.status).to.eq(200);
            expect(resp.body).to.be.an('array');
        });
    });

    it('loads the API browse page without errors', () => {
        cy.visitPortal('/apis');
        cy.get('body').should('be.visible');
        cy.get('body').should('not.contain.text', '500');
    });
});
```

**Backend (Jest + Supertest)** — `backend/`:

```js
const client = require('../support/client');

describe('Organizations REST API', () => {
    beforeAll(async () => {
        await client.login('admin');       // real session login via platform-api
    });

    it('lists organizations', async () => {
        const res = await client.as('admin').get('/organizations');
        expect(res.status).toBe(200);
        expect(Array.isArray(res.body)).toBe(true);
    });
});
```

## Test Reports

After a run, artifacts are available under `reports/`:

| Report | Location |
|--------|----------|
| Backend JUnit results | `reports/backend-results.xml` |
| Cypress screen recordings | `reports/videos/*.mp4` |
| Cypress failure screenshots | `reports/screenshots/` |

## Authentication in Tests

- **Backend suite** performs **real session logins** against `platform-api` using the
  file-based users defined in `configs/config-platform-api-it.toml`
  (`admin`/`admin`, `publisher`/`publisher`, `developer`/`developer`).
- **UI suite** uses both real login flows (`auth/` specs) and, for admin-protected REST
  calls, an IT API key injected via the `x-wso2-api-key` header.

## Adding New Tests

**Backend (Jest):**
1. Add a `*.spec.js` file under the relevant `backend/<feature>/` directory.
2. Use the helpers in `backend/support/` (`client.js` for authenticated requests,
   `db.js` to assert database side effects, `webhook-sink.js` for webhook delivery, etc.).
3. Run `make test-backend` to verify before committing.

**UI (Cypress):**
1. Add a `*.cy.js` file under the relevant `ui/cypress/e2e/<area>/` directory.
2. Use the custom commands in `ui/cypress/support/` and fixtures in `ui/cypress/fixtures/`.
3. Run `make test` to verify before committing.
