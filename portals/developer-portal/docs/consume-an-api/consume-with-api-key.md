# Consume an API Secured with API Key

API keys are bound to a specific API. You generate a key directly for an API, and that key authenticates your requests to that API.

## Prerequisites

The API must have API Key authentication enabled. Check the API's documentation or security section in the specification to confirm.

If the API requires a subscription, [subscribe to it](subscriptions.md) first.

---

## Generate an API Key

1. Sign in to the Developer Portal.
2. Click **APIs** from the sidebar and open the API you want to invoke.
3. Click **Manage Keys** (or navigate to **My APIs** → select the API → **Manage Keys**).
4. Select the **Production** or **Sandbox** tab based on your environment.

   > **Note:** Sandbox keys can only be used in the sandbox environment.

5. Click **Generate Key**.
6. In the **Generate API Key** dialog, enter a name for the key (e.g. `my-prod-key`).
7. Click **Generate** and wait for the key to be created.
8. **Copy the API key immediately.** The key will not be visible in the UI after you close this dialog.
9. Click **Done**.

---

## Invoke the API

Include the generated API key in the `api-key` request header when calling the API:

```bash
curl -X GET "https://api.example.com/orders/v1/orders" \
  -H "api-key: <YOUR_API_KEY>"
```

Replace `<YOUR_API_KEY>` with the key you copied and the URL with the API's actual endpoint.

---

## Rotate an API Key

If a key is compromised or you want to rotate it as a security practice:

1. Navigate to the API's **Manage Keys** page.
2. Click **Regenerate** next to the key.
3. Confirm the regeneration. The old key is immediately invalidated.
4. Copy the new key from the dialog.

> **Important:** Update all services using the old key before or immediately after regenerating. The old key stops working as soon as regeneration is complete.

---

## Revoke an API Key

To permanently invalidate a key:

1. Navigate to the API's **Manage Keys** page.
2. Click **Revoke** next to the key.
3. Confirm the revocation.

Revoked keys cannot be recovered. Generate a new key if you need access again.

---

## Key Lifecycle Events

When you generate, regenerate, or revoke an API key, the portal sends a real-time webhook event to the API Gateway. The gateway enforces the change immediately — there is no propagation delay.

## Related

- [Subscribe to an API](subscriptions.md) — subscribe if the API requires a subscription
- [Consume with OAuth2](consume-with-oauth2.md) — alternative for OAuth2-secured APIs
