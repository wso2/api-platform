# Manage the Organization

An organization owns an API catalog, its applications, subscriptions, and branding. **A Developer Portal instance serves exactly one organization**, named by `organization.handle` in its configuration. That handle appears in every portal URL (`/<orgHandle>/views/<viewName>`).

The database schema is still multi-organization, so one shared database can hold many organizations — but each is served by its own portal instance, and an instance rejects anything belonging to another one:

| Surface | A different organization gets |
|---|---|
| Page URL (`/<orgHandle>/...`) | `404 Not Found` |
| `organization` request header on an API-key call | `403 Forbidden` |
| Organization claim in a session or bearer token | `403 Forbidden` (login itself is refused) |
| `GET`/`PUT /organizations/{orgId}` | `403 Forbidden` |

An organization that doesn't exist gets the same `403` as one that does but isn't this portal's, so the responses can't be used to discover which organizations the shared database holds.

## Configure the Organization

```toml
[api_portal.organization]
handle       = "acme"     # URL slug: /acme/views/default
display_name = "Acme Corp"
```

`handle` is **required** — the portal refuses to start without it, since a single-organization portal that doesn't know its organization would reject every request. (Design mode is the exception: it renders from disk and has no organization to pin.)

Both values also accept environment references, which is how a containerized deployment sets them:

```toml
handle       = '{{ env "APIP_DP_ORGANIZATION_HANDLE" "default" }}'
display_name = '{{ env "APIP_DP_ORGANIZATION_DISPLAY_NAME" "Default" }}'
```

> `organization.default_name` is the **deprecated** name for `handle`. It still works, with a warning at startup — rename it.

### Matching the Platform API in local auth mode

With `auth.mode = "local"`, `handle` **must** match the Platform API's `[platform_api.auth.file.organization].id`. That value is what the Platform API puts in the `org_handle` claim of the tokens this portal verifies, so a mismatch means every login is refused with a generic credential error (the specific reason is logged server-side only).

In `auth.mode = "idp"`, the organization claim your IDP asserts must resolve to this organization — matched against its handle, display name, or `idpRefId`.

## Provisioning

The organization is created on startup if it doesn't already exist, along with a `default` label, a `default` view, and the default subscription plans (unless `auto_create_subscription_plans = false`). The seeder is idempotent: an existing organization missing any of those defaults gets them added, and its `display_name` is **not** overwritten — so an edit made through the settings UI survives restarts.

Because provisioning is the seeder's job, the organization lifecycle is not client-driven. These operations return `405 Method Not Allowed`:

| Operation | Why |
|---|---|
| `POST /organizations` | The seeder owns creation |
| `GET /organizations` | Listing is inherently cross-organization |
| `DELETE /organizations/{orgId}` | An instance is bound to its organization for its whole lifetime |

They remain in the OpenAPI spec, and the code behind them is intact, so they can be re-enabled without an API change.

## Read the Organization

```bash
curl -k https://localhost:9543/api/v0.9/organizations/acme -H "Authorization: Bearer $TOKEN"
```

## Update the Organization

```yaml
# org-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha2
kind: Organization

metadata:
  name: acme

spec:
  displayName: Acme Corporation
  businessOwner: New Owner
  businessOwnerEmail: new-owner@acme.com
```

```bash
curl -k -X PUT https://localhost:9543/api/v0.9/organizations/acme \
  -H "Authorization: Bearer $TOKEN" \
  -F "organization=@org-update.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | The org handle. **Immutable** — must equal `organization.handle`; any other value returns `400` |
| `spec.displayName` | Yes | Human-friendly organization name shown in the portal UI |
| `spec.idpRefId` | No | The org claim value asserted by your Identity Provider at SSO login. **Immutable** — changing it returns `400` |
| `spec.cpRefId` | No | Control Plane reference ID, included in outbound webhook event payloads. Not used for authentication |
| `spec.businessOwner` | No | Contact name for the organization owner |
| `spec.businessOwnerContact` | No | Business owner's phone or contact string |
| `spec.businessOwnerEmail` | No | Business owner's email address |
| `spec.labels` | No | Labels to upsert (array of `{name, displayName}`) |
| `spec.views` | No | Views to upsert (array of `{handle, name, labels}`) |

The handle and `idpRefId` are immutable because they are what page URLs and incoming token organization claims are matched against. Renaming either would leave the running instance unable to find its own organization — every page returning `404` and every login `403` — until an operator edited the configuration to match.

---

## Local Auth (Development Only)

For local development and first-time setup, the portal ships with a built-in username/password login form. Credentials are validated by a Platform API sidecar — the Developer Portal never handles raw passwords directly.

### How it works

1. The user submits the login form.
2. The Developer Portal forwards the credentials to the Platform API (`POST /api/portal/v0.9/auth/login`).
3. The Platform API verifies the bcrypt-hashed password and returns a signed JWT containing `dp:*` scopes.
4. The Developer Portal stores the JWT in the server-side session and uses the scopes for all subsequent authorization checks.

### Configuration

Users and their scopes are defined in `configs/config-platform-api.toml`. Running `./scripts/setup.sh` (see the [Quick Start](../introduction/quick-start.md)) generates this file with a single admin user. To edit it by hand — or to create a static, no-dependencies starting point without the setup script — copy the template instead:

```bash
cp configs/config-platform-api-template.toml configs/config-platform-api.toml
```

Add or modify users in the `[[auth.file_based.users]]` sections:

```toml
[[auth.file_based.users]]
username      = "admin"
password_hash = "$2y$10$..."   # bcrypt hash — see below
scopes        = "dp:org_read dp:org_manage dp:api_read dp:api_manage ..."

[[auth.file_based.users]]
username      = "developer"
password_hash = "$2y$10$..."
scopes        = "dp:api_read dp:app_read dp:app_write dp:subscription_read"
```

Generate a bcrypt password hash with:

```bash
htpasswd -bnBC 12 "" <password> | tr -d ':\n'
```

### Scope-based authorization

Every devportal REST API operation requires a specific `dp:*` scope. Users without the required scope receive a `403 Forbidden` response. Common scope sets:

| Access level | Scopes to grant |
|---|---|
| Full admin | All `dp:*_manage` scopes + `dp:*_read` |
| API publisher | `dp:api_manage dp:api_content_manage dp:org_read dp:label_read` |
| Developer / subscriber | `dp:api_read dp:app_read dp:app_write dp:subscription_read dp:subscription_write` |

See `configs/config-platform-api-template.toml` for the complete scope list used by the default admin user.

### Session persistence and scripted access

`auth.jwt.private_key`/`public_key` are **mandatory**: the Platform API requires the RS256 keypair at startup and never generates one — it fails to start if either is missing. `scripts/setup.sh` provisions the pair into `resources/keys/`. The Platform API signs with the private key; the devportal is pointed at only the public half via `auth.local.public_key_path` so it can verify Platform API-issued JWTs locally without a network round-trip (and so sessions survive restarts):

```bash
# Generated once by scripts/setup.sh — PEM files, not env vars (a multi-line PEM
# cannot live in an env file). Both config.tomls read them via {{ file "..." }}.
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out resources/keys/jwt_private.pem
openssl rsa -in resources/keys/jwt_private.pem -pubout -out resources/keys/jwt_public.pem
```

For scripts and CLI tools, get a Bearer token directly from the Platform API and pass it on each request — no session cookie required:

```bash
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=<admin-username>&password=<admin-password>" | jq -r .token)

curl -sk -H "Authorization: Bearer $TOKEN" https://localhost:9543/api/v0.9/organizations/acme
```

The token is verified locally by the Developer Portal against the Platform API's RS256 public key (`auth.local.public_key_path`), with no extra call to the Platform API per request.

> **Note:** Local auth is for development only. For production, set `auth.mode = "idp"` and configure the OIDC identity provider under `[api_portal.auth.idp]`.

---

## Default Organization

Out of the box `organization.handle` is `default`, so a fresh install comes up at `/default/views/default` with a `default` view and `default` label. Set `handle` and `display_name` before first boot to name it something else — the handle cannot be changed afterwards without also updating the configuration (see [Update the Organization](#update-the-organization)).
