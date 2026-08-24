# API Control Plane

Oxygen UI console for the API Platform, replacing the legacy `choreo-console`.

## Scope

This app intentionally implements only the MVP core console:

- Auth shell and protected routes
- Organization and project navigation
- Project home
- Component list, create, and detail
- Deploy, test, manage, runtime logs, and minimal settings

Non-MVP legacy pages are hidden rather than ported.

## Architecture

A Go BFF (Backend-for-Frontend, `bff/`) serves the SPA, proxies all
browser→backend traffic same-origin, and owns authentication: tokens live in
a server-side session (HttpOnly cookie) and never reach the browser. It
supports two auth modes (`[auth] mode` in `configs/config.toml`):

| Mode | When to use |
|---|---|
| `basic` (default) | Platform API's own file-based user store — no external IdP needed |
| `oidc` | Delegates to a confidential (or public+PKCE) external IdP |

## Development

Three processes, run side by side: Platform API, the BFF, then the portal's
own dev server.

```bash
# Terminal 1 — Platform API (one-time setup, then run it)
cd <REPO_ROOT>/platform-api
./scripts/setup-local-dev.sh          # first time only — generates local certs/keys/admin creds
make run-local                        # or: make setup-local-dev && make run-local

# Terminal 2 — the BFF, proxying to the running Platform API
cd <REPO_ROOT>/portals/api-control-plane
CONTROL_PLANE_URL=https://localhost:9243 make bff-run

# Terminal 3 — the Vite dev server, proxying same-origin BFF paths to it
cd <REPO_ROOT>/portals/api-control-plane
npm install
npm run dev
```

Visit `https://localhost:3000`. Vite's dev server proxies `/api/*`,
`/proxy/*`, and the runtime-config scripts to the BFF (default
`http://localhost:8082`, override with `BFF_DEV_TARGET`) — everything else
(hot reload, the app shell) is served by Vite itself. See `make help` for
the full target list.

## Validation

```bash
npm run typecheck
npm run test
npm run build
```
