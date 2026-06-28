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

Update platform-api/src/config/config.toml and set the following configuration:

```bash
[auth.file_based]
enabled = true
```
This enables file-based authentication, allowing users configured in the file-based authentication settings to log in.

Terminal 2:
```bash
cd platform-api/src
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
