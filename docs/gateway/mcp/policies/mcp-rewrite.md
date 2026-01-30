# MCP Rewrite

## Overview

The MCP Rewrite policy enables API administrators to expose user-facing names for Model Context Protocol (MCP) tools, resources, and prompts while mapping them to different backend capability names. This policy supports three types of MCP capabilities: tools, resources, and prompts. For each capability type, you can define a list of user-facing capabilities with optional mappings to backend names, and optionally specify additional metadata fields to be returned in list responses.

When a list is provided for a capability type, only the configured capabilities are included in list responses. Requests for unlisted capabilities are rejected with an appropriate error. The policy rewrites request payloads to use backend capability names when configured, and rewrites list responses to return user-facing values.

## Features

- **Tool Rewriting**: Define user-facing tool names and map them to backend tool names with custom schemas and descriptions.
- **Resource Rewriting**: Define user-facing resource identifiers and map them to backend resource identifiers with custom descriptions.
- **Prompt Rewriting**: Define user-facing prompt names and map them to backend prompt names with custom metadata.
- **Flexible Metadata**: Include additional fields (beyond `name`, `description`, `target`, etc.) in capability definitions for custom metadata in list responses.
- **Optional Mapping**: Omit the `target` field to expose capabilities as-is without mapping to a different backend name.

## Configuration

The MCP Rewrite policy uses a single-level configuration model where all parameters are configured per-MCP-API/route in the API definition YAML.

### User Parameters (API Definition)

These parameters are configured per MCP Proxy by the API developer:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tools` | array | No | List of tools to expose and optionally rewrite. Each entry must include `name` and `description`, and should include `inputSchema`. When provided (non-empty), only these tools are included in tools/list responses. |
| `tools[].name` | string | Yes | User-facing tool name exposed to clients (1-256 characters). |
| `tools[].description` | string | Yes | User-facing tool description returned in tools/list. |
| `tools[].inputSchema` | string | Yes | Tool input schema returned in tools/list. |
| `tools[].outputSchema` | string | No | Tool output schema returned in tools/list. |
| `tools[].target` | string | No | Backend tool name to use when forwarding requests. If omitted, the `name` is used. |
| `resources` | array | No | List of resources to expose and optionally rewrite. Each entry must include `name` and `uri`. When provided (non-empty), only these resources are included in resources/list responses. |
| `resources[].name` | string | Yes | User-facing resource identifier exposed to clients (1-1024 characters). |
| `resources[].uri` | string | Yes | User-facing resource URI returned in resources/list (1-2048 characters). |
| `resources[].description` | string | No | User-facing resource description returned in resources/list. |
| `resources[].target` | string | No | Backend resource identifier (URI) to use when forwarding requests. If omitted, the `uri` is used. |
| `prompts` | array | No | List of prompts to expose and optionally rewrite. Each entry must include `name`. When provided (non-empty), only these prompts are included in prompts/list responses. |
| `prompts[].name` | string | Yes | User-facing prompt name exposed to clients (1-256 characters). |
| `prompts[].description` | string | No | User-facing prompt description returned in prompts/list. |
| `prompts[].target` | string | No | Backend prompt name to use when forwarding requests. If omitted, the `name` is used. |

> **Note**: Additional custom fields can be included in `tools`, `resources`, and `prompts` definitions and will be returned in the corresponding list responses.

## MCP Proxy Definition Examples

### Example 1: Basic Tool Rewriting

Expose tools with different names than the backend:

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
    - name: mcp-rewrite
      version: v0.1.0
      params:
        tools:
          - name: list-files
            description: List files in a directory
            inputSchema: '{"type": "object", "properties": {"path": {"type": "string"}}}'
            target: backend_list_files
          - name: read-file
            description: Read file contents
            inputSchema: '{"type": "object", "properties": {"path": {"type": "string"}}}'
            target: backend_read_file
  tools:
    ...
```

### Example 2: Resource Rewriting with URI Mapping

Expose resources with user-friendly URIs mapped to backend resources:

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
    - name: mcp-rewrite
      version: v0.1.0
      params:
        resources:
          - name: user-docs
            uri: file:///user-documentation
            description: User documentation files
            target: file:///internal/docs/users
          - name: api-specs
            uri: file:///api-specifications
            description: API specification files
            target: file:///internal/specs/api
  resources:
    ...
```

### Example 3: Prompt and Tool Rewriting Combined

Rewrite prompts and tools with metadata:

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
    - name: mcp-rewrite
      version: v0.1.0
      params:
        tools:
          - name: create-document
            description: Create a new document
            inputSchema: '{"type": "object", "properties": {"title": {"type": "string"}}}'
            target: create_doc
        prompts:
          - name: summarize
            description: Summarize content
            target: summarize_content
            category: text-processing
  tools:
    ...
```

## Use Cases

1. **Semantic Naming**: Use user-friendly names in the API while keeping backend names internal or legacy.
2. **AI Readiness**: Redefine name, description, input schema, etc. in a way that is friendly to the AI agents.
