# Gateway Integration Tests

End-to-end integration tests for the API Gateway, validating API deployment, routing, policy enforcement, and service health.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Test Suite (Godog)                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐    │
│  │ Feature Files │  │ Step Defs     │  │ Test State            │    │
│  │ (Gherkin)     │  │ (Go)          │  │ (HTTP Client/Context) │    │
│  └───────────────┘  └───────────────┘  └───────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Docker Compose Environment                     │
│  ┌───────────────────┐  ┌─────────────────┐  ┌──────────────────┐   │
│  │ gateway-controller│  │  router         │  │  policy-engine   │   │
│  │  :9090 (REST)     │  │   :8080 (HTTP)  │  │    :9002         │   │
│  │  :18000 (xDS)     │  │   :9901 (Admin) │  │                  │   │
│  └───────────────────┘  └─────────────────┘  └──────────────────┘   │
│                              │                                      │
│                              ▼                                      │
│                      ┌────────────────┐                             │
│                      │ sample-backend │                             │
│                      │     :9080      │                             │
│                      └────────────────┘                             │
└─────────────────────────────────────────────────────────────────────┘
```

**Components:**
- **Godog** - BDD test framework (Go implementation of Cucumber)
- **Docker Compose** - Orchestrates gateway services for testing
- **Coverage Collector** - Gathers code coverage from instrumented binaries
- **Test Reporter** - Generates JSON/formatted test reports

## Prerequisites

- Go 1.25.1 or later
- Docker and Docker Compose
- Built `gateway-controller:coverage` image

## Quick Start

```bash
# Build coverage-instrumented image and run tests
make test-all

# Or run separately:
make build-coverage-image
make test
```

## Project Structure

```
gateway/it/
├── features/               # Gherkin feature files (.feature)
│   ├── health.feature      # Health check scenarios
│   └── api_deploy.feature  # API deployment scenarios
├── steps/                  # Reusable step definitions
│   ├── http_steps.go       # HTTP request steps
│   └── assert_steps.go     # Response assertion steps
├── steps_health.go         # Health-specific steps
├── steps_api.go            # API deployment steps
├── state.go                # Test state management
├── setup.go                # Docker Compose lifecycle
├── coverage.go             # Coverage collection
├── reporter.go             # Test reporting
├── suite_test.go           # Main test entry point
├── docker-compose.test.yaml
├── Makefile
├── CONTRIBUTING.md         # Guide for writing new tests
└── README.md
```

## Make Commands

| Command | Description |
|---------|-------------|
| `make test` | Run integration tests |
| `make test-all` | Build coverage images + run tests |
| `make test-verbose` | Run tests with verbose output |
| `make build-coverage` | Build coverage-instrumented images |
| `make clean` | Remove containers, volumes, and artifacts |
| `make check-docker` | Verify Docker is available |
| `make coverage-report` | Open coverage HTML report in browser |

## Running Tests

```bash
# Run all tests
make test-all

# Run with Go directly (with extended timeout for coverage builds)
go test -v -timeout 30m ./...

# Run specific scenario (use @tag)
go test -v -timeout 30m ./... -godog.tags="@wip"
```

**Note:** The default Go test timeout is 10 minutes. The full integration test suite with coverage instrumentation typically takes longer to complete, so a 30-minute timeout is recommended.

## Test Reports

After running tests, reports are available at:

| Report | Location |
|--------|----------|
| Test Results (JSON) | `reports/integration-test-results.json` |
| Coverage Summary | `coverage/integration-test-coverage.txt` |
| Coverage HTML | `coverage/integration-test-coverage.html` |
| Coverage JSON | `coverage/integration-test-coverage-report.json` |

## Example Test Scenario

```gherkin
Feature: API Deployment and Invocation

  Background:
    Given the gateway services are running

  Scenario: Deploy a simple HTTP API and invoke it successfully
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: weather-api-v1.0
      spec:
        displayName: Weather-API
        version: v1.0
        context: /weather/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
      """
    Then the response should be successful
    And I wait for 2 seconds
    When I send a GET request to "http://localhost:8080/weather/v1.0/us/seattle"
    Then the response should be successful
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed instructions on:
- Writing feature files
- Available step definitions
- Adding new steps
- Best practices
