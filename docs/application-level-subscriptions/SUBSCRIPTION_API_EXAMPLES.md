## Subscription API Examples

This document provides sample `curl` commands and payloads for the subscription-related REST APIs in:

- `platform-api` (control plane)
- `gateway-controller` (admin API)

Adjust base URLs, authentication headers, and IDs as appropriate for your environment.

---

## 1. Platform-API: `/api/v1/subscriptions`

Assumptions:

- Base URL: `https://platform-api.example.com`
- All requests include a valid bearer token with org-scoped permissions:

```bash
AUTH_HEADER="Authorization: Bearer $PLATFORM_TOKEN"
BASE_URL="https://platform-api.example.com"
```

### 1.1 Create a Subscription

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

**Expected behaviour:**

- `201 Created` with a JSON body containing the created subscription.
- `400 Bad Request` if `apiId` or `applicationId` is missing/empty.
- `404 Not Found` if the API does not exist or does not belong to the caller’s organization.
- `409 Conflict` if a subscription for the same `(apiId, applicationId)` already exists.

### 1.2 List Subscriptions (Filtered)

List subscriptions for a given API and application:

```bash
curl -X GET "$BASE_URL/api/v1/subscriptions?apiId=c9f2b6ae-1234-5678-9abc-def012345678&applicationId=a1b2c3d4-e5f6-7890-abcd-ef1234567890" \
  -H "$AUTH_HEADER"
```

List all subscriptions for a given application across APIs:

```bash
curl -X GET "$BASE_URL/api/v1/subscriptions?applicationId=a1b2c3d4-e5f6-7890-abcd-ef1234567890" \
  -H "$AUTH_HEADER"
```

List all subscriptions (org-scoped):

```bash
curl -X GET "$BASE_URL/api/v1/subscriptions" \
  -H "$AUTH_HEADER"
```

### 1.3 Get a Subscription by ID

```bash
SUBSCRIPTION_ID="e7d9b1a0-1234-5678-9abc-def012345678"

curl -X GET "$BASE_URL/api/v1/subscriptions/$SUBSCRIPTION_ID" \
  -H "$AUTH_HEADER"
```

**Expected behaviour:**

- `200 OK` with the subscription representation.
- `404 Not Found` if the subscription does not exist or is not in the caller’s organization.

### 1.4 Update Subscription Status

Set a subscription to `INACTIVE`:

```bash
curl -X PUT "$BASE_URL/api/v1/subscriptions/$SUBSCRIPTION_ID" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "INACTIVE"
  }'
```

**Expected behaviour:**

- `200 OK` with the updated subscription.
- `400 Bad Request` for invalid status values.
- `404 Not Found` if the subscription is not found.

### 1.5 Delete a Subscription

```bash
curl -X DELETE "$BASE_URL/api/v1/subscriptions/$SUBSCRIPTION_ID" \
  -H "$AUTH_HEADER"
```

**Expected behaviour:**

- `204 No Content` on success.
- `404 Not Found` if the subscription does not exist.

---

## 2. Platform-API: Internal Bulk Sync Endpoint

This endpoint is intended for gateways and typically uses a gateway token:

- Base URL: `https://platform-api.internal.example.com`

```bash
GATEWAY_TOKEN="..."  # token issued for the gateway
AUTH_HEADER="api-key: $GATEWAY_TOKEN"
BASE_INTERNAL_URL="https://platform-api.internal.example.com"
API_ID="c9f2b6ae-1234-5678-9abc-def012345678"

curl -X GET "$BASE_INTERNAL_URL/api/internal/v1/apis/$API_ID/subscriptions" \
  -H "$AUTH_HEADER"
```

**Expected behaviour:**

- `200 OK` with a JSON array of subscriptions for the given API ID within the gateway’s organization.
- `404 Not Found` if the API does not exist or is not visible to the gateway.

---

## 3. Gateway-Controller Admin API: `/subscriptions`

Assumptions:

- Gateway-controller admin API is exposed at: `https://gateway-controller.example.com`
- Authentication uses an admin token or equivalent:

```bash
ADMIN_TOKEN="..."
ADMIN_AUTH_HEADER="Authorization: Bearer $ADMIN_TOKEN"
ADMIN_BASE_URL="https://gateway-controller.example.com"
```

### 3.1 Create a Subscription

```bash
curl -X POST "$ADMIN_BASE_URL/subscriptions" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "apiId": "deployment-id-or-handle",
    "applicationId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "status": "ACTIVE"
  }'
```

**Notes:**

- `apiId` can be either a deployment ID or an API handle; the handler resolves it to the internal deployment ID before persisting.
- The gateway’s configured `gateway_id` is applied automatically; you do not pass it in the request body.

### 3.2 List Subscriptions

List all subscriptions on this gateway:

```bash
curl -X GET "$ADMIN_BASE_URL/subscriptions" \
  -H "$ADMIN_AUTH_HEADER"
```

Filter by API:

```bash
curl -X GET "$ADMIN_BASE_URL/subscriptions?apiId=deployment-id-or-handle" \
  -H "$ADMIN_AUTH_HEADER"
```

Filter by application:

```bash
curl -X GET "$ADMIN_BASE_URL/subscriptions?applicationId=a1b2c3d4-e5f6-7890-abcd-ef1234567890" \
  -H "$ADMIN_AUTH_HEADER"
```

Filter by status:

```bash
curl -X GET "$ADMIN_BASE_URL/subscriptions?status=ACTIVE" \
  -H "$ADMIN_AUTH_HEADER"
```

Filters can be combined, for example:

```bash
curl -X GET "$ADMIN_BASE_URL/subscriptions?apiId=deployment-id-or-handle&applicationId=a1b2c3d4-e5f6-7890-abcd-ef1234567890&status=ACTIVE" \
  -H "$ADMIN_AUTH_HEADER"
```

### 3.3 Get a Subscription by ID

```bash
GW_SUBSCRIPTION_ID="sub-uuid-on-gateway"

curl -X GET "$ADMIN_BASE_URL/subscriptions/$GW_SUBSCRIPTION_ID" \
  -H "$ADMIN_AUTH_HEADER"
```

### 3.4 Update a Subscription

```bash
curl -X PUT "$ADMIN_BASE_URL/subscriptions/$GW_SUBSCRIPTION_ID" \
  -H "$ADMIN_AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "INACTIVE"
  }'
```

### 3.5 Delete a Subscription

```bash
curl -X DELETE "$ADMIN_BASE_URL/subscriptions/$GW_SUBSCRIPTION_ID" \
  -H "$ADMIN_AUTH_HEADER"
```

**Expected behaviour for admin API:**

- `201 Created` on successful creation.
- `200 OK` on successful read/update, with a JSON subscription representation.
- `204 No Content` on successful delete.
- `400/404/409/500` as per OpenAPI error responses for validation, not-found, conflict, or unexpected storage errors.

---

## 4. Example: API Definition with Subscription Validation Policy

To enforce subscriptions for a specific API, attach the `subscription-validation` policy in the API definition, alongside your authentication policy. The example below assumes:

- JWT authentication populates `x-wso2-application-id` from a claim such as `azp`.
- The `subscription-validation` policy has been compiled into the gateway (as defined in `gateway/system-policies/subscriptionvalidation/policy-definition.yaml`).

```yaml
apiVersion: gateway.wso2.com/v1alpha1
kind: API
metadata:
  name: sample-api
spec:
  basePath: /sample
  version: v1
  routes:
    - path: /hello
      methods: [GET]
      backend:
        url: http://sample-backend:8080
      policies:
        # Authentication policy: validates JWT and sets x-wso2-application-id
        - name: jwt-auth
          version: v0
          parameters:
            claimMappings:
              azp: x-wso2-application-id

        # Subscription validation policy: enforces ACTIVE subscription for (apiId, applicationId)
        - name: subscription-validation
          version: v0.1.0
          parameters:
            enabled: true
            applicationIdMetadataKey: x-wso2-application-id
            forbiddenStatusCode: 403
            forbiddenMessage: "Subscription required for this API"
```

**Runtime behaviour:**

- Requests with a valid JWT and an **ACTIVE** subscription for `(apiId, applicationId)` proceed to the backend.
- Requests with no subscription, or with `INACTIVE`/`REVOKED` subscriptions, receive a `403` response from the `subscription-validation` policy.

