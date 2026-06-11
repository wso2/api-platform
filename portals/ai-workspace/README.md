# AI Workspace FrontEnd

The frontend implementation will cover the following key areas:
* **Client Logic**
* **Session Management**
* **User Interface (UI)**
* **User Experience (UX)**
---

## Technology Stack

The frontend will be built using the following technologies:

* **React**
* **Vite**
* **TypeScript**
* **Oxygen UI (new version)**
---

## Project file tree 📁

Overview of the AI Workspace source layout:
- All file names use camelCase by convention
- All component names use PascalCase by convention

```
workspaces/apps/ai-workspace/
├── README.md (Web application overview)
├── index.html (Web application entry HTML)
├── package.json (Web application dependencies)
├── src/ (Source code)
│   ├── App.tsx (Root application component)
│   ├── apis/ (Context-based API definitions)
│   │   └── projectApis.ts (Choreo project–related APIs)
│   │   └── etc
│   ├── auth/ (Authentication-related logic)
│   │   ├── logout.ts (Logout + session clear logic)
│   │   ├── mockUsers.config.ts (Mock users for local dev)
│   │   └── permissions.ts (Roles, scopes, checkPermission)
│   ├── clients/ (API client implementations)
│   │   └── choreoApiClient.ts (Choreo API client)
│   ├── contexts/ (React context providers)
│   │   ├── AppAuthContext.tsx (Auth context + useAppAuth hook)
│   │   ├── OIDCAppAuthProvider.tsx (OIDC auth provider)
│   │   ├── MockAuthProvider.tsx (Mock auth provider)
│   │   └── AppShellContext.tsx (Global AppShell context)
│   ├── pages/ (Application pages)
│   │   ├── appShell/
│   │   │   ├── appShellMain.tsx (AppShell entry point)
│   │   │   └── appShellPages/
│   │   │       ├── page1/
│   │   │       │   ├── main.tsx
│   │   │       │   ├── component1.tsx
│   │   │       │   └── component2.tsx
│   │   │       └── page2/
│   │   │           ├── main.tsx
│   │   │           ├── component1.tsx
│   │   │           └── component2.tsx
│   │   └── login/
│   │       ├── login.tsx (Login page)
│   │       └── signinCallback.tsx (Sign-in redirect page)
│   ├── utils/ (Shared utilities)
│   │   ├── cookies.ts (Cookie utilities)
│   │   └── types.ts (Shared type definitions)
│   ├── components/ (Reusable UI components)
│   │   ├── sharedComponent1.tsx 
│   │   └── sharedComponent2.tsx 
│   ├── main.tsx (Vite application entry point)
│   ├── config.env.ts (Centralized environment configuration)
│   └── styles.css (Global styles)
├── tsconfig.json (TypeScript configuration)
└── vite.config.ts (Vite configuration)
```

---

## Authentication

The app supports two auth modes controlled by the `VITE_DISABLE_AUTH` environment variable.

### OIDC Mode (default — `VITE_DISABLE_AUTH=false`)

Uses [`react-oidc-context`](https://github.com/authelia/react-oidc-context) with auto-discovery via `{VITE_OIDC_AUTHORITY}/.well-known/openid-configuration`. The app is configured to target **Bijira's Asgardeo SP** in production, but works with any OIDC-compliant provider.

| Env var | Purpose | Default |
|---|---|---|
| `VITE_OIDC_AUTHORITY` | IdP issuer URL | `https://localhost:8090` |
| `VITE_OIDC_CLIENT_ID` | OIDC client ID | _(empty)_ |
| `VITE_OIDC_REDIRECT_URI` | Post-login redirect | `https://{DOMAIN}/signin` |
| `VITE_OIDC_POST_LOGOUT_REDIRECT_URI` | Post-logout redirect | `https://{DOMAIN}/login` |
| `VITE_OIDC_END_SESSION_ENDPOINT` | Override end-session URL | _(auto-discovered)_ |
| `VITE_OIDC_SCOPE` | Space-separated OAuth2 scopes to request | _(all platform scopes)_ |
| `VITE_OIDC_IDP_HINT_PARAM` | Query param for federated IdP hint | `fidp` |
| `VITE_OIDC_ORG_CLAIM` | JWT claim carrying the org UUID | `organization` |
| `VITE_OIDC_USERNAME_CLAIM` | JWT claim for display name | `given_name` |
| `VITE_OIDC_EMAIL_CLAIM` | JWT claim for email | `email` |

Social/federated IdP hint values (passed as `{VITE_OIDC_IDP_HINT_PARAM}=<value>`):

| Env var | Default value |
|---|---|
| `VITE_SOCIAL_IDP_GOOGLE` | `google` |
| `VITE_SOCIAL_IDP_GITHUB` | `github` |
| `VITE_SOCIAL_IDP_MICROSOFT` | `microsoft` |
| `VITE_SOCIAL_IDP_ENTERPRISE` | `EnterpriseIDP` |

**Provider:** `OIDCAppAuthProvider` wraps `react-oidc-context`'s `useAuth` hook and exposes the unified `AppAuthContext`.

### Mock Auth Mode (`VITE_DISABLE_AUTH=true`)

Replaces OIDC with a local username/password login form. No IdP is needed — credentials are checked against `src/auth/mockUsers.config.ts` in memory.

**Default users:**

| Username | Password | Role |
|---|---|---|
| `admin` | `admin` | `admin` |
| `developer` | `developer` | `developer` |
| `viewer` | `viewer` | `viewer` |

Users are stored in `sessionStorage` and a mock JWT (alg: none) is minted locally. To add or change users, edit `MOCK_USERS` in `src/auth/mockUsers.config.ts`.

Set `VITE_DEV_ORG_ID` to the UUID of your seeded local org (must match the platform-api database).

**Provider:** `MockAuthProvider` exposes the same `AppAuthContext` shape as OIDC mode — components are auth-mode agnostic.

### Roles and Permissions

Five platform roles are defined in `src/auth/permissions.ts`:

| Role | Description |
|---|---|
| `admin` | Full manage access to all resources |
| `developer` | Manage projects, apps, proxies; no gateway management |
| `publisher` | Publish APIs, manage dev portals; read-only on most resources |
| `operator` | Manage gateways and deployments; read-only on others |
| `viewer` | Read-only across all resources |

Permission resolution is controlled by `VITE_PERMISSION_MODE`:
- **`scope`** (default) — the raw OAuth2 scopes from the JWT access token are used directly.
- **`role`** — the `platform_role` (or `role`) JWT claim is used to expand to a predefined scope set from `ROLE_SCOPES`.

Use the `hasPermission(scope)` function from `useAppAuth()` to guard UI elements. It handles `:manage` parent-scope inheritance automatically (e.g. `api-platform:gateway:manage` grants all `api-platform:gateway:*` scopes).

### Auth Context API

```ts
const { isAuthenticated, isLoading, user, accessToken, hasPermission, login, logout } = useAppAuth();
```

| Field | Type | Description |
|---|---|---|
| `isAuthenticated` | `boolean` | Whether a user session is active |
| `isLoading` | `boolean` | Auth state is still being resolved |
| `user` | `AppUser \| null` | `{ name, email, role, scopes }` |
| `accessToken` | `string \| null` | Bearer token for API calls |
| `hasPermission(scope)` | `(string) => boolean` | Scope-level permission check |
| `login(credentials?)` | `async () => void` | Redirects to IdP (OIDC) or logs in locally (mock) |
| `logout()` | `async () => void` | Clears session and redirects to `/login` |

## Local dev workflow

```
  cd platform-api         && make build   # builds ghcr.io/.../platform-api:latest
  cd portals/ai-workspace && make build   # builds ghcr.io/.../ai-workspace:latest
  docker-compose -f distribution/docker-compose.yaml --project-directory . up
```


## Docker Distribution

The `distribution/` folder contains the standalone compose release of AI Workspace. It brings up two containers — the React SPA served by nginx, and the Platform API Go backend — with no other dependencies.

```
distribution/
├── docker-compose.yaml   # Orchestrates ai-workspace + platform-api
└── configs/
    └── config.toml       # Non-sensitive runtime settings (mounted into both containers)
```

### Port allocation

| Service | Port | Protocol |
|---|---|---|
| AI Workspace (nginx) | `5380` | HTTPS |
| Platform API | `9243` | HTTPS |

Port **5380** was chosen for AI Workspace to avoid collision with the API Gateway, which occupies **8080**. All other commonly used ports in this repo (8081, 8090, 9001–9092, 18000–18001) were surveyed and excluded.

### HTTPS

Both containers are served over HTTPS. Both fall back to a self-signed certificate when no cert is explicitly provided — browsers will show a trust warning in that case.

#### Providing your own certificate (removes the browser warning)

Both services support mounting a CA-signed (or locally trusted) certificate via Docker volume. Uncomment the relevant lines in `docker-compose.yaml` under each service, then place your files in a `certs/` directory next to the compose file:

```
distribution/
├── docker-compose.yaml
├── configs/
│   └── config.toml
└── certs/                        ← create this directory
    ├── ai-workspace.crt          ← PEM certificate or full chain
    ├── ai-workspace.key          ← PEM private key
    ├── platform-api.crt          ← PEM certificate or full chain
    └── platform-api.key          ← PEM private key
```

**AI Workspace** reads from `/etc/ai-workspace/tls/tls.crt` and `/etc/ai-workspace/tls/tls.key` (the mount targets in `docker-compose.yaml`). If both files are present at container startup, `entrypoint.sh` copies them into place; otherwise it generates a self-signed cert.

**Platform API** reads from its `TLS_CERT_DIR` (default `/app/data/certs`, configured via `[tls] cert_dir` in `config.toml`). If `cert.pem` and `key.pem` exist in that directory at startup, it uses them; otherwise it generates and saves a self-signed cert there. The mount targets in `docker-compose.yaml` write directly into the cert dir under the expected file names.

#### Self-signed fallback

When no cert is mounted, both containers auto-generate a self-signed certificate at startup. The self-signed cert is valid for 3650 days so it won't expire between rebuilds. To suppress the browser warning without providing a CA cert, add the cert to your system trust store:

```sh
# AI Workspace — extract the cert from the running container
docker cp ai-workspace:/tmp/nginx/tls.crt ./ai-workspace.crt

# Platform API — cert is persisted in the data volume
docker cp platform-api:/app/data/certs/cert.pem ./platform-api.crt
```

Then add both `.crt` files to your OS trust store (Keychain on macOS, Certificate Manager on Windows, `/usr/local/share/ca-certificates/` on Linux).

### nginx (`nginx.docker.conf`)

nginx is the sole process in the AI Workspace container. Key decisions:

- **All writable paths under `/tmp/`** — pid, cache, logs, and temp files all use `/tmp/` subtrees so the non-root user can write them. The Dockerfile symlinks `/var/cache/nginx`, `/var/log/nginx`, and `/var/run/nginx` to `/tmp/nginx/*` equivalents.
- **SPA fallback** — `try_files $uri $uri/ /index.html` on the root location sends all unmatched paths to the React app for client-side routing.
- **`index.html` served with `no-store`** — prevents the browser from caching the entry HTML, ensuring it always fetches the latest `runtime-config.js` on reload.
- **`/runtime-config.js` aliased from `/tmp/`** — `entrypoint.sh` writes runtime `VITE_*` env vars to `/tmp/runtime-config.js` at startup; nginx aliases this path to serve it. This allows runtime configuration without rebuilding the image.
- **TLS cipher suite** — matches the gateway's explicit ECDHE+AES-GCM/ChaCha20 list (Mozilla Intermediate). `ssl_prefer_server_ciphers on` ensures TLS 1.2 clients use the server's preferred cipher rather than a weaker client choice. Session cache (`shared:SSL:10m`) improves performance; `ssl_session_tickets off` preserves forward secrecy by preventing ticket-key reuse. HSTS (`max-age=31536000; includeSubDomains`) tells browsers to enforce HTTPS permanently once visited.
- **WebSocket upgrade headers** — the `map $http_upgrade $connection_upgrade` block and `Upgrade`/`Connection` headers on the proxy location support WebSocket pass-through for streaming API responses.
- **Reverse proxy to Platform API** — `/api-proxy/` is stripped and forwarded to `https://platform-api:9243/`. Read/send timeouts are set to 3600s to accommodate long-running streaming requests.
- **Security headers on `index.html`** — `X-Frame-Options`, `Content-Security-Policy`, `X-Content-Type-Options`, `X-XSS-Protection`, and `X-Permitted-Cross-Domain-Policies` are set for the entry HTML only (not static assets, which don't need them).
- **gzip** — enabled for text, CSS, SVG, JS, JSON, fonts, and zip.

### `entrypoint.sh` startup sequence

1. Create `/tmp/nginx/` subdirectories and re-establish symlinks for nginx's writable paths.
2. Read `[ai_workspace]` section from the mounted `config.toml` and export matched keys as `VITE_*` env vars (env vars already set take priority).
3. Write all current `VITE_*` env vars into `/tmp/runtime-config.js` so the SPA can read runtime config without a rebuild.
4. If a user cert is mounted at `/etc/ai-workspace/tls/`, copy it into `/tmp/nginx/`; otherwise generate a self-signed cert there.
5. Start nginx in the foreground (`daemon off`).

### Non-root container user

The container runs as UID/GID **10001** (`aiworkspace`). This satisfies the `CKV_CHOREO_1` Checkov policy. All files the process needs to write at runtime are under `/tmp/`, which is world-writable. Static app files under `/app/` are owned by this user at build time via `COPY --chown`.

## Other Documentations
[Mock Backend Documentation](workspaces/apps/ai-workspace/mock-service/README.md)
[Production Documentation](workspaces/apps/ai-workspace/production/README.md)
