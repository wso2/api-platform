# Subscription Design — Implementation Plan

## 1. Executive Summary

**Problem:** REST APIs in the API Platform lack subscription management with rate limiting. The previous design tied subscriptions to `(apiId, applicationId)` pairs from OAuth/JWT tokens. The new design introduces Subscription Plans, decoupled Subscription Tokens, and plan-based rate limiting.

**Solution:** 
1. **Subscription Plans** — Organization-scoped entities (Gold, Silver, Bronze, etc.) with throttle limits, billing info, and expiry.
2. **Subscription Tokens** — Each subscription generates an opaque token sent via `Subscription-Key` header. `applicationId` becomes optional (backward compat).
3. **Subscription-based Rate Limiting** — Rate limits derived from the subscription plan, keyed by the subscription token.

---

## 2. Implementation Phases

### Phase 1 — Subscription Plans Entity
- **Platform-API**: `subscription_plans` table, model, repository, service, handler, OpenAPI.
  - **Status:** Implemented. CRUD endpoints at `/api/v1/subscription-plans`.
- **Gateway-Controller**: `subscription_plans` table, model, storage, handlers, OpenAPI.
  - **Status:** Implemented. CRUD endpoints at `/api/v1/subscription-plans`.

### Phase 2 — API-Level Subscription Plans Field
- `RestAPIConfig` model extended with `SubscriptionPlans []string`.
- YAML generation includes `subscriptionPlans` in API deployment spec.
- Gateway-controller OpenAPI `APIConfigData` updated with `subscriptionPlans`.
  - **Status:** Implemented.

### Phase 3 — Subscription Token and DB Changes
- **Platform-API**:
  - `subscriptions` table: `application_id` nullable, new `subscription_token` (generated via `crypto/rand`), new `subscription_plan_uuid` FK.
  - Unique constraint: `(api_uuid, subscription_token)`.
  - Model, repository, service, handler updated.
  - **Status:** Implemented. POST response includes `subscriptionToken`.
- **Gateway-Controller**:
  - `subscriptions` table: `application_id` nullable, new `subscription_token`, new `subscription_plan_id` FK.
  - Unique constraint: `(api_id, subscription_token, gateway_id)`.
  - Model, storage, handlers updated.
  - **Status:** Implemented.

### Phase 4 — xDS and SDK Changes
- SDK `SubscriptionData` extended with `SubscriptionToken`, `ThrottleLimitCount`, `ThrottleLimitUnit`, `StopOnQuotaReach`.
- `SubscriptionStore` supports dual-key lookups:
  - `IsActiveByToken(apiID, token)` — returns `(bool, *SubscriptionEntry)` with rate limit info.
  - `IsActive(apiID, applicationID)` — backward compatible.
- Subscription snapshot manager joins with `subscription_plans` for rate limit data.
  - **Status:** Implemented.

### Phase 5 — Subscription Validation Policy Changes
- Policy definition: `enabled`, `subscriptionKeyHeader`, and `subscriptionKeyCookie` configurable.
- Hardcoded defaults: `applicationIdMetadataKey="x-wso2-application-id"`, `forbiddenStatusCode=403`, `forbiddenMessage="Subscription required for this API"`.
- Request flow: check `Subscription-Key` header first; if not found, check `subscriptionKeyCookie` (when configured); fall back to `applicationId` metadata.
- Token path enables plan-based rate limiting.
  - **Status:** Implemented. Header and cookie token sources supported.

### Phase 6 — WebSocket Event Changes
- New events: `subscriptionPlan.created`, `subscriptionPlan.updated`, `subscriptionPlan.deleted`.
- Updated subscription events include `subscriptionToken` and `subscriptionPlanId`.
- Gateway-controller handles all new event types.
  - **Status:** Implemented.

### Phase 7 — Testing and Docs
- Documentation updated: `IMPLEMENTATION_NOTES.md`, `IMPLEMENTATION_PLAN.md`, `SUBSCRIPTION_API_EXAMPLES.md`.
  - **Status:** Implemented.

---

## 3. File Impact Summary

- **Platform-API** (~15 files): schema (2), model (3 new + 2 modified), repository (1 new + 1 modified), service (1 new + 1 modified), handler (1 new + 1 modified), OpenAPI (1), events (2), dto/api.go (1), utils/api.go (1), server.go (1), constants (1), interfaces (1)
- **Gateway-Controller** (~12 files): schema (2), models (2), storage (3), handlers (1), OpenAPI (2), WebSocket client (1), events (1), xDS snapshot (1)
- **SDK** (~2 files): subscription_store.go, subscription_xds.go
- **Policy** (~1 file): subscription-validation policy definition
- **Docs** (~3 files): IMPLEMENTATION_NOTES.md, IMPLEMENTATION_PLAN.md, SUBSCRIPTION_API_EXAMPLES.md
