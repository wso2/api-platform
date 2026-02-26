## Subscription API Examples

This document provides sample `curl` commands and payloads for the subscription-related REST APIs.

---

## 0. REST API Creation with Subscription Plans

When creating or updating a REST API, you can enable subscription plans by including `subscriptionPlans` in the payload. Each plan name must exist in the organization and have status `ACTIVE`; otherwise the request returns `400 Bad Request`.

### 0.1 Create REST API with Subscription Plans (Platform-API)

```bash
curl -X POST "$BASE_URL/api/v1/rest-apis" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "id": "weather-api",
    "name": "Weather API",
    "kind": "RestApi",
    "context": "/weather/v1.0",
    "version": "v1.0",
    "createdBy": "admin",
    "projectId": "019cd669-ae04-7a24-9ef9-06726be1c169",
    "organizationId": "b0860d7b-d7cc-4316-a3e5-c9403ac3ed91",
    "lifeCycleStatus": "CREATED",
    "transport": ["http", "https"],
    "subscriptionPlans": ["Gold", "Silver"],
    "operations": [
      {
        "name": "wildcard",
        "description": "wildcard",
        "request": {
          "method": "GET",
          "path": "/*"
        }
      }
    ],
    "upstream": {
      "main": {
        "url": "https://example.com/weather-api"
      }
    },
    "policies": [
      {
        "name": "subscription-validation",
        "version": "v0",
        "params": {
          "enabled": true,
          "subscriptionKeyHeader": "Subscription-Key"
        }
      }
    ]
  }'
```

**Notes:**
- `subscriptionPlans` is an array of plan names (e.g. `["Gold", "Silver"]`). Create these plans first via `POST /api/v1/subscription-plans`.
- Plans must exist in the organization and be `ACTIVE`. Invalid or inactive plans return `400 Bad Request`.
- The same field can be used when updating an API via `PUT /api/v1/rest-apis/{apiId}`.

---

## 1. Subscription Plans

### 1.1 Create a Subscription Plan

```bash
curl -X POST "$BASE_URL/api/v1/subscription-plans" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "planName": "Gold",
    "billingPlan": "Commercial",
    "stopOnQuotaReach": true,
    "throttleLimitCount": 10000,
    "throttleLimitUnit": "Hour"
  }'
```

**Response (201):**
```json
{
  "id": "plan-uuid-1",
  "planName": "Gold",
  "billingPlan": "Commercial",
  "stopOnQuotaReach": true,
  "throttleLimitCount": 10000,
  "throttleLimitUnit": "Hour",
  "organizationId": "org-uuid",
  "status": "ACTIVE",
  "createdAt": "2026-02-26T10:00:00Z",
  "updatedAt": "2026-02-26T10:00:00Z"
}
```

### 1.2 List Subscription Plans

```bash
curl -X GET "$BASE_URL/api/v1/subscription-plans" \
  -H "$AUTH_HEADER"
```

### 1.3 Update a Subscription Plan

```bash
curl -X PUT "$BASE_URL/api/v1/subscription-plans/$PLAN_ID" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "throttleLimitCount": 20000,
    "status": "ACTIVE"
  }'
```

### 1.4 Delete a Subscription Plan

```bash
curl -X DELETE "$BASE_URL/api/v1/subscription-plans/$PLAN_ID" \
  -H "$AUTH_HEADER"
```

---

## 2. Subscriptions (Token-Based)

### 2.1 Create a Subscription (with Plan)

```bash
curl -X POST "$BASE_URL/api/v1/subscriptions" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "apiId": "c9f2b6ae-1234-5678-9abc-def012345678",
    "subscriptionPlanId": "plan-uuid-1"
  }'
```

**Response (201):**
```json
{
  "id": "sub-uuid-1",
  "apiId": "c9f2b6ae-1234-5678-9abc-def012345678",
  "subscriptionToken": "a3f8c9d2e1b0...64-char-hex-token",
  "subscriptionPlanId": "plan-uuid-1",
  "organizationId": "org-uuid",
  "status": "ACTIVE",
  "createdAt": "2026-02-26T10:01:00Z",
  "updatedAt": "2026-02-26T10:01:00Z"
}
```

### 2.2 Create a Subscription (Legacy with ApplicationId)

```bash
curl -X POST "$BASE_URL/api/v1/subscriptions" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "apiId": "c9f2b6ae-1234-5678-9abc-def012345678",
    "applicationId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "status": "ACTIVE"
  }'
```

### 2.3 List Subscriptions

```bash
curl -X GET "$BASE_URL/api/v1/subscriptions?apiId=c9f2b6ae-1234-5678-9abc-def012345678" \
  -H "$AUTH_HEADER"
```

### 2.4 Update / Delete Subscriptions

Same as before — use subscription UUID in path.

---

## 3. Using the Subscription Token

Once a subscription is created, the response includes a `subscriptionToken`. Clients can send the token in two ways:

### 3.1 Via Header (default)

```bash
curl -X GET "https://gateway.example.com/weather/v1/current" \
  -H "Subscription-Key: a3f8c9d2e1b0...64-char-hex-token"
```

### 3.2 Via Cookie (when configured)

When the policy is configured with `subscriptionKeyCookie`, clients can send the token in a cookie:

```bash
curl -X GET "https://gateway.example.com/weather/v1/current" \
  -H "Cookie: sub-key=a3f8c9d2e1b0...64-char-hex-token"
```

The `subscription-validation` policy:
1. Reads the `Subscription-Key` header first.
2. If not found and `subscriptionKeyCookie` is set, reads the token from the named cookie.
3. Looks up the token in the subscription store.
4. If active, enforces plan-based rate limits (if any).
5. If not found, returns `403 Forbidden`.

---

## 4. Example: API Definition with Subscription Validation

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api
spec:
  displayName: Weather API
  version: v1.0
  context: /weather
  subscriptionPlans:
    - Gold
    - Silver
    - Bronze
  upstream:
    main:
      url: http://weather-backend:8080
  policies:
    - name: jwt-auth
      version: v0
      params:
        claimMappings:
          azp: x-wso2-application-id

    - name: subscription-validation
      version: v0
      params:
        enabled: true
        subscriptionKeyHeader: Subscription-Key
  operations:
    - method: GET
      path: /current
    - method: GET
      path: /forecast
```

**Runtime behaviour:**
- Requests with a valid `Subscription-Key` header (or token in `subscriptionKeyCookie` when configured) and an ACTIVE subscription proceed to the backend.
- Header takes precedence over cookie when both are present.
- If the subscription's plan has throttle limits, rate limiting is enforced.
- Requests without a subscription token fall back to `applicationId` from JWT metadata (backward compat).
- Requests with no subscription return `403 Forbidden`.

---

## 5. Gateway-Controller Admin API

### 5.1 Subscription Plans

```bash
# Create
curl -X POST "$ADMIN_BASE_URL/subscription-plans" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"planName": "Gold", "throttleLimitCount": 10000, "throttleLimitUnit": "Hour"}'

# List
curl -X GET "$ADMIN_BASE_URL/subscription-plans" -H "$ADMIN_AUTH_HEADER"

# Get by ID
curl -X GET "$ADMIN_BASE_URL/subscription-plans/$PLAN_ID" -H "$ADMIN_AUTH_HEADER"

# Update
curl -X PUT "$ADMIN_BASE_URL/subscription-plans/$PLAN_ID" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"throttleLimitCount": 20000}'

# Delete
curl -X DELETE "$ADMIN_BASE_URL/subscription-plans/$PLAN_ID" -H "$ADMIN_AUTH_HEADER"
```

### 5.2 Subscriptions

```bash
# Create (with plan)
curl -X POST "$ADMIN_BASE_URL/subscriptions" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "apiId": "019cd179-d924-753b-bc60-deba63e0c495",
    "subscriptionPlanId": "plan-uuid-1"
  }'

# Create (legacy with applicationId)
curl -X POST "$ADMIN_BASE_URL/subscriptions" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "apiId": "019cd179-d924-753b-bc60-deba63e0c495",
    "applicationId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
  }'

# List by API
curl -X GET "$ADMIN_BASE_URL/subscriptions?apiId=019cd179-d924-753b-bc60-deba63e0c495" \
  -H "$ADMIN_AUTH_HEADER"

# Get by ID
curl -X GET "$ADMIN_BASE_URL/subscriptions/$SUBSCRIPTION_ID" -H "$ADMIN_AUTH_HEADER"

# Update status
curl -X PUT "$ADMIN_BASE_URL/subscriptions/$SUBSCRIPTION_ID" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"status": "INACTIVE"}'

# Delete
curl -X DELETE "$ADMIN_BASE_URL/subscriptions/$SUBSCRIPTION_ID" -H "$ADMIN_AUTH_HEADER"
```

Response includes `subscriptionToken`, `subscriptionPlanId`, and all subscription fields.
