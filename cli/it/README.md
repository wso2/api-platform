# CLI Integration Tests

End-to-end integration tests for the `ap` CLI, validating commands against real gateway, MCP server, and developer portal infrastructure.

## Prerequisites

- Go 1.25.1 or later
- Docker and Docker Compose
- Network access to pull Docker images

Important notes:

- Docker Compose: the test runner uses the `docker compose up --wait` flag
  which requires Docker Compose v2 (the `docker compose` plugin). Ensure
  your Docker installation provides Compose v2+; older `docker-compose`
  binaries without the `--wait` flag will not work.

- Configuration backup: the integration suite will temporarily back up your
  real CLI config file (`~/.wso2ap/config.yaml`) to
  `~/.wso2ap/config.yaml.backup` and write a clean config during test
  execution. The original config will be restored after the suite finishes
  (whether tests pass or fail). If you rely on a custom local config,
  please be aware of this behavior.

## Quick Start

```bash
# Run all integration tests
make test

# Clean up containers and logs
make clean

# Install dependencies
make deps
```

### Developer portal tests

The devportal scenarios bring up a postgres + developer-portal stack (reusing
`portals/developer-portal/it/docker-compose.test.yaml` with an override that
publishes port 3000 to the host).

> **`make test` runs every suite.** The gateway suite needs locally-built
> coverage images (`gateway-controller-coverage:test`, etc.); without them the
> run fails at infra startup with a registry `denied` error. To run **only** the
> developer-portal suite (no gateway images needed):

```bash
# Builds the developer-portal :test image, then runs only the devportal suite
make test-devportal
```

`make test-devportal` is shorthand for `IT_SUITES=devportal make test` with the
devportal image built first. `IT_SUITES` scopes both which feature dirs run and
which infrastructure starts:

```bash
IT_SUITES=devportal       # only developer-portal suite
IT_SUITES=gateway         # only gateway suite
IT_SUITES=gateway,devportal  # both (default when unset)
```

To run the full suite (`make test`), build both the gateway images and the
developer-portal image first:

```bash
make build-images            # gateway coverage images
make build-devportal-image   # developer-portal :test image
make test
```

The portal is reached over plain HTTP on `http://localhost:3000` and authenticated
with the api-key seeded via `DP_ADVANCED_APIKEY_KEYVALUE` (`devportal-it-test-key`),
which grants admin access. Org-scoped scenarios target the seeded `ACME`
organization (`1ba42a09-45c0-40f8-a1bf-e4aa7cde1575`) from
`portals/developer-portal/database/02-seed_org.postgres.sql`.

Scenarios tagged `@wip` are skipped (via the `~@wip` tag filter). These cover
commands that still depend on server-side changes or fixtures not yet in place —
notably the `application` happy-path commands (the CLI targets the org-scoped
`/o/{orgId}/devportal/v1/applications` path, which the server spec does not yet
expose). Remove the `@wip` tag once those are ready.

The **end-to-end flow** (`features/devportal/e2e.feature`, `DP-E2E-001`) exercises
the real publish lifecycle in one ordered scenario:

1. `ap apiproject init` scaffolds an API project in a per-scenario temp dir.
2. The provided echo artifacts (`resources/devportal/echo-devportal.yaml` +
   `echo-definition.yaml`) are staged into the project's `devportal/` portal root.
3. `ap devportal build` produces `build/devportal.zip`.
4. `ap devportal rest-api publish` uploads it to the ACME org; the generated API
   ID is captured from the response and reused by later steps.
5. `ap devportal rest-api get` reads the API back.
6. `ap devportal api-key generate` issues a key for the API.
7. `ap devportal sub-plan publish` creates a new subscription plan
   (`resources/devportal/subscription-plan.yaml`).
8. `ap devportal subscription create` subscribes to the API on the `Gold` plan
   (the API carries the seeded `Gold`/`Bronze` policies).

## Configuration

Tests can be enabled/disabled by editing `test-config.yaml`:

```yaml
tests:
  gateway:
    manage:
      - id: GW-MANAGE-001
        name: gateway add with valid parameters
        enabled: true  # Set to false to skip
        requires: [CLI, GATEWAY]
```

## Test Structure

```
cli/it/
├── test-config.yaml      # Enable/disable tests
├── features/             # Gherkin feature files
│   ├── gateway/
│   └── devportal/
├── steps/                # Step definitions
├── resources/            # Test resources
│   ├── gateway/
│   └── devportal/
└── logs/                 # Test logs (git-ignored)
```

## Infrastructure Dependencies

| ID | Component | Description |
|----|-----------|-------------|
| `CLI` | CLI Binary | The `ap` binary built from `cli/src/` |
| `GATEWAY` | Gateway Stack | Docker Compose services: controller, router, policy-engine |
| `MCP_SERVER` | MCP Server | MCP backend for generate command tests |
| `DEVPORTAL` | Developer Portal | Postgres + developer-portal stack (port 3000) for devportal command tests |

## Logs

Each test writes to a separate log file in `logs/`:
- `logs/GW-MANAGE-001-gateway-add.log`
- `logs/GW-API-001-api-list.log`
- `logs/PHASE-1.log` (Phase 1 infrastructure setup)
- etc.

## Documentation

See [INTEGRATION-TESTS.md](INTEGRATION-TESTS.md) for detailed documentation.
