# Policy Definitions

This directory contains policy definition files that are loaded at controller startup.

## Overview

Policy definitions are loaded from this directory when the Gateway Controller starts. Each policy definition file describes a reusable policy module that can be referenced in API configurations.

## File Format

Policy definitions can be provided in either JSON or YAML format.

### Required Fields

- `name` (string): Unique policy name
- `version` (string): Semantic version following the pattern `v\d+\.\d+\.\d+` (e.g., `v1.0.0`)
- `flows` (object): Flow configuration indicating what the policy needs from the engine

### Optional Fields

- `description` (string): Human-readable description of the policy
- `parametersSchema` (object): JSON Schema describing the parameters accepted by this policy

## Flow Configuration

The `flows` object can contain:

- `request`: Configuration for request flow
- `response`: Configuration for response flow
- Additional custom flow properties

Each flow configuration supports:

- `requireHeader` (boolean): Whether headers are required for this flow
- `requireBody` (boolean): Whether body content is required for this flow

## Example Policy (JSON)

```json
{
  "name": "APIKeyValidation",
  "version": "v1.0.0",
  "description": "Validates API keys from request headers",
  "flows": {
    "request": {
      "requireHeader": true,
      "requireBody": false
    }
  },
  "parametersSchema": {
    "type": "object",
    "properties": {
      "header": {
        "type": "string",
        "description": "The header name to extract the API key from"
      }
    }
  }
}
```

## Example Policy (YAML)

```yaml
name: JWTValidation
version: v1.0.0
description: Validates JWT tokens from request headers
flows:
  request:
    requireHeader: true
    requireBody: false
parametersSchema:
  type: object
  properties:
    issuer:
      type: string
      description: The expected issuer of the JWT token
```

## Configuration

By default, the controller looks for policy files in the `policies` directory. You can change this location by setting the `POLICY_DIRECTORY` environment variable:

```bash
export POLICY_DIRECTORY=/path/to/policies
```

## Validation

On startup, the controller will:

1. Scan the policy directory for `.json`, `.yaml`, and `.yml` files
2. Load and parse each file
3. Validate the policy definitions
4. Check for duplicate policy names/versions
5. Make the policies available for reference in API configurations

If any validation errors occur, the controller will fail to start and log the specific errors.

## Managing Policies

To add a new policy:
1. Create a new JSON or YAML file in this directory
2. Restart the Gateway Controller

To update a policy:
1. Modify the policy file (you may want to increment the version)
2. Restart the Gateway Controller

To remove a policy:
1. Delete the policy file
2. Restart the Gateway Controller

## Important Notes

- Policy definitions are loaded once at startup - changes require a restart
- Policy names and versions must be unique across all files
- Policy definitions are stored in memory only (no database persistence)
- Policies referenced in API configurations must exist at the time the API is created
