# JWT Authentication

## Overview

The JWT Authentication policy validates JWT (JSON Web Token) access tokens using one or more JWKS (JSON Web Key Set) providers. This policy is essential for securing APIs by verifying the authenticity and validity of bearer tokens before allowing access to protected resources.

## Features

- Validates JWT tokens using multiple key managers (JWKS providers)
- Supports both remote JWKS endpoints and local certificates
- Configurable issuer, audience, and scope validation
- Supports custom claim validation and claim-to-header mappings
- Configurable cache TTL for JWKS responses
- Multiple allowed signing algorithms (RS256, ES256, etc.)
- Clock skew tolerance (leeway) for exp/nbf claims
- Customizable error responses

## Configuration

The JWT Authentication policy uses a two-level configuration model:

- **System Parameters**: Configured by the administrator in `config.yaml` under `policy_configurations.jwtauth_v010`
- **User Parameters**: Configured per-API/route in the API definition YAML

### System Parameters (config.yaml)

These parameters are set by the administrator and apply globally to all JWT authentication policies:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `keymanagers` | array | Yes | - | List of key manager definitions with JWKS endpoints or local certificates. |
| `jwkscachettl` | string | No | `"5m"` | Duration for caching JWKS responses (e.g., "5m", "1h"). |
| `jwksfetchtimeout` | string | No | `"5s"` | Timeout for HTTP fetch of JWKS. |
| `jwksfetchretrycount` | integer | No | `3` | Number of retries for JWKS fetch on transient failures. |
| `jwksfetchretryinterval` | string | No | `"2s"` | Interval between JWKS fetch retries. |
| `allowedalgorithms` | array | No | `["RS256", "ES256"]` | Allowed JWT signing algorithms. |
| `leeway` | string | No | `"30s"` | Clock skew allowance for exp/nbf checks. |
| `authheaderscheme` | string | No | `"Bearer"` | Expected scheme prefix in the authorization header. |
| `headername` | string | No | `"Authorization"` | Header name to extract token from. |
| `onfailurestatuscode` | integer | No | `401` | HTTP status code to return on authentication failure. |
| `errormessageformat` | string | No | `"json"` | Format of error response: "json", "plain", or "minimal". |
| `errormessage` | string | No | `"Authentication failed."` | Custom error message for authentication failures. |
| `validateissuer` | boolean | No | `true` | Whether to validate the token's issuer claim against configured key managers. |

#### Key Manager Configuration

Each key manager in the `keymanagers` array supports the following structure:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Unique name for this key manager (used in user-level `issuers` configuration). |
| `issuer` | string | No | Issuer (iss) value associated with keys from this provider. |
| `jwks.remote.uri` | string | Conditional | JWKS endpoint URL. Required if using remote JWKS. |
| `jwks.remote.certificatePath` | string | No | Path to CA certificate file for validating self-signed JWKS endpoints. |
| `jwks.remote.skipTlsVerify` | boolean | No | If true, skip TLS certificate verification. Use with caution. |
| `jwks.local.inline` | string | Conditional | Inline PEM-encoded certificate or public key. |
| `jwks.local.certificatePath` | string | Conditional | Path to certificate or public key file. |

> **Note**: Either `jwks.remote` or `jwks.local` must be specified, but not both.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `issuers` | array | No | - | List of issuer names (referencing entries in system `keymanagers`) to use for validating tokens. If omitted, runtime tries to match token `iss` claim to available key managers. |
| `audiences` | array | No | - | List of acceptable audience values; token must contain at least one. |
| `requiredScopes` | array | No | - | List of scopes that must be present in the token. |
| `requiredClaims` | object | No | - | Map of claimName → expectedValue for custom claim validation. |
| `claimMappings` | object | No | - | Map of claimName → downstream header name to expose claims for downstream services. |
| `authHeaderPrefix` | string | No | - | Override for the authorization header scheme prefix. Takes precedence over system configuration. |

## System Configuration Example

Add the following to your `gateway/configs/config.yaml` file under `policy_configurations`:

```yaml
policy_configurations:
  jwtauth_v010:
    keymanagers:
    - name: PrimaryIDP
      issuer: https://idp.example.com/oauth2/token
      jwks:
        remote:
          uri: https://idp.example.com/oauth2/jwks
          skipTlsVerify: false
    - name: SecondaryIDP
      issuer: https://auth.example.org/oauth2/token
      jwks:
        remote:
          uri: https://auth.example.org/oauth2/jwks
          skipTlsVerify: false
    jwkscachettl: "5m"
    jwksfetchtimeout: "5s"
    jwksfetchretrycount: 3
    jwksfetchretryinterval: "2s"
    allowedalgorithms:
      - RS256
      - ES256
    leeway: "30s"
    authheaderscheme: Bearer
    headername: Authorization
    onfailurestatuscode: 401
    errormessageformat: json
    errormessage: "Authentication failed."
    validateissuer: true
```


## API Definition Examples

### Example 1: Basic JWT Authentication

Apply JWT authentication to an API using a specific key manager:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: my-secure-api
  version: v1.0
  context: /api
  upstream:
    main:
      url: https://backend-service:8080/api
  policies:
    - name: jwt-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
  operations:
    - method: GET
      path: /info
    - method: POST
      path: /data
```

### Example 2: Specific Issuer and Audience Validation

Validate tokens from a specific issuer with audience requirements:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: customer-api
  version: v1.0
  context: /customers
  upstream:
    main:
      url: https://customer-service:8080
  policies:
    - name: jwt-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
        audiences:
          - https://api.example.com
          - my-api-client
  operations:
    - method: GET
      path: /list
    - method: GET
      path: /{id}
    - method: POST
      path: /create
```

### Example 3: Scope-Based Access Control

Require specific scopes for API access:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: data-api
  version: v1.0
  context: /data
  upstream:
    main:
      url: https://data-service:8080
  policies:
    - name: jwt-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
        requiredScopes:
          - read:data
          - write:data
  operations:
    - method: GET
      path: /records
    - method: POST
      path: /records
    - method: DELETE
      path: /records/{id}
```

### Example 4: Custom Claim Validation

Validate custom claims in the token for admin-only APIs:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: admin-api
  version: v1.0
  context: /admin
  upstream:
    main:
      url: https://admin-service:8080
  policies:
    - name: jwt-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
        requiredClaims:
          role: admin
          department: engineering
  operations:
    - method: GET
      path: /users
    - method: POST
      path: /users
    - method: DELETE
      path: /users/{id}
```

### Example 5: Claim Mappings to Headers

Forward claims to downstream services as headers:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: user-api
  version: v1.0
  context: /users
  upstream:
    main:
      url: https://user-service:8080
  policies:
    - name: jwt-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
        claimMappings:
          sub: X-User-ID
          email: X-User-Email
          role: X-User-Role
  operations:
    - method: GET
      path: /profile
    - method: PUT
      path: /profile
```

### Example 6: Multiple Issuers

Support tokens from multiple identity providers:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: federated-api
  version: v1.0
  context: /federated
  upstream:
    main:
      url: https://backend-service:8080
  policies:
    - name: jwt-auth
      version: v0.1.0
      params:
        issuers:
          - PrimaryIDP
          - SecondaryIDP
        audiences:
          - https://api.example.com
  operations:
    - method: GET
      path: /resources
    - method: POST
      path: /resources
```


## Use Cases

1. **API Security**: Protect APIs by requiring valid JWT tokens from trusted identity providers.

2. **Multi-Tenant Authentication**: Support multiple identity providers (key managers) for different tenants or partners.

3. **Fine-Grained Access Control**: Use scopes and custom claims to implement role-based or attribute-based access control.

4. **Service-to-Service Authentication**: Validate machine-to-machine tokens with specific audience and issuer requirements.

5. **Claim Propagation**: Forward user identity information to backend services via headers for further authorization decisions.

6. **Federation**: Accept tokens from multiple federated identity providers while maintaining consistent security policies.

