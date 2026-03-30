---
title: "Overview"
---
# Subscription Validation

## Overview

The Subscription Validation policy ensures that incoming requests are associated with an active subscription for the target API. It primarily validates a subscription token provided in the request, with support for both header and cookie sources. When subscription plans include throttle limits, this policy also enforces per-subscription rate limits.

## Features

- Validates subscriptions per API using subscription tokens.
- Supports subscription tokens from a configurable HTTP header
- Optional lookup of subscription tokens from a configurable cookie
- Enforces plan-based throttling (rate limiting) when a subscription plan defines limits and `StopOnQuotaReach` is enabled

## Configuration

This policy uses a single-level configuration model where all parameters are configured per API or route in the API definition YAML.

### User Parameters (API Definition)

These parameters are configured per API or route by the API developer:

| Parameter               | Type    | Required | Default            | Description |
|-------------------------|---------|----------|--------------------|-------------|
| `subscriptionKeyHeader` | string  | No       | `Subscription-Key` | Name of the HTTP request header that carries the subscription token. This is the primary source and is checked before cookie fallback. |
| `subscriptionKeyCookie` | string  | No       | `""`               | Optional cookie name that can carry the subscription token. When non-empty, the cookie value is checked only if the header does not contain a token. |

At runtime the policy behaves as follows:

1. It first attempts to read the subscription token from `subscriptionKeyHeader`.
2. If no header token is present and `subscriptionKeyCookie` is non-empty, it attempts to read the token from the named cookie.
3. If a token is found, it is validated against the subscription store for the target API.
4. If a matching active subscription is not found, the request is rejected.
5. When the matched subscription plan includes throttle limits, the policy enforces per-subscription rate limits and can block requests when the quota is exceeded.

### System Parameters

This policy does not require explicit system-level configuration; it relies on the gateway's subscription store, which is managed by the platform.

## Error Responses

When validation fails, the policy returns an immediate JSON error response and stops further processing.

- **Forbidden / No Active Subscription**
  - **Status code:** `403`
  - **Body (example):**
    ```json
    {
      "error": "forbidden",
      "message": "Subscription required for this API"
    }
    ```

- **Rate Limit Exceeded**
  - When a subscription has a throttle plan with `StopOnQuotaReach=true` and the quota is exceeded:
  - **Status code:** `429`
  - **Body (example):**
    ```json
    {
      "error": "rate_limit_exceeded",
      "message": "Subscription quota exceeded"
    }
    ```

## Adding the Policy Module

Inside `gateway/build.yaml`, ensure the policy module is added under `policies`:

```yaml
- name: subscription-validation
  gomodule: github.com/wso2/gateway-controllers/policies/subscription-validation@v0
```

## Reference Scenarios

### Example 1: Basic Subscription Token Validation (Header Only)

Require a subscription token from the default `Subscription-Key` header:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: subscription-validation
      version: v0
      params:
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /plans/{planId}
```

In this configuration, requests must include a valid subscription token:

```http
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Subscription-Key: tok-1
```

### Example 2: Custom Header Name

Use a custom header name for the subscription token:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: subscription-validation
      version: v0
      params:
        subscriptionKeyHeader: X-Subscription-Token
  operations:
    - method: GET
      path: /{country_code}/{city}
```

Requests must now send the token in `X-Subscription-Token`:

```http
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
X-Subscription-Token: tok-1
```

### Example 3: Cookie-Based Subscription Token

Read the subscription token from a cookie when the header is not present:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: subscription-validation
      version: v0
      params:
        subscriptionKeyHeader: Subscription-Key
        subscriptionKeyCookie: sub-key
  operations:
    - method: GET
      path: /{country_code}/{city}
```

If the `Subscription-Key` header is missing, the policy will look for a cookie:

```http
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Cookie: sub-key=tok-1
```

### Example 4: Plan-Based Rate Limiting

When a subscription has a throttle plan configured (for example, request limits per minute) and `StopOnQuotaReach` is enabled, the policy enforces rate limits per subscription:

- Requests within the allowed quota are passed through.
- Once the configured limit is exceeded, additional requests are rejected with `429 Too Many Requests` and a `rate_limit_exceeded` error.

This allows you to combine subscription validation with fine-grained quota enforcement for different subscription tiers.

