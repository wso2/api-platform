# Authentication

## Overview

Every Platform API request is authenticated with a **Bearer JWT** in the `Authorization` header and
authorized against the **OAuth2 scopes** carried in that token. There is no Basic Auth path and no
API-key path for this API — the access token is the only credential.

```
Authorization: Bearer <access_token>
```

Two things must be true for a request to succeed:

1. **Authentication** — the token's signature, expiry and issuer validate, and it carries an
   `organization` claim identifying the caller's organization.
2. **Authorization** — the token's `scope` claim (or, in role mode, its roles expanded to scopes)
   satisfies the scope declared on that specific operation. Every operation page in this
   documentation lists its required scopes under an **Authentication** heading.

All operations are automatically scoped to the organization in the token. An organization ID is
never read from a path, query or body parameter, so a token for one organization can never reach
another organization's resources.

## Authentication Modes

The mode is selected once at startup with `[platform_api.auth] mode`. Only the selected mode's
configuration section is read; the others are ignored.

| Mode | How tokens are obtained | Signature verified with | Typical use |
|---|---|---|---|
| `file` | The built-in login endpoint issues RS256 JWTs for locally configured users | `auth.jwt.public_key_file` | Quickstart, air-gapped setup |
| `external_token` | Tokens are minted outside the Platform API by a trusted issuer | `auth.jwt.public_key_file` | Existing token service |
| `idp` | An external Identity Provider's token endpoint | The IDP's JWKS (`auth.idp.jwks_url`) | Production (Asgardeo, Keycloak, Auth0, Entra ID, WSO2 IS) |

**Supported signing algorithms**: `RS256`, `RS384`, `RS512` (RSA) and, in IDP mode, `ES256`,
`ES384`, `ES512` (ECDSA). Symmetric (`HS*`) and unsigned (`none`) tokens are always rejected.

### Mode: `idp` (recommended for production)

Tokens are validated against the IDP's JWKS endpoint. `jwks_url` and `issuer` are required.

```toml
[platform_api.auth]
mode = "idp"
scope_validation = true

[platform_api.auth.idp]
name     = "asgardeo"
jwks_url = "https://accounts.example.com/oauth2/jwks"
issuer   = ["https://accounts.example.com"]   # list of accepted issuers
audience = []                                  # accepted "aud" values; empty skips the audience check
validation_mode = "scope"                      # "scope" (default) or "role"
role_mappings   = ""                           # path to a role→scope YAML file, used when validation_mode = "role"
```

Obtain a token from the IDP as usual (client credentials, authorization code, …) and send it as a
Bearer token. The Platform API never issues tokens in this mode.

### Mode: `file` (local users)

The login endpoint authenticates a username/password against `[[platform_api.auth.file.users]]` and
issues an RS256 JWT signed with `auth.jwt.private_key_file`. Suitable for initial or air-gapped
setup — prefer an IDP in production.

```toml
[platform_api.auth]
mode = "file"

[platform_api.auth.file.organization]
id           = "default"
display_name = "Default"
uuid         = "99089a17-72e0-4dd8-a2f4-c8dfbb085295"   # emitted as the `organization` claim

[[platform_api.auth.file.users]]
username      = "admin"
password_hash = "$2a$12$..."     # bcrypt; htpasswd -bnBC 12 "" <password> | tr -d ':\n'
scopes        = "ap:organization:manage ap:rest_api:manage ap:gateway:manage"

[platform_api.auth.jwt]
issuer           = "platform-api"
public_key_file  = "/etc/platform-api/keys/jwt_public.pem"
private_key_file = "/etc/platform-api/keys/jwt_private.pem"
token_ttl        = "1h"
```

The user's `scopes` string becomes the `scope` claim of the issued token — trim it to restrict what
that user can do.

#### Obtaining a token

`POST /api/portal/v0.9/auth/login` (form-encoded; this path bypasses the auth middleware):

```shell
curl -X POST https://localhost:9243/api/portal/v0.9/auth/login \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'username=admin' \
  -d 'password=<password>'
```

> 200 Response

```json
{
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": 1767225600
}
```

Use `token` as the Bearer credential on every subsequent call. Its lifetime is `auth.jwt.token_ttl`.

### Mode: `external_token`

Tokens are minted by a trusted external issuer and only *verified* here, using
`auth.jwt.public_key_file`. The Platform API does not sign anything in this mode, and
`private_key_file` is not required. Token expiry is whatever `exp` the issuer set.

## Required Claims

| Claim | Required | Purpose |
|---|---|---|
| `organization` | Yes | UUID of the caller's organization. Every operation is scoped to it. A token without it is rejected. |
| `scope` | Yes (in `scope` validation mode) | Space-separated list of granted scopes, checked against the operation's declared scopes. |
| `iss` | Yes | Must match the configured issuer (`auth.jwt.issuer`, or one of `auth.idp.issuer`). |
| `exp` | Yes | Expired tokens are rejected. |
| `sub` | Yes | The user's unique ID. |
| `aud` | Conditional | Enforced in IDP mode when `auth.idp.audience` is non-empty. |
| `org_name`, `org_handle`, `username`, `email` | No | Identity metadata surfaced to handlers. |

### Claim name mappings

Claim names are configurable, so the same mapping is used for both issuing and validating tokens
and the two can never drift apart. Each value is either a flat claim name (`org_id`) or a
dot-separated path into a nested claim (`realm_access.roles`) — useful for IDPs like Keycloak.

```toml
[platform_api.auth.claim_mappings]
organization = "organization"   # claim carrying the org ID
org_name     = "org_name"
org_handle   = "org_handle"
user_id      = "sub"
username     = "username"
email        = "email"
scope        = "scope"          # space-separated scope string
roles        = ""               # e.g. "realm_access.roles" (Keycloak) or "roles" (Asgardeo)
```

## Authorization

### Scope naming convention

Scopes follow `ap:<resource>[:<subresource>]:<action>`:

| Form | Grants |
|---|---|
| `ap:rest_api:read` | A single action on a resource |
| `ap:rest_api:manage` | Every action on that resource (read, create, update, delete) |
| `ap:rest_api:deployment:manage` | Every action on the **subresource** only — not on the parent `rest_api` |

An operation may declare several acceptable scopes. The token needs **at least one** of them —
which is why `ap:rest_api:create` and `ap:rest_api:manage` typically both appear on a create
operation.

### Validation modes

`auth.idp.validation_mode` selects how effective scopes are derived:

- **`scope`** (default) — the token's `scope` claim is used directly.
- **`role`** — the roles found at `claim_mappings.roles` are expanded into scopes via the YAML file
  at `auth.idp.role_mappings`. When a token carries several roles, the effective scopes are the
  **union** of all matching role entries (most permissive wins).

```yaml
# role_mappings YAML — requires a server restart to take effect
roles:
  - name: platform-admin
    scopes:
      - ap:organization:manage
      - ap:rest_api:manage
      - ap:gateway:manage
  - name: platform-operator
    scopes:
      - ap:organization:read
      - ap:rest_api:read
```

### Disabling scope checks

```toml
[platform_api.auth]
scope_validation = false
```

This authenticates callers but performs **no** authorization checks. Use it only to temporarily
bypass authorization during development — never in a deployed environment.

### Unauthenticated paths

`[platform_api.auth] skip_paths` lists path prefixes that bypass the auth middleware entirely —
health and metrics probes, the file-mode login endpoint, and the internal gateway routes (which
authenticate with a gateway token instead). Setting this key **replaces** the built-in default
list, so keep any entries you still need.

## Error Responses

Authentication and authorization failures are deliberately uniform — the response never reveals
*why* a credential was rejected. The specific reason is logged internally with a tracking ID.

| Status | Meaning | Cause |
|---|---|---|
| 401 | Unauthorized | Missing `Authorization` header, non-Bearer header, malformed/expired/invalid-signature token, wrong issuer, or missing `organization` claim |
| 403 | Forbidden | Token is valid, but its effective scopes do not satisfy the operation's required scopes |

A `401` carries the identical payload for every one of those causes, so it cannot be used to probe
which part of a credential was wrong.

## Troubleshooting

- **Every request returns 401 after switching to IDP mode** — check that `jwks_url` is reachable
  from the Platform API and that the token's `iss` is listed in `auth.idp.issuer`.
- **401 with a token that the IDP says is valid** — the `organization` claim is probably missing or
  under a different name. Point `claim_mappings.organization` at the claim your IDP actually emits.
- **401 only for some clients in IDP mode** — `auth.idp.audience` is non-empty and those tokens were
  minted for a different client. Add the audience value or leave the list empty to skip the check.
- **403 on operations the user should be able to perform** — compare the token's `scope` claim
  against the scopes listed on that operation's page. In `role` mode, confirm the role name in the
  token matches an entry in the `role_mappings` file (it requires a restart after editing).
- **403 in `role` mode on every operation** — `claim_mappings.roles` is unset, so no roles are read
  from the token. Set it to the claim path your IDP uses (`realm_access.roles` for Keycloak,
  `roles` for Asgardeo/Entra ID).
- **Login endpoint returns 401 in `file` mode** — the stored `password_hash` must be a bcrypt hash;
  a plaintext password there will never match.

## Scope Reference

The complete set of scopes defined by this API, as declared in the OpenAPI `OAuth2Security` scheme.

- OAuth2, flow: `clientCredentials`
- Token URL (file mode): [https://localhost:9243/oauth2/token](https://localhost:9243/oauth2/token)

|Scope|Scope Description|
|---|---|
|ap:api_key:read|Read API keys owned by the current user|
|ap:application:api_key:create|Create an application API key|
|ap:application:api_key:delete|Delete an application API key|
|ap:application:api_key:manage|Full access to application API keys|
|ap:application:api_key:read|Read application API keys|
|ap:application:association:api_key:read|Read API keys for an application association|
|ap:application:association:create|Create an application association|
|ap:application:association:delete|Delete an application association|
|ap:application:association:manage|Full access to application associations|
|ap:application:association:read|Read application associations|
|ap:application:create|Create an application|
|ap:application:delete|Delete an application|
|ap:application:manage|Full access to applications|
|ap:application:read|Read applications|
|ap:application:update|Update an application|
|ap:gateway:create|Create a gateway|
|ap:gateway:delete|Delete a gateway|
|ap:gateway:manage|Full access to gateways|
|ap:gateway:manifest:read|Read a gateway manifest|
|ap:gateway:read|Read gateways|
|ap:gateway_custom_policy:create|Create a gateway custom policy|
|ap:gateway_custom_policy:delete|Delete a gateway custom policy|
|ap:gateway_custom_policy:manage|Full access to gateway custom policies|
|ap:gateway_custom_policy:read|Read gateway custom policies|
|ap:gateway:token:create|Create a gateway token|
|ap:gateway:token:delete|Delete a gateway token|
|ap:gateway:token:manage|Full access to gateway tokens|
|ap:gateway:token:read|Read gateway tokens|
|ap:gateway:update|Update a gateway|
|ap:llm_provider:api_key:create|Create an LLM provider API key|
|ap:llm_provider:api_key:delete|Delete an LLM provider API key|
|ap:llm_provider:api_key:manage|Full access to LLM provider API keys|
|ap:llm_provider:api_key:read|Read LLM provider API keys|
|ap:llm_provider:create|Create an LLM provider|
|ap:llm_provider:delete|Delete an LLM provider|
|ap:llm_provider:deployment:create|Deploy an LLM provider|
|ap:llm_provider:deployment:delete|Delete an LLM provider deployment|
|ap:llm_provider:deployment:manage|Full access to LLM provider deployments|
|ap:llm_provider:deployment:read|Read LLM provider deployments|
|ap:llm_provider:deployment:restore|Restore an LLM provider deployment|
|ap:llm_provider:deployment:undeploy|Undeploy an LLM provider deployment|
|ap:llm_provider:manage|Full access to LLM providers|
|ap:llm_provider:read|Read LLM providers|
|ap:llm_provider:update|Update an LLM provider|
|ap:llm_proxy:api_key:create|Create an LLM proxy API key|
|ap:llm_proxy:api_key:delete|Delete an LLM proxy API key|
|ap:llm_proxy:api_key:manage|Full access to LLM proxy API keys|
|ap:llm_proxy:api_key:read|Read LLM proxy API keys|
|ap:llm_proxy:create|Create an LLM proxy|
|ap:llm_proxy:delete|Delete an LLM proxy|
|ap:llm_proxy:deployment:create|Deploy an LLM proxy|
|ap:llm_proxy:deployment:delete|Delete an LLM proxy deployment|
|ap:llm_proxy:deployment:manage|Full access to LLM proxy deployments|
|ap:llm_proxy:deployment:read|Read LLM proxy deployments|
|ap:llm_proxy:deployment:restore|Restore an LLM proxy deployment|
|ap:llm_proxy:deployment:undeploy|Undeploy an LLM proxy deployment|
|ap:llm_proxy:manage|Full access to LLM proxies|
|ap:llm_proxy:read|Read LLM proxies|
|ap:llm_proxy:update|Update an LLM proxy|
|ap:llm_template:create|Create an LLM provider template|
|ap:llm_template:delete|Delete an LLM provider template|
|ap:llm_template:manage|Full access to LLM provider templates|
|ap:llm_template:read|Read LLM provider templates|
|ap:llm_template:update|Update an LLM provider template|
|ap:mcp_proxy:create|Create an MCP proxy|
|ap:mcp_proxy:delete|Delete an MCP proxy|
|ap:mcp_proxy:deployment:create|Deploy an MCP proxy|
|ap:mcp_proxy:deployment:delete|Delete an MCP proxy deployment|
|ap:mcp_proxy:deployment:manage|Full access to MCP proxy deployments|
|ap:mcp_proxy:deployment:read|Read MCP proxy deployments|
|ap:mcp_proxy:deployment:restore|Restore an MCP proxy deployment|
|ap:mcp_proxy:deployment:undeploy|Undeploy an MCP proxy deployment|
|ap:mcp_proxy:manage|Full access to MCP proxies|
|ap:mcp_proxy:read|Read MCP proxies|
|ap:mcp_proxy:update|Update an MCP proxy|
|ap:organization:create|Create an organization|
|ap:organization:manage|Full access to an organization|
|ap:organization:read|Read an organization|
|ap:project:create|Create a project|
|ap:project:delete|Delete a project|
|ap:project:manage|Full access to projects|
|ap:project:read|Read projects|
|ap:project:update|Update a project|
|ap:rest_api:api_key:create|Create an API key for a REST API|
|ap:rest_api:api_key:delete|Delete an API key of a REST API|
|ap:rest_api:api_key:manage|Full access to a REST API's API keys|
|ap:rest_api:api_key:update|Update an API key of a REST API|
|ap:rest_api:create|Create a REST API|
|ap:rest_api:delete|Delete a REST API|
|ap:rest_api:deployment:create|Deploy a REST API|
|ap:rest_api:deployment:delete|Delete a REST API deployment|
|ap:rest_api:deployment:manage|Full access to REST API deployments|
|ap:rest_api:deployment:read|Read REST API deployments|
|ap:rest_api:deployment:restore|Restore a REST API deployment|
|ap:rest_api:deployment:undeploy|Undeploy a REST API deployment|
|ap:rest_api:gateway:create|Add gateways to a REST API|
|ap:rest_api:gateway:manage|Full access to a REST API's gateways|
|ap:rest_api:gateway:read|Read a REST API's gateways|
|ap:rest_api:manage|Full access to REST APIs|
|ap:rest_api:read|Read REST APIs|
|ap:rest_api:update|Update a REST API|
|ap:secret:create|Create a secret|
|ap:secret:delete|Delete a secret|
|ap:secret:manage|Full access to secrets|
|ap:secret:read|Read secrets|
|ap:secret:update|Update a secret|
|ap:subscription:create|Create a subscription|
|ap:subscription:delete|Delete a subscription|
|ap:subscription:manage|Full access to subscriptions|
|ap:subscription:read|Read subscriptions|
|ap:subscription:update|Update a subscription|
|ap:subscription_plan:create|Create a subscription plan|
|ap:subscription_plan:delete|Delete a subscription plan|
|ap:subscription_plan:manage|Full access to subscription plans|
|ap:subscription_plan:read|Read subscription plans|
|ap:subscription_plan:update|Update a subscription plan|
