# CLI Integration Tests

End-to-end integration tests for the `ap` CLI, validating commands against real gateway and MCP server infrastructure.

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
│   └── gateway/
├── steps/                # Step definitions
├── resources/            # Test resources
│   └── gateway/
└── logs/                 # Test logs (git-ignored)
```

## Infrastructure Dependencies

| ID | Component | Description |
|----|-----------|-------------|
| `CLI` | CLI Binary | The `ap` binary built from `cli/src/` |
| `GATEWAY` | Gateway Stack | Docker Compose services: controller, router, policy-engine |
| `MCP_SERVER` | MCP Server | MCP backend for generate command tests |

## Logs

Each test writes to a separate log file in `logs/`:
- `logs/GW-MANAGE-001-gateway-add.log`
- `logs/GW-API-001-api-list.log`
- `logs/PHASE-1.log` (Phase 1 infrastructure setup)
- etc.

## Documentation

See [INTEGRATION-TESTS.md](INTEGRATION-TESTS.md) for detailed documentation.
