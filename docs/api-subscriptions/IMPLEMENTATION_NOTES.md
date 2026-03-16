# Subscription Design — Implementation Notes

This document captures how the **Subscription Design** (including Subscription Plans, Subscription Tokens, and token-based rate limiting) is implemented in the codebase. It supersedes the earlier "Application-Level Subscriptions" notes.

---

## 1. Platform-API

### 1.1 Data & Model

- **Subscription Plans**
  - New `subscription_plans` table (SQLite/Postgres) with columns:
    - `uuid`, `plan_name`, `billing_plan`, `stop_on_quota_reach`, `throttle_limit_count`, `throttle_limit_unit`, `expiry_time`, `organization_uuid`, `status`, timestamps.
    - Unique constraint: `(organization_uuid, plan_name)`.
    - Status: `ACTIVE`, `INACTIVE`.
  - Go model: `SubscriptionPlan` in `platform-api/src/internal/model/subscription_plan.go`.

- **Subscriptions**
  - `subscriptions` table updated:
    - `application_id` is now **nullable** (optional).
    - `subscription_token VARCHAR(512)` — stores **encrypted** token (AES-256-GCM) for retrieval on GET; legacy rows have hash.
    - `subscription_token_hash VARCHAR(64) NOT NULL` — SHA-256 hash for uniqueness and gateway sync.
    - New `subscription_plan_uuid VARCHAR(40)` — FK to `subscription_plans(uuid)`.
    - Unique constraint: `(api_uuid, subscription_token_hash)` for token-based; `(api_uuid, application_id, organization_uuid)` for app-based.
    - Index on `subscription_token_hash`.
  - Encryption key: `DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY` (32 bytes, 64 hex or 44 base64); falls back to `JWT_SECRET_KEY`.
  - Go model: `Subscription` with `ApplicationID *string`, `SubscriptionToken string` (decrypted for API), `SubscriptionTokenHash` (internal).

### 1.2 Repositories, Services, and Handlers

- **Subscription Plan Repository** (`subscription_plan_repository.go`)
  - CRUD: Create, GetByID, GetByNameAndOrg, ListByOrganization, Update, Delete, ExistsByNameAndOrg.
- **Subscription Plan Service** (`subscription_plan_service.go`)
  - Validates uniqueness by `(planName, orgUUID)`.
  - Defaults `status` to `ACTIVE` and `stopOnQuotaReach` to `true`.
- **Subscription Plan Handler** (`subscription_plan_handler.go`)
  - `POST /api/v1/subscription-plans`
  - `GET /api/v1/subscription-plans`
  - `GET /api/v1/subscription-plans/{planId}`
  - `PUT /api/v1/subscription-plans/{planId}`
  - `DELETE /api/v1/subscription-plans/{planId}`

- **Subscription Repository** — Updated to handle new columns; generates subscription token via `crypto/rand`; encrypts before storage; decrypts on read for GET/List. Legacy hashed tokens return empty string.
- **Subscription Service** — `CreateSubscription` now accepts optional `applicationId *string` and `subscriptionPlanId *string`.
- **Subscription Handler** — `POST /api/v1/subscriptions` body: `{ apiId (required), applicationId?, subscriptionPlanId?, status? }`. Only `apiId` is required; `applicationId` is optional for token-based subscriptions. Response includes `subscriptionToken` (decrypted, visible on GET/List), `subscriptionPlanId`, and all other subscription fields.

### 1.3 API Configuration

- **`RestAPIConfig`** has a new `SubscriptionPlans []string` field listing plan names available for the API.
- **YAML generation** (`GenerateAPIDeploymentYAML`) includes `subscriptionPlans` in the output spec.
- **Subscription plan validation**: When creating or updating a REST API with `subscriptionPlans`, platform-api validates that each plan name exists in the organization and has status `ACTIVE`. Invalid or inactive plans return `400 Bad Request` with `ErrSubscriptionPlanNotFoundOrInactive`.

### 1.4 Gateway Events (WebSocket)

- **Subscription plan events**: `subscriptionPlan.created`, `subscriptionPlan.updated`, `subscriptionPlan.deleted`.
- **Subscription events**: Updated to include `subscriptionToken` and `subscriptionPlanId` in payloads.

---

## 2. Gateway-Controller

### 2.1 Data & Storage

- **Subscription Plans table** — New in both SQLite and Postgres schemas. Columns mirror platform-api but scoped by `gateway_id`.
- **Subscriptions table** — Updated: `application_id` nullable, new `subscription_token TEXT NOT NULL`, new `subscription_plan_id TEXT` FK to plans. Unique constraint: `(api_id, subscription_token, gateway_id)`.

- **Storage interface** — Extended with:
  - `SaveSubscriptionPlan`, `GetSubscriptionPlanByID`, `ListSubscriptionPlans`, `UpdateSubscriptionPlan`, `DeleteSubscriptionPlan`
  - Subscription CRUD updated for new columns.

### 2.2 Admin REST API

- New `/subscription-plans` CRUD endpoints (OpenAPI + handlers).
- `/subscriptions` endpoints updated:
  - `POST /subscriptions` accepts `{ apiId (required), applicationId?, subscriptionPlanId?, status? }`.
  - Response includes `subscriptionToken`, `subscriptionPlanId`, and all subscription fields.
  - `applicationId` is no longer required (optional for token-based subscriptions).

### 2.3 WebSocket Client & Sync

- Handles `subscriptionPlan.created`, `subscriptionPlan.updated`, `subscriptionPlan.deleted` events.
- Updated subscription event handlers for `subscriptionToken` and `subscriptionPlanId`.

---

## 3. xDS: Subscription State

### 3.1 SDK Types

- `SubscriptionData` now includes:
  - `SubscriptionToken string` — primary identifier for token-based lookups.
  - `ApplicationId string` — optional, for backward compatibility.
  - `ThrottleLimitCount int`, `ThrottleLimitUnit string`, `StopOnQuotaReach bool` — rate limit info from the plan.

### 3.2 Subscription Store

- Dual-key lookup:
  - **Token-based** (`IsActiveByToken(apiID, token)`) — returns `(bool, *SubscriptionEntry)` with rate limit info.
  - **ApplicationId-based** (`IsActive(apiID, applicationID)`) — backward compatible, returns `bool`.
- `SubscriptionEntry` struct holds `Status`, `ThrottleLimitCount`, `ThrottleLimitUnit`, `StopOnQuotaReach`.

### 3.3 Subscription Snapshot Manager

- Joins `subscriptions` with `subscription_plans` when building xDS snapshot.
- Includes `subscriptionToken` and plan rate limit info in each `SubscriptionData` entry.

---

## 4. Subscription Validation Policy

### 4.1 Policy Definition

- Parameters:
  - `enabled` (bool, default `true`)
  - `subscriptionKeyHeader` (string, default `Subscription-Key`)
  - `subscriptionKeyCookie` (string, default `""`) — optional cookie name for the subscription token
- Hardcoded defaults in policy Go code:
  - `applicationIdMetadataKey` → `"x-wso2-application-id"`
  - `forbiddenStatusCode` → `403`
  - `forbiddenMessage` → `"Subscription required for this API"`

### 4.2 Request Flow

1. Check `enabled` — if `false`, no-op.
2. Read `Subscription-Key` header (configurable name).
3. If header present → `store.IsActiveByToken(apiID, token)`:
   - Active → check rate limits from plan, then continue.
   - Not found → `403 Forbidden`.
4. If header not present and `subscriptionKeyCookie` is set → parse `Cookie` header and read the named cookie:
   - If cookie present → `store.IsActiveByToken(apiID, token)` (same as step 3).
5. If still no token → fall back to `applicationId` from metadata:
   - Present → `store.IsActive(apiID, applicationId)`:
     - Active → continue (legacy flow, no rate limiting).
     - Not found → `403 Forbidden`.
   - Not present → `401 Unauthorized`.
6. Rate limiting (token path only):
   - If subscription has throttle limits → check/decrement counter.
   - Counter key: `subscription:<subscriptionToken>`.
   - If exceeded and `stopOnQuotaReach` → `429 Too Many Requests`.
   - If exceeded and `!stopOnQuotaReach` → log and continue.

---

## 5. How to Enable Subscription Validation

1. **Create subscription plans** via `POST /api/v1/subscription-plans` (Gold, Silver, Bronze, etc.). Plans must be `ACTIVE`.
2. **Configure the API** — Add plan names to the API's `subscriptionPlans` field when creating or updating the API. Platform-API validates that each plan exists in the organization and is ACTIVE; invalid plans return 400.
3. **Attach the `subscription-validation` policy** to the API's policy chain.
4. **Create subscriptions** via `POST /api/v1/subscriptions` — optionally with `subscriptionPlanId`.
5. **Use the subscription token** — Clients send `Subscription-Key: <token>` header, or (when `subscriptionKeyCookie` is configured) send the token in a cookie.
6. **Alternatively** (legacy) — Configure JWT/auth policy to map application ID claim to `x-wso2-application-id` metadata.

---

## 6. Gateway-Controller Subscription Token Storage

- **Events**: Platform-API sends **plain** subscription token in WebSocket events (created/updated/deleted). Transport is WSS (TLS).
- **Storage**: Gateway encrypts the token (AES-256-GCM) before storing. Also stores `subscription_token_hash` for validation and uniqueness.
- **Schema**: `subscription_token` (encrypted), `subscription_token_hash` (SHA-256). Unique constraint on `(api_id, subscription_token_hash, gateway_id)`.
- **Token storage**: Gateway stores only `subscription_token_hash` for validation. Use Platform-API to retrieve the original token.
- **GET /subscriptions**: Decrypts and returns plain token.
- **xDS/Policy**: Uses `subscription_token_hash` for validation (policy hashes client token and looks up).
