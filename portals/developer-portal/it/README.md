# Developer Portal Integration Tests

End-to-end integration tests for the Developer Portal, validating UI rendering, REST API behaviour, and service health using Cypress.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Test Suite (Cypress)                         │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐    │
│  │  Spec Files   │  │  Commands     │  │  Fixtures             │    │
│  │  (JS/cy.js)   │  │  (custom)     │  │  (org.json, ...)      │    │
│  └───────────────┘  └───────────────┘  └───────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Docker Compose Environment                     │
│  ┌────────────────────────┐        ┌─────────────────────────────┐  │
│  │  developer-portal      │        │  postgres                   │  │
│  │    :3000 (HTTP)        │◄───────│    :5432                    │  │
│  │    /health             │        │    (seeded with ACME org)   │  │
│  └────────────────────────┘        └─────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

**Components:**
- **Cypress** — E2E and REST API test framework
- **Docker Compose** — Orchestrates the devportal and its Postgres database for testing
- **Seed Data** — ACME organisation and default view pre-loaded from `artifacts/docker-init/`

## Prerequisites

- Docker and Docker Compose
- Built `developer-portal` image (run `make build` from `portals/developer-portal/`)

## Quick Start

```bash
# Build the developer-portal image and run all tests
cd portals/developer-portal && make build
cd it && make test
```

## Project Structure

```
portals/developer-portal/it/
├── cypress/
│   ├── e2e/
│   │   ├── smoke.cy.js          # Health, portal load, and asset tests
│   │   └── api-listing.cy.js    # REST API and API browse page tests
│   ├── fixtures/
│   │   └── org.json             # Seeded ACME organisation IDs and handles
│   └── support/
│       ├── commands.js          # Custom Cypress commands (apiRequest, visitPortal, ...)
│       └── e2e.js               # Global support file (imports commands)
├── reports/                     # Test artifacts (generated at runtime)
│   ├── videos/                  # Cypress screen recordings
│   └── screenshots/             # Screenshots on failure
├── cypress.config.js            # Cypress configuration
├── docker-compose.test.yaml     # Test environment orchestration
├── Makefile                     # Developer commands
├── package.json
└── README.md
```

## Make Commands

| Command | Description |
|---------|-------------|
| `make test` | Run all Cypress specs headlessly inside Docker (CI-friendly) |
| `make open` | Open Cypress interactive UI against a locally running devportal |
| `make deps` | Install Node dependencies (only needed for `make open`) |
| `make clean` | Remove test containers, volumes, and report artifacts |

## Running Tests

```bash
# Run all tests (headless, inside Docker)
make test

# Open the interactive Cypress UI against a local devportal
# First start the devportal: docker compose up (from portals/developer-portal/)
make open
```

**Note:** `make open` launches Cypress directly in E2E mode (`--e2e`), bypassing the Launchpad setup screen and showing the spec list immediately.

## Test Reports

After running tests, reports are available at:

| Report | Location |
|--------|----------|
| Screen recordings | `reports/videos/*.mp4` |
| Failure screenshots | `reports/screenshots/` |

## Custom Commands

| Command | Description |
|---------|-------------|
| `cy.visitPortal(path)` | Navigate to a path inside the ACME/default portal view |
| `cy.portalUrl(path)` | Build a URL under the ACME/default view without visiting it |
| `cy.apiRequest(method, path, options)` | `cy.request` wrapper that injects the IT API key header for admin-protected endpoints |

## Example Test

```js
describe('Developer Portal — API Listing', () => {
    context('REST API', () => {
        it('GET /devportal/organizations returns 200 with an array', () => {
            cy.apiRequest('GET', '/devportal/organizations').then((resp) => {
                expect(resp.status).to.eq(200);
                expect(resp.body).to.be.an('array');
            });
        });
    });

    context('UI — API browse page', () => {
        it('loads the API list page without errors', () => {
            cy.visitPortal('/apis');
            cy.get('body').should('be.visible');
            cy.get('body').should('not.contain.text', '500');
        });
    });
});
```

## Authentication in Tests

The IT environment enables API key authentication so that Cypress can call admin-protected REST endpoints without a login session. The key is configured in `docker-compose.test.yaml` and automatically injected by `cy.apiRequest()` via the `x-wso2-api-key` request header.

UI tests that exercise browser-side rendering do not require authentication — they access the public-facing portal routes directly.

## Adding New Tests

1. Create a new spec file under `cypress/e2e/` (e.g., `my-feature.cy.js`).
2. Use `cy.visitPortal(path)` for UI tests and `cy.apiRequest(method, path)` for REST API tests.
3. Reference `cy.fixture('org')` to access the seeded ACME organisation identifiers.
4. Run `make test` to verify before committing.
