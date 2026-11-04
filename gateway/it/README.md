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
| `make test` | Run integration tests (excludes `@load` and `@faultflow`) |
| `make test-load` | Run **only** the `@load` scenarios — local only, not run by CI |
| `make test-faultflow` | Run the failover features against a stack with the fault flow enabled |
| `make test-postgres` | Run integration tests with Postgres backend |
| `make test-all` | Build coverage images + run tests |
| `make test-verbose` | Run tests with verbose output |
| `make build-coverage` | Build coverage-instrumented images |
| `make clean` | Remove containers, volumes, and artifacts |
| `make check-docker` | Verify Docker is available |
| `make coverage-report` | Open coverage HTML report in browser |

## Load Scenarios

Scenarios tagged `@load` are **excluded from every ordinary run and from CI**. They drive tens of
thousands of requests and take minutes; in CI they would dominate the suite's runtime and turn an
unrelated failure into a long wait.

```bash
make test-load        # run ONLY the @load scenarios
IT_TAGS= make test    # run everything, load scenarios included
IT_TAGS=@model-failover make test   # any godog tag expression
```

The filter lives in `suite_test.go` (`getTags`), defaulting to `~@load`. `IT_TAGS` overrides it,
including with an empty value.

Note that the tag filter only selects **within** the feature files godog loads, so `IT_TAGS=@load`
on its own runs nothing and still reports `PASS` — there is no file in the default list carrying
that tag. `make test-load` therefore sets `IT_FEATURE_PATHS` as well; override `LOAD_FEATURES` to
point it elsewhere.

Run `make test-load` locally before changing anything that affects how often a target is attempted,
or what an attempt costs — those are the properties a functional scenario cannot catch, because they
only show up over a large number of requests.

## Fault-flow Scenarios

```bash
make test-faultflow   # failover features, with the fault flow handling upstream failures
```

`policy_engine.fault_policies.handle_upstream_faults` is a process-level setting with no env
override and no per-API form, so exercising it means starting a differently-configured stack
rather than selecting a tag. `docker-compose.test.faultflow.override.yaml` layers onto the base
compose file, and `test-config.faultflow.toml` is a one-stanza FRAGMENT layered onto the base
config — the policy engine accepts `-config` more than once and merges last-wins, and the
entrypoint forwards every `--pol.*` argument in order. Nothing duplicates the base, so nothing can
drift from it; prefer this shape over the full-copy `vhosts`/`postgres` variants when adding a new
posture.

With the flag on, a failing response goes through the fault chain *instead of* the response flow.
A policy that only attaches to the response flow therefore stops running on exactly the responses
it cares about — not degraded, inert — while every scenario in the default run still passes. The
failure mode is silence, which is why the `@faultflow` scenarios assert the engine actually
diverted the response *before* asserting anything else: a run whose overlay stopped applying would
otherwise be indistinguishable from an ordinary green one.

`@faultflow` is excluded from every other target. In an ordinary run those scenarios would not
merely be irrelevant — they would fail, because they assert a posture the default stack does not
have.

## Running Tests

```bash
# Run all tests
make test-all

# Run tests with Postgres-backed gateway-controller
make test-postgres

# Run tests with an explicit compose file
COMPOSE_FILE=docker-compose.test.postgres.yaml make test

# Run with Go directly (with extended timeout for coverage builds)
COMPOSE_FILE=docker-compose.test.yaml go test -v -timeout 30m ./...

# Run specific scenario (use @tag)
COMPOSE_FILE=docker-compose.test.postgres.yaml go test -v -timeout 30m ./... -godog.tags="@wip"
```

**Note:** The default Go test timeout is 10 minutes. The full integration test suite with coverage instrumentation typically takes longer to complete, so a 30-minute timeout is recommended.
**Note:** The Postgres test compose keeps the database internal to the Docker network and does not expose a host port.

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
      apiVersion: gateway.api-platform.wso2.com/v1
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
