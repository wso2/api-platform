# ap - WSO2 API Platform CLI

`ap` is a command-line tool for managing and interacting with the WSO2 API Platform.

## Quick Start

```bash
# Build the CLI
cd src
make build

# Add a gateway
./build/ap gateway add -n dev -s http://localhost:9090

# Generate MCP configuration
./build/ap gateway mcp generate -s http://localhost:3001/mcp -o target

# Show version
./build/ap version
```

**💡 Tip:** All flags support first-letter shortcuts: `--name` → `-n`, `--server` → `-s`, `--token` → `-t`, etc.

## 📖 Full Documentation

For complete usage instructions, examples, and command reference, see **[src/HELP.md](src/HELP.md)**.

Quick command reference:
- `ap gateway add` - Add a gateway configuration
- `ap gateway mcp generate` - Generate MCP configuration
- `ap version` - Show version information

## Development

### Prerequisites

- Go 1.25.x or higher
- Make

### Building from Source

```bash
cd src

# Build for your OS (runs ALL tests first, shows summary table)
# Tests always run to completion - build continues even if tests fail
make build

# Build without running tests (faster for development)
make build-skip-tests

# Build for all platforms (Linux, MacOS, Windows - amd64 & arm64)
# Runs tests first
make build-all

# Build all platforms without running tests
make build-all-skip-tests

# Run tests only
make test

# Clean build artifacts
make clean
```

**Note:** `make build` and `make build-all` run all tests to completion and display a summary table:
```
===============================================
Test Results Summary
===============================================
Test Name                                | Status
-----------------------------------------+---------
TestFlagValuesUnique                     | ✓ PASS
TestGatewayBuildCommand                  | ✓ PASS
===============================================
ℹ Full test logs: tests/logs/test-results.log
```

Detailed logs are always saved to `tests/logs/test-results.log` for debugging.

### Testing

The CLI includes automated tests that can be configured via `tests/test-config.yaml`:

```bash
# Run all tests
make test

# Configure tests
# Edit tests/test-config.yaml to enable/disable specific tests
# Set 'enabled: false' to skip a test

# Test resources
# Test manifests and files are located in tests/resources/
```

**Test Configuration Example:**
```yaml
tests:
  - name: TestGatewayBuildCommand
    description: Test gateway build command with policy manifest
    enabled: true  # Set to false to skip this test
```

### Project Structure

```
cli/
├── README.md              # This file
└── src/
    ├── HELP.md           # Complete CLI documentation (single source of truth)
    ├── cmd/              # Command definitions
    │   ├── root.go       # Root command
    │   └── gateway/      # Gateway subcommands
    ├── internal/         # Internal packages
    │   ├── config/       # Config management (~/.ap/config.yaml)
    │   ├── gateway/      # Gateway client
    │   └── mcp/          # MCP generator
    ├── utils/            # Shared utilities
    └── main.go           # Entry point
```
