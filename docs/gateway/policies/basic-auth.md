---
title: "Overview"
---
# Basic Auth

## Overview

The Basic Auth policy implements HTTP Basic Authentication to protect APIs with username and password credentials. It validates incoming requests against configured credentials, enforces authentication before allowing access to downstream services, and sets authentication metadata for downstream policies and logging. The policy provides flexible authentication modes, including an optional unauthenticated pass-through option for conditional authentication scenarios.

## Features

- **HTTP Basic Authentication**: Validates `Authorization` header with Base64-encoded username:password credentials
- **Secure Credential Comparison**: Uses constant-time comparison to prevent timing attacks
- **Flexible Authentication Modes**: Optional `allowUnauthenticated` flag to permit unauthenticated requests with metadata tracking
- **RFC 7235 Compliance**: Proper WWW-Authenticate header formatting with custom realm support
- **Metadata Tracking**: Sets authentication metadata (`auth.success`, `auth.username`, `auth.method`) for downstream policies
- **401 Unauthorized Responses**: Standard HTTP status with proper error messaging for failed authentication
- **Input Validation**: Validates Base64 encoding and credential format at request time
- **Configurable Realm**: Custom authentication realm for browser-based auth prompts

## Configuration

The Basic Auth policy uses a single-level configuration where all parameters are configured in the API definition YAML.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `username` | string | Yes | - | Expected username for authentication. Compared against the username extracted from the Basic auth header using constant-time comparison. Length: 1-256 characters. |
| `password` | string | Yes | - | Expected password for authentication. Compared against the password extracted from the Basic auth header using constant-time comparison. Length: 1-256 characters. |
| `allowUnauthenticated` | boolean | No | `false` | If `true`, allows unauthenticated requests to proceed to upstream services. Authentication status is still recorded in metadata (`auth.success = false`). If `false` (default), returns 401 Unauthorized response for authentication failures. |
| `realm` | string | No | `"Restricted"` | Authentication realm displayed in the WWW-Authenticate header. Used in browser authentication prompts to identify the protected resource. Must be non-empty if specified. Length: 1-256 characters. Defaults to "Restricted". |

**Note:**

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: basic-auth
  gomodule: github.com/wso2/gateway-controllers/policies/basic-auth@v0
```

## Reference Scenarios

### Example 1: Basic Authentication with Default Settings

Enforce Basic Auth with default realm and deny unauthenticated access:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: protected-api-v1.0
spec:
  displayName: Protected API
  version: v1.0
  context: /protected/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: basic-auth
      version: v0
      params:
        username: "admin"
        password: "secure-password-123"
  operations:
    - method: GET
      path: /data
    - method: POST
      path: /submit
    - method: PUT
      path: /update
```

### Example 2: Custom Realm Configuration

Specify a custom authentication realm for improved user experience:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: customer-api-v1.0
spec:
  displayName: Customer Portal API
  version: v1.0
  context: /customer/$version
  upstream:
    main:
      url: http://customer-service:8080
  policies:
    - name: basic-auth
      version: v0
      params:
        username: "customer_user"
        password: "customer_password"
        realm: "Customer Portal"
  operations:
    - method: GET
      path: /profile
    - method: POST
      path: /orders
    - method: GET
      path: /invoices
```

### Example 3: Optional Authentication with Metadata Tracking

Allow unauthenticated access while recording authentication status for downstream processing:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: optional-auth-api-v1.0
spec:
  displayName: Optional Auth API
  version: v1.0
  context: /optional/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: basic-auth
      version: v0
      params:
        username: "service_user"
        password: "service_password"
        allowUnauthenticated: true
        realm: "API Service"
  operations:
    - method: GET
      path: /public
    - method: GET
      path: /restricted
    - method: POST
      path: /submit
```


## How it Works

* **Request Validation Flow**: The policy extracts the `Authorization` header from the request, validates that it uses the "Basic" scheme, Base64-decodes the credentials, parses the "username:password" format, and compares both values using constant-time comparison to prevent timing attacks.

* **Credential Format**: Clients send credentials as `Authorization: Basic <base64(username:password)>`. For example, username "admin" and password "secret" would be encoded as "Basic YWRtaW46c2VjcmV0".

* **Constant-Time Comparison**: The policy compares both username and password using Go's `subtle.ConstantTimeCompare()` function to prevent timing-based attacks where attackers could infer correct characters by measuring response times.

* **Authentication Success**: When credentials match, the policy sets metadata (`auth.success=true`, `auth.username=<username>`, `auth.method=basic`) and allows the request to proceed to upstream services without modification.

* **Authentication Failure Handling**: On failed authentication, the policy either returns an immediate 401 Unauthorized response (default) or allows the request to proceed with `auth.success=false` metadata if `allowUnauthenticated=true`. The 401 response includes the `WWW-Authenticate` header following RFC 7235 for browser-based auth prompts.

* **Metadata for Downstream Policies**: Downstream policies can inspect the `auth.*` metadata to make decisions based on authentication status, enabling conditional processing and logging of authentication events.


## Notes

* **Security and credential management**: Store credentials securely, transmit them only over HTTPS, rotate them regularly, avoid hardcoding, exclude `Authorization` headers from logs, and rely on strong, randomly generated credentials with constant-time comparison to prevent attacks.
* **Authentication flow and metadata usage**: Use authentication metadata for conditional logic, auditing, and access control, apply `allowUnauthenticated` selectively, and combine with rate limiting and conditional policies to mitigate brute-force attempts and trigger alerts on failures.
* **Operational best practices**: Configure clear realm values, monitor authentication metrics and logs, ensure consistent credentials across gateway instances, apply authentication selectively to sensitive routes, and maintain secure documentation and credential inventories.


## Related Policies

- **JWT Auth**: Use as an alternative to Basic Auth for token-based authentication with better performance and stateless design
- **API Key Auth**: Use for machine-to-machine communication when Basic Auth is insufficient
- **Log Message**: Combine to audit authentication attempts while carefully excluding the Authorization header from logs
- **Rate Limiting**: Protect against brute force attacks on authenticated endpoints by limiting request rates
- **Add Headers**: Add authentication headers to upstream requests or responses based on authentication status
- **Request Rewrite**: Conditionally rewrite requests based on authentication metadata for downstream service compatibility
