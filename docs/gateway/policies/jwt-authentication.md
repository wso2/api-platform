Introduce **JwtAuthentication Policy (v1.0.0)** featuring:
- JWKS signature verification (RSA algorithms)
- Token validation (expiration, audience, scopes, custom claims)
- JWKS caching with TTL and retry logic
- Claim-to-header mapping for downstream services
- Configurable error responses (401/403)

## Configuration Schema

```yaml
name: JwtAuthentication
version: v1.0.0
description: |
  Validates JWT access tokens using one or more JWKS providers (key managers).
  System-level configuration holds key manager endpoints and fetch behavior.
  User-level configuration holds per-route assertions (selected issuers, audiences, scopes, claims and mappings).

parameters:
  user:
    type: object
    additionalProperties: false
    properties:
      issuers:
        type: array
        description: >-
          List of issuer names (referencing entries in `system.keyManagers`) to use
          for validating tokens for this route. If omitted, runtime will try to
          match token `iss` claim to available key managers or try all providers.
        items:
          type: string

      audiences:
        type: array
        description: List of acceptable audience values; token must contain at least one.
        items:
          type: string

      requiredScopes:
        type: array
        description: List of scopes that must be present in the token (space-delimited 'scope' claim or array 'scp').
        items:
          type: string

      requiredClaims:
        type: object
        description: Map of claimName -> expectedValue. Runtime may support equality or regex-based matching.
        additionalProperties:
          type: string

      claimMappings:
        type: object
        description: Map claimName -> downstream header or context key to expose for downstream services.
        additionalProperties:
          type: string

      authHeaderPrefix:
        type: string
        description: Override for the authorization header scheme prefix (e.g., "Bearer", "JWT"). If specified at user-level, takes precedence over system configuration for this route.
          
  system:
    type: object
    additionalProperties: false
    properties:
      keyManagers:
        type: array
        description: >-
          List of key manager (JWKS provider) definitions. Each entry must include
          a unique `name` used to reference this provider from `user.issuers`, and
          a `uri` pointing to the JWKS endpoint. Optionally include an `issuer`
          value to associate tokens from that issuer with this provider.
        items:
          type: object
          additionalProperties: false
          required: ["name", "uri"]
          properties:
            name:
              type: string
              description: Unique name for this key manager/provider (used in `user.issuers`).
            uri:
              type: string
              format: uri
              description: JWKS endpoint URL (e.g., https://idp.example/.well-known/jwks.json).
            issuer:
              type: string
              description: Optional issuer (iss) value associated with keys from this provider.
        "wso2/defaultValue": "$config(JWTAuth.KeyManagers)"

      jwksCacheTtl:
        type: string
        description: Duration string for JWKS caching (e.g., "5m"). If omitted a default is used.
        "wso2/defaultValue": "$config(JWTAuth.JwksCacheTtl)"

      jwksFetchTimeout:
        type: string
        description: Timeout for HTTP fetch of JWKS, e.g., "5s".
        "wso2/defaultValue": "$config(JWTAuth.JwksFetchTimeout)"

      jwksFetchRetryCount:
        type: integer
        description: Number of retries for JWKS fetch on transient failures.
        "wso2/defaultValue": "$config(JWTAuth.JwksFetchRetryCount)"

      jwksFetchRetryInterval:
        type: string
        description: Interval between JWKS fetch retries, e.g., "2s".
        "wso2/defaultValue": "$config(JWTAuth.JwksFetchRetryInterval)"

      allowedAlgorithms:
        type: array
        description: Allowed JWT signing algorithms (e.g., ["RS256","ES256"]).
        items:
          type: string
        "wso2/defaultValue": "$config(JWTAuth.AllowedAlgorithms)"

      leeway:
        type: string
        description: Clock skew allowance for exp/nbf checks, e.g., "30s".
        "wso2/defaultValue": "$config(JWTAuth.Leeway)"

      authHeaderScheme:
        type: string
        description: Expected scheme prefix in the authorization header (e.g., "Bearer"). If set, runtime enforces the scheme; if omitted, runtime may accept raw header values or strip known schemes.
        default: Bearer
        "wso2/defaultValue": "$config(JWTAuth.AuthHeaderScheme)"

      headerName:
        type: string
        description: Header name to extract token from (default "Authorization").
        default: Authorization
        "wso2/defaultValue": "$config(JWTAuth.HeaderName)"

      onFailureStatusCode:
        type: integer
        description: HTTP status code to return on authentication failure (401 for Unauthorized, 403 for Forbidden).
        default: 401
        "wso2/defaultValue": "$config(JWTAuth.OnFailureStatusCode)"

      errorMessageFormat:
        type: string
        description: Format of error response on JWT validation failure. Supported values are "json" (structured error), "plain" (plain text), or "minimal" (minimal response).
        default: json
        "wso2/defaultValue": "$config(JWTAuth.ErrorMessageFormat)"

    required: ["keyManagers"]


```

## Example System Configuration

```yaml
JWTAuth:
    keyManagers:
    - uri: https://auth0.example.com/.well-known/jwks.json
      issuer: https://auth0.example.com/
    - uri: https://okta.example.com/oauth2/default/.well-known/jwks.json
      issuer: https://okta.example.com/

    jwksCacheTtl: 5m
    jwksFetchTimeout: 5s
    jwksFetchRetryCount: 3
    jwksFetchRetryInterval: 2s
    allowedAlgorithms: [RS256, ES256]
    leeway: 30s
    onFailureStatusCode: 401
    errorMessageFormat: json
```

## Example API/Per-Route Configuration

```yaml
# Route-specific JWT validation
name: JwtAuthentication
version: v1.0.0
params:
    issuers: ["https://auth0.example.com/"]
    audiences: [api-service]
    requiredScopes: [read:users, write:posts]
    requiredClaims:
        org_id: "acme-corp"
        environment: "production"
    claimMappings:
        sub: x-user-id
        org_id: x-org-id
        aud: x-audience
    authHeaderPrefix: Bearer
```
