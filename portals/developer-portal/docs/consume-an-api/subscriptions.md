# Subscribe to an API

A subscription grants you access to a specific API under a chosen subscription plan, which determines your rate limits and quota. Subscriptions are made directly to an API — no application is required.

## Subscribe to an API

1. Sign in to the Developer Portal.
2. Click **APIs** from the sidebar.
3. Find the API you want to access and open it.
4. Click **Subscribe** in the API's banner (or scroll to the **Subscription plans** section).
5. Click **Subscribe** on the plan card you want (e.g. Bronze, Gold, Unlimited).

The subscription is created immediately when you click **Subscribe** on a plan — there is no separate confirmation step. Once subscribed, you can invoke the API under the terms of the chosen plan. If the API uses API key authentication, you can now [generate an API key](consume-with-api-key.md) for it.

## Subscription Plans

Subscription plans control how much of the API you can consume:

| Plan | Typical limits |
|---|---|
| Bronze | Conservative rate limits — suitable for development and testing |
| Silver | Moderate limits — suitable for small production workloads |
| Gold | High capacity — suitable for production at scale |
| Unlimited | No rate limits |

Available plans are defined by the API publisher. Contact the API owner for details on what each plan includes.

## Subscriptionless APIs

Some APIs are configured to allow direct invocation without subscribing. For these APIs:

1. Navigate to the API in the catalog.
2. Click the **Try-Out** tab.
3. Invoke the API directly using your credentials.

> **Note:** Subscriptionless access is typically for testing and exploration. For production use, subscribing is recommended — it gives you quota management and key lifecycle control.

## View Your Subscriptions

Your active subscriptions are listed under **Subscriptions** in the Developer Portal sidebar. From there you can see which APIs you are subscribed to, the active plan for each, and manage or cancel subscriptions.

## Cancel a Subscription

To cancel a subscription:

1. Go to **Subscriptions** in the sidebar.
2. Find the API subscription you want to cancel.
3. Click **Unsubscribe**.

## Related

- [Consume with API Key](consume-with-api-key.md) — generate an API key for a subscribed API
- [Consume with OAuth2](consume-with-oauth2.md) — generate OAuth2 credentials via an application
- [Subscription Plans](../administer/subscription-plans.md) — admin guide for managing plans
