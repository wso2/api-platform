# JWT Authentication

## Overview

The JWT Authentication policy validates JWT access tokens using one or more JWKS (JSON Web Key Set) providers. It is typically applied to operations that require bearer token authentication before requests are forwarded upstream.

## Features

- Validates JWTs using multiple key managers (JWKS providers)
- Supports remote JWKS endpoints and local certificates
- Configurable issuer, audience, scope, and claim validation
- Claim-to-header mappings for downstream services
- Configurable user ID claim extraction for analytics
- Configurable JWKS cache and retry settings
- Allowed signing algorithm allowlist
- Authorization header scheme enforcement and clock skew tolerance
- Customizable error responses
- Optional `userIdClaim` mapping for analytics

## Configuration

JWT Authentication uses two levels of configuration.

- System parameters live in `gateway/configs/config.toml` under `policy_configurations.jwtauth_v010`.
- User parameters are defined in the API configuration under `policies`.

### System Parameters (config.toml)

| Parameter | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `keymanagers` | array | Yes | - | List of key manager definitions with JWKS configuration. |
| `jwkscachettl` | string | No | `"5m"` | JWKS cache TTL. |
| `jwksfetchtimeout` | string | No | `"5s"` | JWKS fetch timeout. |
| `jwksfetchretrycount` | integer | No | `3` | JWKS fetch retry count. |
| `jwksfetchretryinterval` | string | No | `"2s"` | JWKS fetch retry interval. |
| `allowedalgorithms` | array | No | `["RS256", "ES256"]` | Allowed JWT signing algorithms. |
| `leeway` | string | No | `"30s"` | Clock skew allowance for exp/nbf. |
| `authheaderscheme` | string | No | `"Bearer"` | Expected authorization scheme prefix. |
| `headername` | string | No | `"Authorization"` | Header name to extract the token from. |
| `onfailurestatuscode` | integer | No | `401` | HTTP status code on authentication failure. |
| `errormessageformat` | string | No | `"json"` | Error format: `"json"`, `"plain"`, or `"minimal"`. |
| `errormessage` | string | No | `"Authentication failed."` | Error message body for failures. |
| `validateissuer` | boolean | No | `true` | Validate the token `iss` claim against key managers. |

#### Key Manager Configuration

Each entry in `keymanagers` must include a unique `name` and either `jwks.remote` or `jwks.local`.

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Unique key manager name. |
| `issuer` | string | No | Optional issuer (`iss`) value for this key manager. |
| `jwks.remote.uri` | string | Conditional | JWKS endpoint URL. Required if using remote JWKS. |
| `jwks.remote.certificatePath` | string | No | CA cert path for self-signed JWKS endpoints. |
| `jwks.remote.skipTlsVerify` | boolean | No | Skip TLS verification (use with caution). |
| `jwks.local.inline` | string | Conditional | Inline PEM certificate or public key. |
| `jwks.local.certificatePath` | string | Conditional | Path to certificate or public key file. |

### User Parameters (API Definition)

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `issuers` | array | No | List of key manager names (or issuer values) to use. If omitted, runtime matches token `iss` or tries all key managers. |
| `audiences` | array | No | Acceptable audience values. Token must contain at least one. |
| `requiredScopes` | array | No | Required scopes. Uses space-delimited `scope` claim or array `scp` claim. |
| `requiredClaims` | object | No | Map of claim name to expected value. |
| `claimMappings` | object | No | Map of claim name to downstream header name. |
| `userIdClaim` | string | No | `"sub"` | Claim name to extract the user ID from the JWT token for analytics. If not specified, defaults to "sub" claim. |
| `authHeaderPrefix` | string | No | Overrides the configured authorization header scheme for this route. |
| `userIdClaim` | string | No | Claim name to extract user ID for analytics. Defaults to `sub`. |

## System Configuration Example

```toml
[policy_configurations.jwtauth_v010]
jwkscachettl = "5m"
jwksfetchtimeout = "5s"
jwksfetchretrycount = 3
jwksfetchretryinterval = "2s"
allowedalgorithms = ["RS256", "ES256"]
leeway = "30s"
authheaderscheme = "Bearer"
headername = "Authorization"
onfailurestatuscode = 401
errormessageformat = "json"
errormessage = "Authentication failed."
validateissuer = true

[[policy_configurations.jwtauth_v010.keymanagers]]
name = "PrimaryIDP"
issuer = "https://idp.example.com/oauth2/token"

[policy_configurations.jwtauth_v010.keymanagers.jwks.remote]
uri = "https://idp.example.com/oauth2/jwks"
skipTlsVerify = false

[[policy_configurations.jwtauth_v010.keymanagers]]
name = "SecondaryIDP"
issuer = "https://auth.example.org/oauth2/token"

[policy_configurations.jwtauth_v010.keymanagers.jwks.remote]
uri = "https://auth.example.org/oauth2/jwks"
skipTlsVerify = false
```

## API Definition Examples

### Example 1: Basic JWT Authentication

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: jwt-auth-basic-api
spec:
  displayName: JWT Auth Basic API
  version: v1.0
  context: /jwt-auth-basic/$version
  upstream:
    main:
      url: http://sample-backend:9080/api/v1
  operations:
    - method: GET
      path: /health
    - method: GET
      path: /protected
      policies:
        - name: jwt-auth
          version: v0
          params:
            issuers:
              - PrimaryIDP
```

### Example 2: Audience and Scope Validation

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: jwt-auth-audience-api
spec:
  displayName: JWT Auth Audience API
  version: v1.0
  context: /jwt-auth-audience/$version
  upstream:
    main:
      url: http://sample-backend:9080/api/v1
  operations:
    - method: GET
      path: /protected
      policies:
        - name: jwt-auth
          version: v0
          params:
            issuers:
              - PrimaryIDP
            audiences:
              - "test-audience"
            requiredScopes:
              - read:data
```

### Example 3: Claim Mapping to Downstream Headers

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: jwt-auth-claims-api
spec:
  displayName: JWT Auth Claims API
  version: v1.0
  context: /jwt-auth-claims/$version
  upstream:
    main:
      url: http://sample-backend:9080/api/v1
  operations:
    - method: GET
      path: /profile
      policies:
        - name: jwt-auth
          version: v0
          params:
            issuers:
              - PrimaryIDP
            claimMappings:
              sub: X-User-ID
              email: X-User-Email
              role: X-User-Role
```

## Use Cases

1. Secure APIs by requiring valid JWT tokens from trusted identity providers.
2. Support multiple issuers for multi-tenant or federated authentication.
3. Enforce scopes and claims for fine-grained access control.
4. Propagate user identity to upstream services via claim mappings.
