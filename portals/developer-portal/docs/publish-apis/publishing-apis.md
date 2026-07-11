# Publishing APIs

Publishing an API makes it discoverable in the portal catalog. You upload an API manifest file together with the API's definition file. Developers can then find, read documentation for, and subscribe to the API.

## Supported API Types

| Type | Kind | Definition Format |
|---|---|---|
| REST | `RestApi` | OpenAPI 2.0 / 3.x (YAML or JSON) |
| Async | `WS` | AsyncAPI 2.x / 3.x (YAML or JSON) |
| GraphQL | `GraphQL` | GraphQL schema SDL |
| SOAP | `SOAP` | WSDL (XML) |
| WebSub | `WebSubApi` | AsyncAPI |
| MCP | `MCP` | MCP server specification |

## Step 1 — Create the API Manifest

Create an API manifest file using the appropriate `kind`. The file **must** be named `api.yaml`, `mcp.yaml`, or `devportal.yaml`.

```yaml
# api.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: RestApi   # RestApi | WS | GraphQL | SOAP | WebSubApi

metadata:
  name: order-api-v1   # API handle — used internally and in URLs

spec:
  type: REST           # REST | WS | GRAPHQL | SOAP | WEBSUB
  displayName: Order API
  version: v1.0
  description: Create and manage customer orders
  status: PUBLISHED

  tags:
    - ecommerce
    - orders

  labels:
    - default

  subscriptionPlans:
    - Bronze
    - Gold
    - Unlimited

  visibility: PUBLIC
  visibleGroups: []

  businessInformation:
    businessOwner: Commerce Team
    businessOwnerEmail: commerce@example.com
    technicalOwner: Orders Team
    technicalOwnerEmail: orders-team@example.com

  endpoints:
    productionUrl: https://api.example.com/orders
    sandboxUrl: https://sandbox.example.com/orders
```

For an MCP server, use `mcp.yaml` instead:

```yaml
# mcp.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: MCP

metadata:
  name: my-mcp-v1

spec:
  type: MCP
  displayName: My MCP Server
  version: 1.0.0
  description: MCP server exposing AI tools.
  status: PUBLISHED

  labels:
    - default

  subscriptionPlans:
    - Gold

  visibility: PUBLIC

  businessInformation:
    businessOwner: Platform Team
    businessOwnerEmail: platform-team@example.com
    technicalOwner: Platform Team
    technicalOwnerEmail: platform-team@example.com

  endpoints:
    productionUrl: https://your-mcp-host.example.com
```

## Step 2 — Upload the API

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

Send the manifest and definition together as a multipart upload:

```bash
# REST API with OpenAPI definition
curl -k -X POST "https://localhost:3000/api/v0.9/apis" \
  -H "Authorization: Bearer $TOKEN" \
  -F "api=@api.yaml" \
  -F "apiDefinition=@openapi.yaml;type=application/yaml"
```

```bash
# GraphQL API
curl -k -X POST "https://localhost:3000/api/v0.9/apis" \
  -H "Authorization: Bearer $TOKEN" \
  -F "api=@api.yaml" \
  -F "apiDefinition=@schema.graphql;type=application/graphql"
```

```bash
# MCP server (note: MCP servers are created under /mcp-servers, not /apis).
# An MCP server's contract is its tools schema (schemaDefinition) — it has no apiDefinition.
curl -k -X POST "https://localhost:3000/api/v0.9/mcp-servers" \
  -H "Authorization: Bearer $TOKEN" \
  -F "api=@mcp.yaml" \
  -F "schemaDefinition=@schemaDefinition.yaml;type=application/yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | API handle — URL-safe identifier used internally |
| `spec.type` | Yes | API type: `REST`, `WS`, `GRAPHQL`, `SOAP`, `WEBSUB`, or `MCP` |
| `spec.displayName` | Yes | Display name shown in the catalog |
| `spec.version` | Yes | Version string (e.g. `v1.0`, `2.3`) |
| `spec.description` | No | Short description shown in the catalog listing |
| `spec.status` | No | `PUBLISHED` (default) or `DEPRECATED` |
| `spec.tags` | No | Tags for search and filtering |
| `spec.labels` | No | Labels that control which views the API appears in |
| `spec.subscriptionPlans` | No | Names of subscription plans available for this API |
| `spec.visibility` | No | `PUBLIC` (visible to all) or `PRIVATE` (restricted) |
| `spec.visibleGroups` | No | Groups that can see a `PRIVATE` API |
| `spec.endpoints.productionUrl` | No | Production gateway URL |
| `spec.endpoints.sandboxUrl` | No | Sandbox gateway URL |
| `spec.businessInformation` | No | Business and technical owner contact details |

The response includes the `apiId` needed for subsequent steps.

## Step 3 — Update the API Definition (Optional)

`PUT /apis/{apiId}` replaces the definition file, but it always requires the metadata alongside it. Include `apiMetadata` with at least `id` (matching the API's existing handle), `name`, and `endPoints`, together with the new definition file:

```bash
# OpenAPI YAML
curl -k -X PUT \
  "https://localhost:3000/api/v0.9/apis/{apiId}" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'apiMetadata={"id":"{apiId}","name":"Order API","endPoints":{"productionURL":"https://api.example.com/orders","sandboxURL":"https://sandbox.example.com/orders"}}' \
  -F "apiDefinition=@openapi.yaml;type=application/yaml"
```

```bash
# AsyncAPI YAML
curl -k -X PUT \
  "https://localhost:3000/api/v0.9/apis/{apiId}" \
  -H "Authorization: Bearer $TOKEN" \
  -F 'apiMetadata={"id":"{apiId}","name":"Order API","endPoints":{"productionURL":"https://api.example.com/orders","sandboxURL":"https://sandbox.example.com/orders"}}' \
  -F "apiDefinition=@asyncapi.yaml;type=application/yaml"
```

The uploaded definition is shown in the API's **Try-Out** tab and is exposed at the machine-readable spec endpoint for AI agent consumption.

> **Note:** `POST`/`PUT /apis/{apiId}/assets` is a different endpoint used to upload the API's static content package (landing-page assets and documents) — see [API Content and Docs](api-content-and-docs.md). It does not accept the API definition file.

## Step 4 — Add Documentation Content (Optional)

Add landing page content and documentation sections to give developers context about the API. See [API Content and Docs](api-content-and-docs.md) for details.

## Access Control Patterns

The portal supports three distinct consumption patterns. The pattern is determined by the combination of `spec.subscriptionPlans` in the API manifest and the `securitySchemes` (and optional extension headers) in the OpenAPI definition.

---

### 1. API Key Only

The consumer generates an API key from the portal and uses it directly to invoke the API — no subscription is needed.

**`api.yaml`**

```yaml
spec:
  subscriptionPlans: []   # no subscription required
```

**`openapi.yaml`** (relevant excerpt)

```yaml
components:
  securitySchemes:
    ApiKeyHeader:
      type: apiKey
      in: header
      name: X-API-Key
```

Consumers go to the API's **Manage Keys** page, generate a key, and include it as `X-API-Key` on every request.

---

### 2. API Key with Direct Subscription

The consumer subscribes to a plan and then generates an API key bound to that API. The key alone is sufficient to invoke the API — the gateway enforces the plan tier via the key's subscription association.

**`api.yaml`**

```yaml
spec:
  subscriptionPlans:
    - Gold
    - Bronze
```

**`openapi.yaml`** (relevant excerpt)

```yaml
components:
  securitySchemes:
    ApiKeyHeader:
      type: apiKey   # presence of this scheme marks the API as API-key capable
      in: header
      name: X-API-Key
```

Consumers subscribe to a plan first, then generate an API key. They send only the API key header on each request.

---

### 3. API Key with Token-Based Subscription

The consumer subscribes to a plan and generates an API key. They also receive a **subscription token** at subscription time. Both the API key and the subscription token must be sent as headers on every request.

**`api.yaml`**

```yaml
spec:
  subscriptionPlans:
    - Gold
    - Silver
    - Bronze
```

**`openapi.yaml`** (relevant excerpt)

```yaml
components:
  securitySchemes:
    ApiKeyHeader:
      type: apiKey   # marks the API as API-key capable
      in: header
      name: X-API-Key
  parameters:
    SubscriptionTokenHeader:
      name: X-Subscription-Token
      x-header-type: subscription-token   # marks the API as token-based subscription
      in: header
      required: true
      schema:
        type: string
```

Consumers subscribe to a plan (receiving a subscription token), generate an API key, and include both `X-API-Key` and `X-Subscription-Token` on every request.

---

### Summary

| Pattern | `subscriptionPlans` | `securitySchemes` | `x-header-type: subscription-token` |
|---|---|---|---|
| API key only | `[]` | `apiKey` | No |
| API key + direct subscription | one or more plans | `apiKey` | No |
| API key + token subscription | one or more plans | `apiKey` | Yes |

## Update an API

```yaml
# api-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: RestApi

metadata:
  name: order-api-v1

spec:
  description: Updated description for the Order API
  labels:
    - default
    - ecommerce
  subscriptionPlans:
    - Bronze
    - Gold
```

```bash
curl -k -X PUT https://localhost:3000/api/v0.9/apis/{apiId} \
  -H "Authorization: Bearer $TOKEN" \
  -F "api=@api-update.yaml"
```

## Delete an API

```bash
curl -k -X DELETE https://localhost:3000/api/v0.9/apis/{apiId} \
  -H "Authorization: Bearer $TOKEN"
```

> **Note:** Deleting an API removes it from the catalog immediately. Existing subscriptions to the API are not automatically cancelled — notify subscribers before deletion.

## List APIs

```bash
curl -k https://localhost:3000/api/v0.9/apis -H "Authorization: Bearer $TOKEN"
```

## Get an API

```bash
curl -k https://localhost:3000/api/v0.9/apis/{apiId} -H "Authorization: Bearer $TOKEN"
```

## Related

- [API Content and Docs](api-content-and-docs.md) — upload landing page content and documentation
- [Subscription Plans](../administer/subscription-plans.md) — create the plans referenced in `spec.subscriptionPlans`
- [Manage Views](../administer/manage-views.md) — configure labels and views
- [API Workflows](manage-api-workflows.md) — publish multi-step workflows for this API
- [Consume with API Key](../consume-an-api/consume-with-api-key.md) — developer view of generating API keys
- [Subscribe to an API](../consume-an-api/subscriptions.md) — developer view of subscriptions
