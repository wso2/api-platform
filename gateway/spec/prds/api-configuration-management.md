# FR1: Dynamic API Configuration Management

## Overview

The Gateway-Controller accepts, validates, and persists API configurations in YAML/JSON format. Configurations use a composite key identity `{name}/{version}` and are stored in an embedded bbolt database with in-memory maps for fast runtime access.

## Requirements

### Configuration Format
- Accept both YAML and JSON formats
- Support `api-platform.wso2.com/v1` API specification schema
- Validate against OpenAPI-derived schema
- Parse complex nested structures (operations, upstream, policies)

### Validation
- Syntax validation (well-formed YAML/JSON)
- Schema validation (required fields present)
- Semantic validation (valid URLs, HTTP methods, paths)
- Return structured errors with field paths (e.g., `spec.operations[0].path`)

### Identity Management
- Composite key format: `{name}/{version}` (e.g., "PetStore/v1")
- Unique constraint on name/version combination
- Return 409 Conflict for duplicate keys

### Persistence
- Store in bbolt embedded key-value database
- ACID guarantees for configuration operations
- Atomic updates to in-memory maps + database
- Survive Gateway-Controller restarts

### In-Memory Maps
- Load all configurations from database on startup
- Maintain synchronized in-memory maps for fast access
- Primary data source for xDS snapshot generation
- Update atomically with database on configuration changes

## Success Criteria

- Parse and validate API configurations in <1 second
- Return detailed field-level errors for invalid configurations
- Prevent duplicate name/version combinations
- Configuration persists across restarts
- In-memory maps stay synchronized with database

## User Scenarios

**Scenario 1**: Deploy new API configuration
```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @petstore-v1.yaml

# Response:
{
  "status": "success",
  "message": "API configuration created successfully",
  "id": "PetStore/v1",
  "created_at": "2025-10-13T10:30:00Z"
}
```

**Scenario 2**: Validation error with field paths
```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @invalid-config.yaml

# Response:
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    },
    {
      "field": "spec.operations[0].method",
      "message": "Invalid HTTP method: INVALID"
    }
  ]
}
```

**Scenario 3**: Duplicate name/version
```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @petstore-v1.yaml

# Response:
{
  "status": "error",
  "message": "API configuration already exists",
  "error": "Conflict: API 'PetStore/v1' already deployed"
}
```

## Implementation Notes

- Use go-playground/validator/v10 for validation
- Custom validation for URLs, HTTP methods, path patterns
- bbolt bucket: `apis` with key `{name}/{version}`
- In-memory map: `map[string]*models.APIConfig`
- Audit log entry for each configuration operation
