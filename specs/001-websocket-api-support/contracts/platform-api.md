# Contract: platform-api — `/websocket-apis`

Mirrors `/graphql-apis` exactly in shape; final source of truth is `platform-api/resources/
openapi.yaml`. New scopes (FR-051) guard every endpoint below — deny-by-default (GO-AUTH-007), no
endpoint is reachable on token validity alone.

## New OAuth2 scopes

| Scope | Guards |
|---|---|
| `ap:websocket_api:manage` | Create/update/delete, AsyncAPI definition update, API keys. |
| `ap:websocket_api:read` | Get/list/AsyncAPI definition read, builds. |
| `ap:websocket_api:deployment:manage` | Deploy/undeploy/restore to a gateway. |
| `ap:websocket_api:deployment:read` | Deployment status/listing. |

Added to `resources/role-to-scope-mapping.yaml` and the AI Workspace BFF default OIDC scopes
(FR-051), same locations as the GraphQL scopes.

## Resource endpoints

| Method | Path | Scope | Notes |
|---|---|---|---|
| `POST` | `/websocket-apis` | `manage` | Create from file, URL, or inline AsyncAPI content (FR-052). URL fetches go through the shared SSRF-safe client — never a direct fetch (`ssrf-prevention.md`). |
| `GET` | `/websocket-apis` | `read` | List, org-scoped from verified JWT claims only (GO-AUTH-005) — never a request-supplied org/tenant filter. |
| `GET` | `/websocket-apis/{id}` | `read` | Get. |
| `PUT` | `/websocket-apis/{id}` | `manage` | Update. |
| `DELETE` | `/websocket-apis/{id}` | `manage` | Delete. |
| `GET` | `/websocket-apis/{id}/definition` | `read` | Get the stored AsyncAPI definition. |
| `PUT` | `/websocket-apis/{id}/definition` | `manage` | Replace the definition (re-derives channels; re-validates). |
| `POST` | `/websocket-apis/validate-definition` | `manage` | Validate without persisting — mirrors `/graphql-apis/validate-schema`. |
| `GET` | `/websocket-apis/{id}/gateways` | `read` | Gateways this API is associated with. |

## API key sub-resource

| Method | Path | Scope |
|---|---|---|
| `POST`/`GET` | `/websocket-apis/{id}/api-keys` | `manage` / `read` |
| `PUT`/`DELETE` | `/websocket-apis/{id}/api-keys/{keyId}` | `manage` |
| `POST` | `/websocket-apis/{id}/api-keys/{keyId}/regenerate` | `manage` |

## Deployment sub-resource

| Method | Path | Scope |
|---|---|---|
| `POST` | `/websocket-apis/{id}/deployments` | `deployment:manage` |
| `GET` | `/websocket-apis/{id}/deployments` | `deployment:read` |
| `DELETE` | `/websocket-apis/{id}/deployments/{deploymentId}` | `deployment:manage` (undeploy) |
| `POST` | `/websocket-apis/{id}/deployments/{deploymentId}/restore` | `deployment:manage` |
| `GET`/`POST` | `/websocket-apis/{id}/builds` | `deployment:read` / `deployment:manage` |

## AsyncAPI acceptance rules (FR-052)

- Accepts AsyncAPI 2.x and 3.x, JSON or YAML, from file upload, URL, or inline content.
- Derives: `info` (name/version/description), `channels` (2.x) or `channels` + `operations` (3.x),
  channel parameters, `servers` filtered to `ws`/`wss` protocol (becomes the upstream).
- Rejects a document with no `ws`/`wss` server and no explicit upstream override.
- Size and parse-time limits apply (config-sourced, per `file-access.md`/`error-handling.md`
  patterns already used for other document uploads).

## Control-plane ↔ gateway integration (not a client-facing endpoint)

FR-054 requires `WebSocketApi` and its API keys to appear in: connected-gateway bulk sync, live
deployment events, gateway-first artifact push (read-only in the control plane), and API key sync.
This is **not** a new endpoint — it's new payload content flowing through the existing
control-plane↔gateway live channel (`platform-api/internal/websocket/`, confirmed distinct from
this feature's artifact — see plan.md's "out of scope" note). No new contract is needed here
beyond: the existing sync payload's artifact-kind union gains `WebSocketApi` as a case.
