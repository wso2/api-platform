---
title: "Overview"
---
# Dynamic Endpoint

## Overview

The Dynamic Endpoint policy routes requests to a named upstream definition at request time. It is useful when an API or operation needs to target a specific upstream definition without changing the primary API structure.

The policy sets the SDK `UpstreamName` field during the request phase. The configured `targetUpstream` value must match the `name` of an entry in the API's `upstreamDefinitions` section.

## Features

- Routes requests to a named upstream definition
- Uses request-phase upstream selection
- Simple per-route configuration with a single required parameter
- Useful for APIs that need explicit routing to alternate upstream definitions

## Configuration

The Dynamic Endpoint policy uses a single-level configuration model where parameters are configured per API or route in the API definition YAML.

### User Parameters (API Definition)

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `targetUpstream` | string | Yes | - | Name of the upstream definition to route requests to. This must match the `name` of an entry in `upstreamDefinitions`. |

**Note:**

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: dynamic-endpoint
  gomodule: github.com/wso2/gateway-controllers/policies/dynamic-endpoint@v0
```

## Reference Scenarios

### Example 1: Route an Operation to a Named Upstream Definition

Apply the policy to route requests for a specific operation to the `orders-v2` upstream definition:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: orders-api-v1.0
spec:
  displayName: Orders API
  version: v1.0
  context: /orders/$version
  upstreamDefinitions:
    - name: orders-v1
      basePath: /api/v1
      upstreams:
        - url: http://orders-v1.internal:8080
    - name: orders-v2
      basePath: /api/v2
      upstreams:
        - url: http://orders-v2.internal:8080
  operations:
    - method: GET
      path: /orders/*
      policies:
        - name: dynamic-endpoint
          version: v0
          params:
            targetUpstream: orders-v2
```

### Example 2: Use Different Upstream Definitions on Different Operations

Different operations can point to different named upstream definitions by using separate policy instances:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: customer-api-v1.0
spec:
  displayName: Customer API
  version: v1.0
  context: /customer/$version
  upstreamDefinitions:
    - name: customer-read
      basePath: /api/read
      upstreams:
        - url: http://customer-read.internal:8080
          weight: 100
    - name: customer-write
      basePath: /api/write
      upstreams:
        - url: http://customer-write.internal:8080
          weight: 100
  operations:
    - method: GET
      path: /profiles/*
      policies:
        - name: dynamic-endpoint
          version: v0
          params:
            targetUpstream: customer-read
    - method: POST
      path: /profiles
      policies:
        - name: dynamic-endpoint
          version: v0
          params:
            targetUpstream: customer-write
```

## How It Works

1. The policy reads the configured `targetUpstream` value during initialization.
2. During the request phase, it sets `UpstreamName` to that configured value.
3. The gateway uses the named upstream definition for routing.
4. If `targetUpstream` is empty, the policy leaves upstream routing unchanged.

## Notes

- `targetUpstream` is required by the policy definition and should always be configured.
- The configured value must exactly match an upstream definition name.
- The selected upstream definition can include its own `basePath` and one or more weighted `upstreams`.
- This policy only changes request routing. It does not modify headers, body content, or responses.
