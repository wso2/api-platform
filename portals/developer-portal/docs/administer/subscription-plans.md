# Subscription Plans

A subscription plan (also called a subscription plan) is a named usage tier that controls how much of an API a developer can consume. You attach one or more plans to each API you publish, and developers choose a plan when subscribing.

Plans can enforce:
- Request rate limits (`requestcount`)
- Event count limits (`eventcount`) for async APIs

## Default Plans

When `generateDefaultSubPlans: true` is set in `config.yaml` (the default), the portal automatically creates four standard plans for every new organization:

| Plan | Description |
|---|---|
| `Bronze` | 1,000 requests |
| `Silver` | 2,000 requests |
| `Gold` | 5,000 requests |
| `Unlimited` | Unlimited requests |

You can create additional custom plans alongside these defaults.

## Create a Subscription Plan

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

Use the `SubscriptionPlan` manifest format:

```yaml
# plan.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: SubscriptionPlan

metadata:
  name: Enterprise

spec:
  displayName: Enterprise Plan
  type: requestcount
  requestCount: 10000
  description: Dedicated capacity for enterprise customers
```

```bash
curl -X POST http://localhost:3000/api/v0.9/subscription-plans \
  -H "Authorization: Bearer $TOKEN" \
  -F "subscriptionPlan=@plan.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | Unique plan identifier (used in API `subscriptionPlans` lists) |
| `spec.displayName` | Yes | Human-friendly name shown to developers in the portal |
| `spec.type` | Yes | `requestcount` (rate-limited) or `eventcount` |
| `spec.requestCount` | No | Maximum requests allowed. Use `-1` for unlimited |
| `spec.description` | No | Additional description shown to developers |

### Bulk Create

To create multiple plans in one request, use the `SubscriptionPlanList` kind:

```yaml
# plans.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: SubscriptionPlanList

items:
  - metadata:
      name: Starter
    spec:
      displayName: Starter

      type: requestcount
      requestCount: 500
      description: Allows 500 requests per minute

  - metadata:
      name: Pro
    spec:
      displayName: Pro

      type: requestcount
      requestCount: 20000
      description: Allows 20000 requests per minute
```

```bash
curl -X POST http://localhost:3000/api/v0.9/subscription-plans \
  -H "Authorization: Bearer $TOKEN" \
  -F "subscriptionPlan=@plans.yaml"
```

## List Subscription Plans

```bash
curl http://localhost:3000/api/v0.9/subscription-plans -H "Authorization: Bearer $TOKEN"
```

## Get a Subscription Plan

```bash
curl http://localhost:3000/api/v0.9/subscription-plans/{planId} \
  -H "Authorization: Bearer $TOKEN"
```

## Update a Subscription Plan

```yaml
# plan-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: SubscriptionPlan

metadata:
  name: Enterprise

spec:
  displayName: Enterprise Plan
  type: requestcount
  requestCount: 20000
  description: Updated capacity for enterprise customers
```

```bash
curl -X PUT http://localhost:3000/api/v0.9/subscription-plans \
  -H "Authorization: Bearer $TOKEN" \
  -F "subscriptionPlan=@plan-update.yaml"
```

## Delete a Subscription Plan

```bash
curl -X DELETE "http://localhost:3000/api/v0.9/subscription-plans/{planId}" \
  -H "Authorization: Bearer $TOKEN"
```

> **Note:** Deleting a plan that has active subscriptions will prevent those subscriptions from renewing. Verify there are no active subscribers before deleting a plan.

---

## Attaching Plans to APIs

When you [publish an API](../publish-apis/publishing-apis.md), specify which subscription plans are available for it in the `spec.subscriptionPlans` list. Developers see these plans when they subscribe. If no plans are specified, the API is accessible without a subscription plan.

