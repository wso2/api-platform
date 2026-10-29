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
scopes        = "ap:organization:manage ap:api:rest:manage ap:gateway:manage"

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

Scopes are permissions of the tree in `platform-api/resources/permission-tree.yaml`: a node path
and an action, `ap:<node>[:<sub-node>...]:<action>`. Every API kind lives under `ap:api`
(`ap:api:rest`, `ap:api:graphql`, `ap:api:llm`, `ap:api:mcp`, `ap:api:agent`); secrets and API
keys under `ap:credential`; docs and thumbnails under `ap:content`.

| Form | Grants |
|---|---|
| `ap:api:rest:deployment:create` | One action on one node |
| `ap:api:rest:deployment:manage` | Every action on that node and on every node beneath it |
| `ap:api:rest:read` | Every read on that node and on every node beneath it (builds, deployments, key listings) |
| `ap:api:rest:create` | Creating a REST API only — a parent's `create`, `update`, `delete`, `restore`, `undeploy` never reaches a child |
| `ap:api:read`, `ap:api:manage` | Every read / every operation on every API kind |
| `ap:read`, `ap:manage` | The root: every read / every operation of the Platform API |

Only `read` and `manage` flow down the tree. An operation's `security` block lists **every**
scope that admits it — the node's own action and `manage`, `manage` on each ancestor and, for a
read, `read` on each ancestor — and the token needs **at least one** of them. Nothing is inferred
from the scope string at request time.

**Pre-tree names.** Earlier releases named scopes per resource and verb (`ap:rest_api:manage`,
`ap:llm_proxy:deployment:read`, `ap:api_key:all:manage`). The spec no longer declares them, but a
token, IDP role or role file that still carries one keeps working: `legacy-scope-mapping.yaml`
names the tree permission each one grants and the server translates them on every request. An old
name reaches exactly the operations it reached before, plus only the reads a parent's `read` now
flows down to.

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
      - ap:manage                            # the root admits every operation
      - ap:credential:api_key:all:manage     # every user's API keys — never implied
  - name: platform-operator
    scopes:
      - ap:organization:read
      - ap:api:read
      - ap:api:rest:deployment:manage
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

The complete set of scopes defined by this API, as declared in the OpenAPI `OAuth2Security` scheme
(the permission tree's permissions that at least one operation accepts, plus the spec's
`x-reserved-permissions`: permissions a role may grant ahead of the operation that will accept
them, marked *reserved* below — no operation admits them yet; pre-tree names are not
declared — see [Scope naming convention](#scope-naming-convention)).

- OAuth2, flow: `clientCredentials`
- Token URL declared in the spec: `https://localhost:9243/oauth2/token`

The declared token URL describes the `clientCredentials` flow an external IDP is expected to expose,
and is what generated clients target in `idp` mode — the Platform API does not serve it itself.
In `file` mode there is no client-credentials endpoint; obtain a token from the username/password
login endpoint described under [Obtaining a token](#obtaining-a-token)
(`POST /api/portal/v0.9/auth/login`) instead.

|Scope|Scope Description|
|---|---|
|ap:api:agent:api_key:create|Create an API key for an Agent proxy|
|ap:api:agent:api_key:delete|Revoke an API key of an Agent proxy|
|ap:api:agent:api_key:manage|Full access to an Agent proxy's API keys|
|ap:api:agent:api_key:read|Read Agent proxy API keys|
|ap:api:agent:api_key:update|Update an API key of an Agent proxy|
|ap:api:agent:build:create|Prepare a build of an Agent proxy|
|ap:api:agent:build:delete|Delete a build of an Agent proxy|
|ap:api:agent:build:manage|Full access to Agent proxy builds|
|ap:api:agent:build:read|Read Agent proxy builds|
|ap:api:agent:create|Create an Agent proxy|
|ap:api:agent:delete|Delete an Agent proxy|
|ap:api:agent:deployment:create|Deploy an Agent proxy|
|ap:api:agent:deployment:delete|Delete an Agent proxy deployment|
|ap:api:agent:deployment:manage|Full access to Agent proxy deployments|
|ap:api:agent:deployment:read|Read Agent proxy deployments|
|ap:api:agent:deployment:restore|Restore an Agent proxy deployment|
|ap:api:agent:deployment:undeploy|Undeploy an Agent proxy deployment|
|ap:api:agent:manage|Full access to Agent proxies|
|ap:api:agent:read|Read Agent proxies|
|ap:api:agent:update|Update an Agent proxy|
|ap:api:graphql:api_key:create|Create an API key for a GraphQL API|
|ap:api:graphql:api_key:delete|Delete an API key of a GraphQL API|
|ap:api:graphql:api_key:manage|Full access to a GraphQL API's API keys|
|ap:api:graphql:api_key:update|Update an API key of a GraphQL API|
|ap:api:graphql:build:create|Prepare a build of a GraphQL API|
|ap:api:graphql:build:delete|Delete a build of a GraphQL API|
|ap:api:graphql:build:manage|Full access to GraphQL API builds|
|ap:api:graphql:build:read|Read GraphQL API builds|
|ap:api:graphql:create|Create a GraphQL API|
|ap:api:graphql:delete|Delete a GraphQL API|
|ap:api:graphql:deployment:create|Deploy a GraphQL API|
|ap:api:graphql:deployment:delete|Delete a GraphQL API deployment|
|ap:api:graphql:deployment:manage|Full access to GraphQL API deployments|
|ap:api:graphql:deployment:read|Read GraphQL API deployments|
|ap:api:graphql:deployment:restore|Restore a GraphQL API deployment|
|ap:api:graphql:deployment:undeploy|Undeploy a GraphQL API deployment|
|ap:api:graphql:gateway:create|Add gateways to a GraphQL API|
|ap:api:graphql:gateway:manage|Full access to a GraphQL API's gateways|
|ap:api:graphql:gateway:read|Read a GraphQL API's gateways|
|ap:api:graphql:manage|Full access to GraphQL APIs|
|ap:api:graphql:read|Read GraphQL APIs|
|ap:api:graphql:update|Update a GraphQL API|
|ap:api:llm:api_key:create|Create an LLM proxy API key|
|ap:api:llm:api_key:delete|Delete an LLM proxy API key|
|ap:api:llm:api_key:manage|Full access to LLM proxy API keys|
|ap:api:llm:api_key:read|Read LLM proxy API keys|
|ap:api:llm:build:create|Prepare a build of an LLM proxy|
|ap:api:llm:build:delete|Delete a build of an LLM proxy|
|ap:api:llm:build:manage|Full access to LLM proxy builds|
|ap:api:llm:build:read|Read LLM proxy builds|
|ap:api:llm:create|Create an LLM proxy|
|ap:api:llm:delete|Delete an LLM proxy|
|ap:api:llm:deployment:create|Deploy an LLM proxy|
|ap:api:llm:deployment:delete|Delete an LLM proxy deployment|
|ap:api:llm:deployment:manage|Full access to LLM proxy deployments|
|ap:api:llm:deployment:read|Read LLM proxy deployments|
|ap:api:llm:deployment:restore|Restore an LLM proxy deployment|
|ap:api:llm:deployment:undeploy|Undeploy an LLM proxy deployment|
|ap:api:llm:manage|Full access to LLM proxies|
|ap:api:llm:read|Read LLM proxies|
|ap:api:llm:update|Update an LLM proxy|
|ap:api:manage|Full access to APIs of every kind|
|ap:api:mcp:build:create|Prepare a build of an MCP proxy|
|ap:api:mcp:build:delete|Delete a build of an MCP proxy|
|ap:api:mcp:build:manage|Full access to MCP proxy builds|
|ap:api:mcp:build:read|Read MCP proxy builds|
|ap:api:mcp:create|Create an MCP proxy|
|ap:api:mcp:delete|Delete an MCP proxy|
|ap:api:mcp:deployment:create|Deploy an MCP proxy|
|ap:api:mcp:deployment:delete|Delete an MCP proxy deployment|
|ap:api:mcp:deployment:manage|Full access to MCP proxy deployments|
|ap:api:mcp:deployment:read|Read MCP proxy deployments|
|ap:api:mcp:deployment:restore|Restore an MCP proxy deployment|
|ap:api:mcp:deployment:undeploy|Undeploy an MCP proxy deployment|
|ap:api:mcp:manage|Full access to MCP proxies|
|ap:api:mcp:read|Read MCP proxies|
|ap:api:mcp:update|Update an MCP proxy|
|ap:api:read|Read APIs of every kind|
|ap:api:rest:api_key:create|Create an API key for a REST API|
|ap:api:rest:api_key:delete|Delete an API key of a REST API|
|ap:api:rest:api_key:manage|Full access to a REST API's API keys|
|ap:api:rest:api_key:update|Update an API key of a REST API|
|ap:api:rest:build:create|Prepare a build of a REST API|
|ap:api:rest:build:delete|Delete a build of a REST API|
|ap:api:rest:build:manage|Full access to REST API builds|
|ap:api:rest:build:read|Read REST API builds|
|ap:api:rest:create|Create a REST API|
|ap:api:rest:delete|Delete a REST API|
|ap:api:rest:deployment:create|Deploy a REST API|
|ap:api:rest:deployment:delete|Delete a REST API deployment|
|ap:api:rest:deployment:manage|Full access to REST API deployments|
|ap:api:rest:deployment:read|Read REST API deployments|
|ap:api:rest:deployment:restore|Restore a REST API deployment|
|ap:api:rest:deployment:undeploy|Undeploy a REST API deployment|
|ap:api:rest:gateway:create|Add gateways to a REST API|
|ap:api:rest:gateway:manage|Full access to a REST API's gateways|
|ap:api:rest:gateway:read|Read a REST API's gateways|
|ap:api:rest:manage|Full access to REST APIs|
|ap:api:rest:read|Read REST APIs|
|ap:api:rest:update|Update a REST API|
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
|ap:content:document:manage|Full access to API documents|
|ap:content:document:read|Read API documents|
|ap:content:manage|Full access to API content (docs and thumbnails, every API kind)|
|ap:content:read|Read API content (docs and thumbnails, every API kind)|
|ap:content:thumbnail:manage|Full access to API thumbnails|
|ap:content:thumbnail:read|Read API thumbnails|
|ap:credential:api_key:all:manage|Manage API keys created by any user in the organization|
|ap:credential:api_key:read|Read API keys owned by the current user|
|ap:credential:secret:create|Create a secret|
|ap:credential:secret:delete|Delete a secret|
|ap:credential:secret:manage|Full access to Secrets|
|ap:credential:secret:read|Read Secrets|
|ap:credential:secret:update|Update a secret|
|ap:gateway:create|Create a gateway|
|ap:gateway:delete|Delete a gateway|
|ap:gateway:manage|Full access to gateways|
|ap:gateway:manifest:read|Read a gateway manifest|
|ap:gateway:read|Read gateways|
|ap:gateway:token:create|Create a gateway token|
|ap:gateway:token:delete|Delete a gateway token|
|ap:gateway:token:manage|Full access to gateway tokens|
|ap:gateway:token:read|Read gateway tokens|
|ap:gateway:update|Update a gateway|
|ap:gateway_policy:create|Create a gateway custom policy|
|ap:gateway_policy:delete|Delete a gateway custom policy|
|ap:gateway_policy:manage|Full access to Gateway custom policies|
|ap:gateway_policy:read|Read Gateway custom policies|
|ap:llm_provider:api_key:create|Create an LLM provider API key|
|ap:llm_provider:api_key:delete|Delete an LLM provider API key|
|ap:llm_provider:api_key:manage|Full access to LLM provider API keys|
|ap:llm_provider:api_key:read|Read LLM provider API keys|
|ap:llm_provider:build:create|Prepare a build of an LLM provider|
|ap:llm_provider:build:delete|Delete a build of an LLM provider|
|ap:llm_provider:build:manage|Full access to an LLM provider's builds|
|ap:llm_provider:build:read|Read builds of an LLM provider|
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
|ap:llm_provider:template:create|Create an LLM provider template|
|ap:llm_provider:template:delete|Delete an LLM provider template|
|ap:llm_provider:template:manage|Full access to LLM provider templates|
|ap:llm_provider:template:read|Read LLM provider templates|
|ap:llm_provider:template:update|Update an LLM provider template|
|ap:llm_provider:update|Update an LLM provider|
|ap:manage|Full access to every Platform API resource and operation|
|ap:organization:create|Create an organization|
|ap:organization:manage|Full access to an organization|
|ap:organization:read|Read an organization|
|ap:portal:create|Create an API Portal|
|ap:portal:delete|Delete an API Portal|
|ap:portal:draft:manage|Full access to Portal drafts|
|ap:portal:draft:read|Read Portal drafts|
|ap:portal:draft:update|Edit an API's draft for one portal|
|ap:portal:graphql_api:deprecate|Deprecate Portal Graphql Api|
|ap:portal:graphql_api:manage|Full access to Publishing GraphQL APIs to a portal|
|ap:portal:graphql_api:publish|Publish Portal Graphql Api|
|ap:portal:graphql_api:unpublish|Unpublish Portal Graphql Api|
|ap:portal:manage|Full access to API Portals and publishing to them|
|ap:portal:mcp_proxy:manage|Full access to publishing MCP proxies to an API Portal (reserved — no operation accepts it yet)|
|ap:portal:mcp_proxy:publish|Publish an MCP proxy to an API Portal (reserved — no operation accepts it yet)|
|ap:portal:mcp_proxy:unpublish|Unpublish an MCP proxy from an API Portal (reserved — no operation accepts it yet)|
|ap:portal:publication:read|Read Portal listings (incl. /api-publications)|
|ap:portal:read|Read API Portals and publishing to them|
|ap:portal:rest_api:deprecate|Deprecate Portal Rest Api|
|ap:portal:rest_api:manage|Full access to Publishing REST APIs to a portal|
|ap:portal:rest_api:publish|Publish Portal Rest Api|
|ap:portal:rest_api:unpublish|Unpublish Portal Rest Api|
|ap:portal:update|Update an API Portal|
|ap:project:create|Create a project|
|ap:project:delete|Delete a project|
|ap:project:manage|Full access to projects|
|ap:project:read|Read projects|
|ap:project:update|Update a project|
|ap:publication:read|Read an API's publication status across every portal|
|ap:read|Read every Platform API resource|
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
