# Secrets Management

AI Workspace lets you store sensitive credentials as **secrets** and reference them securely in artifact configurations. Secrets are encrypted at rest using AES-GCM-256. Plaintext values are never written to the database and are **never returned in any API response** — not even the creation response.

Use secrets to avoid embedding raw API keys, tokens, or passwords directly in LLM provider configurations, MCP proxy configs, or API backend settings.

## How It Works

1. Create a secret via the Platform API with a unique `handle` and the plaintext `value`.

2. Reference the secret in any artifact configuration using the placeholder syntax:

```text
{{ secret "your-secret-handle" }}
```

3. When an artifact that contains a placeholder is deployed, the gateway resolves it with the decrypted value at runtime — the plaintext never appears in the control-plane database or configuration files.

4. To rotate a credential, call `PUT /api/v0.9/secrets/{handle}` with the new value. Because artifacts reference the secret by handle, no artifact changes or redeployment are required.

## Auto-Encryption in the AI Workspace UI

When you create or update an **LLM Provider** or **MCP Proxy** through the AI Workspace UI and fill in an upstream API key or auth value, the UI automatically:

1. Creates a secret via `POST /api/v0.9/secrets` using a deterministic handle derived from the resource ID (e.g. `wso2-openai-provider-api-key`).

2. Substitutes the credential with the `{{ secret "handle" }}` placeholder before saving the resource.

The raw credential is sent to the secrets API only once and is never stored in the artifact configuration. Re-saving a resource that already contains a placeholder skips the secret creation step.

## API Reference

The secrets API is available at `/api/v0.9/secrets`. All requests require a valid JWT with the appropriate scope.

### Required Scopes

| Scope | Grants |
|-------|--------|
| `ap:secret:read` | List secrets and get metadata by handle |
| `ap:secret:create` | Create a new secret |
| `ap:secret:update` | Rotate / update an existing secret value |
| `ap:secret:delete` | Delete a secret |
| `ap:secret:manage` | All of the above |

---

### Create a Secret

```http
POST /api/v0.9/secrets
Content-Type: multipart/form-data
```

Stores a new encrypted secret. The plaintext value is never returned — not even in this response.

> **Why multipart/form-data?**
> The `value` field is designed to carry arbitrary binary content in a future release (e.g. TLS certificates, SSH private keys). Using `multipart/form-data` now avoids a breaking content-type change when file-based secrets are introduced.

**Request fields**

| Field | Required | Description |
|-------|----------|-------------|
| `handle` | Yes | Unique identifier within the organization. Used in `{{ secret "handle" }}` references. Immutable after creation. |
| `name` | Yes | Human-readable display name for the secret. |
| `value` | Yes | The sensitive value to encrypt and store. |
| `description` | No | Optional description. |
| `type` | No | Secret type. `GENERIC` (default) for API keys and tokens. |

**Example request**

```http
POST /api/v0.9/secrets
Authorization: Bearer <JWT>
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="handle"

wso2-openai-key
------FormBoundary
Content-Disposition: form-data; name="name"

WSO2 OpenAI API Key
------FormBoundary
Content-Disposition: form-data; name="description"

API key for WSO2 OpenAI integration
------FormBoundary
Content-Disposition: form-data; name="type"

GENERIC
------FormBoundary
Content-Disposition: form-data; name="value"

sk-xxx
------FormBoundary--
```

**Response — 201 Created**

```json
{
  "uuid": "a1b2c3d4-...",
  "handle": "wso2-openai-key",
  "name": "WSO2 OpenAI API Key",
  "createdAt": "2026-01-12T10:00:00Z",
  "updatedAt": "2026-01-12T10:00:00Z"
}
```

The response does not include the `value`. Store the plaintext in a secure location before submitting it — it cannot be retrieved later.

**Error responses**

| Status | Reason |
|--------|--------|
| 400 | Missing required fields or invalid request |
| 409 | A secret with the same `handle` already exists in the organization |

---

### List Secrets

```http
GET /api/v0.9/secrets
```

Returns metadata for all secrets in the organization. Plaintext values are never included.

**Query parameters**

| Parameter | Default | Max | Description |
|-----------|---------|-----|-------------|
| `limit` | 25 | 100 | Maximum number of results to return |
| `offset` | 0 | — | Number of results to skip for pagination |

**Response — 200 OK**

```json
{
  "list": [
    {
      "uuid": "a1b2c3d4-...",
      "handle": "wso2-openai-key",
      "name": "WSO2 OpenAI API Key",
      "type": "GENERIC",
      "provider": "IN_BUILT",
      "createdAt": "2026-01-12T10:00:00Z",
      "updatedAt": "2026-01-12T10:00:00Z"
    }
  ],
  "pagination": {
    "total": 1,
    "limit": 25,
    "offset": 0
  }
}
```

---

### Get Secret Metadata

```http
GET /api/v0.9/secrets/{handle}
```

Returns metadata for a single secret. The plaintext value is not included.

**Response — 200 OK**

```json
{
  "uuid": "a1b2c3d4-...",
  "handle": "wso2-openai-key",
  "name": "WSO2 OpenAI API Key",
  "type": "GENERIC",
  "provider": "IN_BUILT",
  "createdAt": "2026-01-12T10:00:00Z",
  "updatedAt": "2026-01-12T10:00:00Z"
}
```

**Error responses**

| Status | Reason |
|--------|--------|
| 404 | No secret found with the given handle in this organization |

---

### Rotate a Secret

```http
PUT /api/v0.9/secrets/{handle}
Content-Type: multipart/form-data
```

Re-encrypts and stores a new value. Because `handle` is immutable, all `{{ secret "handle" }}` placeholders across existing resources remain valid without modification. The plaintext value is not returned in the response.

**Request fields**

| Field | Required | Description |
|-------|----------|-------------|
| `value` | Yes | The new sensitive value to encrypt and store. |
| `name` | No | Updated display name. |
| `description` | No | Updated description. |

**Example request**

```http
PUT /api/v0.9/secrets/wso2-openai-key
Authorization: Bearer <JWT>
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="value"

sk-new-value
------FormBoundary
Content-Disposition: form-data; name="name"

WSO2 OpenAI API Key (rotated)
------FormBoundary
Content-Disposition: form-data; name="description"

Rotated on 2026-06-26 — old key decommissioned
------FormBoundary--
```

**Response — 200 OK**

```json
{
  "uuid": "a1b2c3d4-...",
  "handle": "wso2-openai-key",
  "name": "WSO2 OpenAI API Key (rotated)",
  "createdAt": "2026-01-12T10:00:00Z",
  "updatedAt": "2026-06-26T11:30:00Z"
}
```

**Notes**

- If the secret's status is `DEPRECATED` (previously soft-deleted), a successful rotation sets it back to `ACTIVE`. The secret can again be referenced by new resources and will be included in gateway sync.
- The gateway picks up the updated value on its next sync cycle — no redeployment of referencing artifacts is required.

**Error responses**

| Status | Reason |
|--------|--------|
| 404 | No secret found with the given handle |

---

### Delete a Secret

```http
DELETE /api/v0.9/secrets/{handle}
```

Soft-deletes a secret by setting its status to `DEPRECATED`. Deletion is blocked with `409 Conflict` if the secret is currently referenced by any artifact — either in its saved configuration or in a snapshot currently deployed to a gateway.

**Response — 204 No Content**

**Error responses**

| Status | Reason |
|--------|--------|
| 404 | No secret found with the given handle |
| 409 | Secret is still referenced by one or more artifacts |

**409 response example**

```json
{
  "error": "secret is referenced by active resources",
  "references": [
    { "type": "llm_provider", "handle": "openai-provider", "name": "OpenAI Provider" },
    { "type": "mcp_proxy", "handle": "my-mcp-proxy", "name": "My MCP Proxy" }
  ]
}
```

---

## Referencing Secrets in Artifact Configurations

Use the following placeholder syntax wherever a configuration field accepts a sensitive string value:

```text
{{ secret "your-secret-handle" }}
```

**Example — LLM provider upstream API key**

```yaml
spec:
  upstream:
    auth:
      type: api-key
      header: Authorization
      value: 'Bearer {{ secret "wso2-openai-key" }}'
```

**Validation at save time**

When creating or updating any resource that contains `{{ secret "..." }}` references, the Platform API validates that an active secret with the referenced handle exists in the organization. If any placeholder cannot be resolved, the request is rejected with `400 Bad Request` and the list of unresolvable handles.

---

## Credential Rotation

To rotate a credential without touching artifact configurations:

1. Call `PUT /api/v0.9/secrets/{handle}` with the new value.
2. The gateway picks up the updated secret on the next sync cycle.

No artifact changes or redeployment are required because resources reference the secret by handle, not by value.

---

## Deleting a Secret Safely

Attempting to delete a secret that is still in use returns HTTP 409. To remove it cleanly:

1. Inspect the `references` list in the 409 response.
2. Update each referencing artifact to remove or replace the `{{ secret "handle" }}` reference.
3. Redeploy the updated artifacts to the gateway.
4. Retry `DELETE /api/v0.9/secrets/{handle}`.

---

## Configuration

The following environment variable controls encryption for the Platform API:
**required and never auto-generated** — the Platform API fails to start if missing or malformed:

| Key | Purpose |
|-----|---------|
| `ENCRYPTION_KEY` | Encrypts secrets, subscription tokens, and WebSub HMAC secrets at rest. |

Generate a stable key with:

```sh
openssl rand -hex 32
```

For Docker Compose deployments, set the key in a `.env` file next to `docker-compose.yaml`. First generate a key:

```sh
openssl rand -hex 32
# example output: a3f1e2d4b5c6...
```

### How keys are provided

Never write a raw key value into `config-platform-api.toml`, and never hardcode one as
a literal env var in `docker-compose.yaml`. Instead, the config file **references** each
key with a Go-template token that is resolved at startup — from an environment variable
or, preferably, from a mounted file:

```toml
# config-platform-api.toml
encryption_key = '{{ env "ENCRYPTION_KEY" }}'          # from an environment variable

# or

encryption_key = '{{ file "/secrets/platform-api/encryption_key" }}'   # from a mounted file (preferred)
```

Resolution fails closed: if a referenced env var is unset/empty or a referenced file is
missing (or outside the allowed directories), the Platform API refuses to start.

**Preferred — mounted files (`{{ file "..." }}`).** Mount each secret as a file (for
example a Docker/Kubernetes secret) under an allowed directory (`/etc/platform-api` or
`/secrets/platform-api`; override with `APIP_CONFIG_FILE_SOURCE_ALLOWLIST`) and reference
it with `{{ file "..." }}`. The value never appears in the environment or the compose file.

**Simple — an `.env` file (`{{ env "..." }}`).** For local/quickstart Docker Compose, put
the values in a `.env` file next to `docker-compose.yaml` (it is git-ignored) and let the
compose file inject them into the container via `env_file`:

```sh
cp .env.example .env
# then, in .env:
AUTH_JWT_SECRET_KEY=$(openssl rand -hex 32)
ENCRYPTION_KEY=$(openssl rand -hex 32)
```

> **Warning:** Use the **same** stable `ENCRYPTION_KEY` across restarts and across all replicas. Changing it makes existing encrypted secrets unreadable.
