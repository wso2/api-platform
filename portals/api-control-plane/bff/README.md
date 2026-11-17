# api-control-plane-bff

The Backend-for-Frontend for the API Control Plane console. It serves the SPA,
owns authentication, and is the only thing the browser talks to.

## What it does

**Holds the tokens.** Login happens here, not in the browser. The access token
lives in an HttpOnly session cookie; the SPA never sees one. Two modes, set by
`[auth] mode`: `basic` (Platform API's own file-based user store, no external
IdP) or `oidc` (confidential or public+PKCE code exchange against a configured
IdP). The BFF does not *validate* tokens — Platform API does — it only decodes
claims for display and reads `exp`.

**Proxies everything same-origin.** `/proxy/*` reverse-proxies to Platform API,
stripping the browser's cookie and injecting the session's bearer token on the
way out. Extra `[[control_plane.upstreams]]` mount alongside it, each with its
own TLS trust so one upstream's settings never leak onto another. Because
nothing is cross-origin, there is no CORS layer — which is what makes the
`X-Requested-By` header a sufficient CSRF defense.

**Serves the SPA and its runtime config.** Static files with a client-side
routing fallback, plus `/api-platform.env.config.js`, so pointing a build at a
different backend is a restart rather than a rebuild.

## Routes

| Route | Purpose |
|---|---|
| `GET /healthz` | Liveness |
| `GET /api-platform.env.config.js` | Runtime config the SPA reads before init |
| `POST /api/login` · `POST /api/logout` · `GET /api/session` | Session lifecycle (basic mode) |
| `GET /api/auth/login` · `GET /api/auth/callback` | OIDC code flow |
| `{proxy_prefix}/*` | Same-origin reverse proxy to Platform API and named upstreams |
| `/*` | SPA static files and routing fallback |

## Layout

| Package | Owns |
|---|---|
| `server` | Routes, middleware, handlers, session cookie |
| `auth` | File-based login and the OIDC code flow |
| `session` | Session store and JWT claim mapping |
| `proxy` | Reverse proxy and upstream transports |
| `config` | `config.toml` loading and startup validation |
| `configinterpolate` | `{{ env }}` / `{{ file }}` token expansion |

## Configuration

One or more `-config` TOML files, merged last-wins, with `{{ env }}` and
`{{ file }}` tokens resolved at load. There is no default path and no silent
fallback. Startup validates the *effective* result rather than each key in
isolation and refuses to boot on anything that would be unbounded or
ambiguous at runtime.

```bash
go run . -config ../configs/config.toml
```

## Validation

```bash
go vet ./...
go test ./...
```
