
<h1 id="wso2-api-developer-portal-core-devportal-routes">WSO2 API Developer Portal Core - Devportal Routes v0.9.0</h1>

Fine-grained Developer Portal API for managing organizations,
API metadata and content, applications, subscriptions, application appKeyMappings, and API flows.

All resources, including organization lifecycle endpoints
(`/api/v0.9/organizations`, `/api/v0.9/organizations/{orgId}`), are served under `/api/v0.9`.
Operations declare the least-privilege OAuth2 scopes required for each resource action.

Base URLs:
* <a href="https://localhost:3000/api/v0.9">https://localhost:3000/api/v0.9</a>
* <a href="http://localhost:3000/api/v0.9">http://localhost:3000/api/v0.9</a>

## Table of Contents

### [Authentication](authentication.md)

### [Organizations](organizations.md)

- [Create an organization](organizations.md#create-an-organization)
- [List organizations](organizations.md#list-organizations)
- [Update an organization](organizations.md#update-an-organization)
- [Get an organization](organizations.md#get-an-organization)
- [Delete an organization](organizations.md#delete-an-organization)

### [Organization Content](organization-content.md)

- [Get a theme asset](organization-content.md#get-a-theme-asset)
- [Apply a theme](organization-content.md#apply-a-theme)
- [Reset theme to defaults](organization-content.md#reset-theme-to-defaults)
- [Download the current theme](organization-content.md#download-the-current-theme)

### [APIs](apis.md)

- [Create API metadata](apis.md#create-api-metadata)
- [List API metadata](apis.md#list-api-metadata)
- [Get API metadata](apis.md#get-api-metadata)
- [Update API metadata](apis.md#update-api-metadata)
- [Delete API metadata](apis.md#delete-api-metadata)

### [API Content](api-content.md)

- [Upload API content](api-content.md#upload-api-content)
- [Replace API content](api-content.md#replace-api-content)
- [Get an API content file](api-content.md#get-an-api-content-file)
- [Delete API content files](api-content.md#delete-api-content-files)

### [MCP Servers](mcp-servers.md)

- [Create MCP server metadata](mcp-servers.md#create-mcp-server-metadata)
- [List MCP server metadata](mcp-servers.md#list-mcp-server-metadata)
- [Get MCP server metadata](mcp-servers.md#get-mcp-server-metadata)
- [Update MCP server metadata](mcp-servers.md#update-mcp-server-metadata)
- [Delete MCP server metadata](mcp-servers.md#delete-mcp-server-metadata)

### [MCP Server Content](mcp-server-content.md)

- [Upload MCP server content](mcp-server-content.md#upload-mcp-server-content)
- [Replace MCP server content](mcp-server-content.md#replace-mcp-server-content)
- [Get an MCP server content file](mcp-server-content.md#get-an-mcp-server-content-file)
- [Delete MCP server content files](mcp-server-content.md#delete-mcp-server-content-files)

### [MCP Server Keys](mcp-server-keys.md)

- [Generate an MCP server API key](mcp-server-keys.md#generate-an-mcp-server-api-key)
- [List MCP server API keys](mcp-server-keys.md#list-mcp-server-api-keys)
- [Regenerate an MCP server API key](mcp-server-keys.md#regenerate-an-mcp-server-api-key)
- [Revoke an MCP server API key](mcp-server-keys.md#revoke-an-mcp-server-api-key)
- [Associate an MCP server API key with an application](mcp-server-keys.md#associate-an-mcp-server-api-key-with-an-application)
- [Remove an MCP server API key's application association](mcp-server-keys.md#remove-an-mcp-server-api-keys-application-association)

### [Subscription Plans](subscription-plans.md)

- [List subscription plans](subscription-plans.md#list-subscription-plans)
- [Create subscription plans](subscription-plans.md#create-subscription-plans)
- [Upsert subscription plans](subscription-plans.md#upsert-subscription-plans)
- [Get a subscription plan](subscription-plans.md#get-a-subscription-plan)
- [Delete a subscription plan](subscription-plans.md#delete-a-subscription-plan)

### [Labels](labels.md)

- [Create a label](labels.md#create-a-label)
- [List labels](labels.md#list-labels)
- [Get a label](labels.md#get-a-label)
- [Update a label](labels.md#update-a-label)
- [Delete a label](labels.md#delete-a-label)

### [Applications](applications.md)

- [List applications for the authenticated user](applications.md#list-applications-for-the-authenticated-user)
- [Create an application](applications.md#create-an-application)
- [Get an application](applications.md#get-an-application)
- [Update an application](applications.md#update-an-application)
- [Delete an application](applications.md#delete-an-application)

### [Subscriptions](subscriptions.md)

- [Create a subscription](subscriptions.md#create-a-subscription)
- [List subscriptions](subscriptions.md#list-subscriptions)
- [Get a subscription](subscriptions.md#get-a-subscription)
- [Update a subscription](subscriptions.md#update-a-subscription)
- [Delete a subscription](subscriptions.md#delete-a-subscription)
- [Change subscription plan](subscriptions.md#change-subscription-plan)
- [Regenerate subscription token](subscriptions.md#regenerate-subscription-token)

### [API Keys](api-keys.md)

- [List all API keys for the current user](api-keys.md#list-all-api-keys-for-the-current-user)
- [Generate an API key](api-keys.md#generate-an-api-key)
- [List API keys](api-keys.md#list-api-keys)
- [Regenerate an API key](api-keys.md#regenerate-an-api-key)
- [Revoke an API key](api-keys.md#revoke-an-api-key)
- [Associate an API key with an application](api-keys.md#associate-an-api-key-with-an-application)
- [Remove an API key's application association](api-keys.md#remove-an-api-keys-application-association)
- [List API keys associated with an application](api-keys.md#list-api-keys-associated-with-an-application)

### [Views](views.md)

- [Create a view](views.md#create-a-view)
- [List views](views.md#list-views)
- [Update a view](views.md#update-a-view)
- [Get a view](views.md#get-a-view)
- [Delete a view](views.md#delete-a-view)

### [Application Keys](application-keys.md)

- [Map an OAuth client_id to a Developer Portal application](application-keys.md#map-an-oauth-clientid-to-a-developer-portal-application)
- [Generate an OAuth access token](application-keys.md#generate-an-oauth-access-token)
- [Remove an OAuth client_id mapping](application-keys.md#remove-an-oauth-clientid-mapping)

### [API Workflows](api-workflows.md)

- [Create an API workflow](api-workflows.md#create-an-api-workflow)
- [List API workflows](api-workflows.md#list-api-workflows)
- [Get an API workflow](api-workflows.md#get-an-api-workflow)
- [Update an API workflow](api-workflows.md#update-an-api-workflow)
- [Delete an API workflow](api-workflows.md#delete-an-api-workflow)
- [Generate an API workflow agent prompt](api-workflows.md#generate-an-api-workflow-agent-prompt)

### [Key Managers](key-managers.md)

- [Create a key manager](key-managers.md#create-a-key-manager)
- [List key managers](key-managers.md#list-key-managers)
- [Get a key manager](key-managers.md#get-a-key-manager)
- [Update a key manager](key-managers.md#update-a-key-manager)
- [Delete a key manager](key-managers.md#delete-a-key-manager)

### [Webhook Events](webhook-events.md)

- [List webhook events](webhook-events.md#list-webhook-events)
- [Get a webhook event](webhook-events.md#get-a-webhook-event)

### [Webhook Subscribers](webhook-subscribers.md)

- [Create a webhook subscriber](webhook-subscribers.md#create-a-webhook-subscriber)
- [List webhook subscribers](webhook-subscribers.md#list-webhook-subscribers)
- [Get a webhook subscriber](webhook-subscribers.md#get-a-webhook-subscriber)
- [Update a webhook subscriber](webhook-subscribers.md#update-a-webhook-subscriber)
- [Delete a webhook subscriber](webhook-subscribers.md#delete-a-webhook-subscriber)
- [List recent deliveries for a webhook subscriber](webhook-subscribers.md#list-recent-deliveries-for-a-webhook-subscriber)

### [Schemas](schemas.md)

