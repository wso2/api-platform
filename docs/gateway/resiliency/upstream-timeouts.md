# Gateway Timeouts

This guide explains how to configure **timeouts** for the API Platform Gateway so that requests to slow or unreachable backends, and slow downstream clients, fail within a predictable time instead of hanging indefinitely.

Timeouts are configured at **two levels**:

| Level | Where it is configured | Scope | Typical use |
|-------|------------------------|-------|-------------|
| **Router level** | The gateway controller's `config.toml` (`[router.*]`) | Global defaults applied to **all** traffic on the gateway | Operator-wide baselines (connect timeout, default route timeout, downstream/HCM timeouts) |
| **API level** | The API definition — the `resilience` block (RestApi, LLM Provider, LLM Proxy) and `upstreamDefinitions[].timeout.connect` (RestApi) | A single API, its routes, and its upstream clusters | Per-API/per-route overrides for backends that are faster or slower than the global default |

The router level establishes the **defaults**. The API level **overrides** those defaults for a specific API (and, for REST APIs, a specific operation). When an API does not specify a value, the router-level default applies.

---

## 1. Router-level timeouts (config.toml)

Router-level timeouts are global defaults that apply to every request handled by the gateway, regardless of which API serves it. They are configured in the gateway controller configuration. You can use `configs/config-template.toml` as a reference when creating your own `config.toml`.

There are two groups:

- **Upstream timeouts** (`[router.upstream.timeouts]`) — govern the connection to, and the response from, the **backend**.
- **HTTP Connection Manager (HCM) timeouts** (`[router.http_listener.timeouts]`) — govern the **downstream** connection between the client and the gateway.

### 1.1 Upstream timeouts

```toml
[router.upstream.timeouts]
route_timeout_ms      = 60000   # default per-route request→response timeout
route_idle_timeout_ms = 300000  # default per-route stream idle timeout
connect_timeout_ms    = 5000    # TCP connection establishment timeout
```

| Setting | Default | Maps to (Envoy) | Description |
|---------|---------|-----------------|---------|
| `connect_timeout_ms` | `5000` | Cluster `connect_timeout` | How long the router waits to **establish a TCP connection** to an upstream endpoint before failing the request. |
| `route_timeout_ms` | `60000` | `RouteAction.timeout` | Default maximum time for the **entire request→upstream-response** on a route. This is the default that an API's `resilience.timeout` overrides. |
| `route_idle_timeout_ms` | `300000` | `RouteAction.idle_timeout` | Default **per-route stream idle** timeout. This is the default that an API's `resilience.idleTimeout` overrides. |

> All three values must be **positive** (greater than `0`). To disable a *route* timeout for a specific API, set its `resilience` value to `0s` (see [API-level timeouts](#2-api-level-timeouts)).

### 1.2 HTTP Connection Manager (downstream) timeouts

These govern the connection between the **client and the gateway**. A value of `"0s"` disables the corresponding timeout.

```toml
# HTTP Connection Manager (downstream) timeouts
# A value of zero ("0s") disables any of these timeout settings.
[router.http_listener.timeouts]
request_timeout         = "0s"   # max duration for the entire downstream request
request_headers_timeout = "0s"   # max duration to receive the complete request headers
stream_idle_timeout     = "5m"   # idle timeout for a single HTTP stream/request
idle_timeout            = "1h"   # idle timeout for the downstream connection
```

| Setting | Default | Maps to (Envoy) | Description |
|---------|---------|-----------------|---------|
| `request_timeout` | `0s` (disabled) | HCM `request_timeout` | Max time for the gateway to receive the **entire** downstream request. |
| `request_headers_timeout` | `0s` (disabled) | HCM `request_headers_timeout` | Max time to receive the **complete request headers**. |
| `stream_idle_timeout` | `5m` | HCM `stream_idle_timeout` | Idle timeout for a single HTTP **stream/request**. |
| `idle_timeout` | `1h` | HCM `common_http_protocol_options.idle_timeout` | Idle timeout for the downstream **connection**. |

Durations use Go duration syntax with a single unit (for example `"30s"`, `"500ms"`, `"5m"`, `"1h"`). If a value is left unset, the Envoy default for that field applies.

> **Warning:** `idle_timeout` can also be disabled via setting this value to zero explicitly. Disabling this timeout has a highly likelihood of yielding connection leaks due to lost TCP FIN packets, etc..

### 1.3 Setting router-level timeouts in different deployments

#### Standalone / local

Configure the timeouts directly in `configs/config.toml` using the structure shown above.

#### Kubernetes with Helm

When deploying the gateway via the Helm chart, the same settings are controlled through Helm values under `gateway.config.router.*`. Example `values.yaml` snippet:

```yaml
gateway:
  config:
    router:
      upstream:
        timeouts:
          connect_timeout_ms: 5000
          route_timeout_ms: 60000
          route_idle_timeout_ms: 300000
      http_listener:
        timeouts:
          request_timeout: "0s"
          request_headers_timeout: "0s"
          stream_idle_timeout: "5m"
          idle_timeout: "1h"
```

The chart renders these values into the generated `config.toml` used by the gateway controller.

---

## 2. API-level timeouts

Two of the timeout layers are naturally per-API and can be tuned directly in the API definition, each through its own field:

| Surface | Field(s) | Maps to (Envoy) | Overrides router default |
|---------|----------|-----------------|--------------------------|
| `upstreamDefinitions[].timeout.connect` | `connect` | Cluster `connect_timeout` | `connect_timeout_ms` |
| `resilience` block | `timeout`, `idleTimeout` | `RouteAction.timeout` / `idle_timeout` | `route_timeout_ms` / `route_idle_timeout_ms` |

The HCM (downstream) timeouts have no per-API equivalent and remain router-level only.

### 2.1 Connect timeout per upstream (`upstreamDefinitions[].timeout.connect`)

When an API routes through a named **upstream definition**, that definition can set its own `connect` timeout, which overrides the router-level `connect_timeout_ms` for the cluster built from it. Because the setting lives on the upstream definition, different upstreams used by the same API can have different connect timeouts.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: orders-api
spec:
  displayName: Orders API
  version: v1.0
  context: /orders/$version
  upstreamDefinitions:
    - name: orders-backend
      timeout:
        connect: 6000ms        # overrides connect_timeout_ms for this cluster
      upstreams:
        - url: http://orders.internal:8080
  upstream:
    main:
      ref: orders-backend      # this API routes through the orders-backend cluster
  operations:
    - method: GET
      path: /list
```

- `connect` is a duration string (for example `6000ms`, `5s`).
- It governs only **TCP connection establishment** (plus the TLS handshake for HTTPS upstreams) — not the response wait, which is the route timeout below.
- An API that uses a direct `upstream.main.url` (instead of an `upstreamDefinitions` ref) uses the router-level `connect_timeout_ms`.

### 2.2 Route timeouts (the `resilience` block)

The `resilience` block lets an individual API override the router-level **route** timeouts for its own traffic. It maps to Envoy's `RouteAction` timeouts and is available on `RestApi`, `LlmProvider`, and `LlmProxy` resources.

It supports two fields:

| Field | Maps to (Envoy) | Overrides router default |
|-------|-----------------|--------------------------|
| `timeout` | `RouteAction.timeout` | `route_timeout_ms` |
| `idleTimeout` | `RouteAction.idle_timeout` | `route_idle_timeout_ms` |

> **Duration format:** `timeout` and `idleTimeout` take single-unit duration strings (for example `30s`, `500ms`, `1.5m`). Use `0s` to disable a timeout, or omit the field to fall back to the router-level default. Compound (`1h30m`), negative (`-30s`), and unitless (`0`, `30`) values are rejected.


### 2.3 REST APIs: API level and operation level

For `RestApi`, the `resilience` block can be set at **two levels**:

- **API level** — applies to every operation/route of the API.
- **Operation level** — applies to that single operation and overrides the API-level value.

Precedence is **most-specific-wins, per field**: an operation's `timeout` overrides the API's `timeout`, while still inheriting the API's `idleTimeout` if the operation does not set its own. If neither level sets a field, the router-level default applies.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: orders-api
spec:
  displayName: Orders API
  version: v1.0
  context: /orders/$version
  upstream:
    main:
      url: https://backend.example.com
  # API-level: applies to all operations unless overridden.
  resilience:
    timeout: 15s
    idleTimeout: 30s
  operations:
    - method: GET
      path: /list
    - method: POST
      path: /reports
      # Operation-level override: this slow report endpoint gets a longer
      # route timeout, but still inherits the API-level idleTimeout (30s).
      resilience:
        timeout: 60s
```

Resolution for the example above:

| Route | `timeout` | `idleTimeout` |
|-------|-----------|---------------|
| `GET /list` | `15s` (API level) | `30s` (API level) |
| `POST /reports` | `60s` (operation level) | `30s` (inherited from API level) |

### 2.4 LLM Provider and LLM Proxy: API level only

For `LlmProvider` and `LlmProxy`, the `resilience` block is supported at the **API level only**. LLM routes are generated by the gateway from the access-control configuration and policy attachments (rather than authored as explicit operations), so there is no operation-level override. A single API-level block applies to **all** routes generated for the LLM API.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: openai-provider
spec:
  displayName: OpenAI Provider
  version: v1.0
  template: openai
  context: /openai
  upstream:
    url: https://api.openai.com/v1
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
  # Applies to every route generated for this provider.
  resilience:
    timeout: 60s
    idleTimeout: 30s
```

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProxy
metadata:
  name: openai-proxy
spec:
  displayName: OpenAI Proxy
  version: v1.0
  context: /openai-proxy
  provider:
    id: openai-provider
  resilience:
    timeout: 75s
```

> **LLM Proxy note (double hop):** a request to a proxy traverses two routes — `client → proxy route → (loopback) → provider route → backend`. The **proxy's** `resilience.timeout` bounds the whole proxied call, while the backing **provider's** `resilience.timeout` bounds the provider→backend call. For a meaningful end-to-end budget, keep the proxy timeout greater than or equal to the provider timeout; the shorter of the two effectively wins.


---

## 3. Practical guidance

### Connect timeout (`connect_timeout_ms`)

- **Decrease** when backends are highly available and you want to fail fast on unhealthy or misconfigured targets, freeing resources quickly.
- **Increase** when backends sit behind slower networks/load balancers, or may experience cold starts or scaling events that briefly delay connection establishment.
- Avoid setting it too low — it may cause **false-positive timeouts** during short periods of backend slowness or network jitter.
- Set the **router default** for the common case, and override **per upstream** via `upstreamDefinitions[].timeout.connect` (see §2.1) when a specific backend needs a different connection budget.

### Route timeout (`route_timeout_ms` / `resilience.timeout`)

- Set the **router default** to a sane upper bound for typical backends, and use **API/operation-level** `resilience.timeout` for endpoints that are known to be faster (tighter budget) or legitimately slower (e.g. report generation, LLM completions).
- For streaming/long-lived responses (e.g. SSE from LLM backends), be mindful of `idleTimeout`: a long total `timeout` with a short `idleTimeout` can still cut a slow token stream. Leave `idleTimeout` generous (or unset) for streaming providers.

### HCM/downstream timeouts

- `request_headers_timeout` is a useful protection against slow-header (Slowloris-style) clients; enable it with a small value (for example `"5s"`) if your clients are expected to send headers promptly.
- `request_timeout` is disabled by default; enable it only if you want a hard cap on how long the gateway will spend receiving an entire downstream request.

---

## 4. Example scenarios

### A slow-to-connect backend

A backend may take a few seconds to accept new connections during peak load. Give connections more time than the default while still failing in a bounded time when the backend is unreachable:

```toml
[router.upstream.timeouts]
connect_timeout_ms = 6000
```

The router gives each upstream connection attempt up to 6 seconds; if the backend is down, requests fail after ~6 seconds (`503`) instead of hanging.

### One slow endpoint on an otherwise fast API

The API as a whole should respond within 5 seconds, but a single report endpoint legitimately takes longer:

```yaml
spec:
  resilience:
    timeout: 5s
  operations:
    - method: GET
      path: /summary
    - method: POST
      path: /reports
      resilience:
        timeout: 60s
```

`GET /summary` is bounded at 5 seconds; `POST /reports` is allowed up to 60 seconds; both inherit the router-level `route_idle_timeout_ms`.

### Disabling the route timeout for a long-running API

```yaml
spec:
  resilience:
    timeout: 0s   # no route (response) timeout for this API
```

The route will wait indefinitely for the upstream response (bounded only by connect and idle timeouts). Use with care.
