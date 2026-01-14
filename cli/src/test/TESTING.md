# CLI Testing Guide

## Overview

The CLI includes an automated test suite with configurable test execution. Tests can be enabled/disabled via a YAML configuration file.

## Test Structure

```
tests/
├── test-config.yaml              # Test configuration (enable/disable tests)
├── flags_test.go                 # Flag uniqueness tests
├── gateway_build_test.go         # Gateway build command tests
└── resources/
    └── test-policy-manifest.yaml # Test policy manifest file
```

## Running Tests

```bash
# Run all enabled tests
make test

# Build without running tests (faster for development)
make build-skip-tests

# Build with tests (default)
make build
```

## Test Configuration

Tests can be enabled or disabled by editing `tests/test-config.yaml`:

```yaml
tests:
  - name: TestFlagValuesUnique
    description: Verify all CLI flags have unique values
    enabled: true
  
  - name: TestGatewayBuildCommand
    description: Test gateway build command with policy manifest
    enabled: true  # Set to false to skip this test
```

### Disabling a Test

To temporarily disable a test during development:

```yaml
  - name: TestGatewayBuildCommand
    description: Test gateway build command with policy manifest
    enabled: false  # ← Changed to false
```

Or simply comment it out:

```yaml
  # - name: TestGatewayBuildCommand
  #   description: Test gateway build command with policy manifest
  #   enabled: true
```

## Adding Test Resources

Test manifests and files should be placed in `tests/resources/`:

```bash
# Add a new test manifest
tests/resources/
├── test-policy-manifest.yaml        # Current test manifest
├── test-manifest-invalid.yaml       # For validation error tests (future)
└── test-manifest-local-policy.yaml  # For local policy tests (future)
```

## Writing New Tests

1. **Add test function** to appropriate `*_test.go` file
2. **Register in config**: Add entry to `tests/test-config.yaml`
3. **Check enabled status**: Use `isTestEnabled("TestName")` helper
4. **Add resources**: Place test files in `tests/resources/`

Example:

```go
func TestMyNewCommand(t *testing.T) {
    if !isTestEnabled("TestMyNewCommand") {
        t.Skip("Test disabled in test-config.yaml")
        return
    }
    
    // Test implementation...
}
```

Then add to `test-config.yaml`:

```yaml
  - name: TestMyNewCommand
    description: Description of what this tests
    enabled: true
```

## Test Logs

When you run `make build`, all test output is saved to `tests/logs/test-results.log`. This file contains:
- Full test execution details
- Test output and logging statements
- Stack traces for failures
- Timing information

**Location:** `cli/src/tests/logs/test-results.log`

If tests fail, the build process will display:
```
⚠ Some tests failed. Check logs at: tests/logs/test-results.log
```

The `tests/logs/` directory is gitignored, so logs won't be committed to the repository.

## CI/CD Integration

For CI pipelines:

```bash
# Always run tests in CI
make build

# Or explicitly run tests first
make test && make build-skip-tests

# Check test logs in CI
cat tests/logs/test-results.log
```

Set `enabled: false` in `test-config.yaml` for tests that require external services not available in CI.
