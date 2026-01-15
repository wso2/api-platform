# Contributing to Gateway Integration Tests

This guide explains how to add new integration tests to the gateway test suite.

## Quick Start

1. Create a feature file in `features/`
2. Implement step definitions
3. Register steps in `suite_test.go`
4. Run tests with `make test`

## Project Structure

```
gateway/it/
├── features/           # Gherkin feature files
│   └── health.feature  # Example: health check scenarios
├── steps/              # Reusable step definition packages
│   ├── http_steps.go   # Common HTTP request steps
│   └── assert_steps.go # Common assertion steps
├── steps_health.go     # Feature-specific step definitions
├── state.go            # Test state management
├── setup.go            # Docker Compose lifecycle
├── suite_test.go       # Main test entry point
└── docker-compose.test.yaml
```

## Writing Feature Files

Feature files use Gherkin syntax. Place them in `features/`.

### Example Feature

```gherkin
Feature: API Authentication
  As an API consumer
  I want to authenticate with the gateway
  So that I can access protected resources

  Background:
    Given the gateway services are running

  Scenario: Valid API key is accepted
    Given I set header "X-API-Key" to "valid-key"
    When I send a GET request to "http://localhost:8080/api/resource"
    Then the response status code should be 200

  Scenario: Invalid API key is rejected
    Given I set header "X-API-Key" to "invalid-key"
    When I send a GET request to "http://localhost:8080/api/resource"
    Then the response status code should be 401
    And the response body should contain "unauthorized"
```

## Available Step Definitions

### HTTP Request Steps (from `steps/http_steps.go`)

```gherkin
# Headers
Given I set header "Content-Type" to "application/json"
Given I clear all headers

# GET requests
When I send a GET request to "http://localhost:9090/health"
When I send a GET request to the "gateway-controller" service at "/health"

# POST requests
When I send a POST request to "http://localhost:9090/api"
When I send a POST request to "http://localhost:9090/api" with body:
  """
  {"name": "test"}
  """

# PUT/PATCH/DELETE
When I send a PUT request to "http://localhost:9090/api/1" with body:
  """
  {"name": "updated"}
  """
When I send a DELETE request to "http://localhost:9090/api/1"
```

### Assertion Steps (from `steps/assert_steps.go`)

```gherkin
# Status code
Then the response status code should be 200
Then the response should be successful
Then the response should be a client error
Then the response should be a server error

# Headers
Then the response header "Content-Type" should be "application/json"
Then the response header "X-Custom" should contain "value"
Then the response header "X-Custom" should exist

# Body
Then the response body should contain "success"
Then the response body should not contain "error"
Then the response body should be empty
Then the response body should match pattern "id.*\d+"

# JSON
Then the response should be valid JSON
Then the JSON response should have field "status"
Then the JSON response field "status" should be "ok"
Then the JSON response field "count" should be 5
Then the JSON response field "active" should be true
Then the JSON response should have 3 items
```

## Adding New Steps

### 1. Create Step Definition File

For feature-specific steps, create a file like `steps_myfeature.go`:

```go
package it

import (
    "fmt"
    "github.com/cucumber/godog"
)

func RegisterMyFeatureSteps(ctx *godog.ScenarioContext, state *TestState) {
    ctx.Step(`^I configure the API with name "([^"]*)"$`, state.configureAPI)
    ctx.Step(`^the API "([^"]*)" should be deployed$`, state.apiShouldBeDeployed)
}

func (s *TestState) configureAPI(name string) error {
    // Implementation
    s.SetContextValue("api_name", name)
    return nil
}

func (s *TestState) apiShouldBeDeployed(name string) error {
    // Verification logic
    return nil
}
```

### 2. Register Steps

Add registration in `suite_test.go`:

```go
func InitializeScenario(ctx *godog.ScenarioContext) {
    // ... existing code ...

    if testState != nil {
        RegisterHealthSteps(ctx, testState)
        RegisterMyFeatureSteps(ctx, testState)  // Add this
    }
}
```

### 3. Use Shared State

Access shared state via `TestState`:

```go
// Store values
s.SetContextValue("key", value)

// Retrieve values
val, ok := s.GetContextValue("key")
str, ok := s.GetContextString("key")
num, ok := s.GetContextInt("key")

// Access last HTTP response
s.LastResponse.StatusCode
s.LastResponse.Body
```

## Using Common Steps Package

For reusable HTTP and assertion steps:

```go
import "github.com/wso2/api-platform/gateway/it/steps"

func InitializeScenario(ctx *godog.ScenarioContext) {
    httpSteps := steps.NewHTTPSteps(testState.HTTPClient, map[string]string{
        "gateway-controller": "http://localhost:9090",
        "router":             "http://localhost:8080",
    })
    httpSteps.Register(ctx)

    assertSteps := steps.NewAssertSteps(httpSteps)
    assertSteps.Register(ctx)

    ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
        httpSteps.Reset()
        return ctx, nil
    })
}
```

## Running Tests

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose

# Clean up containers
make clean

# Check Docker availability
make check-docker
```

## Best Practices

1. **One scenario per behavior** - Each scenario should test one specific behavior
2. **Use Background for setup** - Common setup steps go in Background
3. **Descriptive step names** - Steps should read like natural language
4. **Reuse common steps** - Use `steps/` package for common HTTP/assertion patterns
5. **Clean state** - State is reset between scenarios automatically
6. **Avoid hardcoded values** - Use configuration or context for dynamic values

## Debugging

### View container logs

```bash
docker compose -f docker-compose.test.yaml -p gateway-it logs -f
```

### Check service health

```bash
curl http://localhost:9090/health
curl http://localhost:9901/ready
```

### Run single scenario

```bash
go test -v ./... -godog.tags="@wip"
```

Add `@wip` tag to your scenario:

```gherkin
@wip
Scenario: My scenario under development
  ...
```

## Troubleshooting Colima

If you are using Colima and encounter a "root docker not found" error, set the following environment variables:

```bash
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
export DOCKER_HOST=unix://${HOME}/.colima/default/docker.sock
```

