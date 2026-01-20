# MCP Authorization

## Overview

The MCP Authorization policy provides fine-grained access control for Model Context Protocol (MCP) server resources. It enables API administrators to define authorization rules based on user claims and scopes extracted from validated JWT tokens, controlling access to specific MCP tools, resources, prompts, and JSON-RPC methods.

> **Prerequisite**: The MCP Authorization policy requires the [MCP Authentication policy](./mcp-authentication.md) to be applied first. The MCP Authentication policy validates and extracts JWT claims that are used by the authorization policy for access control decisions.

## Features

- **Tool-Level Access Control**: Restrict access to specific MCP tools based on user claims and scopes
- **Resource-Level Access Control**: Control access to specific MCP resources based on authorization rules
- **Prompt-Level Access Control**: Manage access to specific MCP prompts
- **JSON-RPC Method-Level Access Control**: Apply authorization rules at the JSON-RPC method level (e.g., `tools/call`, `resources/read`, `prompts/get`) for fine-grained control. Only methods under `tools/`, `resources/`, and `prompts/` are evaluated.
- **Flexible Rule-Based Authorization**: Define multiple authorization rules with attribute matching (exact or wildcard)
- **Claim-Based Validation**: Validate custom claims (e.g., department, role, team) in user tokens
- **Scope-Based Validation**: Require specific OAuth scopes for accessing protected resources
- **Wildcard Matching**: Use wildcard patterns ("*") to create default rules for all resources of a type

## Configuration

The MCP Authorization policy uses a single-level configuration model where all parameters are configured per-MCP-API/route in the API definition YAML.

### User Parameters (API Definition)

These parameters are configured per MCP Proxy by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `rules` | array | Yes | - | List of authorization rules that define access control policies for MCP resources. |
| `rules[].attribute` | object | Yes | - | The MCP resource attribute to which the authorization rule applies. |
| `rules[].attribute.type` | string | Yes | - | Type of MCP resource: "tool", "resource", "prompt", "method". |
| `rules[].attribute.name` | string | No | - | Name or identifier of the resource. Use "*" for wildcard matching (applies to all resources of the specified type). Examples: "list_files" for tools, "file:///some_resource" for resources, "weather_summary" for prompts, "tools/call" for methods. |
| `rules[].requiredScopes` | array | No | - | List of OAuth scopes required to access this resource. The token must contain all of the specified scopes. |
| `rules[].requiredClaims` | object | No | - | Map of claim names to expected values. All specified claims must be present in the token with matching values. |

## MCP Proxy Definition Examples

### Example 1: Basic Tool Access Control

Restrict access to specific tools based on scopes:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  vhost: mcp1.gw.example.com
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
    - name: mcp-authz
      version: v0.1.0
      params:
        rules:
          - attribute:
              type: tool
              name: list_files
            requiredScopes:
              - mcp:tool:read
          - attribute:
              type: tool
              name: create_file
            requiredScopes:
              - mcp:tool:write
          - attribute:
              type: tool
              name: "*"
            requiredScopes:
              - mcp:tool:execute
  tools:
    ...
```

### Example 2: Claim-Based Resource Access

Control resource access based on user claims:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  vhost: mcp1.gw.example.com
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
    - name: mcp-authz
      version: v0.1.0
      params:
        rules:
          - attribute:
              type: resource
              name: "file:///private/main"
            requiredClaims:
              department: "engineering"
            requiredScopes:
              - mcp:resource:read
          - attribute:
              type: resource
              name: "file:///public/main"
            requiredScopes:
              - mcp:resource:read
  tools:
    ...
```

### Example 3: Role-Based Prompt Access

Restrict prompt access based on user roles:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  vhost: mcp1.gw.example.com
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
    - name: mcp-authz
      version: v0.1.0
      params:
        rules:
          - attribute:
              type: prompt
              name: "admin_summary"
            requiredClaims:
              role: "admin"
            requiredScopes:
              - mcp:prompt:admin
          - attribute:
              type: prompt
              name: "*"
            requiredScopes:
              - mcp:prompt:read
  tools:
    ...
```

### Example 4: Multi-Level Authorization

Combine different resource types with varying access requirements:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  vhost: mcp1.gw.example.com
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
        requiredScopes:
          - mcp:access
    - name: mcp-authz
      version: v0.1.0
      params:
        rules:
          # Restrictive tool access
          - attribute:
              type: tool
              name: "execute_command"
            requiredClaims:
              department: "platform"
              role: "admin"
            requiredScopes:
              - mcp:tool:execute:admin
          # General tool access
          - attribute:
              type: tool
              name: "*"
            requiredScopes:
              - mcp:tool:execute
          # Resource access for finance department
          - attribute:
              type: resource
              name: "file:///finance/*"
            requiredClaims:
              department: "finance"
            requiredScopes:
              - mcp:resource:read:finance
          # Public resources
          - attribute:
              type: resource
              name: "*"
            requiredScopes:
              - mcp:resource:read
  tools:
    ...
```

## Authorization Decision Examples

**Scenario 1**: User with scope `mcp:tool:read` attempts to call `list_files` tool
- Rule: `attribute.type="tool", attribute.name="list_files", requiredScopes=["mcp:tool:read"]`
- Result: ✅ Access Granted

**Scenario 2**: User with scope `mcp:tool:execute` (no write scope) attempts to call `create_file` tool
- Rule: `attribute.type="tool", attribute.name="create_file", requiredScopes=["mcp:tool:write"]`
- Result: ❌ Access Denied (insufficient scopes)

**Scenario 3**: User with claim `department="engineering"` attempts to read resource `file:///private/code`
- Rule: `attribute.type="resource", attribute.name="file:///private/code", requiredClaims={department="engineering"}`
- Result: ✅ Access Granted

**Scenario 4**: User with claim `department="finance"` (no engineering) attempts to read resource `file:///private/code`
- Rule: `attribute.type="resource", attribute.name="file:///private/code", requiredClaims={department="engineering"}`
- Result: ❌ Access Denied (claim mismatch)

## Error Handling

When authorization fails, the policy returns:
- **HTTP Status**: `403 Forbidden`
- **Response Body**: JSON error response with a reason message
- **WWW-Authenticate Header**: Contains information about required scopes for the denied resource

## Related Policies

- [MCP Authentication Policy](./mcp-authentication.md) - Validates JWT tokens and is a prerequisite for MCP Authorization
- [JWT Authentication Policy](../../policies/jwt-authentication.md) - Base JWT token validation mechanism
