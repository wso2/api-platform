---
title: "Overview"
---
# MCP Access Control List

## Overview

The MCP Access Control Logic(***ACL***) List policy provides access control for Model Context Protocol (***MCP***) tools, resources, and prompts using an allow/deny mode with exceptions. This policy filters list responses and enforces access rules on request paths based on configured mode and exceptions. Unlike the [MCP Rewrite policy](./mcp-rewrite.md), this policy does not rewrite capability names or modify list entry contents -- it purely controls visibility and access.

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

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `tools` | `ToolACLConfig` object | Conditional | - | ACL configuration for tools. |
| `resources` | `ResourceACLConfig` object | Conditional | - | ACL configuration for resources. |
| `prompts` | `PromptACLConfig` object | Conditional | - | ACL configuration for prompts. |

> **Note**: At least one of `tools`, `resources`, or `prompts` must be specified.

### ToolACLConfig Configuration

Each `ToolACLConfig` object supports the following fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `mode` | string | No | `"deny"` | ACL mode for tools: `allow` (allow all except listed exceptions) or `deny` (deny all except listed exceptions). |
| `exceptions` | string array | No | - | List of tool names that are exceptions to the selected mode. Tool names must be 1-256 characters. |

### ResourceACLConfig Configuration

Each `ResourceACLConfig` object supports the following fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `mode` | string | No | `"deny"` | ACL mode for resources: `allow` (allow all except listed exceptions) or `deny` (deny all except listed exceptions). |
| `exceptions` | string array | No | - | List of resource URIs that are exceptions to the selected mode. Resource URIs must be 1-2048 characters. |

### PromptACLConfig Configuration

Each `PromptACLConfig` object supports the following fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `mode` | string | No | `"deny"` | ACL mode for prompts: `allow` (allow all except listed exceptions) or `deny` (deny all except listed exceptions). |
| `exceptions` | string array | No | - | List of prompt names that are exceptions to the selected mode. Prompt names must be 1-256 characters. |



For each capability type (tools, resources, prompts):

- **Missing capability config**: All capabilities of that type are allowed (no restrictions).
- **mode: allow, exceptions: [...]**: Allow all capabilities except those listed in exceptions.
- **mode: deny, exceptions: [...]**: Deny all capabilities except those listed in exceptions.

**Note:**

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: mcp-acl-list
  gomodule: github.com/wso2/gateway-controllers/policies/mcp-acl-list@v0
```

## Reference Scenarios

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
      version: v0
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
      version: v0
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
      version: v0
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


## Notes

**Comparison with MCP Rewrite Policy**

| Aspect | MCP ACL List | MCP Rewrite |
|--------|--------------|-------------|
| **Primary Purpose** | Access control via allow/deny | Capability name mapping |
| **Rewrites Names** | No | Yes |
| **Filters Lists** | Yes | Yes |
| **Enforces Request Paths** | Yes | Yes |
| **Configuration Complexity** | Simple (mode + exceptions) | Detailed (names, descriptions, targets) |
| **Metadata Modification** | No | Yes |

Both policies can be used together: use MCP ACL List for access control and MCP Rewrite for name mapping.

**Access control and security enforcement**
Deny access to sensitive operations, internal resources, or non-approved tools to meet security and compliance requirements.

**Policy-driven authorization**
Combine with authentication and authorization policies to implement role-based access and tenant- or environment-specific restrictions.

**Operational governance**
Control costs and risk by restricting access to expensive, experimental, or beta features during rollout phases.
