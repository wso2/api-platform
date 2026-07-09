# Manage Organizations

An organization is the top-level multi-tenant unit in the Developer Portal. Each organization has its own API catalog, applications, subscriptions, and branding. The organization handle appears in every portal URL (`/<orgHandle>/views/<viewName>`).

## Create an Organization

Create an `org.yaml` file using the Organization manifest format:

```yaml
# org.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: Organization

metadata:
  name: acme                     # orgHandle — used in portal URLs

spec:
  displayName: Acme Corp
  idpRefId: ACME                 # value of the org claim for this org's users
  cpRefId: cp-ref-acme            # optional Control Plane reference ID
  businessOwner: Platform Team
  businessOwnerContact: "+1-202-555-0147"
  businessOwnerEmail: platform-team@acme.com

  labels:
    - name: default
      displayName: Default

  views:
    - handle: default
      name: Default View
      labels:
        - default
```

```bash
curl -k -X POST https://localhost:3000/api/v0.9/organizations \
  -H "Authorization: Bearer $TOKEN" \
  -F "organization=@org.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | URL-safe org handle used in all portal URLs (becomes `orgHandle`) |
| `spec.displayName` | Yes | Human-friendly organization name shown in the portal UI |
| `spec.idpRefId` | Yes | The org claim value asserted by your Identity Provider at SSO login. The portal matches an authenticated user's org claim against this value to resolve which organization they belong to — it must exactly match, or login fails for that org's users. |
| `spec.cpRefId` | No | Control Plane reference ID, included in outbound webhook event payloads. Not used for authentication. |
| `spec.businessOwner` | No | Contact name for the organization owner |
| `spec.businessOwnerContact` | No | Business owner's phone or contact string |
| `spec.businessOwnerEmail` | No | Business owner's email address |
| `spec.labels` | No | Labels to create for this org (array of `{name, displayName}`). Defaults to a single `default` label if omitted |
| `spec.views` | No | Views to create for this org (array of `{handle, name, labels}`). Defaults to a single `default` view if omitted |

After creation, the organization is accessible at `/<orgHandle>/views/<viewName>` once a view is created for it.

## List Organizations

```bash
curl -k https://localhost:3000/api/v0.9/organizations -H "Authorization: Bearer $TOKEN"
```

## Update an Organization

```yaml
# org-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: Organization

metadata:
  name: acme

spec:
  displayName: Acme Corporation
  businessOwner: New Owner
  businessOwnerEmail: new-owner@acme.com
```

```bash
curl -k -X PUT https://localhost:3000/api/v0.9/organizations/{orgId} \
  -H "Authorization: Bearer $TOKEN" \
  -F "organization=@org-update.yaml"
```

## Delete an Organization

```bash
curl -k -X DELETE https://localhost:3000/api/v0.9/organizations/{orgId} -H "Authorization: Bearer $TOKEN"
```

> **Warning:** Deleting an organization removes all of its views, APIs, subscriptions, and applications. This action is irreversible.

---

## Local Auth (Development Only)

For local development and first-time setup, the portal ships with a built-in username/password login form. Credentials are validated by a Platform API sidecar — the Developer Portal never handles raw passwords directly.

### How it works

1. The user submits the login form.
2. The Developer Portal forwards the credentials to the Platform API (`POST /api/portal/v0.9/auth/login`).
3. The Platform API verifies the bcrypt-hashed password and returns a signed JWT containing `dp:*` scopes.
4. The Developer Portal stores the JWT in the server-side session and uses the scopes for all subsequent authorization checks.

### Configuration

Users and their scopes are defined in `configs/config-platform-api.toml`. Copy the example file to get started:

```bash
cp configs/config-platform-api.toml.example configs/config-platform-api.toml
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

See `configs/config-platform-api.toml.example` for the complete scope list used by the default admin user.

### Session persistence and scripted access

The Platform API signs login JWTs with its `ENCRYPTION_KEY`. In demo mode this is auto-generated (and persisted next to the database) if unset; pin it so sessions survive restarts and set the **same value** in both services so the devportal can verify JWTs locally without a network round-trip:

```bash
# In .env (read by both services via docker-compose env_file / APIP_DP_* override)
ENCRYPTION_KEY=<64-hex-char-string>   # openssl rand -hex 32
```

For scripts and CLI tools, get a Bearer token directly from the Platform API and pass it on each request — no session cookie required:

```bash
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=admin&password=admin" | jq -r .token)

curl -sk -H "Authorization: Bearer $TOKEN" https://localhost:3000/api/v0.9/organizations
```

The token is verified locally by the Developer Portal using the shared `ENCRYPTION_KEY` (the Platform API's signing key) with no extra call to the Platform API per request.

> **Note:** Local auth is for development only. For production, configure the global OIDC identity provider via `APIP_DP_IDP_*` environment variables.

---

## Default Organization

When the portal starts (or via the Docker init scripts), a default organization named `ACME` with a `default` view and `default` label is created automatically. You can rename or reconfigure it after first boot.
