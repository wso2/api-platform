# Integration Tests

This directory is for integration tests that test the CLI with real gateway instances and external dependencies.

## Running Integration Tests

```bash
# From cli/src directory
go test -v ../test/integration/...
```

## Test Structure

Integration tests should:
- Test end-to-end workflows
- Require actual gateway instances or external services
- Take longer to run than unit tests
- Be placed in appropriately named subdirectories

## Example

```
integration/
  gateway/
    api_lifecycle_test.go
  mcp/
    proxy_operations_test.go
```
