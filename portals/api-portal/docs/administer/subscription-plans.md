# Subscription Plans

A subscription plan (also called a subscription plan) is a named usage tier that controls how much of an API a developer can consume. You attach one or more plans to each API you publish, and developers choose a plan when subscribing.

A plan's limits are its conditions:
- Requests (`request-count`)
- Events (`event-count`), for event-based APIs (WebSocket, WebSub)
- Tool calls (`tool-call-count`), for MCP servers
- Tokens (`token-count`), for AI APIs
- Custom conditions, key-value settings that a policy reads

A plan saved before conditions shows its request limits as a request-count condition and its event limits as an event-count condition.

## Default Plans

When `auto_create_subscription_plans = true` is set in `config.toml` under `[organization]` (the default), the portal automatically creates five standard plans for every new organization:

| Plan | Description |
|---|---|
| `Bronze` | 1,000 requests per minute |
| `Silver` | 2,000 requests per minute |
| `Gold` | 5,000 requests per minute |
| `Unlimited` | Unlimited requests |
| `AsyncUnlimited` | Unlimited events, for async/webhook APIs |

You can create additional custom plans alongside these defaults.

## Create a Subscription Plan

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=<admin-username>&password=<admin-password>" | jq -r .token)
> ```

Use the `SubscriptionPlan` manifest format:

```yaml
# plan.yaml
apiVersion: api-portal.api-platform.wso2.com/v1
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
curl -k -X POST https://localhost:9543/api-portal/api/v0.9/subscription-plans \
  -H "Authorization: Bearer $TOKEN" \
  -F "subscriptionPlan=@plan.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | Unique plan identifier (used in API `subscriptionPlans` lists) |
| `spec.displayName` | Yes | Human-friendly name shown to developers in the portal |
| `spec.type` | Yes | `requestcount` (rate-limited) or `eventcount` |
| `spec.requestCount` | No | Maximum requests per minute, with `type: requestcount`. Use `-1` for unlimited. For other windows, use `spec.conditions` |
| `spec.eventCount` | No | Maximum events per minute, with `type: eventcount`: an event-count condition. Use `-1` for unlimited |
| `spec.description` | No | Additional description shown to developers |
| `spec.conditions` | No | The plan's limits; see [Conditions](#conditions) |
| `spec.pricing` | No | A paid plan's pricing: the same fields as `pricing` in JSON, under the same rules. A plan without it is free |

### Conditions

Give a plan its limits as `spec.conditions` in YAML, or `conditions` in JSON:

```yaml
spec:
  displayName: Enterprise Plan
  conditions:
    - type: request-count
      params:
        limits:
          - { count: 10000, timeAmount: 1, timeUnit: MINUTE }
          - { count: 1000000, timeAmount: 1, timeUnit: DAY }
        onExceed: reject
    - type: event-count
      params:
        limits:
          - { count: 50000, timeAmount: 1, timeUnit: HOUR }
    - type: tool-call-count
      params:
        limits:
          - { count: 500, timeAmount: 1, timeUnit: HOUR }
    - type: token-count
      params:
        limits:
          - { count: 2000000, timeAmount: 1, timeUnit: MONTH }
        tokenType: total
    - id: region
      type: custom
      params: { zone: eu }
```

The portal checks conditions with the same rules as the Platform API:

- A plan has at most 20 conditions.
- It has at most one of each built-in type. Each built-in type has a fixed `id`: `requests`, `events`, `tool-calls` or `tokens`.
- A built-in condition has 1 to 10 limits. Each limit is a `count` per `timeAmount` `timeUnit`, where the unit is `MINUTE`, `HOUR`, `DAY` or `MONTH`. No two limits can cover the same length of time, so 60 `MINUTE` and 1 `HOUR` clash.
- `onExceed` is `reject` (the default) or `allow`.
- `tokenType` is `total` (the default), `prompt` or `completion`. It applies to token conditions only.
- A custom condition has its own `id`, made of lowercase letters, digits and hyphens. Its `params` are key-value pairs, at most 4 KB.

Sending `conditions` replaces all of the plan's conditions; `[]` removes them. Leaving it out keeps them.

Older clients can still send `limits` alone. The `REQUEST_COUNT` limits then set the request-count condition and the `EVENT_COUNT` limits the event-count condition; a type with no such limits has its condition removed, and the plan's other conditions stay as they were. Each needs a `timeUnit`; one without it is refused. Unlimited (`-1`) limits and other limit types are not kept. Responses still carry `limits`: the windows of the request-count condition as `REQUEST_COUNT` limits and of the event-count condition as `EVENT_COUNT` limits.

**Upgrading:** conditions are stored in the `subscription_plan_conditions` table, which replaces `subscription_plan_limits`. SQLite adds the table at startup. For PostgreSQL and SQL Server, re-run `schema.postgres.sql` or `schema.sqlserver.sql`; the scripts are safe to run again. Until then, plans keep only their request and event limits, and conditions of other types are refused. A plan saved before the upgrade reads its request and event limits as request-count and event-count conditions. It moves to the new table the next time a save gives its `conditions` or `limits`; a save that changes only other fields, such as the name, leaves it where it is.

A paid plan carries its pricing in `spec.pricing`. Pricing is given only when the plan is created and can't be updated after that. Write amounts as quoted decimals, as in JSON. A usage price names an existing meter by its id in `meterId`; create the meter first.

```yaml
# paid-plans.yaml
apiVersion: api-portal.api-platform.wso2.com/v1
kind: SubscriptionPlanList
items:
  - metadata:
      name: starter
    spec:
      displayName: Starter
      type: requestcount
      requestCount: 10000
      pricing:
        pricingType: FLAT_RATE
        currency: USD
        billingPeriod: MONTH
        flatAmount: "29.00"
  - metadata:
      name: pay-as-you-go
    spec:
      displayName: Pay as you go
      pricing:
        pricingType: USAGE_BASED
        currency: USD
        billingPeriod: MONTH
        prices:
          - meterId: successful-calls
            pricingModel: PER_UNIT
            unitAmount: "0.002"
            filters:
              - field: HTTP method
                operator: IN
                values: [GET, POST]
```

### Bulk Create

To create multiple plans in one request, use the `SubscriptionPlanList` kind:

```yaml
# plans.yaml
apiVersion: api-portal.api-platform.wso2.com/v1
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
curl -k -X POST https://localhost:9543/api-portal/api/v0.9/subscription-plans \
  -H "Authorization: Bearer $TOKEN" \
  -F "subscriptionPlan=@plans.yaml"
```

## List Subscription Plans

```bash
curl -k https://localhost:9543/api-portal/api/v0.9/subscription-plans -H "Authorization: Bearer $TOKEN"
```

## Get a Subscription Plan

```bash
curl -k https://localhost:9543/api-portal/api/v0.9/subscription-plans/{planId} \
  -H "Authorization: Bearer $TOKEN"
```

## Update a Subscription Plan

```yaml
# plan-update.yaml
apiVersion: api-portal.api-platform.wso2.com/v1
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
curl -k -X PUT https://localhost:9543/api-portal/api/v0.9/subscription-plans \
  -H "Authorization: Bearer $TOKEN" \
  -F "subscriptionPlan=@plan-update.yaml"
```

## Delete a Subscription Plan

```bash
curl -k -X DELETE "https://localhost:9543/api-portal/api/v0.9/subscription-plans/{planId}" \
  -H "Authorization: Bearer $TOKEN"
```

> **Note:** A plan that has subscriptions cannot be deleted. The request is refused with `409` and code `PLAN_IN_USE`, for example "The plan Gold is in use and can't be deleted." Suspended subscriptions, and paid subscriptions that run until the end of their billing period, still count.

---

## Attaching Plans to APIs

When you [publish an API](../publish-apis/publishing-apis.md), specify which subscription plans are available for it in the `spec.subscriptionPlans` list. Developers see these plans when they subscribe. If no plans are specified, the API is accessible without a subscription plan.

