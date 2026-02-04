# Request Transformation Policy

## Overview

The Request Transformation Policy enables rewriting request paths and HTTP methods before requests reach backend services. This is useful for API versioning migrations, protocol adaptations, and request normalization.

## Problem Statement

Teams need to:
1. **Rewrite API paths** - Migrate clients from `/api/v1/*` to `/api/v2/*` without breaking existing integrations
2. **Change HTTP methods** - Transform GET requests to POST when backends require different methods

## Architecture

### Component Responsibilities

| Component | Responsibility | Technology |
|-----------|---------------|------------|
| **Policy Engine** | Applies path rewrite; marks requests for method change via dynamic metadata | External Processor (gRPC) |
| **Method Rewriter** | Reads metadata and applies method change | Lua Script (file-based, mounted to Envoy) |
| **Gateway Controller** | Injects configuration into Envoy | Platform Controller |

### Why Lua for Method Rewriting?

Envoy's External Processor cannot directly modify HTTP methods. The workaround:
1. Policy Engine sets target method in dynamic metadata key: `request_transformation.target_method`
2. Lua script reads this metadata and performs the actual method change

### Filter Chain Order

```
Request → ext_proc (Policy Engine) → Lua Filter → Upstream
```

## Policy Definition

**Name:** `request-transformation`  
**Version:** `v0.1.0`

### policy-definition.yaml

```yaml
name: request-transformation
version: v0.1.0
description: |
  Transforms incoming requests by rewriting paths, query parameters, and/or HTTP methods before forwarding to upstream services.
  Supports multiple path rewrite types: prefix replacement, full path replacement, and regex-based substitution.
  Supports query parameter rewrite rules (replace, remove, add, append, and regex-based substitution).
  Method rewriting changes the HTTP method of the request.
  Optional match conditions allow rewrites to apply only when headers and/or query parameters match.

parameters:
  type: object
  properties:
    match:
      type: object
      description: Optional match conditions that gate whether rewrites are applied
      properties:
        headers:
          type: array
          description: All header matchers must succeed for the policy to apply
          minItems: 1
          items:
            type: object
            properties:
              name:
                type: string
                description: Header name to match (case-insensitive)
                minLength: 1
              type:
                type: string
                description: Match type for the header
                enum:
                  - Exact
                  - Regex
                  - Present
              value:
                type: string
                description: Value to match (required for Exact and Regex)
                minLength: 1
                maxLength: 2048
            required:
              - name
              - type
        queryParams:
          type: array
          description: All query param matchers must succeed for the policy to apply
          minItems: 1
          items:
            type: object
            properties:
              name:
                type: string
                description: Query parameter name to match
                minLength: 1
              type:
                type: string
                description: Match type for the query parameter
                enum:
                  - Exact
                  - Regex
                  - Present
              value:
                type: string
                description: Value to match (required for Exact and Regex)
                minLength: 1
                maxLength: 2048
            required:
              - name
              - type
      minProperties: 1
    pathRewrite:
      type: object
      description: Configuration for rewriting the request path
      properties:
        type:
          type: string
          description: The type of path rewrite to perform
          enum:
            - ReplacePrefixMatch
            - ReplaceFullPath
            - ReplaceRegexMatch
        replacePrefixMatch:
          type: string
          description: |
            The value to replace the matched prefix with.
            Required when type is ReplacePrefixMatch.
            The prefix to match is determined by the operation path this policy is attached to.
          maxLength: 2048
        replaceFullPath:
          type: string
          description: |
            The exact path to replace the entire request path with.
            Required when type is ReplaceFullPath.
          minLength: 1
          maxLength: 2048
          pattern: "^/.*"
        replaceRegexMatch:
          type: object
          description: |
            Regex-based path rewrite configuration.
            Required when type is ReplaceRegexMatch.
            Uses RE2 regex syntax: https://github.com/google/re2/wiki/Syntax
          properties:
            pattern:
              type: string
              description: Regular expression pattern to match against the path
              minLength: 1
              maxLength: 1024
            substitution:
              type: string
              description: |
                Replacement string. May include numbered capture groups (e.g., \1, \2).
              maxLength: 2048
          required:
            - pattern
            - substitution
      required:
        - type
    queryRewrite:
      type: object
      description: Configuration for rewriting query parameters
      properties:
        rules:
          type: array
          description: List of query parameter rewrite rules, applied in order
          minItems: 1
          items:
            type: object
            properties:
              action:
                type: string
                description: Rewrite action to apply
                enum:
                  - Replace
                  - Remove
                  - Add
                  - Append
                  - ReplaceRegexMatch
              name:
                type: string
                description: Query parameter name to target
                minLength: 1
              value:
                type: string
                description: Value used by Replace, Add, and Append
                maxLength: 2048
              separator:
                type: string
                description: Optional separator used by Append (default is empty string)
                maxLength: 64
              pattern:
                type: string
                description: Regular expression pattern to match against the parameter value
                minLength: 1
                maxLength: 1024
              substitution:
                type: string
                description: Replacement string (supports \\1, \\2, etc. for capture groups)
                maxLength: 2048
            required:
              - action
              - name
      required:
        - rules
    methodRewrite:
      type: string
      description: HTTP method to change the request to
      enum:
        - GET
        - POST
        - PUT
        - DELETE
        - PATCH
        - HEAD
        - OPTIONS
  anyOf:
    - required: [pathRewrite]
    - required: [queryRewrite]
    - required: [methodRewrite]

systemParameters:
  type: object
  properties: {}
```

## Parameter Reference

### match

Optional match conditions that gate whether rewrites are applied. All conditions in `headers` and `queryParams` must be satisfied (logical AND).

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `headers` | array | No | Header matchers (all must match) |
| `queryParams` | array | No | Query parameter matchers (all must match) |

#### Header/Query Matcher

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Header or query parameter name |
| `type` | string | Yes | Match type: `Exact`, `Regex`, `Present` |
| `value` | string | Conditional | Required when type is `Exact` or `Regex` |

### pathRewrite

Rewrites the request path. Supports three rewrite types.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Rewrite type: `ReplacePrefixMatch`, `ReplaceFullPath`, or `ReplaceRegexMatch` |
| `replacePrefixMatch` | string | Conditional | Replacement value for prefix (required when type is `ReplacePrefixMatch`) |
| `replaceFullPath` | string | Conditional | Full path replacement (required when type is `ReplaceFullPath`) |
| `replaceRegexMatch` | object | Conditional | Regex pattern and substitution (required when type is `ReplaceRegexMatch`) |

#### Type: ReplacePrefixMatch

Replaces the matched prefix portion of the path. The prefix to match is **automatically determined** by the operation path this policy is attached to.

**Constraint:** The incoming request path must start with the operation path for this rewrite to apply. You cannot match a different prefix than the operation path.

| Request Path | Operation Path | replacePrefixMatch | Result |
|--------------|----------------|-------------------|--------|
| `/api/v1/users/123` | `/api/v1` | `/api/v2` | `/api/v2/users/123` |
| `/old/resource/1` | `/old` | `/new` | `/new/resource/1` |
| `/api/v1` | `/api/v1` | `/api/v2` | `/api/v2` |
| `/api/v1/users` | `/api/v1` | `/` | `/users` |

> **Note:** If you need to match and replace a path pattern different from the operation path, use `ReplaceRegexMatch` instead.

#### Type: ReplaceFullPath

Replaces the entire request path with the specified value. Query parameters are preserved.

| Request Path | replaceFullPath | Result |
|--------------|-----------------|--------|
| `/any/path/here` | `/fixed/destination` | `/fixed/destination` |
| `/api/v1/users?id=1` | `/v2/users` | `/v2/users?id=1` |

#### Type: ReplaceRegexMatch

Uses [RE2 regular expressions](https://github.com/google/re2/wiki/Syntax) to match and replace portions of the path. This is the same regex engine used by Envoy and Envoy Gateway.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pattern` | string | Yes | RE2 regex pattern to match against the path |
| `substitution` | string | Yes | Replacement string (supports `\1`, `\2`, etc. for capture groups) |

**RE2 Syntax Highlights:**
- Capture groups: `([^/]+)` captures one or more non-slash characters
- Numbered references: `\1`, `\2` in substitution refer to captured groups
- Anchors: `^` (start), `$` (end)
- Case-insensitive: `(?i)` prefix
- Character classes: `[0-9]`, `[a-zA-Z]`, `\d`, `\w`

**Examples:**

| Request Path | pattern | substitution | Result |
|--------------|---------|--------------|--------|
| `/service/foo/v1/api` | `^/service/([^/]+)(/.*)$` | `\2/instance/\1` | `/v1/api/instance/foo` |
| `/xxx/one/yyy/one/zzz` | `one` | `two` | `/xxx/two/yyy/two/zzz` |
| `/xxx/one/yyy/one/zzz` | `^(.*?)one(.*)$` | `\1two\2` | `/xxx/two/yyy/one/zzz` |
| `/users/123/profile` | `^/users/([0-9]+)/(.*)$` | `/v2/accounts/\1/\2` | `/v2/accounts/123/profile` |
| `/aaa/XxX/bbb` | `(?i)/xxx/` | `/yyy/` | `/aaa/yyy/bbb` |

> **Reference:** Full RE2 syntax documentation: https://github.com/google/re2/wiki/Syntax

### methodRewrite

Changes the HTTP method of the request.

| Value | Description |
|-------|-------------|
| `GET` | Change to GET request |
| `POST` | Change to POST request |
| `PUT` | Change to PUT request |
| `DELETE` | Change to DELETE request |
| `PATCH` | Change to PATCH request |
| `HEAD` | Change to HEAD request |
| `OPTIONS` | Change to OPTIONS request |

### queryRewrite

Rewrites query parameters using an ordered list of rules.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `rules` | array | Yes | List of query rewrite rules (applied in order) |

#### Query Rewrite Rule

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | Yes | `Replace`, `Remove`, `Add`, `Append`, `ReplaceRegexMatch` |
| `name` | string | Yes | Query parameter name to target |
| `value` | string | Conditional | Required for `Replace`, `Add`, `Append` |
| `separator` | string | No | Used by `Append` (default: empty string) |
| `pattern` | string | Conditional | Required for `ReplaceRegexMatch` |
| `substitution` | string | Conditional | Required for `ReplaceRegexMatch` |

**Action semantics:**
- `Replace`: Replace all existing values for `name` with `value` (adds it if missing).
- `Remove`: Remove all values for `name`.
- `Add`: Add an additional `name=value` entry (preserves existing values).
- `Append`: Append `value` to each existing value for `name` (creates `name=value` if missing). If `separator` is provided, it is inserted between the old value and `value`.
- `ReplaceRegexMatch`: For each value of `name`, apply regex replacement using `pattern` and `substitution`.

## Usage Examples

### Example 1: Prefix Replacement (Version Migration)

Rewrite `/api/v1/*` to `/api/v2/*`:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api-v1.0
spec:
  displayName: My-API
  version: v1.0
  context: /myapi/$version
  upstream:
    main:
      url: http://backend:8080
  operations:
    - method: GET
      path: /api/v1
      policies:
        - name: request-transformation
          version: v0.1.0
          params:
            pathRewrite:
              type: ReplacePrefixMatch
              replacePrefixMatch: "/api/v2"
```

### Example 2: Full Path Replacement

Redirect all requests to a fixed endpoint:

```yaml
operations:
  - method: GET
    path: /legacy/endpoint
    policies:
      - name: request-transformation
        version: v0.1.0
        params:
          pathRewrite:
            type: ReplaceFullPath
            replaceFullPath: "/v2/new-endpoint"
```

### Example 3: Regex Path Rewrite

Extract path segments and reorganize:

```yaml
operations:
  - method: GET
    path: /users
    policies:
      - name: request-transformation
        version: v0.1.0
        params:
          pathRewrite:
            type: ReplaceRegexMatch
            replaceRegexMatch:
              pattern: "^/users/([0-9]+)/orders/([0-9]+)$"
              substitution: "/v2/orders/\\2/user/\\1"
```

This transforms `/users/123/orders/456` → `/v2/orders/456/user/123`

### Example 4: Method Rewrite

Convert GET to POST for a specific operation:

```yaml
operations:
  - method: GET
    path: /query
    policies:
      - name: request-transformation
        version: v0.1.0
        params:
          methodRewrite: "POST"
```

### Example 5: Combined Path and Method Rewrite

```yaml
operations:
  - method: GET
    path: /legacy/search
    policies:
      - name: request-transformation
        version: v0.1.0
        params:
          pathRewrite:
            type: ReplaceFullPath
            replaceFullPath: "/v2/query"
          methodRewrite: "POST"
```

### Example 6: Conditional Query Rewrite

Only add a `source=legacy` query parameter when a header is present:

```yaml
operations:
  - method: GET
    path: /search
    policies:
      - name: request-transformation
        version: v0.1.0
        params:
          match:
            headers:
              - name: x-client-id
                type: Present
          queryRewrite:
            rules:
              - action: Add
                name: source
                value: legacy
```

## Validation Rules

| Condition | Error Message |
|-----------|---------------|
| Neither `pathRewrite`, `queryRewrite`, nor `methodRewrite` specified | `at least one of 'pathRewrite', 'queryRewrite', or 'methodRewrite' must be specified` |
| `match` provided but empty | `match must include at least one header or query parameter matcher` |
| `match.headers[].type` is `Exact` or `Regex` and `value` missing | `match.headers[].value is required when type is Exact or Regex` |
| `match.queryParams[].type` is `Exact` or `Regex` and `value` missing | `match.queryParams[].value is required when type is Exact or Regex` |
| `pathRewrite.type` missing | `pathRewrite.type is required` |
| `pathRewrite.type` is `ReplacePrefixMatch` but `replacePrefixMatch` missing | `replacePrefixMatch is required when type is ReplacePrefixMatch` |
| `pathRewrite.type` is `ReplaceFullPath` but `replaceFullPath` missing | `replaceFullPath is required when type is ReplaceFullPath` |
| `pathRewrite.type` is `ReplaceFullPath` and `replaceFullPath` doesn't start with `/` | `replaceFullPath must start with '/'` |
| `pathRewrite.type` is `ReplaceRegexMatch` but `replaceRegexMatch` missing | `replaceRegexMatch is required when type is ReplaceRegexMatch` |
| `replaceRegexMatch.pattern` missing or empty | `replaceRegexMatch.pattern is required` |
| `replaceRegexMatch.substitution` missing | `replaceRegexMatch.substitution is required` |
| `replaceRegexMatch.pattern` is invalid RE2 regex | `replaceRegexMatch.pattern is not a valid RE2 regular expression: <error>` |
| `queryRewrite.rules` missing or empty | `queryRewrite.rules must contain at least one rule` |
| `queryRewrite.rules[].action` missing | `queryRewrite.rules[].action is required` |
| `queryRewrite.rules[].name` missing | `queryRewrite.rules[].name is required` |
| `queryRewrite.rules[].action` is `Replace`, `Add`, or `Append` but `value` missing | `queryRewrite.rules[].value is required for Replace, Add, and Append` |
| `queryRewrite.rules[].action` is `ReplaceRegexMatch` but `pattern` missing | `queryRewrite.rules[].pattern is required for ReplaceRegexMatch` |
| `queryRewrite.rules[].action` is `ReplaceRegexMatch` but `substitution` missing | `queryRewrite.rules[].substitution is required for ReplaceRegexMatch` |
| `methodRewrite` is not a valid HTTP method | `methodRewrite must be one of: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS` |

## Implementation Notes

### Lua Script Delivery

The method rewrite Lua script is delivered via xDS as an **inline string**:

1. Gateway controller loads the Lua script from a source file at startup
2. Script is embedded in the Lua HTTP filter configuration as `inline_code`
3. Configuration is pushed to Envoy via xDS

**Why inline over file mount:**
- Self-contained in xDS configuration - no external file dependencies
- Atomic updates - script changes are deployed with xDS push
- No coordination with Kubernetes volume mounts or ConfigMaps required

### Dynamic Metadata Key

The Policy Engine sets the target method in dynamic metadata:

| Namespace | Key | Value |
|-----------|-----|-------|
| `envoy.filters.http.lua` | `request_transformation.target_method` | Target HTTP method (e.g., `POST`) |

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Regex pattern doesn't match request path | Request passes through unchanged |
| Prefix doesn't match request path | Request passes through unchanged |
| Match conditions fail | Request passes through unchanged |
| Query rewrite rule references missing param | Rule is skipped for that parameter |
| Query regex pattern doesn't match | Parameter value remains unchanged |
| Lua script fails to read metadata | Request proceeds with original method; error logged |
| Invalid method in metadata | Request proceeds with original method; error logged |
