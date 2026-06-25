
<h1 id="wso2-api-developer-portal-core-devportal-routes">WSO2 API Developer Portal Core - Devportal Routes v1.0.0</h1>

Fine-grained Developer Portal API for managing organizations,
API metadata and content, applications, subscriptions, application credentials, and API flows.

Organization-scoped resources are served under `/o/{orgId}/devportal/v1`. Operations declare
the least-privilege OAuth2 scopes required for each resource action.

Base URLs:
* <a href="https://devportal.api-platform.io">https://devportal.api-platform.io</a>
* <a href="http://localhost:3000">http://localhost:3000</a>
* <a href="https://localhost:{port}">https://localhost:{port}</a>
    * **port** -  Default: 9443

## Table of Contents

### [Authentication](authentication.md)

### [Organizations](organizations.md)

- [Create an organization](organizations.md#create-an-organization)
- [List organizations](organizations.md#list-organizations)
- [Update an organization](organizations.md#update-an-organization)
- [Get an organization](organizations.md#get-an-organization)
- [Delete an organization](organizations.md#delete-an-organization)

### [Organization Content](organization-content.md)

- [Upload organization layout content](organization-content.md#upload-organization-layout-content)
- [Replace organization layout content](organization-content.md#replace-organization-layout-content)
- [Get a single organization layout asset](organization-content.md#get-a-single-organization-layout-asset)
- [Delete organization layout content](organization-content.md#delete-organization-layout-content)
- [List organization layout assets by file type](organization-content.md#list-organization-layout-assets-by-file-type)

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

### [Subscription Plans](subscription-plans.md)

- [List subscription plans](subscription-plans.md#list-subscription-plans)
- [Create subscription plans](subscription-plans.md#create-subscription-plans)
- [Upsert subscription plans](subscription-plans.md#upsert-subscription-plans)
- [Get a subscription plan](subscription-plans.md#get-a-subscription-plan)
- [Delete a subscription plan](subscription-plans.md#delete-a-subscription-plan)

### [Labels](labels.md)

- [Create labels](labels.md#create-labels)
- [Upsert labels](labels.md#upsert-labels)
- [List labels](labels.md#list-labels)
- [Delete labels](labels.md#delete-labels)

### [Applications](applications.md)

- [List applications for the authenticated user](applications.md#list-applications-for-the-authenticated-user)
- [Create an application](applications.md#create-an-application)
- [Update an application](applications.md#update-an-application)
- [Delete an application](applications.md#delete-an-application)

### [Subscriptions](subscriptions.md)

- [Create a subscription](subscriptions.md#create-a-subscription)
- [List subscriptions](subscriptions.md#list-subscriptions)
- [Get a subscription](subscriptions.md#get-a-subscription)
- [Update a subscription](subscriptions.md#update-a-subscription)
- [Delete a subscription](subscriptions.md#delete-a-subscription)

### [API Keys](api-keys.md)

- [Generate an API key](api-keys.md#generate-an-api-key)
- [List API keys](api-keys.md#list-api-keys)
- [Regenerate an API key](api-keys.md#regenerate-an-api-key)
- [Revoke an API key](api-keys.md#revoke-an-api-key)

### [Views](views.md)

- [Create a view](views.md#create-a-view)
- [List views](views.md#list-views)
- [Update a view](views.md#update-a-view)
- [Get a view](views.md#get-a-view)
- [Delete a view](views.md#delete-a-view)

### [Application Keys](application-keys.md)

- [Generate OAuth keys for a Developer Portal application](application-keys.md#generate-oauth-keys-for-a-developer-portal-application)
- [Generate an OAuth access token](application-keys.md#generate-an-oauth-access-token)
- [Revoke OAuth keys](application-keys.md#revoke-oauth-keys)
- [Update OAuth keys](application-keys.md#update-oauth-keys)
- [Clean up OAuth key artifacts](application-keys.md#clean-up-oauth-key-artifacts)

### [API Flows](api-flows.md)

- [Create an API flow](api-flows.md#create-an-api-flow)
- [List API flows](api-flows.md#list-api-flows)
- [Get an API flow](api-flows.md#get-an-api-flow)
- [Update an API flow](api-flows.md#update-an-api-flow)
- [Delete an API flow](api-flows.md#delete-an-api-flow)
- [Generate an API flow agent prompt](api-flows.md#generate-an-api-flow-agent-prompt)

### [Utilities](utilities.md)

- [Create a temporary Arazzo file](utilities.md#create-a-temporary-arazzo-file)

### [Authentication](authentication.md)

### [Key Managers](key-managers.md)

- [Create a key manager](key-managers.md#create-a-key-manager)
- [List key managers](key-managers.md#list-key-managers)
- [Discover available key managers](key-managers.md#discover-available-key-managers)
- [Get a key manager](key-managers.md#get-a-key-manager)
- [Update a key manager](key-managers.md#update-a-key-manager)
- [Delete a key manager](key-managers.md#delete-a-key-manager)

### [Webhook Events](webhook-events.md)

- [List webhook events](webhook-events.md#list-webhook-events)
- [Get a webhook event](webhook-events.md#get-a-webhook-event)
- [Retry a failed webhook delivery](webhook-events.md#retry-a-failed-webhook-delivery)

### [Webhook Subscribers](webhook-subscribers.md)

- [Create a webhook subscriber](webhook-subscribers.md#create-a-webhook-subscriber)
- [List webhook subscribers](webhook-subscribers.md#list-webhook-subscribers)
- [Get a webhook subscriber](webhook-subscribers.md#get-a-webhook-subscriber)
- [Update a webhook subscriber](webhook-subscribers.md#update-a-webhook-subscriber)
- [Delete a webhook subscriber](webhook-subscribers.md#delete-a-webhook-subscriber)
- [List recent deliveries for a webhook subscriber](webhook-subscribers.md#list-recent-deliveries-for-a-webhook-subscriber)

### [Schemas](schemas.md)

