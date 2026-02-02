# MCP ACL List

## Overview

The MCP ACL List policy provides access control for Model Context Protocol (MCP) tools, resources, and prompts using an allow/deny mode with exceptions. This policy filters list responses and enforces access rules on request paths based on configured mode and exceptions. Unlike the [MCP Rewrite policy](./mcp-rewrite.md), this policy does not rewrite capability names or modify list entry contents—it purely controls visibility and access.

The policy operates on three types of MCP capabilities: tools, resources, and prompts. For each type, you can specify a mode (allow or deny) and a list of exceptions. Requests for capabilities not matching the access control rules are rejected with an appropriate error.

## Features

- **Tool-Level Access Control**: Allow or deny access to specific tools using allow/deny mode with exceptions.
- **Resource-Level Access Control**: Control access to specific resources (identified by URI) using flexible ACL rules.
- **Prompt-Level Access Control**: Manage access to specific prompts using configurable access modes.
- **Flexible ACL Modes**: Support both allow-with-exceptions and deny-with-exceptions patterns.
- **List Filtering**: Filter list responses to only include capabilities that match the access control rules.
- **Request Path Enforcement**: Enforce the same allow/deny rules on request paths, rejecting access to denied capabilities.

## Configuration

The MCP ACL List policy uses a single-level configuration model where all parameters are configured per-MCP-API/route in the API definition YAML.

### User Parameters (API Definition)

These parameters are configured per MCP Proxy by the API developer:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tools` | object | No | ACL configuration for tools with `mode` (allow or deny) and optional `exceptions` list. |
| `tools.mode` | string | Yes | ACL mode for tools: "allow" (allow all except exceptions) or "deny" (deny all except exceptions). |
| `tools.exceptions` | array | No | List of tool names that are exceptions to the mode (1-256 characters each). |
| `resources` | object | No | ACL configuration for resources with `mode` (allow or deny) and optional `exceptions` list. |
| `resources.mode` | string | Yes | ACL mode for resources: "allow" (allow all except exceptions) or "deny" (deny all except exceptions). |
| `resources.exceptions` | array | No | List of resource URIs that are exceptions to the mode (1-2048 characters each). |
| `prompts` | object | No | ACL configuration for prompts with `mode` (allow or deny) and optional `exceptions` list. |
| `prompts.mode` | string | Yes | ACL mode for prompts: "allow" (allow all except exceptions) or "deny" (deny all except exceptions). |
| `prompts.exceptions` | array | No | List of prompt names that are exceptions to the mode (1-256 characters each). |

## Access Control Logic

For each capability type (tools, resources, prompts):

- **Missing capability config**: All capabilities of that type are allowed (no restrictions).
- **mode: allow, exceptions: [...]**: Allow all capabilities except those listed in exceptions.
- **mode: deny, exceptions: [...]**: Deny all capabilities except those listed in exceptions.

## MCP Proxy Definition Examples

### Example 1: Deny Specific Tools

Deny access to certain tools while allowing all others:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-acl-list
      version: v0.1.0
      params:
        tools:
          mode: allow
          exceptions:
            - delete-all
            - drop-database
  tools:
    ...
```

### Example 2: Allow Only Specific Resources

Allow access to only whitelisted resources:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-acl-list
      version: v0.1.0
      params:
        resources:
          mode: deny
          exceptions:
            - file:///public/documents
            - file:///public/images
  resources:
    ...
```

### Example 3: Mixed Access Control

Apply different access control rules to different capability types:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: mcp-server-api-v1.0
spec:
  displayName: mcp-server-api
  version: v1.0
  context: /mcpserver
  upstream:
    url: https://mcp-backend:8080
  policies:
    - name: mcp-acl-list
      version: v0.1.0
      params:
        tools:
          mode: allow
          exceptions:
            - admin-only-tool
            - deprecated-tool
        resources:
          mode: allow
          exceptions:
            - file:///internal-resources
        prompts:
          mode: deny
          exceptions:
            - standard-prompt
            - approved-prompt
  tools:
    ...
```

## Use Cases

1. **Sensitive Operation Blocking**: Deny access to tools or resources that perform sensitive operations (e.g., delete, modify system configuration).
2. **Public API Restriction**: Allow only specific public resources while denying access to internal resources.
3. **Role-Based Access**: Combine this policy with authentication/authorization policies to implement role-based access control.
4. **Gradual Feature Rollout**: Deny access to beta or experimental tools while they are being tested.
5. **Compliance and Security**: Enforce compliance policies by denying access to resources or tools that are not approved for a specific tenant or environment.
6. **Cost Control**: Deny access to expensive or resource-intensive operations.

## Comparison with MCP Rewrite Policy

| Aspect | MCP ACL List | MCP Rewrite |
|--------|--------------|-------------|
| **Primary Purpose** | Access control via allow/deny | Capability name mapping |
| **Rewrites Names** | No | Yes |
| **Filters Lists** | Yes | Yes |
| **Enforces Request Paths** | Yes | Yes |
| **Configuration Complexity** | Simple (mode + exceptions) | Detailed (names, descriptions, targets) |
| **Metadata Modification** | No | Yes |

Both policies can be used together: use MCP ACL List for access control and MCP Rewrite for name mapping.
