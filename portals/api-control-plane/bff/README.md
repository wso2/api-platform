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

**Relays requests to a gateway.** `POST /api/gateway-relay/invoke` sends one
try-out request to a gateway and returns the whole response as JSON. This is a
*separate* path from the reverse proxy above, and deliberately so: that one
injects the caller's Platform API token, which must never reach a gateway. The
relay package has no access to the session token at all.

The browser names an API and a gateway, never a URL. The BFF resolves the
invoke address from the Platform API using the caller's own token, so a user
who cannot read that API gets nothing — the same validation every proxied
request gets by being forwarded there. See
[Gateway relay egress](#gateway-relay-egress).

## Routes

| Route | Purpose |
|---|---|
| `GET /healthz` | Liveness |
| `GET /api-platform.env.config.js` | Runtime config the SPA reads before init |
| `POST /api/login` · `POST /api/logout` · `GET /api/session` | Session lifecycle (basic mode) |
| `GET /api/auth/login` · `GET /api/auth/callback` | OIDC code flow |
| `POST /api/gateway-relay/invoke` | Gateway relay — one request to a deployed gateway |
| `{proxy_prefix}/*` | Same-origin reverse proxy to Platform API and named upstreams |
| `/*` | SPA static files and routing fallback |

## Layout

| Package | Owns |
|---|---|
| `server` | Routes, middleware, handlers, session cookie |
| `auth` | File-based login and the OIDC code flow |
| `session` | Session store and JWT claim mapping |
| `proxy` | Reverse proxy and upstream transports |
| `gatewayrelay` | Gateway relay |
| `egress` | Where the relay may connect |
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

## Gateway relay egress

The Test console page relays its try-out requests through the BFF, so the browser never
calls a gateway cross-origin. The BFF resolves the target from the Platform
API; `allow_hosts` is the second layer, bounding what a resolved address is
allowed to look like:

```toml
[api_control_plane.gateway_relay.egress]
environment = "production"
allow_hosts = ["https://*.gw.svc.cluster.local:9443"]
```

The shipped `config.toml` reads it from `APIP_ACP_GATEWAY_RELAY_EGRESS_ALLOW_HOSTS`
as a comma-separated list, e.g.
`APIP_ACP_GATEWAY_RELAY_EGRESS_ALLOW_HOSTS="https://gw-a:8443,https://*.gw.svc.cluster.local:9443"`.

**Nothing is reachable unless an entry matches.** An empty list — the shipped
default — refuses every try-out with `TARGET_NOT_ALLOWED` and warns at startup.
A gateway endpoint is tenant-settable data, so resolution can only promise
"a gateway someone registered", not "a sensible address"; this is what says
what one may look like.

Each entry is `[scheme://]host[:port]`. Scheme and port constrain only when
written; the host may carry `*` anywhere.

| Entry | Matches |
|---|---|
| `gw.example.com` | any scheme, any port |
| `https://gw.example.com:8443` | that combination only |
| `*.gw.example.com` | `a.gw.example.com`, `a.b.gw.example.com` — not `gw.example.com`, not `evilgw.example.com` |
| `gw-*.example.com` | `gw-eu.example.com` |
| `*` | everything |

Prefer the narrowest entry you can state. Gateways live in private space, so
private addresses stay dialable — which leaves the entry as the only thing
separating a gateway from any other in-cluster HTTP service (Elasticsearch
`:9200`, etcd `:2379`, kubelet `:10250`). Pinning the port is the cheapest way
to tell them apart.

`environment` is `production` (the default, including when the key is absent)
or `development`:

| | `production` | `development` |
|---|---|---|
| Private, loopback, CGNAT addresses | refused | allowed |
| IP-literal target host | refused | allowed |
| Link-local / metadata | refused | refused |

Production is what a deployment gets by forgetting to choose. Development
exists because a gateway there genuinely is on `localhost` or a ClusterIP; it
does not relax the floor below.

Refused in both and not configurable: link-local (where the cloud metadata
endpoint `169.254.169.254` lives), the unspecified address, multicast and
broadcast, and the IPv6 prefixes that embed an IPv4 address and route to it.
These are checked against each **resolved** address at dial time, and the
connection is made to the address that was validated — so an allowed name that
resolves, or rebinds, to one of them is still refused.

Startup rejects an entry that would not do what it reads as doing: a scheme
other than http/https, a path or query, an invalid port. Refusals reach the
caller as a generic `403`; the reason is logged server-side only.

Gateway TLS trust is separate, under `[api_control_plane.gateway_relay]`:
`ca_file` appends a PEM bundle to the system roots, and `tls_skip_verify`
(off, dev only) cannot be combined with it.

## Validation

```bash
go vet ./...
go test ./...
```
