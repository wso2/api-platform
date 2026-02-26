# Application-Level Subscriptions — Implementation Notes

This document captures how the **Application-Level Subscriptions for REST APIs** feature is implemented in the codebase, based on the high-level plan in `IMPLEMENTATION_PLAN.md`.

It is organized by component and focuses on actual files, types, and flows.

---

## 1. Platform-API

### 1.1 Data & Model

- **Subscriptions**
  - New `subscriptions` table added to the platform-api schema (SQLite/Postgres).
  - Subscriptions reference:
    - `api_uuid` → REST API (`rest_apis`).
    - `application_id` → opaque application identifier (owned by DevPortal/STS).
    - `organization_uuid` → owning organization.
    - `status` → `ACTIVE`, `INACTIVE`, `REVOKED`.
    - Timestamps and uniqueness constraints on `(api_uuid, application_id)`.
    - Tenant consistency is enforced by a composite foreign key on `(api_uuid, organization_uuid)` referencing `artifacts(uuid, organization_uuid)`, ensuring a subscription’s API and organization always match.
  - Represented in Go as a `Subscription` model under `platform-api/src/internal/model`.

### 1.2 Repositories, Services, and Handlers

- **Repository**
  - `SubscriptionRepository` provides CRUD operations over the `subscriptions` table.
- **Service**
  - `SubscriptionService` implements business logic for:
    - Create, update, and delete subscriptions.
    - Enforcing uniqueness and basic validation.
    - Emitting subscription lifecycle events to gateways via `GatewayEventsService`.
- **REST Handlers**
  - New endpoints under root-level `/api/v1/subscriptions`:
    - `POST /api/v1/subscriptions` → create subscription (body includes `apiId`, `applicationId`, `status`).
    - `GET /api/v1/subscriptions?apiId=&applicationId=&status=` → list subscriptions filtered by API and/or application.
    - `GET /api/v1/subscriptions/{subscriptionId}` → retrieve a single subscription.
    - `PUT /api/v1/subscriptions/{subscriptionId}` → update subscription (typically `status`).
    - `DELETE /api/v1/subscriptions/{subscriptionId}` → perform a hard delete and permanently remove the subscription record.
  - OpenAPI spec updated in `platform-api/src/resources/openapi.yaml` to use `/subscriptions` instead of `/rest-apis/{apiId}/subscriptions`.

### 1.3 Internal Gateway API (Bulk Sync)

- New internal endpoint:
  - `GET /api/internal/v1/apis/{apiId}/subscriptions`
  - Returns the authoritative list of subscriptions for the given REST API ID.
  - Used by gateway-controller on connect (or reconnection) to perform **bulk sync** and reconcile its local subscriptions DB.
- Implemented via:
  - `GatewayInternalAPIService` method `ListSubscriptionsForAPI`.
  - Corresponding handler and route in `platform-api/src/internal/handler/gateway_internal.go`.

### 1.4 Gateway Events (WebSocket)

- Subscription lifecycle events:
  - `subscription.created`
  - `subscription.updated`
  - `subscription.deleted`
- Implemented as:
  - Strongly-typed event payload structs under `platform-api/src/internal/model`.
  - New methods on `GatewayEventsService` for broadcasting subscription events to connected gateways.
  - Integration from `SubscriptionService` so that all CRUD operations emit the appropriate events to the relevant gateways.

---

## 2. Gateway-Controller

### 2.1 Data & Storage

- **Subscriptions table**
  - New `subscriptions` table added in:
    - SQLite schema.
    - Postgres schema (`gateway/gateway-controller/pkg/storage/gateway-controller-db.postgres.sql`).
  - Columns:
    - `id` (subscription ID).
    - `gateway_id`.
    - `api_id` (deployment ID; equals platform API UUID for REST APIs).
    - `application_id` (opaque ID).
    - `status` (`ACTIVE`, `INACTIVE`, `REVOKED`).
    - `created_at`, `updated_at`.
  - Unique constraint: `(api_id, application_id, gateway_id)`.

- **Storage interface & implementation**
  - `sql_store.go` extended with:
    - `SaveSubscription`
    - `GetSubscriptionByID`
    - `ListSubscriptionsByAPI`
    - `UpdateSubscription`
    - `DeleteSubscription`
  - Unique-constraint helpers:
    - SQLite: `isSubscriptionUniqueConstraintError`.
    - Postgres: `isPostgresSubscriptionUniqueConstraintError`.
  - `SaveSubscription` maps underlying DB unique violations to a common `ErrConflict`.

### 2.2 Admin REST API: `/subscriptions`

- OpenAPI spec:
  - Extended in `gateway/gateway-controller/api/openapi.yaml` and `resources/openapi.yaml` with root-level endpoints:
    - `POST /subscriptions`
    - `GET /subscriptions`
    - `GET /subscriptions/{subscriptionId}`
    - `PUT /subscriptions/{subscriptionId}`
    - `DELETE /subscriptions/{subscriptionId}`
  - `SubscriptionCreateRequest` includes `apiId`, `applicationId`, and optional `status` to support cross-API usage.
  - Defines request/response DTOs for subscription operations.
- Handlers:
  - Implemented in `gateway/gateway-controller/pkg/api/handlers/handlers.go`:
    - `CreateSubscription`
    - `ListSubscriptions`
    - `GetSubscription`
    - `UpdateSubscription`
    - `DeleteSubscription`
  - All handlers:
    - Use the shared storage interface.
    - Derive `gateway_id` from configuration/context.
    - Return structured error responses consistent with existing admin APIs.
    - Accept `apiId` as either a deployment ID or API handle and normalize it to the internal deployment ID before persisting or filtering.
    - Treat `apiId`, `applicationId`, and `status` as optional filters on `GET /subscriptions`; omitting `apiId` returns subscriptions for all APIs on the gateway.

### 2.3 WebSocket Client & Sync

- **Event handling**
  - `gateway/gateway-controller/pkg/controlplane/client.go` extended to:
    - Recognize `subscription.created`, `subscription.updated`, `subscription.deleted` event types.
    - Decode event payloads and map platform `apiId` (REST API UUID) to local deployment IDs.
    - Upsert/delete corresponding entries in the local `subscriptions` table.
- **Bulk sync on connect**
  - On WebSocket connection/ack:
    - Gateway-controller enumerates existing REST API deployments.
    - For each REST API deployment ID, it calls:
      - `GET /api/internal/v1/apis/{apiId}/subscriptions` on platform-api.
    - Results are reconciled into the local subscriptions table.
  - HTTP client logic for bulk fetch implemented in:
    - `gateway/gateway-controller/pkg/utils/api_utils.go` (e.g. `FetchSubscriptionsForAPI`).

---

## 3. xDS: Subscription State (Gateway-Controller → Policy Engine)

### 3.1 SDK Types

- Located at `sdk/gateway/policyengine/v1`:
  - `SubscriptionData`
    - `APIId` (REST API / deployment ID).
    - `ApplicationId` (opaque ID from DevPortal/STS).
    - `Status` (string, e.g. `ACTIVE`).
  - `SubscriptionStateResource`
    - `Subscriptions []SubscriptionData`
    - `Version int64`
    - `Timestamp time.Time`

### 3.2 Subscription Snapshot Manager

- New package under gateway-controller:
  - `gateway/gateway-controller/pkg/subscriptionxds`
  - Responsibilities:
    - Query the local `subscriptions` table for all `ACTIVE` subscriptions.
    - Build a `SubscriptionStateResource` snapshot.
    - Store it in a `LinearCache` keyed by a custom type URL (`SubscriptionStateTypeURL`).
    - Update the snapshot whenever subscriptions change or at startup.

### 3.3 Policy xDS Combined Cache and Server

- Combined cache:
  - `gateway/gateway-controller/pkg/policyxds/combined_cache.go` extended to:
    - Include a fourth cache for `SubscriptionState`.
    - Forward watch and delta-watch requests appropriately based on the requested type URL.
    - Merge subscription state responses into the outgoing ADS stream.
- Server wiring:
  - `gateway/gateway-controller/pkg/policyxds/server.go` updated to:
    - Accept a `subscriptionSnapshotMgr`.
    - Pass `subscriptionSnapshotMgr.GetCache()` into the combined cache.
  - `gateway/gateway-controller/cmd/controller/main.go`:
    - Instantiates the subscription snapshot manager.
    - Calls `UpdateSnapshot` at startup.
    - Injects its cache into the policy xDS server.

---

## 4. Policy Engine Runtime

### 4.1 Subscription State Store and Handler

- SDK-level store:
  - `sdk/gateway/policyengine/v1/subscription_store.go`
  - Exposes:
    - `SubscriptionStore` (map of `apiId → applicationId → status`).
    - `GetSubscriptionStoreInstance()` singleton accessor.
    - `ReplaceAll([]SubscriptionData)` — state-of-the-world update that rebuilds the snapshot off-lock and swaps it under a short write-lock to minimize contention with request-path lookups.
    - `IsActive(apiID, applicationID string) bool` — returns `true` only when status is `ACTIVE`.
- xDS client integration:
  - `gateway/gateway-runtime/policy-engine/internal/xdsclient/subscription_handler.go`:
    - `SubscriptionStateHandler`:
      - Unwraps the Any/Struct envelope from xDS.
      - Deserializes to `SubscriptionStateResource`.
      - Calls `store.ReplaceAll` with the current snapshot.
  - `gateway/gateway-runtime/policy-engine/internal/xdsclient/handler.go`:
    - `ResourceHandler` now holds:
      - A shared `SubscriptionStore` (from SDK singleton).
      - A `SubscriptionStateHandler` instance.
    - Wires subscription state handling alongside policy chains and other resources.
  - `gateway/gateway-runtime/policy-engine/internal/xdsclient/client.go`:
    - Sends and processes `DiscoveryRequest/DiscoveryResponse` messages for `SubscriptionStateTypeURL`.
    - Tracks a separate version for subscription state.

### 4.2 `subscriptionValidation` Policy

- Location:
  - Code: `gateway/system-policies/subscriptionvalidation/subscription_validation.go`
  - Definition: `gateway/system-policies/subscriptionvalidation/policy-definition.yaml`
- Parameters:
  - `enabled` (bool, default `true`).
  - `applicationIdMetadataKey` (string, default `x-wso2-application-id`).
  - `forbiddenStatusCode` (int, default `403`).
  - `forbiddenMessage` (string, default `"Subscription required for this API"`).
- Behaviour:
  - Runs in **request** phase only; header processing enabled, body processing skipped.
  - If `enabled=false` → no-op.
  - Reads:
    - `apiId` from `SharedContext.APIId` (populated from route metadata).
    - `applicationId` from `SharedContext.Metadata[applicationIdMetadataKey]`.
  - If required metadata is missing or empty:
    - Missing or empty `applicationId` → immediate `401` with a JSON error body (unauthenticated caller or missing application context).
    - Missing or empty `apiId` in the shared context → immediate `403` (fail-closed) with a JSON error body, rather than bypassing validation.
  - Otherwise:
    - Delegates to `SubscriptionStore.IsActive(apiID, applicationId)`.
    - If not active:
      - Returns an immediate `403` with a JSON payload (`{"error":"forbidden","message": ...}`).
    - If active:
      - Returns `nil` and lets the chain continue.

### 4.3 Policy Registration

- Builder-time registration:
  - The Gateway Builder discovers `policy-definition.yaml` under `gateway/system-policies/subscriptionvalidation`.
  - `plugin_registry.go.tmpl` generates code that registers:
    - Name: `subscription-validation`
    - Version: `v0.1.0`
    - Factory: `subscriptionvalidation.GetPolicy`
- Usage as a reusable policy:
  - The compiled `subscription-validation` policy is available like any other gateway policy (for example, `jwt-auth`, `api-key-auth`, or `set-headers`).
  - APIs opt in by attaching this policy by name and version in their policy configuration; there is no automatic system-level injection from gateway-controller.

---

## 5. Authentication and Application ID (`x-wso2-application-id`)

- **Canonical metadata key**
  - Analytics and subscription validation both use:
    - `x-wso2-application-id` as the runtime application identifier.
  - Defined by:
    - `AppIDKey = "x-wso2-application-id"` in `gateway/gateway-runtime/policy-engine/internal/analytics/constants.go`.
- **JWT policy configuration**
  - The JWT auth policy remains generic and does not hardcode subscription behaviour.
  - Recommended configuration (documented in `docs/gateway/policies/jwt-authentication.md`):

    ```yaml
    policies:
      - name: jwt-auth
        version: v0
        params:
          claimMappings:
            azp: x-wso2-application-id
    ```

  - This maps the `azp` claim (or whichever claim carries the application ID) into the `x-wso2-application-id` metadata key.
  - `subscriptionValidation` then reads this metadata key via `applicationIdMetadataKey`.

---

## 6. Testing & Guardrails

### 6.1 Unit Tests

- **Gateway-controller system policies**
  - `gateway/gateway-controller/pkg/utils/system_policies_test.go`
    - Tests verify analytics system-policy injection and parameter merging.

- **Subscription store (SDK)**
  - `sdk/gateway/policyengine/v1/subscription_store_test.go`
    - Verifies:
      - `ReplaceAll` builds the correct `apiId → applicationId → status` map.
      - `IsActive` returns `true` **only** for `ACTIVE` subscriptions and `false` for missing/inactive entries and empty IDs.

- **`subscriptionValidation` policy**
  - `gateway/system-policies/subscriptionvalidation/subscription_validation_test.go`
    - Uses a small fake store to isolate policy logic.
    - Covers:
      - `mergeConfig` (default vs overrides).
      - Disabled policy (no action).
      - Active subscription (no action).
      - Missing or inactive subscription (immediate `403` JSON response).
      - Missing application ID metadata (immediate `403`).

### 6.2 Integration Testing (Recommended)

While not part of the initial implementation, the intended end-to-end test flow is:

1. Create a REST API in platform-api.
2. Create an application in DevPortal/STS and obtain a token whose claim (e.g. `azp`) maps to the application ID.
3. Create a subscription (platform-api) linking the API and application.
4. Deploy the API to a gateway.
5. Let subscription events and bulk sync populate gateway-controller and policy-engine subscription state.
6. Call the API:
   - With a token whose application has an active subscription → expect `200`.
   - With a token whose application has **no** subscription → expect `403` from `subscriptionValidation`.

---

## 7. How to Enable Subscription Validation

At a high level, to enable subscription enforcement for REST APIs:

1. **Ensure JWT (or auth) policy sets application ID metadata**
   - Configure `claimMappings` so that the application identifier claim (for example, `azp` or `client_id`) is mapped to `x-wso2-application-id`.
2. **Attach the `subscription-validation` policy to the API**
   - In the API's policy configuration, add `subscription-validation` (version `v0.1.0`) to the appropriate phase (typically request policies), just like other policies such as `jwt-auth`, `api-key-auth`, or `set-headers`.
3. **Create subscriptions in platform-api**
   - Use the root-level `/api/v1/subscriptions` endpoint (and any DevPortal UI) to manage which applications are subscribed to which APIs, using `apiId`/`applicationId` filters as needed.

Once these are in place, the runtime path for each API call is:

- Auth policy validates the token and populates `x-wso2-application-id` into metadata.
- Policy engine receives `api_id` from route metadata and subscription state via xDS.
- `subscriptionValidation` checks `(api_id, application_id)` in the subscription store and either:
  - Allows the call to proceed (active subscription).
  - Returns a `403` with a JSON error response (no active subscription).

