# Application-Level Subscriptions for REST APIs — Implementation Plan

## 1. Executive Summary

**Problem:** REST APIs in the API Platform are not tied to Applications. Token validation only checks issuer/signature; it does not verify that the calling Application is subscribed to the invoked API. Any valid token can call any API.

**Solution:** Introduce Application and Subscription entities, CRUD APIs in platform-api and gateway-controller, WebSocket sync of subscription data to gateways, and subscription validation in the gateway policy engine so requests are allowed only when the Application (identified from the token) is subscribed to the target API.

**Out of scope (per requirements):** API-level policy in Policy Hub to enable subscription validation — not included in this plan.

---

## 2. Current State (Findings from Codebase)

### 2.1 Platform-API
- **REST APIs:** `rest_apis` table (uuid, description, project_uuid, configuration, etc.; uuid FK to `artifacts`). Routes: `/api/v1/rest-apis`, `/api/v1/rest-apis/{apiId}`, plus gateways, api-keys, deployments, devportals, etc.
- **No Application or Subscription tables.** Tokens are issued per application (STS) but platform does not store applications or API–application links.
- **WebSocket:** Platform broadcasts `api.deployed`, `api.undeployed`, `api.deleted` to gateways. No subscription events yet.
- **Internal gateway API:** `GET /api/internal/v1/apis`, `GET /api/internal/v1/apis/{apiId}` (by gateway token); returns API YAML. No subscription payload today.

### 2.2 Gateway-Controller
- **Storage:** SQLite/Postgres with `deployments` (id, gateway_id, display_name, version, context, kind, handle, status, …) and `api_keys` (id, apiId, name, api_key, …). No `applications` or `subscriptions` tables.
- **REST API:** `/apis`, `/apis/{id}`, `/apis/{id}/api-keys` (create, list, regenerate, update, delete). No `/apis/{id}/subscriptions` yet.
- **Control plane client:** Consumes WebSocket events (`api.deployed`, `api.undeployed`, `api.deleted`, `apiKey.created`, `apiKey.updated`, `apiKey.revoked`). No subscription event handlers.
- **API key sync:** Platform pushes API key create/update/revoke via WebSocket; gateway persists in `api_keys` and uses for validation. Same pattern can be used for subscriptions.

### 2.3 Policy Engine
- **Auth:** JWT (or other) policy validates token and can put claims into request metadata. Analytics already uses `x-wso2-application-id` (and name/owner) from metadata (`AppIDKey` in `internal/analytics/constants.go`). So “Application” at runtime = identifier set by auth policy (e.g. from JWT `azp` or `client_id`).
- **Route metadata:** Envoy sends `xds.route_metadata` with `api_id`, `api_name`, `api_version`, `context`, etc. So policy has both “application id” (from auth) and “api id” (from route) for subscription check.
- **No subscription check today:** No policy verifies “this application is subscribed to this API.”

---

## 3. Data Model

### 3.1 Application Identifier
- **Source of truth for “Application”:** Tokens are issued per application (e.g. OAuth2 client / DevPortal application). The auth policy (e.g. JWT) will place the application identifier in request metadata (e.g. `x-wso2-application-id`), typically from JWT claim `azp` or `client_id`.
- **Recommendation:** Use a single string **application ID** (UUID or opaque ID). Platform and gateway both store subscriptions keyed by `application_id` and `api_id`. No need to replicate full “Application” entity in the gateway if we only need to validate (application_id, api_id) pairs.

### 3.2 Platform-API: New Tables

**applications**
- `uuid` (PK) — application identifier (align with token claim if possible).
- `organization_uuid` (FK → organizations).
- `name` — display name.
- `description` (optional).
- `created_at`, `updated_at`.
- Optional: `external_ref_id` if apps are created in an external IdP/DevPortal.

**subscriptions**
- `uuid` (PK).
- `api_uuid` (FK → rest_apis).
- `application_uuid` (FK → applications).
- `organization_uuid` (FK → organizations).
- `status` — e.g. `ACTIVE`, `INACTIVE`, `REVOKED`.
- `created_at`, `updated_at`.
- UNIQUE(api_uuid, application_uuid) (one subscription per API–application pair).

Indexes: (api_uuid), (application_uuid), (organization_uuid), (status).

### 3.3 Gateway-Controller: New Tables

**applications** (minimal, for FK and optional display)
- `id` (PK) — same as platform application uuid.
- `gateway_id` — gateway instance.
- `name` (optional).
- `created_at`, `updated_at`.
- UNIQUE(id, gateway_id).

**subscriptions**
- `id` (PK) — subscription uuid (or composite key).
- `api_id` (FK → deployments.id) — gateway’s API config id.
- `application_id` (FK → applications.id).
- `gateway_id`.
- `status` — ACTIVE / INACTIVE / REVOKED.
- `created_at`, `updated_at`.
- UNIQUE(api_id, application_id, gateway_id).

Indexes: (api_id), (application_id), (gateway_id), (status) for fast “is this app subscribed to this API?” lookups.

---

## 4. API Design

### 4.1 Platform-API: REST — root-level `/subscriptions`

- **POST /api/v1/subscriptions**  
  Body: `{ "apiId": "api-id-or-handle", "applicationId": "uuid", "status": "ACTIVE" }`.  
  Create a subscription; return 201 + subscription representation. Validate `apiId` and `applicationId` belong to the same org.

- **GET /api/v1/subscriptions**  
  Query: optional `apiId`, `applicationId`, `status`.  
  Return list of subscriptions filtered by API and/or application (org-scoped).

- **GET /api/v1/subscriptions/{subscriptionId}**  
  Return a single subscription (org-scoped).

- **PUT /api/v1/subscriptions/{subscriptionId}**  
  Body: `{ "status": "ACTIVE"|"INACTIVE"|"REVOKED" }` (and optionally other fields).  
  Update subscription.

- **DELETE /api/v1/subscriptions/{subscriptionId}**  
  Remove subscription (or soft-delete via status).

**Applications (if not managed elsewhere):**
- If platform must create applications: **POST/GET/GET by id/PUT/DELETE /api/v1/applications** (org-scoped).  
- If applications are created only in DevPortal/STS, platform might only need a minimal “application registry” or just store `application_id` in subscriptions as a string and validate existence via DevPortal/STS when needed. Plan assumes we add at least `applications` table and optional CRUD for it.

### 4.2 Gateway-Controller: REST — root-level `/subscriptions`

- **POST /subscriptions**  
  Body: `{ "apiId": "deployment-id-or-handle", "applicationId": "uuid", "status": "ACTIVE" }`.  
  Create a subscription for this gateway (where `apiId` is typically the deployment id). Used by control plane sync and/or admin.

- **GET /subscriptions**  
  Query: optional `apiId`, `applicationId`, `status`.  
  List subscriptions for the gateway, filtered by API and/or application.  
  If `apiId` is omitted, subscriptions across all APIs on this gateway are returned (optionally filtered by `applicationId` and/or `status`).

- **GET /subscriptions/{subscriptionId}**  
  Get one subscription by ID.

- **PUT /subscriptions/{subscriptionId}**  
  Update (e.g. status).

- **DELETE /subscriptions/{subscriptionId}**  
  Remove subscription.

All scoped by gateway (implicit `gateway_id` from configuration/state).

---

## 5. WebSocket & Sync (Gateway-Controller)

### 5.1 New Event Types (Platform → Gateway)

- **subscription.created**  
  Payload: `apiId` (platform API uuid), `subscriptionId`, `applicationId`, `status`, optional `applicationName`.  
  Gateway maps platform `apiId` to local deployment id(s) for that API and inserts/updates subscription(s) in its DB.

- **subscription.updated**  
  Payload: `apiId`, `subscriptionId`, `applicationId`, `status`.  
  Gateway updates existing subscription.

- **subscription.deleted**  
  Payload: `apiId`, `subscriptionId`, `applicationId`.  
  Gateway deletes or marks subscription inactive.

When a single API is deployed to multiple gateways, platform can broadcast to each gateway; each gateway only applies subscriptions for APIs it knows (by deployment id).

### 5.2 Gateway-Controller Handling

- In WebSocket message dispatcher (e.g. `gateway/gateway-controller/pkg/controlplane/client.go`), add cases for `subscription.created`, `subscription.updated`, `subscription.deleted`.
- Resolve platform `apiId` to gateway’s deployment `id` (same as current deployment event flow). If API not deployed on this gateway, ignore or store for when API is later deployed (implementation choice).
- Persist to gateway’s `subscriptions` (and optionally `applications`) table; no need to call back platform for full list on each event if events are authoritative.

### 5.3 Initial / Bulk Sync (Optional but Recommended)

- On WebSocket connect or on demand: platform could send **subscriptions.snapshot** or gateway could call **GET /api/internal/v1/apis/{apiId}/subscriptions** (new internal endpoint) to fetch all subscriptions for APIs deployed to that gateway.
- Ensures gateway has full state after reconnect or first connect. Prefer one bulk endpoint per API or one per gateway (list subscriptions for all APIs of the org/gateway).

---

## 6. Policy Engine: Subscription Validation

### 6.1 Behaviour
- After auth (e.g. JWT) runs, request metadata should contain application id (e.g. `x-wso2-application-id`).
- Route metadata contains api id (e.g. from Envoy `api_id`).
- **Subscription validation policy:**  
  - If subscription validation is **disabled** for the API/route: skip check, continue.  
  - If enabled:  
    - Read application id from request metadata. If missing or empty → **401 Unauthorized**.  
    - Read api id from route metadata / shared context. If missing or empty → **403 Forbidden** (fail-closed).  
    - Look up (api_id, application_id) in subscription store (in-memory cache backed by gateway-controller data).  
    - If subscribed (e.g. status ACTIVE) → continue.  
    - Else → 403 Forbidden.

### 6.2 Where Subscription Data Lives in Policy Engine
- Policy engine gets config via xDS from gateway-controller. So gateway-controller should include “subscription validation” data in xDS (e.g. list of (api_id, application_id) for ACTIVE subscriptions, or a flag per API “subscription required” plus that list).
- Alternatively, policy engine could query gateway-controller admin API at runtime (higher latency). Prefer xDS so subscription data is cached in the policy engine and no extra network call per request.

### 6.3 Implementation Options
- **Option A — New policy “subscriptionValidation”:**
  - New policy in policy-engine (and Policy Hub, if/when you add it). Config: e.g. `enabled`, `applicationIdHeader` (default `x-wso2-application-id`), `forbiddenMessage`.  
  - Policy runs after JWT (or auth). Reads app id from metadata, api id from route, checks in-memory map received via xDS.  
  - Requires gateway-controller to push subscription data over xDS (new resource or extend existing policy config).
- **Option B — Extend JWT (or auth) policy:**  
  - Add optional “subscriptionValidation: true” and same semantics inside the existing policy. Tighter coupling; less flexible.
- **Recommendation:** Option A — dedicated **subscriptionValidation** policy. Clear separation; can be toggled per API/route; reuses existing xDS and metadata conventions.

### 6.4 xDS Contract (Gateway-Controller → Policy Engine)
- Add a “subscriptions” or “subscription_validation” resource in xDS (or embed in existing policy config): for each API (by api_id) that has subscription validation enabled, send set of allowed application_ids (or list of (api_id, application_id) with status).  
- Policy engine on load: build in-memory set/map; on request, check (api_id, application_id) in set.  
- When gateway-controller receives subscription events, update DB and push updated xDS snapshot so policy engine gets new set without restart.

---

## 7. Implementation Order (Phased)

### Phase 1 — Data & Platform-API
1. Add **subscriptions** table to platform-api (schema.sql, schema.sqlite.sql, schema.postgres.sql) and run migrations.  
   - **Status:** Implemented as a `subscriptions` table referencing REST APIs and application identifiers.
2. Add repository layer: `SubscriptionRepository` (CRUD).  
   - **Status:** Implemented; used by subscription service and internal gateway APIs.
3. Add **Applications** CRUD (if needed): handler + routes, e.g. `/api/v1/applications`.  
   - **Status:** **Deferred.** Applications are managed in DevPortal/STS; platform-api stores the `application_id` as an opaque identifier in subscriptions and does not expose full Application CRUD.
4. Add **/api/v1/subscriptions** CRUD in platform-api (handler, service, repository). Update OpenAPI spec and regenerate client/server code.  
   - **Status:** Implemented as a root-level `/subscriptions` resource with `apiId` and `applicationId` filters; handlers, service, repository, and OpenAPI spec are in place.
5. (Optional) Internal endpoint for gateways: e.g. **GET /api/internal/v1/subscriptions** (by gateway/org) or **GET /api/internal/v1/apis/{apiId}/subscriptions** for bulk sync.  
   - **Status:** Implemented as **GET `/api/internal/v1/apis/{apiId}/subscriptions`**; used by gateway-controller for bulk sync on connect.

### Phase 2 — Gateway-Controller Storage & REST
1. Add **subscriptions** (and optionally **applications**) tables in gateway-controller (SQLite + Postgres migrations in existing migration flow).  
   - **Status:** Implemented for `subscriptions` with appropriate indexes and unique constraints. A separate `applications` table is not required at the gateway for the current use case.
2. Extend storage interface and `sql_store`: `SaveSubscription`, `GetSubscription`, `ListSubscriptionsByAPI`, `UpdateSubscription`, `DeleteSubscription`; same for applications if needed.  
   - **Status:** Implemented with conflict detection on `(api_id, application_id, gateway_id)` and common error types.
3. Add OpenAPI paths and schemas for **/subscriptions** (POST, GET, GET by id, PUT, DELETE). Generate code (oapi-codegen).  
   - **Status:** Implemented as a root-level `/subscriptions` resource in the gateway-controller admin API OpenAPI document and wired into generated types.
4. Implement handlers that use storage and `gateway_id` from context/config.  
   - **Status:** Implemented: handlers perform CRUD over the local subscriptions table and are protected with the same auth/roles as other admin APIs.

### Phase 3 — WebSocket & Sync
1. Platform: define event payloads **subscription.created**, **subscription.updated**, **subscription.deleted** (e.g. in `platform-api/src/internal/model/...`).  
   - **Status:** Implemented as strongly-typed subscription event DTOs.
2. Platform: on subscription create/update/delete, determine target gateways (e.g. gateways that have this API deployed), and broadcast event to those gateways’ WebSocket connections (reuse existing gateway events service).  
   - **Status:** Implemented via `GatewayEventsService`, reusing the existing WebSocket broadcast infrastructure.
3. Gateway-controller: in WebSocket client, handle new event types; map platform `apiId` to deployment id; persist to subscriptions (and applications) table.  
   - **Status:** Implemented: client handles `subscription.created/updated/deleted`, resolves REST API IDs to deployments and upserts into the local subscriptions table.
4. (Optional) Platform: implement internal **GET subscriptions** for gateway; gateway-controller: on connect or periodically, fetch full subscription list for its APIs and reconcile local DB.  
   - **Status:** Implemented as a per-API bulk sync: on WebSocket connect the gateway-controller calls the internal **GET `/api/internal/v1/apis/{apiId}/subscriptions`** endpoint for each known REST API and reconciles local state.

### Phase 4 — Policy Engine & xDS
1. Gateway-controller: add xDS resource or extend policy config to include “subscription validation” data: for each `api_id` with subscription validation enabled, set of allowed `application_id`s (from subscriptions table with `status=ACTIVE`).  
   - **Status:** Implemented as a dedicated **SubscriptionState** xDS resource published by a snapshot manager in gateway-controller and merged into the existing combined cache.
2. Policy engine: add **subscriptionValidation** policy (policy.yaml + Go). Parameters: e.g. `enabled`, `applicationIdMetadataKey` (default `x-wso2-application-id`), `forbiddenStatusCode`, `forbiddenMessage`. Execution: read `application_id` from metadata, `api_id` from route metadata; check in map from xDS; return 403 if not subscribed.  
   - **Status:** Implemented as a reusable policy in `gateway/system-policies/subscriptionvalidation` plus SDK support (`SubscriptionStore`) and an xDS handler that keeps the in-memory map up to date.
3. Wire policy into builder/templates so it can be compiled and configured per API (when you add Policy Hub support, the “enable subscription validation” toggle will attach this policy).  
   - **Status:** Implemented: the Gateway Builder discovers `policy-definition.yaml`, registers the policy at build time, and APIs can attach `subscription-validation` in their policy configuration via Policy Hub or configuration. There is no automatic system-policy injection from gateway-controller.
4. Ensure JWT (or auth) policy sets `x-wso2-application-id` from token claim (e.g. `azp` or `client_id`). Document required claim for application id.  
   - **Status:** Addressed via configuration and documentation: the JWT policy remains generic, and the recommended setup is to map an application-identifying claim (for example, `azp`) to `x-wso2-application-id` using `claimMappings`. The JWT policy docs have been updated with a concrete example.

### Phase 5 — Testing & Docs
1. Unit tests: repositories, handlers, sync logic, policy.  
   - **Status:** New unit tests cover the subscription xDS store and subscription validation policy behaviour. Existing repository and handler tests have been updated where schemas changed; analytics system-policy injection continues to be covered separately.
2. Integration tests: create API → create application → create subscription → deploy API → trigger event → verify gateway has subscription → call API with token (with/without subscription) and assert 200 vs 403.  
   - **Status:** **Planned.** End-to-end tests for the full subscription lifecycle are a recommended next step but are not part of this initial implementation.
3. Update README/specs: data model, APIs, WebSocket events, and how to enable subscription validation per API.  
   - **Status:** Core docs have been updated: OpenAPI resources for new REST/internal endpoints, JWT policy docs for `x-wso2-application-id`, and this implementation plan annotated with the actual implementation details.

---

## 8. File / Component Checklist (Summary)

| Area | Action |
|------|--------|
| **platform-api** | |
| `internal/database/schema*.sql` | Add `subscriptions` and related indexes (**applications deferred; managed in DevPortal/STS**). |
| `internal/model/` | `subscription.go` (**application model deferred**). |
| `internal/repository/` | `subscription_repository.go` (**application repository deferred**). |
| `internal/service/` | `subscription_service.go` (and wire into api/gateway events; **application service deferred**). |
| `internal/handler/` | subscription_handlers.go for rest-apis; optional application_handlers.go. |
| `resources/openapi.yaml` | Paths for `/subscriptions`, optional `/applications`. |
| `internal/service/gateway_events.go` | Emit subscription.created/updated/deleted; add internal GET subscriptions if needed. |
| `internal/websocket/` | No change if events are JSON payloads; ensure payload size limits allow new events. |
| **gateway-controller** | |
| `pkg/storage/sqlite.go` (and postgres) | Migrations: applications, subscriptions tables. |
| `pkg/storage/interface.go`, `sql_store.go` | Subscription (and application) CRUD. |
| `pkg/models/` | Subscription, Application structs. |
| `resources/openapi.yaml` | `/subscriptions` CRUD. |
| `pkg/api/handlers/` | Subscription handlers. |
| `pkg/controlplane/events.go` | Subscription event payload structs. |
| `pkg/controlplane/client.go` | Handle subscription.created/updated/deleted; resolve apiId → deployment id; persist. |
| xDS | New resource or extended config for subscription allow-list per API. |
| **policy-engine** | |
| New policy | e.g. `policies/subscription-validation/` (policy.yaml + Go); read metadata + route; check allow-list; 403 if not subscribed. |
| xDS client / handler | Parse subscription validation config from xDS and build in-memory map. |
| Docs | How to enable subscription validation; required JWT claim for application id. |

---

## 9. Security & Consistency Notes

- **Authorization:** Platform subscription APIs must enforce org/project scope (only allow managing subscriptions for APIs and applications in the same organization).
- **Gateway:** Subscription REST API should be protected (e.g. same auth as existing admin API or gateway token). Internal GET subscriptions endpoint must require gateway token.
- **Idempotency:** Subscription create/update events should be idempotent (same subscriptionId → upsert) so duplicate or out-of-order WebSocket messages do not leave gateway inconsistent.
- **API deletion:** When an API is deleted (or undeployed), gateway should remove or invalidate its subscriptions for that API (cascade or on api.deleted event).

---

## 10. Open Points for Review

1. **Application ownership:** Are applications created only in DevPortal/STS, or should platform-api host full Application CRUD? (Affects whether we add `/api/v1/applications` or only reference applicationId in subscriptions.)
2. **Mapping platform apiId → gateway deployment id:** Platform uses API uuid; gateway uses deployment id (often same as api uuid when single deployment). Confirm convention (e.g. deployment id = platform api uuid for REST APIs).
3. **Subscription validation default:** Should “subscription required” be opt-in per API (via policy attachment) or default on for all APIs? Plan assumes opt-in via policy.
4. **Bulk sync:** Prefer adding **GET /api/internal/v1/.../subscriptions** for gateways vs. only relying on events (events simpler; bulk sync avoids gaps after reconnect).

Once you confirm or adjust these points, implementation can proceed in the order above.

**Current decisions in this implementation:**

- Applications remain owned by DevPortal/STS; platform-api and gateway store and use opaque `application_id` values for subscriptions without full Application CRUD at the gateway.  
- For REST APIs, deployment IDs in gateway-controller are treated as equal to platform API UUIDs when performing subscription sync and xDS publishing.  
- Subscription validation is **opt-in** per API by attaching the `subscription-validation` policy; there is no default global enforcement via a system policy or configuration flag.  
- Bulk sync is implemented as per-API internal **GET `/api/internal/v1/apis/{apiId}/subscriptions`** plus event-driven updates, combining both approaches for resilience.
