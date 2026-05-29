# Consume an API Secured with OAuth2

The Developer Portal uses OAuth 2.0 bearer token-based authentication for OAuth2-secured APIs. You generate a consumer key and secret for your application, then use them to obtain an access token from the key manager.

## Prerequisites

Before proceeding, ensure you have [created an application](../manage-applications/create-an-application.md). Applications hold the OAuth2 credentials used to authenticate against OAuth2-secured APIs.

If the API also requires a subscription, [subscribe to it](subscriptions.md) first (subscriptions are made directly to the API, independently of your application).

---

## Generate OAuth2 Credentials

1. Sign in to the Developer Portal.
2. In the sidebar, click **Applications**.
3. Click on the application you subscribed with.
4. In the application banner, click **Manage Keys**. This opens the **Manage Keys** page.
5. Select the **Production** or **Sandbox** tab based on your environment.

   > **Note:** Sandbox keys can only be used in the sandbox environment.

6. Click **Generate** to create the consumer key and secret. The portal registers your application with the key manager and returns the credentials.
7. Close the dialog. The consumer key is now visible on the Manage Keys page, but the consumer secret is shown only once — save it securely.

---

## Generate an Access Token

Use the consumer key and secret to obtain an access token from the key manager's token endpoint.

**Client credentials grant (server-to-server):**

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

1. On the **Manage Keys** page, click **Generate** next to the token section.
2. Optionally add scopes in the **Request Permissions** section.
3. Copy the displayed access token.

Alternatively, click **Instructions** on the Manage Keys page to see the exact `curl` command for your application's credentials.

---

## Invoke the API

Include the access token in the `Authorization` header when calling the API:

```bash
curl -X GET "https://api.example.com/orders/v1/orders" \
  -H "Authorization: Bearer <YOUR_ACCESS_TOKEN>"
```

> **Note:** The authorization header name may vary depending on the API's configuration. Check the API's OpenAPI specification (the `securitySchemes` section) for the correct header name and scheme.

---

## Revoke an Access Token

To revoke an access token before it expires, use the key manager's revoke endpoint. You can find the revoke endpoint URL by clicking **Instructions** on the Manage Keys page.

```bash
curl -X POST https://keymanager.example.com/oauth2/revoke \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "token=<ACCESS_TOKEN>" \
  -u "<CONSUMER_KEY>:<CONSUMER_SECRET>"
```

---

## Customize Key Generation Settings

To adjust token expiry times or other key settings:

1. On the **Manage Keys** page, click **View** or **Modify** next to your generated credentials.
2. Update the settings:
   - **Access token expiry time** — how long the token is valid (seconds)
   - **Refresh token expiry time** — how long the refresh token is valid
   - **ID token expiry time**
   - **Allow invoking the API without a secret** — enables public client flows

3. Click **Update** to apply changes.

---

## Related

- [Subscribe to an API](subscriptions.md) — subscribe before generating credentials
- [Consume with API Key](consume-with-api-key.md) — alternative for API-key-secured APIs
- [Key Manager Integration](../administer/key-manager-integration.md) — admin guide for key manager setup
