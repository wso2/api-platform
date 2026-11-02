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

### Without the BFF: behind a gateway or auth proxy

The console can also run with no BFF, behind a component that does the BFF's
job: an API Platform gateway with the `oauth2-session` policies (as in WSO2
Cloud), oauth2-proxy, or any other. Serve the built console as static files
and set `authMode: 'external'` in its runtime config
(`api-platform.env.config.js`).

The component in front must, on the console's own origin:

- **sign the user in:** the console sends the browser to a sign-in URL, with
  the path to come back to as a query parameter;
- **say who is signed in:** an endpoint the console calls on load;
- **sign the user out:** a URL the console sends the browser to;
- **keep the session** behind an HttpOnly cookie, and add the user's token to
  the Platform API calls it proxies. The browser never holds a token.

#### Example: the API Platform gateway

```js
window.__RUNTIME_CONFIG__ = {
  authMode: 'external',
  appBasePath: '/api-platform',
  platformApiBaseUrl: '/api-platform/proxy',      // oauth2-session-proxy
  authLoginUrl: '/api-platform/auth/login',
  authReturnParam: 'returnTo',
  authSessionUrl: '/api-platform/auth/userinfo',
  authSessionFormat: 'sdk',
  authCheckSessionUrl: '/api-platform/auth/checksession',
  authCheckSessionInterval: 30,
  authLogoutUrl: '/api-platform/auth/logout',
  authLogoutStyle: 'post-json',
  authClaimMappings: '{"username":"name","organization":"ouId","org_name":"ouName","org_handle":"ouHandle"}',
};
```

Declare only `POST /logout` on the gateway: a `GET` sign-out can be triggered
by any other site with a link.

#### Settings

Each setting also has a build-time equivalent (`VITE_AUTH_LOGIN_URL`, …), and
can override the defaults of the BFF modes too. The full contract, with every
default, is in `src/config/authContract.ts`.

| Setting | `basic` / `oidc` (the BFF) | `external` |
|---|---|---|
| `authLoginStyle` | `form` / `redirect` | `redirect` |
| `authLoginUrl` | `/api/auth/login` | the sign-in URL |
| `authReturnParam` | `return` | the query parameter the sign-in URL reads |
| `authSessionUrl` | `/api/session` | the "who is signed in" endpoint |
| `authSessionFormat` | `bff` | `claims` by default; `sdk` for the API Platform gateway (see below) |
| `authCheckSessionUrl` | none | none; a session endpoint that only reads the session (the gateway's `/checksession`) turns on polling |
| `authCheckSessionInterval` | 30 | 30 seconds between polls; at least 5 |
| `authLogoutUrl` | `/api/logout` | the sign-out URL |
| `authLogoutStyle` | `post-json` | `navigate` (a plain navigation); `post-json` for the API Platform gateway, which answers `{"redirectUrl"}` |
| `authClaimMappings` | the BFF's `[auth.claim_mappings]` defaults | which claim holds each user field, same keys as `[auth.claim_mappings]` |

#### Session formats

| Format | The session endpoint answers | Scopes come from |
|---|---|---|
| `bff` | `{authenticated, user}`, the user already resolved (this portal's BFF) | `user.scopes` |
| `claims` | the signed-in user's claims; any non-2xx when signed out | the scope claim (`authClaimMappings.scope`, or `scp`) |
| `sdk` | the ThunderID SDK's backend-mode session, `{isSignedIn, user, expiresAt, scope}`, with the ID token's claims in `user` (the gateway's `oauth2-session` policy) | `scope`, space-separated, the field `@thunderid/react`'s `useGrantedPermissions()` reads too |

For `claims` and `sdk`, the console resolves the user from the claims exactly
as the BFF does: both run the shared cases in `testdata/claim-mapping-cases.json`.

**Scopes and the UI.** The console enables or disables actions by the user's
scopes. If the session doesn't report scopes at all, the console offers every
action and the Platform API refuses what the user may not do. The BFF's role
mode (scopes from roles through a mapping file) is not available without the
BFF.

**When the session ends or the server is down.** In every mode:

- A 5xx or no answer from the session endpoint is not a sign-out: a signed-in
  console stays signed in; otherwise it shows a server error.
- A 401 on an API call makes the console ask the session endpoint again (on the
  gateway, that refreshes the token or signs back in through Native SSO). If the
  session is alive, the call is sent once more.
- With `authCheckSessionUrl` set, a visible tab polls it, so it notices a
  session that ended elsewhere. Never `/userinfo`: polling that extends the idle
  timeout, keeping an open tab's session alive forever.

`expiresAt` in the SDK session is the access token's expiry, not the session's;
the gateway renews the token itself, so the console doesn't use it.

### The Test console's gateway relay

The Test console sends requests to a deployed gateway through a relay. That is
a separate setting from sign-in, available in every auth mode:
`gatewayRelayUrl` (`VITE_GATEWAY_RELAY_URL` at build time), by default the
BFF's `/api/gateway-relay/invoke`.

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

## Serving under a sub-path

To serve the console under a path prefix on a shared origin, with an edge that keeps
the prefix on the request, build the SPA with `VITE_APP_BASE_PATH=/console/` and set
`APIP_ACP_SERVER_BASE_PATH=/console` on the BFF. Every route then answers under the
prefix as well as at the root, and cookies are scoped to it. In `oidc` mode, point
`APIP_ACP_AUTH_OIDC_REDIRECT_URL` at `/console/api/auth/callback`.

Cookie `Path` is not a security boundary: script on any app sharing the origin can
reach the console's routes with its session. Share an origin only with apps of equal
trust.

## Validation

```bash
npm run typecheck
npm run test
npm run build
```
