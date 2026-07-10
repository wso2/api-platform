# AI Workspace

The AI Workspace is a React/Vite SPA served by a **Go BFF (Backend-for-Frontend)** inside a Docker container. The BFF serves the SPA, proxies all browser→backend traffic same-origin, and owns authentication: tokens live in a server-side session (HttpOnly cookie) and never reach the browser. It communicates with the **Platform API** Go backend and supports two authentication modes: OIDC (production) and file-based/basic (quickstart). In OIDC mode the BFF is a confidential client and performs the code↔token exchange with the IDP — the UI never holds the client secret or a token.

---

## Quick links

| Topic | File |
|---|---|
| Get running locally in 5 minutes | [QUICKSTART.md](QUICKSTART.md) |
| Production setup (Asgardeo IDP) | [production/README.md](production/README.md) |
| Full runtime configuration reference | [configs/config-template.toml](configs/config-template.toml) |
| Platform API configuration reference | [configs/config-platform-api-template.toml](configs/config-platform-api-template.toml) |

---

## Technology stack

- **React** + **TypeScript** + **Vite**
- **Go BFF** (stdlib-only) — serves the SPA, reverse-proxies `/api/proxy/*` to the Platform API (injecting the session bearer token), and handles login/logout/session + the OIDC code flow
- **Docker Compose** — orchestrates AI Workspace + Platform API

---

## Auth modes

Controlled by `auth_mode` in `configs/config.toml` (or `VITE_AUTH_MODE` env var):

| Mode | When to use |
|---|---|
| `basic` | Quickstart / air-gapped — credentials defined in `config-platform-api.toml` |
| `oidc` | Production — delegates to an external OIDC-compliant IDP (e.g. Asgardeo) |

See [production/README.md](production/README.md) for the full OIDC setup walkthrough.

---

## Configuration

Runtime config is injected by the BFF at startup:

1. Values are read from the `config.toml` mounted at `/etc/ai-workspace/config.toml`.
2. Each key is mapped to a `VITE_*` env var (env vars already set take priority).
3. The BFF serves the browser-safe `VITE_*` values at `GET /runtime-config.js` (as `window.__RUNTIME_CONFIG__`) so the SPA can read them without a rebuild. Secrets (e.g. the OIDC client secret) are never emitted here.

The full key → `VITE_*` mapping and all available options are documented in
[configs/config-template.toml](configs/config-template.toml).

---

## Project layout

```
portals/ai-workspace/
├── configs/
│   ├── config-template.toml             # AI Workspace config reference
│   ├── config-platform-api-template.toml  # Platform API config reference
│   ├── config.toml                      # Active config (gitignored in prod)
│   └── config-platform-api.toml         # Active Platform API config
├── production/
│   └── README.md                        # Production setup guide (Asgardeo)
├── bff/                                # Go BFF — serves SPA, proxy, auth (stdlib-only)
│   ├── main.go
│   └── internal/{config,session,auth,proxy,server,tlsutil}
├── src/
│   ├── config.env.ts                    # Centralised env/runtime config reads
│   ├── contexts/BFFAuthProvider.tsx     # Single auth provider (hydrates from /api/session)
│   ├── App.tsx
│   └── ...
├── Dockerfile                          # 3-stage: SPA build → BFF build → runtime
├── QUICKSTART.md                        # Local setup guide
└── docker-compose.yaml
```

---

## Local development

### Option 1

```bash
# Build images
cd portals/ai-workspace && make build

# Optional
# cd platform-api && make build --> Update docker-compose file in portals/ai-workspace folder, with the new build tag

# Start the stack
docker compose up -d
```

The stack exposes:

| Service | Port | Protocol |
|---|---|---|
| AI Workspace (Go BFF) | `5380` | HTTPS |
| Platform API | `9243` | HTTPS |


### Option 2

Open three separate terminals and run the following commands.

Terminal 1:
```bash
cd portals/ai-workspace
npm run dev
```
This starts the AI Workspace frontend in development mode.

Update platform-api/config/config.toml and set the following configuration:

```bash
[auth.file_based]
enabled = true
```
This enables file-based authentication, allowing users configured in the file-based authentication settings to log in.

Terminal 2:
```bash
cd platform-api
go run ./cmd/main.go -config ./config/config.toml
```
This starts the Platform API using the local configuration file.

Terminal 3:
```bash
cd portals/ai-workspace
make bff-run            # serves /api/* on https://localhost:8081, proxies to the Platform API
```
The Vite dev server proxies `/api` and `/runtime-config.js` to the BFF, so the browser
talks only to the app origin (same topology as production). Set `PLATFORM_API_URL` if the
Platform API is not on `https://localhost:9243`.

Ensure all three services are running before accessing the application.

> **Browser trust warning?** Both services use a self-signed TLS certificate by default. Click **Advanced → Proceed** to continue, then return to the workspace. See [Custom TLS certificates](#custom-tls-certificates) to remove the warning permanently.

---

## Testing with an IDP locally

The stack works with **any OIDC-compliant IDP** (Asgardeo, Keycloak, Auth0, Okta, …). This
guide uses **Asgardeo as the worked example**; the compose wiring itself is provider-agnostic —
you supply your IDP's issuer, JWKS URL and confidential-client credentials.

By default the stack runs in `basic` (file-based) auth mode. In OIDC mode the **BFF is a
confidential client**: it runs the authorization-code + PKCE flow back-channel, holds the
client secret and tokens in a server-side session, and injects the access token on every
`/api/proxy/*` call. The browser never sees a token or the secret.

The flow touches **two** components and both must agree on the IDP:

- **BFF** — runs the login/code exchange and stores the tokens (needs the client *secret*).
- **Platform API** — validates the injected access token via the IDP's JWKS and authorizes
  by its `ap:*` scopes. If this half is misconfigured, login succeeds but every proxied call
  returns `403`.

### 1. Register a confidential application in your IDP

> Register the AI Workspace as a **confidential client** — in Asgardeo this is a
> **Standard-Based Application → OpenID Connect** (a.k.a. *Traditional Web Application*); in
> Keycloak a client with *Client authentication* ON; etc. Do **not** register a public /
> Single-Page Application. The BFF performs a back-channel exchange with a client secret; a
> public-client registration is rejected by the token endpoint with *"The authenticated client
> is not authorized to use the requested grant type."*

For Asgardeo, on the application's **Protocol** tab:

| Setting | Value |
|---|---|
| Allowed grant types | ☑ **Code** and ☑ **Refresh Token** |
| Authorized redirect URL | `https://localhost:5380/api/auth/callback` |
| Client authentication | a confidential method (default `client_secret_basic` works; the BFF also sends the secret in the body) |
| PKCE | enabled (the BFF always sends `S256`) |
| Access Token Type | `JWT` |

The BFF requests the **`offline_access`** scope (included by default) so the IDP returns a
refresh token. Enabling the Refresh Token *grant* is necessary but not sufficient — without
the `offline_access` *scope* most IDPs (Asgardeo, WSO2 IS, Okta, Azure AD) return no refresh
token, and the user is logged out the moment the access token expires. If your IDP gates
`offline_access` behind a consent/allowed-scope list, make sure it's permitted for this app.

Copy the **Client ID** and **Client Secret** — you'll need both below.

> The redirect URL is the **BFF callback** (`/api/auth/callback`), not the SPA `/signin`
> route. It is the same `https://localhost:5380` origin for both run modes below, because the
> Vite dev server and the compose stack both serve the app on port `5380`.

### 2. Register the `ap:*` scopes and grant them to your user

The Platform API authorizes by `ap:*` scopes. With file-based auth the `admin` user gets them
all hard-coded; with an IDP the **access token must carry them**, or proxied calls 403.

1. Register the Platform API resource and all `ap:*` scopes in Asgardeo using the helper
   script (its defaults target local `https://localhost:9243`):

   ```bash
   ASGARDEO_TENANT=<your-tenant> \
   ASGARDEO_CLIENT_ID=<system-app-client-id> \
   ASGARDEO_CLIENT_SECRET=<system-app-client-secret> \
   ./production/scripts/register_asgardeo_scopes.sh
   ```

2. On your application, add that API resource, create a role (e.g. `ap_admin`) with the
   `ap:*` scopes, and assign the role to your test user.

See [production/README.md §1](production/README.md) for the full Asgardeo org/attribute/scope
walkthrough.

### 3. Enable OIDC

#### Option 1 (docker compose) — recommended

`docker-compose.yaml` ships the OIDC env blocks pre-written but commented out. To enable OIDC,
**uncomment the `OIDC` block on both services** (`platform-api` and `ai-workspace`) and put your
IDP's values in a `.env` file next to `docker-compose.yaml`:

```bash
# portals/ai-workspace/.env — generic OIDC (replace with your IDP's URLs/credentials)
OIDC_ISSUER=https://idp.example.com/oauth2/token
OIDC_JWKS_URL=https://idp.example.com/oauth2/jwks
OIDC_CLIENT_ID=<your-client-id>
OIDC_CLIENT_SECRET=<your-client-secret>
OIDC_AUDIENCE=<your-client-id>          # optional; omit to skip the aud check
# OIDC_ORG_ID_CLAIM=org_id              # set only if your IDP names the org claim differently

# Asgardeo example (tenant "acme"):
#   OIDC_ISSUER=https://api.asgardeo.io/t/acme/oauth2/token
#   OIDC_JWKS_URL=https://api.asgardeo.io/t/acme/oauth2/jwks
#   OIDC_ORG_ID_CLAIM=org_id            # Asgardeo names the org UUID claim org_id
```

```bash
docker compose up -d
```

The redirect URLs and `ap:*` scopes are pre-filled in the compose blocks, so `.env` only needs
your IDP endpoints and client credentials. Leave the OIDC blocks commented (the default) to keep
the zero-config file-based quickstart (`admin` / `admin`).

> The Platform API and BFF auth modes are mutually exclusive: enabling the IDP while local JWT
> or file-based auth is also on is rejected at startup. The commented `platform-api` block already
> sets `AUTH_JWT_ENABLED=false` and `AUTH_FILE_BASED_ENABLED=false` — keep those uncommented too.

#### Option 2 (local `make bff-run`)

Running the BFF and Platform API directly (no compose) means no pre-wiring, so configure both
sides by hand.

Edit `platform-api/config/config.toml` so the Platform API validates the Asgardeo token —
remember the three auth modes are mutually exclusive, so turn the local ones off:

```toml
[auth.jwt]
enabled = false            # required: mutually exclusive with the IDP

[auth.idp]
enabled  = true
jwks_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
issuer   = ["https://api.asgardeo.io/t/<your-tenant>/oauth2/token"]
audience = ["<your-client-id>"]   # match Asgardeo's aud, or [] to skip the check

# Asgardeo emits org_id (not the default "organization") — these overrides are required.
[auth.idp.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"

[auth.file_based]
enabled = false            # required: mutually exclusive with the IDP
```

Then export the BFF settings and start it. Point it at the locally published Platform API port
— the `platform-api` compose hostname does **not** resolve outside the compose network:

> `OIDC_ISSUER` is Asgardeo's **token base** — the BFF appends
> `/.well-known/openid-configuration` itself, so do **not** include the discovery suffix.

```bash
cd portals/ai-workspace
export PLATFORM_API_URL=https://localhost:9243        # NOT https://platform-api:9243 when run locally
export PLATFORM_API_TLS_SKIP_VERIFY=true
export AUTH_MODE=oidc
export OIDC_ISSUER=https://api.asgardeo.io/t/<your-tenant>/oauth2/token
export OIDC_CLIENT_ID=<your-client-id>
export OIDC_CLIENT_SECRET=<your-client-secret>
export OIDC_REDIRECT_URL=https://localhost:5380/api/auth/callback
export OIDC_POST_LOGOUT_REDIRECT_URL=https://localhost:5380/login
# Keep `offline_access` — without it the IDP issues no refresh token and the BFF cannot silently renew the session.
export OIDC_SCOPES="openid profile email offline_access ap:organization:manage ap:gateway:manage ap:rest_api:manage ..."
make bff-run
```

### Verify and troubleshoot

`GET /api/session` returning `200` after the Asgardeo redirect means login worked. Common
failures, by symptom:

| Symptom | Cause | Fix |
|---|---|---|
| `unauthorized_client` / *"not authorized to use the requested grant type"* | App registered as SPA, or Code/Refresh grant not enabled | Recreate as Standard-Based OIDC app; enable **Code** + **Refresh Token** (step 1) |
| Platform API exits at startup with *"auth.idp.enabled=true and auth.jwt.enabled=true are mutually exclusive"* | Local auth left on alongside the IDP | Compose: uncomment the full `platform-api` OIDC block (it sets `AUTH_JWT_ENABLED=false` + `AUTH_FILE_BASED_ENABLED=false`). Local: set `auth.jwt.enabled=false` and `auth.file_based.enabled=false` (step 3, Option 2) |
| `502` + `dial tcp: lookup platform-api: no such host` | BFF run locally but `PLATFORM_API_URL` points at the compose hostname | Set `PLATFORM_API_URL=https://localhost:9243` (step 3, Option 2) |
| Proxied calls return `authentication_failed` | Platform API still on local JWT/file-based, validating the IDP token with the wrong validator | Switch it to the IDP — compose: uncomment the `platform-api` OIDC block; local: enable `[auth.idp]` (step 3, Option 2) |
| Proxied calls return `authentication_failed`, Platform API logs `token contains an invalid number of segments` | IDP is issuing **opaque** access tokens — the BFF forwards the access token and the Platform API can only validate a **JWT** via JWKS | Set **Access Token Type = JWT** on the app's Protocol tab (step 1) and re-login |
| Login works, then proxied calls return `403` | Access token lacks `ap:*` scopes, or Platform API IDP/claim mapping wrong | Grant `ap:*` scopes to the user (step 2); check `[auth.idp]` issuer/JWKS/claim mappings |
| User shows as a UUID and email is blank in the UI | Token carries no name/email claims — the BFF falls back to the `sub` (user UUID) | Release the `given_name` (or `name`/`preferred_username`) and `email` claims to the app and ensure the user has those attributes set; the `profile` and `email` scopes must be granted (both are in the default request) |
| Logged out as soon as the access token expires; never silently refreshed | IDP returned no refresh token — `offline_access` scope missing from the request, or not permitted for the app | Keep `offline_access` in `OIDC_SCOPES` (it's in the default); allow it for the app in the IDP (step 1) |
| Refresh fails minutes after login | **Refresh Token** grant not enabled on the app | Enable it on the Protocol tab (step 1) |

## Session lifetime & token refresh

The browser holds no token — it carries only an HttpOnly session cookie. Tokens live in the
BFF's server-side session, and the BFF renews them transparently.

**OIDC mode.** When a proxied request arrives with an access token within 60 s of expiry, the
BFF refreshes it server-side (single-flight per session) before forwarding — invisible to the
user. The session therefore stays alive as long as the user is active and the refresh token is
valid. It ends when:

- the **refresh token expires** (set by the IDP) — this is the effective *idle* bound: a user
  away longer than the refresh-token lifetime is asked to log in again; or
- the **absolute cap** is reached (`SESSION_ABSOLUTE_TTL`, default `8h`) — a hard ceiling that
  does *not* slide on refresh, so even a continuously-active session is re-authenticated once
  per cap window.

Tune the experience with the **access-token lifetime** (shorter = more frequent silent
refreshes) and the **refresh-token lifetime** (longer = users stay logged in across longer
idle gaps) in your IDP, plus `SESSION_ABSOLUTE_TTL` on the BFF.

**File-based / quickstart mode** has no refresh token: the session lasts until the issued
token expires (or the absolute cap), then the user logs in again. This is expected for the
quickstart and needs no setup.

When a session finally ends, the next API call returns `401` and the UI shows a clear
"session expired" prompt with a re-login action — it does not silently fail or loop.

> **Note:** `SESSION_IDLE_TIMEOUT` is currently **not enforced** (reserved for future use).
> The idle bound is the IDP refresh-token lifetime described above, not this setting.

> **High availability (future):** silent refresh is single-flighted per process, which is
> correct for the single-replica distribution. Before running **multiple BFF replicas**,
> configure the IDP to issue **non-rotating refresh tokens** for this client (or a short
> rotation reuse-grace window). Otherwise two replicas refreshing the same session
> concurrently can rotate each other's refresh token out and drop the session. See the BFF
> source comments on `refreshSession` for details.

## Cypress E2E tests

The repository includes Cypress E2E coverage for the local quickstart stack, including the basic login flow and AI Workspace UI CRUD flows.

```bash
cd portals/ai-workspace
docker compose up -d
npm install
```

Use the following commands after the stack is up:

- Headless run in the official Cypress Docker image:
  ```bash
  npm run test:e2e
  ```
- Interactive Cypress UI against `https://localhost:5380`:
  ```bash
  npm run test:e2e:open
  ```
- Alternative interactive command:
  ```bash
  make e2e-open
  ```

`npm run test:e2e` runs against `https://host.docker.internal:5380`, which maps back to your local quickstart stack from inside the Cypress container. The command adds an explicit `host-gateway` mapping so it also works on Linux Docker hosts. `npm run test:e2e:open` runs locally against `https://localhost:5380`.

The quickstart login used by the tests is:

- Username: `admin`
- Password: `admin`

---

## Custom TLS certificates (optional)

Mount your own certificate to remove the browser trust warning.

1. Create a `certs/` directory next to `docker-compose.yaml`.
2. Place your certificate files there:
   ```
   certs/
   ├── ai-workspace.crt
   ├── ai-workspace.key
   ├── platform-api.crt
   └── platform-api.key
   ```
3. Uncomment the TLS volume lines in `docker-compose.yaml` under each service.
4. Restart the stack: `docker compose up -d`

---

## Production hardening (`APIP_DEMO_MODE`)

The stack ships in **demo mode** so the quickstart is zero-config: file-based auth
(`admin` / `admin`) works out of the box and both services fall back to an auto-generated
self-signed TLS certificate. `APIP_DEMO_MODE` controls this — it **defaults to `true`**, and
only an explicit `false` (or `0`) opts into production-grade startup checks.

A single `APIP_DEMO_MODE` drives the **whole stack**: `docker-compose.yaml` passes it to both
the `platform-api` and `ai-workspace` services, so set it once in your shell or `.env`:

```bash
# portals/ai-workspace/.env
APIP_DEMO_MODE=false
```

When `APIP_DEMO_MODE=false`, startup is **stricter on both services** and will **fail fast**
rather than run insecurely:

| Service | Demo mode (`true`, default) | Production (`false`) |
|---|---|---|
| **AI Workspace (BFF)** — auth | Basic / file-based auth allowed | Basic auth **rejected** — OIDC required (`VITE_AUTH_MODE=oidc` + the `OIDC_*` values) |
| **AI Workspace (BFF)** — TLS | Auto-generates a self-signed cert when none is mounted | Self-signed fallback **disabled** — a cert/key must be mounted (`BFF_TLS_CERT_FILE` / `BFF_TLS_KEY_FILE`) |
| **Platform API** — secrets | Generates an ephemeral encryption key when none is set | A stable key is **required** (`PLATFORM_SECRET_ENCRYPTION_KEY` or `DATABASE_ENCRYPTION_KEY`) |

So before flipping `APIP_DEMO_MODE=false`, make sure you have:

1. **OIDC configured on both services** — follow [Testing with an IDP locally](#testing-with-an-idp-locally) (uncomment the OIDC blocks on both compose services and set the `OIDC_*` values). Basic auth is no longer a fallback.
2. **A real TLS certificate mounted** on the BFF (and the Platform API) — follow [Custom TLS certificates](#custom-tls-certificates-optional) above and uncomment the cert volume lines. The self-signed fallback is gone.
3. **A stable secret encryption key** for the Platform API — set `PLATFORM_SECRET_ENCRYPTION_KEY=$(openssl rand -hex 32)` in your `.env` (otherwise encrypted secrets become unreadable after a restart). See [platform-api/README.md](../../platform-api/README.md).

If any of these is missing, the corresponding service exits at startup with a message naming
exactly what to provide.
