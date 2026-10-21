# **REST APIs**

# 1. Summary of Changes

| Endpoint | Method | Change | Breaking? |
| :---- | :---- | :---- | :---- |
| `/websocket-apis` (gateway management API) | POST | New | No |
| `/websocket-apis` (gateway management API) | GET | New | No |
| `/websocket-apis/{id}` (gateway management API) | GET | New | No |
| `/websocket-apis/{id}` (gateway management API) | PUT | New | No |
| `/websocket-apis/{id}` (gateway management API) | DELETE | New | No |
| `/websocket-apis/{id}/api-keys` (gateway management API) | POST | New | No |
| `/websocket-apis/{id}/api-keys` (gateway management API) | GET | New | No |
| `/websocket-apis/{id}/api-keys/{keyId}` (gateway management API) | PUT | New | No |
| `/websocket-apis/{id}/api-keys/{keyId}/regenerate` (gateway management API) | POST | New | No |
| `/websocket-apis/{id}/api-keys/{keyId}` (gateway management API) | DELETE | New | No |
| `/websocket-apis` (control plane) | POST | New | No |
| `/websocket-apis` (control plane) | GET | New | No |
| `/websocket-apis/{id}` (control plane) | GET | New | No |
| `/websocket-apis/{id}` (control plane) | PUT | New | No |
| `/websocket-apis/{id}` (control plane) | DELETE | New | No |
| `/websocket-apis/{id}/definition` (control plane) | GET | New | No |
| `/websocket-apis/{id}/definition` (control plane) | PUT | New | No |
| `/websocket-apis/validate-definition` (control plane) | POST | New | No |
| `/websocket-apis/{id}/gateways` (control plane) | GET | New | No |
| `/websocket-apis/{id}/api-keys[...]` (control plane) | POST/GET/PUT/DELETE | New | No |
| `/websocket-apis/{id}/api-keys/{keyId}/regenerate` (control plane) | POST | New | No |
| `/websocket-apis/{id}/deployments[...]` (control plane) | POST/GET/DELETE | New | No |
| `/websocket-apis/{id}/deployments/{deploymentId}/restore` (control plane) | POST | New | No |
| `/websocket-apis/{id}/builds` (control plane) | GET/POST | New | No |

Every endpoint in this table is new. No existing endpoint changes shape as part of this feature.

# 2. Gateway Management API — WebSocket API Resource

Lets a gateway operator create, inspect, and remove a WebSocket API directly on a standalone gateway, without going through the control plane (the "gateway-first" flow).

```
POST   /websocket-apis
GET    /websocket-apis
GET    /websocket-apis/{id}
PUT    /websocket-apis/{id}
DELETE /websocket-apis/{id}
```

**Audience**: Internal (component-to-component) / Admin — the same audience as the gateway's other management-API resources (for example its existing GraphQL API resource).

**Authorization**

| | |
| :---- | :---- |
| Authentication | Same bearer-token mechanism as every other gateway management endpoint — no new authentication mechanism is introduced. |
| Required scope(s) | The scope check applied to the gateway's other artifact-kind resources; no artifact-kind-specific scope exists at this layer. |
| Tenant scoping | Not applicable at this layer — a standalone gateway is managed directly by its operator; organization scoping applies at the control-plane layer below. |

**Request** (create/update) — like every other gateway artifact kind, a `WebSocketApi` is a
declarative resource with the same `apiVersion`/`kind`/`metadata`/`spec` shape as the existing
`GraphQLApi`/`RestApi` resources, not a flat object. The example below is what an operator
submits — `status` is never submitted; it is populated by the gateway and only appears in the
response shown further below.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: WebSocketApi
metadata:
  name: chat-websocket-v1
  annotations:
    gateway.api-platform.wso2.com/project-id: default
spec:
  displayName: Chat
  version: v1
  context: /chat
  upstream:
    main:
      url: wss://chat-backend:8443/ws
      timeout:                      # reuses the existing UpstreamTimeout schema (RestApi's UpstreamDefinition.timeout), extended
        connect: 5s                 # pre-existing field — connect timeout
        idleTimeout: 120s           # new; optional; upstream connection-pool idle timeout, falls back to gateway config
        maxConnectionDuration: 30m  # new; optional; always resolves to a finite value, never "unlimited"
        maxConnections: 500         # new; optional; concurrent connections to this backend, per gateway instance
    sandbox:
      url: ws://chat-sandbox:8080/ws
      # timeout omitted — falls back to gateway-config defaults, independently of main's settings
  connection:
    idleTimeout: 300s            # optional; falls back to the gateway-config default if omitted
    maxConnectionDuration: 1h    # optional; always resolves to a finite value, never "unlimited"
    maxConnections: 1000         # optional; falls back to the gateway-config default if omitted
  channels:
    - path: /rooms/{roomId}
      policies:
        - name: jwt-auth
          version: v1
          params:
            issuers:
              - PrimaryIdp
            scopes:
              anyOf:
                - chat:write
    - path: /notifications
  policies:                      # API-level, handshake-only (see Validation)
    - name: api-key-auth
      version: v1
      params:
        key: X-API-Key
        in: header
```

The `jwt-auth`/`api-key-auth` policy names and their `params` shapes are each policy's own
existing schema, unchanged by this feature — shown here for a complete, realistic example, not
as something this feature defines.

| Field | Type | Required | Description |
| :---- | :---- | :---- | :---- |
| `metadata.name` | string | Yes | URL-safe identifier; immutable after creation. |
| `metadata.annotations` | map | No | Carries project scoping and other gateway-wide annotations, the same convention as every other kind. |
| `spec.displayName` | string | Yes | Human-readable name. |
| `spec.version` | string | Yes | API version segment used in the path. |
| `spec.context` | string | Yes | Base path segment; unique per gateway and version. |
| `spec.upstream.main.url` | string | Yes | `ws://` or `wss://` only, checked by the shared upstream validator. |
| `spec.upstream.sandbox.url` | string | No | Same validation as `main` when present. |
| `spec.upstream.main.timeout.connect` | duration | No | Pre-existing field, reused as-is — upstream connect timeout. |
| `spec.upstream.main.timeout.idleTimeout` | duration | No | New field — upstream connection-pool idle timeout; defaults from gateway config if omitted; rejected at deploy time if it exceeds the configured upper bound. |
| `spec.upstream.main.timeout.maxConnectionDuration` | duration | No | New field — upstream connection lifetime cap; always resolves to a finite value — never "unlimited," even when omitted. |
| `spec.upstream.main.timeout.maxConnections` | integer | No | New field — concurrent connections to this backend; defaults from gateway config if omitted; enforced per gateway instance. |
| `spec.upstream.sandbox.timeout.*` | — | No | Same four fields as `upstream.main.timeout`, independent of `main`'s values. |
| `spec.connection.idleTimeout` | duration | No | Defaults from gateway config if omitted; rejected at deploy time if it exceeds the configured upper bound. |
| `spec.connection.maxConnectionDuration` | duration | No | Always resolves to a finite value — never "unlimited," even when omitted. |
| `spec.connection.maxConnections` | integer | No | Defaults from gateway config if omitted. |
| `spec.channels` | array | Yes, at least one | Each entry: a `path` (may include `{parameter}` segments) and an optional channel-level `policies` list. |
| `spec.policies` | array | No | API-level policies; must be handshake-only (see Validation). |

**Response** — `201 Created` (create) / `200 OK` (get, list, update) — the same resource echoed
back, with a server-populated `status` block added:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: WebSocketApi
metadata:
  name: chat-websocket-v1
  annotations:
    gateway.api-platform.wso2.com/project-id: default
spec:
  displayName: Chat
  version: v1
  context: /chat
  upstream:
    main:
      url: wss://chat-backend:8443/ws
  channels:
    - path: /rooms/{roomId}
status:
  state: NOT ANSWERED   # exact resource-status enum values not specified in the inputs
```

**Error responses**

| Status | Condition | Body |
| :---- | :---- | :---- |
| 400 | Validation failure — missing required field, no channels, duplicate channel path, invalid upstream URL, or a policy that is not handshake-only | Generic validation error naming the offending field or policy |
| 401 | Missing/invalid credentials | `{"error": "unauthorized", "message": "Invalid or expired credentials."}` |
| 403 | Scope check failed | Generic forbidden body |
| 404 | API not found | Generic not-found body |
| 409 | Conflicting concurrent update | NOT ANSWERED — conflict-handling behaviour is not specified in the inputs |

**Notes**

* Deploying this artifact kind to a gateway below the minimum supported version is refused before translation is attempted; the exact status code for that case is NOT ANSWERED (see the specification document's Open Questions).
* `DELETE` takes effect for new handshakes immediately; an existing open connection is not affected and ends no later than its own maximum connection duration.

# 3. Gateway Management API — API Key Sub-resource

```
POST   /websocket-apis/{id}/api-keys
GET    /websocket-apis/{id}/api-keys
PUT    /websocket-apis/{id}/api-keys/{keyId}
POST   /websocket-apis/{id}/api-keys/{keyId}/regenerate
DELETE /websocket-apis/{id}/api-keys/{keyId}
```

**Audience**: Internal (component-to-component) / Admin.

**Authorization**: Same as §2.

Mirrors the equivalent API-key sub-resource already exposed for the gateway's GraphQL API resource — same create/list/update/regenerate/revoke shape. See §2's error table; a 404 additionally covers an unknown `keyId`.

# 4. Control Plane — WebSocket API Resource

Lets a publisher create a WebSocket API from an AsyncAPI definition, and deploy it to one or more registered gateways.

```
POST   /websocket-apis
GET    /websocket-apis
GET    /websocket-apis/{id}
PUT    /websocket-apis/{id}
DELETE /websocket-apis/{id}
```

**Audience**: Public (to authenticated platform users).

**Authorization**

| | |
| :---- | :---- |
| Authentication | OAuth2 bearer token. |
| Required scope(s) | `ap:websocket_api:manage` for create/update/delete; `ap:websocket_api:read` for get/list. |
| Tenant scoping | Organization resolved from verified token claims only — never from a request parameter or body field. |

**Request** (create) — accepts an AsyncAPI document as a file upload, a URL, or inline content

```json
{
  "source": "url",
  "definition": "https://backend.example.com/specs/chat-asyncapi.yaml"
}
```

| Field | Type | Required | Description |
| :---- | :---- | :---- | :---- |
| `source` | string (enum) | Yes | One of `file`, `url`, `inline`. |
| `definition` | string | Yes | The document content, or its URL. A URL is fetched through the platform's shared SSRF-safe client — never a direct, unvalidated fetch. |

**Response** — `201 Created`

```json
{
  "id": "4a1b2c3d-5e6f-4789-9abc-def012345678",
  "displayName": "Chat",
  "version": "v1",
  "channels": [ { "path": "/rooms/{roomId}" } ],
  "upstream": { "main": { "url": "wss://chat-backend:8443/ws" } }
}
```

**Error responses**

| Status | Condition | Body |
| :---- | :---- | :---- |
| 400 | The document has no WebSocket server and no explicit upstream override, or fails validation | Generic validation error |
| 401 | Missing/invalid credentials | `{"error": "unauthorized", "message": "Invalid or expired credentials."}` |
| 403 | Caller lacks `ap:websocket_api:manage` / `ap:websocket_api:read` | Generic forbidden body |
| 404 | API not found | Generic not-found body |
| 409 | Conflicting concurrent update | NOT ANSWERED — not specified in the inputs |
| 413 | Document exceeds the configured size limit | Generic "too large" body |

**Notes**

* Both AsyncAPI major versions in common current use are accepted, in either JSON or YAML.
* A document using only a non-WebSocket protocol is rejected at creation.

# 5. Control Plane — AsyncAPI Definition, API Key, and Deployment Sub-resources

```
GET    /websocket-apis/{id}/definition
PUT    /websocket-apis/{id}/definition
POST   /websocket-apis/validate-definition
GET    /websocket-apis/{id}/gateways
POST   /websocket-apis/{id}/api-keys            GET /websocket-apis/{id}/api-keys
PUT    /websocket-apis/{id}/api-keys/{keyId}     DELETE /websocket-apis/{id}/api-keys/{keyId}
POST   /websocket-apis/{id}/api-keys/{keyId}/regenerate
POST   /websocket-apis/{id}/deployments          GET /websocket-apis/{id}/deployments
DELETE /websocket-apis/{id}/deployments/{deploymentId}
POST   /websocket-apis/{id}/deployments/{deploymentId}/restore
GET    /websocket-apis/{id}/builds               POST /websocket-apis/{id}/builds
```

**Audience**: Public (to authenticated platform users).

**Authorization**

| | |
| :---- | :---- |
| Authentication | OAuth2 bearer token. |
| Required scope(s) | `ap:websocket_api:manage` / `ap:websocket_api:read` for the definition, validation, gateways, and API-key endpoints; `ap:websocket_api:deployment:manage` / `ap:websocket_api:deployment:read` for the deployment and build endpoints. |
| Tenant scoping | Organization resolved from verified token claims only. |

These mirror the equivalent definition, API-key, and deployment sub-resources already exposed for the control plane's GraphQL API resource, one for one. Error handling follows §4's table; a deployment attempt to a gateway below the minimum supported version is refused before translation (exact status NOT ANSWERED, see the specification document).

**Notes**

* This artifact kind and its API keys are included in the existing connected-gateway bulk-synchronization and live-deployment-event channel the control plane already uses for every other artifact kind — no new synchronization endpoint is introduced for this.
* Undeploying ends new handshakes immediately; an already-open connection is unaffected and ends no later than its own maximum connection duration.

# 6. Changes to Existing Endpoints

None. This feature introduces only new endpoints; no existing endpoint's request or response shape changes.

# 7. OpenAPI Spec

* Spec file(s) updated: the gateway management API's OpenAPI specification (`gateway/gateway-controller/api/management-openapi.yaml`) and the control plane's OpenAPI specification (`platform-api/resources/openapi.yaml`).
* Scopes declared in the spec: `ap:websocket_api:manage`, `ap:websocket_api:read`, `ap:websocket_api:deployment:manage`, `ap:websocket_api:deployment:read`.
* CLI sync required: Yes — the command-line tool's gateway command family gains a new WebSocket API command group and a matching API-key subcommand group.
