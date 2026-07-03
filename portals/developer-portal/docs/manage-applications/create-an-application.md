# Create an Application

An application is a logical representation of a physical application such as a mobile app, web app, device, or CLI tool. In the Developer Portal, applications are used to link OAuth2 client IDs — created directly in a key manager — for invoking OAuth2-secured APIs.

> **Note:** Applications are not required for subscriptions or API key generation. Subscriptions are made directly to an API, and API keys are bound to an API — not to an application. Applications are only needed for OAuth2-secured APIs. You can optionally associate an existing API key with an application afterward for usage analytics — see [Associate an API Key with an Application](../consume-an-api/consume-with-api-key.md#associate-an-api-key-with-an-application).

A developer can have multiple applications with independent OAuth2 client IDs. For example, a `MyApp-Production` application and a `MyApp-Staging` application linked to different OAuth applications in the key manager.

## Create a New Application

1. Sign in to the Developer Portal.
2. In the sidebar, click **Applications**.
3. Click **Create Application** (or **+ Create** if you already have applications).
4. Enter an application name (e.g. `MyApp-Production`).
5. Click **Create**.

The application is created and you are taken to the application detail page.

## Add an Application Description

1. Select your application.
2. Click **+ Add description** in the application header.
3. Enter a description that explains what the application does and who owns it.
4. Click the checkmark (✔) to save.

## Application Details

From the application detail page you can:

| Action | Where |
|---|---|
| Link an OAuth2 client ID | **Manage Keys** → paste client ID → **Add** |
| Generate an access token for testing | **Manage Keys** → **Generate Token** tab |
| Associate an existing API key for analytics | **API Keys** tab → **Associate existing key** |
| Edit or delete the application | Application header menu |

## Delete an Application

To delete an application:

1. Open the application.
2. Click the **Delete** option from the application menu.
3. Confirm deletion.

> **Warning:** Deleting an application removes all stored client ID mappings. It does not contact the key manager — OAuth applications there must be deleted independently if no longer needed. Existing access tokens will stop working when they expire. This action is irreversible.

## Related

- [Consume with OAuth2](../consume-an-api/consume-with-oauth2.md) — generate OAuth2 credentials for your application
- [Subscribe to an API](../consume-an-api/subscriptions.md) — subscribe to APIs (done directly, not through an application)
- [Consume with API Key](../consume-an-api/consume-with-api-key.md) — generate an API key and optionally associate it with an application
