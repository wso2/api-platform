# CLI Integration Tests - Implementation Story

## Overview

This document outlines the implementation plan for CLI (`ap`) integration tests. The tests validate CLI commands against real gateway and MCP server infrastructure using a BDD (Behavior-Driven Development) approach with Godog/Cucumber.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         CLI Integration Test Suite                          │
│  ┌───────────────────┐  ┌───────────────────┐  ┌───────────────────────┐    │
│  │ Feature Files     │  │ Step Definitions  │  │ Test State            │    │
│  │ (Gherkin BDD)     │  │ (Go)              │  │ (CLI Output/Context)  │    │
│  └───────────────────┘  └───────────────────┘  └───────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
            ┌───────────┐   ┌─────────────┐   ┌──────────────┐
            │ CLI Binary│   │ Gateway     │   │ Log Files    │
            │ (ap)      │   │ Docker Stack│   │ (per test)   │
            └───────────┘   └─────────────┘   └──────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│              Docker Compose (gateway/it/docker-compose.test.yaml)           │
│  ┌───────────────────┐  ┌─────────────────┐  ┌──────────────────┐           │
│  │ gateway-controller│  │  router         │  │  policy-engine   │           │
│  │  :9090 (REST)     │  │   :8080 (HTTP)  │  │    :9002         │           │
│  │  :18000 (xDS)     │  │   :9901 (Admin) │  │                  │           │
│  └───────────────────┘  └─────────────────┘  └──────────────────┘           │
│                                                                             │
│  ┌───────────────────┐  ┌─────────────────┐                                 │
│  │ sample-backend    │  │ mcp-server      │                                 │
│  │     :9080         │  │    :3001        │                                 │
│  └───────────────────┘  └─────────────────┘                                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Folder Structure

```
cli/it/
├── go.mod
├── go.sum
├── Makefile                      # make test, make clean
├── README.md                     # Quick start guide
├── INTEGRATION-TESTS.md          # This document
├── test-config.yaml              # Enable/disable tests + dependencies
├── suite_test.go                 # Main Godog test entry point
├── setup.go                      # Docker lifecycle management
├── state.go                      # Test state management
├── reporter.go                   # Test reporting + logging
├── logs/                         # Git-ignored test logs
│   └── .gitignore
├── resources/
│   ├── values.go                 # Shared test values (endpoints, timeouts)
│   └── gateway/                  # Gateway-specific test resources
│       ├── sample-api.yaml       # API definition for testing
│       ├── sample-mcp-config.yaml # MCP config for testing
│       └── build.yaml             # Policy manifest for build tests
├── features/
│   └── gateway/                  # Gateway-related feature files
│       ├── management.feature    # add, list, remove, use, current, health
│       ├── api.feature           # api list, get, delete
│       ├── mcp.feature           # mcp list, get, delete, generate
│       ├── apply.feature         # apply command
│       └── build.feature         # build command
└── steps/
    ├── cli_steps.go              # CLI execution step definitions
    ├── infrastructure_steps.go   # Phase 1 infrastructure setup steps
    └── assert_steps.go           # Assertion step definitions
```

---

## Phase 1: Infrastructure Setup

Phase 1 establishes the required infrastructure before any tests run. All Phase 1 components must pass for Phase 2 to execute.

### Infrastructure Components

| ID | Component | Description | Port(s) | Health Check |
|----|-----------|-------------|---------|--------------|
| `CLI` | CLI Binary | Build `ap` binary from `cli/src/` | N/A | Binary exists and runs `--version` |
| `GATEWAY` | Gateway Stack | Docker Compose: controller, router, policy-engine, sample-backend | 9090, 8080, 9901, 9080 | HTTP GET `localhost:9090/health` |
| `MCP_SERVER` | MCP Server | Docker Compose: mcp-server-backend | 3001 | TCP connect `localhost:3001` |

### Infrastructure Startup Sequence

```
Phase 1: Infrastructure Setup
===============================================
[CLI]        Building CLI binary...
             Command: make build -C ../src
             ✓ PASS (2.3s)

[GATEWAY]    Starting gateway stack...
             Compose: ../../gateway/it/docker-compose.test.yaml
             Services: gateway-controller, router, policy-engine, sample-backend
             Waiting for health checks...
             ✓ PASS (45.2s)

[MCP_SERVER] Starting MCP server...
             Service: mcp-server-backend
             ✓ PASS (5.1s)

Phase 1 Complete: All infrastructure ready
===============================================
```

### Phase 1 Failure Handling

If any Phase 1 component fails:
1. Log detailed error to `logs/phase1-infrastructure.log`
2. Attempt cleanup of partially started services
3. Exit with non-zero code
4. Display clear error message to runner

---

## Phase 2: Test Execution

Phase 2 runs CLI command tests. Each test documents its Phase 1 dependencies, and only required infrastructure is started.

### Dependency Matrix

| Test ID | Test Name | Required Infrastructure |
|---------|-----------|------------------------|
| **Gateway Manage** |||
| GW-MANAGE-001 | gateway add (positive) | CLI, GATEWAY |
| GW-MANAGE-002 | gateway add (negative - missing name) | CLI |
| GW-MANAGE-003 | gateway add (negative - invalid URL) | CLI |
| GW-MANAGE-004 | gateway add (negative - duplicate) | CLI, GATEWAY |
| GW-MANAGE-005 | gateway list | CLI, GATEWAY |
| GW-MANAGE-006 | gateway list (empty) | CLI |
| GW-MANAGE-007 | gateway remove | CLI, GATEWAY |
| GW-MANAGE-008 | gateway remove (non-existent) | CLI |
| GW-MANAGE-009 | gateway use | CLI, GATEWAY |
| GW-MANAGE-010 | gateway use (non-existent) | CLI |
| GW-MANAGE-011 | gateway current | CLI, GATEWAY |
| GW-MANAGE-012 | gateway current (none set) | CLI |
| GW-MANAGE-013 | gateway health | CLI, GATEWAY |
| GW-MANAGE-014 | gateway health (unreachable) | CLI |
| **Gateway Apply** |||
| GW-APPLY-001 | apply valid API yaml | CLI, GATEWAY |
| GW-APPLY-002 | apply invalid yaml | CLI |
| GW-APPLY-003 | apply missing file | CLI |
| GW-APPLY-004 | apply valid MCP yaml | CLI, GATEWAY, MCP_SERVER |
| **Gateway API** |||
| GW-API-001 | api list | CLI, GATEWAY |
| GW-API-002 | api list (empty) | CLI, GATEWAY |
| GW-API-003 | api get | CLI, GATEWAY |
| GW-API-004 | api get (non-existent) | CLI, GATEWAY |
| GW-API-005 | api delete | CLI, GATEWAY |
| GW-API-006 | api delete (non-existent) | CLI, GATEWAY |
| **Gateway MCP** |||
| GW-MCP-001 | mcp generate (positive) | CLI, MCP_SERVER |
| GW-MCP-002 | mcp generate (invalid server) | CLI |
| GW-MCP-003 | mcp generate (unreachable server) | CLI |
| GW-MCP-004 | mcp list | CLI, GATEWAY |
| GW-MCP-005 | mcp list (empty) | CLI, GATEWAY |
| GW-MCP-006 | mcp get | CLI, GATEWAY |
| GW-MCP-007 | mcp get (non-existent) | CLI, GATEWAY |
| GW-MCP-008 | mcp delete | CLI, GATEWAY |
| GW-MCP-009 | mcp delete (non-existent) | CLI, GATEWAY |
| **Gateway Build** |||
| GW-BUILD-001 | build invalid manifest | CLI |
| GW-BUILD-002 | build missing file | CLI |

### Smart Dependency Resolution

When running tests, the framework:
1. Reads enabled tests from `test-config.yaml`
2. Collects all unique infrastructure dependencies
3. Starts only required infrastructure
4. Runs tests in dependency order

**Example**: If only `GW-MCP-001` (mcp generate) is enabled:
- Starts: CLI, MCP_SERVER
- Skips: GATEWAY (not needed)

---

## Test Configuration

### `test-config.yaml` Format

```yaml
# CLI Integration Test Configuration
# ===================================
# Enable/disable tests and view their dependencies.
# Set 'enabled: false' to skip specific tests.
# Phase 1 infrastructure starts automatically based on enabled test dependencies.

# Infrastructure configuration
infrastructure:
  compose_file: ../../gateway/it/docker-compose.test.yaml
  startup_timeout: 120s
  health_check_interval: 5s

# Test definitions with dependencies
tests:
  gateway:
    manage:
      - id: GW-MANAGE-001
        name: gateway add with valid parameters
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-002
        name: gateway add without name flag
        enabled: true
        requires: [CLI]
        
      - id: GW-MANAGE-003
        name: gateway add with invalid server URL
        enabled: true
        requires: [CLI]
        
      - id: GW-MANAGE-004
        name: gateway add duplicate name
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-005
        name: gateway list
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-006
        name: gateway list when empty
        enabled: true
        requires: [CLI]
        
      - id: GW-MANAGE-007
        name: gateway remove existing
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-008
        name: gateway remove non-existent
        enabled: true
        requires: [CLI]
        
      - id: GW-MANAGE-009
        name: gateway use existing
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-010
        name: gateway use non-existent
        enabled: true
        requires: [CLI]
        
      - id: GW-MANAGE-011
        name: gateway current
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-012
        name: gateway current when none set
        enabled: true
        requires: [CLI]
        
      - id: GW-MANAGE-013
        name: gateway health
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MANAGE-014
        name: gateway health unreachable
        enabled: true
        requires: [CLI]

    apply:
      - id: GW-APPLY-001
        name: apply valid API yaml
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-APPLY-002
        name: apply invalid yaml format
        enabled: true
        requires: [CLI]
        
      - id: GW-APPLY-003
        name: apply missing file
        enabled: true
        requires: [CLI]
        
      - id: GW-APPLY-004
        name: apply valid MCP yaml
        enabled: true
        requires: [CLI, GATEWAY, MCP_SERVER]

    api:
      - id: GW-API-001
        name: api list
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-API-002
        name: api list empty
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-API-003
        name: api get existing
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-API-004
        name: api get non-existent
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-API-005
        name: api delete existing
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-API-006
        name: api delete non-existent
        enabled: true
        requires: [CLI, GATEWAY]

    mcp:
      - id: GW-MCP-001
        name: mcp generate valid server
        enabled: true
        requires: [CLI, MCP_SERVER]
        
      - id: GW-MCP-002
        name: mcp generate invalid server URL
        enabled: true
        requires: [CLI]
        
      - id: GW-MCP-003
        name: mcp generate unreachable server
        enabled: true
        requires: [CLI]
        
      - id: GW-MCP-004
        name: mcp list
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MCP-005
        name: mcp list empty
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MCP-006
        name: mcp get existing
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MCP-007
        name: mcp get non-existent
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MCP-008
        name: mcp delete existing
        enabled: true
        requires: [CLI, GATEWAY]
        
      - id: GW-MCP-009
        name: mcp delete non-existent
        enabled: true
        requires: [CLI, GATEWAY]

    build:
      - id: GW-BUILD-001
        name: build invalid manifest
        enabled: true
        requires: [CLI]
        
      - id: GW-BUILD-002
        name: build missing manifest file
        enabled: true
        requires: [CLI]
```

---

## Log Management

### Log File Structure

Each test writes to a separate log file:

```
logs/
├── .gitignore                    # Ignore all logs
├── PHASE-1.log                   # Phase 1 startup logs
├── GW-MANAGE-001-gateway-add.log
├── GW-MANAGE-002-gateway-add-no-name.log
├── GW-API-001-api-list.log
├── GW-MCP-001-mcp-generate.log
├── GW-BUILD-001-build-manifest.log
└── ...
```

### Log Content

Each log file contains:
1. Test ID and name
2. Timestamp
3. CLI command executed
4. Full stdout/stderr output
5. Exit code
6. Docker container logs (if applicable)
7. Pass/Fail status with reason

### Example Log

```
===============================================
Test: GW-001 - gateway add with valid parameters
Timestamp: 2026-01-07T10:30:45Z
===============================================

CLI Command:
  ap gateway add --display-name test-gw --server http://localhost:9090

Environment:
  AP_CONFIG_DIR=/tmp/ap-test-123

Exit Code: 0

Stdout:
  Gateway 'test-gw' added successfully.
  Server: http://localhost:9090
  Authentication: none

Stderr:
  (empty)

Container Logs (gateway-controller):
  2026-01-07T10:30:45Z INFO  Received health check request
  2026-01-07T10:30:46Z INFO  New gateway registered: test-gw

Result: ✓ PASS
Duration: 1.23s
===============================================
```

---

## Implementation Order

### Story 1: Project Setup
- [ ] Create folder structure
- [ ] Create `go.mod` with dependencies (godog, testcontainers-go)
- [ ] Create `.gitignore` for logs
- [ ] Create `Makefile` with `test` and `clean` targets

### Story 2: Configuration Parser
- [ ] Implement `test-config.yaml` parser
- [ ] Implement dependency resolution logic
- [ ] Implement enabled/disabled test filtering

### Story 3: Infrastructure Management
- [ ] Implement `setup.go` - Docker Compose lifecycle
- [ ] Implement CLI binary build step
- [ ] Implement health check waiting
- [ ] Implement graceful shutdown and cleanup

### Story 4: Test Framework Core
- [ ] Implement `suite_test.go` - Godog test suite
- [ ] Implement `state.go` - test state management
- [ ] Implement `reporter.go` - logging and reporting
- [ ] Create `resources/values.go` - shared test values

### Story 5: Step Definitions
- [ ] Implement `steps/infrastructure_steps.go`
- [ ] Implement `steps/cli_steps.go`
- [ ] Implement `steps/assert_steps.go`

### Story 6: Gateway Management Tests
- [ ] Create `features/gateway/management.feature`
- [ ] Create `resources/gateway/` test resources
- [ ] Implement all GW-* test scenarios

### Story 7: Gateway Apply Tests
- [ ] Create `features/gateway/apply.feature`
- [ ] Create `resources/gateway/sample-api.yaml`
- [ ] Implement all APPLY-* test scenarios

### Story 8: Gateway API Tests
- [ ] Create `features/gateway/api.feature`
- [ ] Implement all API-* test scenarios

### Story 9: Gateway MCP Tests
- [ ] Create `features/gateway/mcp.feature`
- [ ] Create `resources/gateway/sample-mcp-config.yaml`
- [ ] Implement all MCP-* test scenarios

### Story 10: Gateway Build Tests
- [ ] Create `features/gateway/build.feature`
- [ ] Create `resources/gateway/build.yaml`
- [ ] Implement all BUILD-* test scenarios

### Story 11: Documentation
- [ ] Create `README.md` with quick start
- [ ] Add CI/CD examples for GitHub Actions and Jenkins

---

## Test Execution Output

### Console Output Format

```
===============================================
CLI Integration Tests
===============================================
Config: test-config.yaml
Enabled: 34/34 tests

Phase 1: Infrastructure Setup
-----------------------------------------------
[CLI]        Building CLI binary...              ✓ PASS  (2.3s)
[GATEWAY]    Starting gateway stack...           ✓ PASS  (45.2s)
[MCP_SERVER] Starting MCP server...              ✓ PASS  (5.1s)
-----------------------------------------------
Phase 1 Complete: 3/3 infrastructure ready

Phase 2: Test Execution
-----------------------------------------------
Gateway Manage:
  [GW-MANAGE-001] gateway add valid params       ✓ PASS  → logs/GW-MANAGE-001-gateway-add.log
  [GW-MANAGE-002] gateway add no name            ✓ PASS  → logs/GW-MANAGE-002-gateway-add-no-name.log
  [GW-MANAGE-003] gateway add invalid URL        ✓ PASS  → logs/GW-MANAGE-003-gateway-add-invalid-url.log
  ...

Gateway Apply:
  [GW-APPLY-001] apply valid API yaml            ✓ PASS  → logs/GW-APPLY-001-apply-api.log
  ...

Gateway API:
  [GW-API-001] api list                          ✓ PASS  → logs/GW-API-001-api-list.log
  ...

Gateway MCP:
  [GW-MCP-001] mcp generate valid                ✓ PASS  → logs/GW-MCP-001-mcp-generate.log
  ...

Gateway Build:
  [GW-BUILD-001] build invalid manifest          ✓ PASS  → logs/GW-BUILD-001-build-invalid-manifest.log
  ...

===============================================
Summary
===============================================
Total:    34 tests
Passed:   34
Failed:   0
Skipped:  0
Duration: 2m 15s

Logs: cli/it/logs/
===============================================
```

### Failure Output

```
Gateway Manage:
  [GW-MANAGE-001] gateway add valid params       ✗ FAIL  → logs/GW-MANAGE-001-gateway-add.log
           Expected exit code 0, got 1
           Error: connection refused

===============================================
Summary
===============================================
Total:    34 tests
Passed:   33
Failed:   1
Skipped:  0

Failed Tests:
  - GW-MANAGE-001: gateway add valid params
    Log: logs/GW-MANAGE-001-gateway-add.log
    Reason: Expected exit code 0, got 1

Duration: 2m 15s
Exit Code: 1
===============================================
```

---

## CI/CD Integration

### GitHub Actions

```yaml
name: CLI Integration Tests

on:
  push:
    paths:
      - 'cli/**'
      - 'gateway/**'
  pull_request:
    paths:
      - 'cli/**'
      - 'gateway/**'

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.1'
      
      - name: Run CLI Integration Tests
        run: |
          cd cli/it
          make test
      
      - name: Upload Test Logs
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: integration-test-logs
          path: cli/it/logs/
```

### Jenkins

```groovy
pipeline {
    agent any
    
    stages {
        stage('CLI Integration Tests') {
            steps {
                dir('cli/it') {
                    sh 'make test'
                }
            }
            post {
                always {
                    archiveArtifacts artifacts: 'cli/it/logs/**'
                }
            }
        }
    }
}
```

---

## Prerequisites

- Go 1.25.1 or later
- Docker and Docker Compose
- Network access to pull Docker images:
  - `ghcr.io/wso2/api-platform/gateway-controller-coverage:0.3.0-SNAPSHOT`
  - `ghcr.io/wso2/api-platform/policy-engine:0.3.0-SNAPSHOT`
  - `ghcr.io/wso2/api-platform/gateway-router:0.3.0-SNAPSHOT`
  - `renukafernando/request-info:latest`
  - `rakhitharr/mcp-everything:v3`

---

## Quick Start

```bash
# Run all integration tests
cd cli/it
make test

# Clean up containers and logs
make clean

# View test configuration
cat test-config.yaml

# Check logs for specific test
cat logs/GW-001-gateway-add.log
```
