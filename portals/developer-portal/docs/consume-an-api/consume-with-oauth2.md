# Consume an API Secured with OAuth2

The Developer Portal uses OAuth 2.0 bearer token-based authentication for OAuth2-secured APIs. The OAuth application itself is created directly in the key manager — the portal only links its client ID to your application and proxies token requests for it.

## Prerequisites

Before proceeding, ensure you have [created an application](../manage-applications/create-an-application.md). Applications hold the OAuth2 client ID(s) used to authenticate against OAuth2-secured APIs.

If the API also requires a subscription, [subscribe to it](subscriptions.md) first (subscriptions are made directly to the API, independently of your application).

You will also need an OAuth application already created in the key manager your organization uses — ask your administrator which key manager(s) are available and how to create an OAuth application there if you haven't done so.

---

## Link a Client ID

1. Sign in to the Developer Portal.
2. In the sidebar, click **Applications**.
3. Click on the application you subscribed with.
4. In the application banner, click **Manage Keys**. This opens the **Manage Keys** page.
5. Select the **Production** or **Sandbox** tab based on your environment.

   > **Note:** Sandbox keys can only be used in the sandbox environment.

6. For the key manager you want to use, paste the **client ID** of the OAuth application you created there, then click **Add**.

The client ID is now visible on the Manage Keys page. The portal does not ask for or store a client secret at this step.

---

## Generate an Access Token

Use the linked client ID and your application's client secret to obtain an access token from the key manager's token endpoint.

**Client credentials grant (server-to-server), calling the key manager directly:**

```bash
curl -X POST https://keymanager.example.com/oauth2/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -u "<CONSUMER_KEY>:<CONSUMER_SECRET>"
```

The response contains the access token:

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiJ9...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**Via the portal UI:**

1. On the **Manage Keys** page, open the **Generate Token** tab for the key manager.
2. Optionally add scopes in the **Request Permissions** section.
3. Click **Generate access token**. You'll be prompted for the client secret — it is used once to proxy the token request and is never stored.
4. Copy the displayed access token.

Alternatively, open the **cURL** tab on the Manage Keys page to copy a ready-made `curl` command (with your client ID already filled in) for generating tokens directly against the key manager, without going through the portal.

---

## Invoke the API

Include the access token in the `Authorization` header when calling the API:

```bash
curl -X GET "https://api.example.com/orders/v1/orders" \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

> **Note:** The authorization header name may vary depending on the API's configuration. Check the API's OpenAPI specification (the `securitySchemes` section) for the correct header name and scheme.

---

## Revoke a Client ID

To remove a linked client ID, go to **Manage Keys** and click **Revoke keys** for that key manager. This only removes the local reference in the portal — it does not deregister or delete the OAuth application in the key manager, and any tokens already issued remain valid until they expire. To invalidate the OAuth application itself or revoke a specific token, use the key manager's own console or revoke endpoint.

---

## Related

- [Subscribe to an API](subscriptions.md) — subscribe before generating credentials
- [Consume with API Key](consume-with-api-key.md) — alternative for API-key-secured APIs
- [Key Manager Integration](../administer/key-manager-integration.md) — admin guide for key manager setup
