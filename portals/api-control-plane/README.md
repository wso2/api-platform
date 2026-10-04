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

## Policy Hub in deployed environments

All builds default to the development Policy Hub URL below. Runtime configuration
overrides `VITE_POLICY_HUB_BASE_URL`, which overrides the hardcoded default.
To override the URL with `configs/config.toml` mounted, set this variable on the BFF:

```bash
APIP_ACP_POLICY_HUB_BASE_URL=https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0
```

For a custom mounted config, include the `[api_control_plane.policy_hub]`
section and `base_url` environment template from `configs/config.toml`.
Restart the BFF after changing configuration. It exposes the URL as
`window.__RUNTIME_CONFIG__.POLICY_HUB_BASE_URL` through
`/api-platform.env.config.js`; no frontend rebuild is needed for URL changes.
If neither override supplies a non-empty URL, the hardcoded default is used.

## Validation

```bash
npm run typecheck
npm run test
npm run build
```
