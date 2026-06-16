# AI Workspace

The AI Workspace is a React/Vite SPA served by nginx inside a Docker container. It communicates with the **Platform API** Go backend and supports two authentication modes: OIDC (production) and file-based/basic (quickstart).

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
- **nginx** — serves the SPA and reverse-proxies `/api-proxy/` to the Platform API
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

Runtime config is injected at container startup via `entrypoint.sh`:

1. Values are read from the `config.toml` mounted at `/etc/ai-workspace/config.toml`.
2. Each key is mapped to a `VITE_*` env var (env vars already set take priority).
3. All `VITE_*` vars are written to `/tmp/runtime-config.js` so the SPA can read them without a rebuild.

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
├── src/
│   ├── config.env.ts                    # Centralised env/runtime config reads
│   ├── App.tsx
│   └── ...
├── entrypoint.sh                        # Container startup — config injection + TLS
├── nginx.docker.conf                    # nginx config
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
| AI Workspace (nginx) | `5380` | HTTPS |
| Platform API | `9243` | HTTPS |


### Option 2

Open two separate terminals and run the following commands.

Terminal 1:
```bash
cd portals/ai-workspace
npm run dev
```
This starts the AI Workspace frontend in development mode.

Terminal 2:
```bash
cd platform-api/src
go run ./cmd/main.go -config ./config/config.toml
```
This starts the Platform API using the local configuration file.

Ensure both services are running before accessing the application.

> **Browser trust warning?** Both services use a self-signed TLS certificate by default. Click **Advanced → Proceed** to continue, then return to the workspace. See [Custom TLS certificates](#custom-tls-certificates) to remove the warning permanently.

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
