# Authentication & Authorization in Gateway Controller

## Overview
The Gateway Controller REST API (the control-plane API used to manage gateway configuration) can be protected using either locally configured users (Basic Auth) or an external Identity Provider (JWT validation via JWKS). Authorization is role-based and enforced per API route.

## How It Works

### Authentication (Who are you?)
You can enable one (or both) of the following:

- **Basic Auth (local users)**: Define usernames/passwords and assign local roles.
- **IDP/JWT (external users)**: Validate incoming JWTs using `jwks_url` optionally `issuer`.

**No Authentication (open access)**: If BOTH `basic.enabled` and `idp.enabled` are set to `false`, all requests to the gateway controller are allowed without authentication.

### Authorization (Are you allowed?)
Gateway Controller routes are protected using **local roles** (for example `admin`, `developer`, `consumer`).

- If **`roles_claim` is NOT configured** in the IDP/JWT setup, **authorization is bypassed** for the Gateway Controller REST API routes (i.e., no role checks are performed).
- If **`roles_claim` IS configured**, you **must** also configure **`role_mapping`**. Without a mapping, the controller cannot translate IDP roles → local roles, and requests will be denied.

## Configuration
In the umbrella gateway config, these settings live under `controller.auth`. (If you run the controller standalone, the same structure applies under the controller's config root.)

### Option A: Basic Auth (local users)
```yaml
controller:
  auth:
    basic:
      enabled: true
      users:
        - username: "platform-admin"
          password: "$bcrypt$..."
          password_hashed: true
          roles: ["admin"]
        - username: "ops"
          password: "ops"
          password_hashed: false
          roles: ["developer"]
```

### Option B: JWT (IDP) with role-based authorization
```yaml
controller:
  auth:
    idp:
      enabled: true
      jwks_url: "https://idp.example.com/oauth2/jwks"
      issuer: "https://idp.example.com/oauth2/token"

      # Turn ON authorization by providing BOTH:
      roles_claim: "groups"   # e.g., "groups", "scope", "roles"
      role_mapping:
        admin: ["gateway-admins", "platform-admins"]
        developer: ["api-developers", "ops"]
        # Optional: give any authenticated user a baseline role
        consumer: ["*"]
```

## Role Mapping Semantics
`role_mapping` is defined as:

```text
local_role -> [idp_role_value_1, idp_role_value_2, ...]
```

Notes:
- **Specific mappings take precedence** over wildcard matches.
- **Wildcard (`"*"`)** means: if a JWT role value does not match any specific mapping, it can still map to the local role that includes `"*"`.
- **One JWT role can grant multiple local roles** by listing it under multiple local roles.
- **Wildcard mapping must be unique**: Do not configure more than one local role with `"*"` (for example `admin: ["*"]` and `consumer: ["*"]`). The Gateway Controller validates configuration and rejects multiple wildcard roles in `role_mapping`.

### Example: One IDP group grants multiple local roles
```yaml
role_mapping:
  admin: ["platform-admins"]
  developer: ["platform-admins", "api-developers"]
```
In this example, a user in `platform-admins` becomes both `admin` and `developer` in the Gateway Controller.

## Troubleshooting (What you’ll observe)
- **Requests are denied after enabling JWT auth**: verify `jwks_url` and (if set) `issuer` match the token you're sending.
- **You enabled `roles_claim` and suddenly everything is forbidden**: add `role_mapping` (mapping is mandatory when `roles_claim` is provided).
- **Users authenticate but don't have expected access**: confirm the token actually contains the configured `roles_claim`, and that its values match what you listed in `role_mapping`.
- **You want authN but not authZ**: keep IDP enabled, but leave `roles_claim` and `role_mapping` unset to bypass authorization checks.
- **You want to disable auth entirely**: set both `basic.enabled` and `idp.enabled` to `false`.

## Testing
Unit tests cover wildcard precedence, one-to-many role grants, and supported claim formats for `roles_claim`.
