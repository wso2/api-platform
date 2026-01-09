# CLI Integration Tests

End-to-end integration tests for the `ap` CLI, validating commands against real gateway and MCP server infrastructure.

## Prerequisites

- Go 1.25.1 or later
- Docker and Docker Compose
- Network access to pull Docker images

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
    management:
      - id: GW-001
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
- `logs/GW-001-gateway-add.log`
- `logs/API-001-api-list.log`
- etc.

## Documentation

See [INTEGRATION-TESTS.md](INTEGRATION-TESTS.md) for detailed documentation.
