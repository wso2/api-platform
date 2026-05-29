
<h1 id="wso2-api-developer-portal-core-devportal-routes">WSO2 API Developer Portal Core - Devportal Routes v1.0.0</h1>

Fine-grained Developer Portal API for managing organizations, identity providers, providers,
API metadata and content, applications, subscriptions, application credentials, billing, invoices, usage, and API flows.

All paths are served under the `/devportal` base path. Operations declare the least-privilege
OAuth2 scopes required for each resource action, and selected billing-related endpoints also
support API key authentication where configured.

Base URLs:
* <a href="http://localhost:3000/devportal">http://localhost:3000/devportal</a>
* <a href="https://localhost:{port}/devportal">https://localhost:{port}/devportal</a>
    * **port** -  Default: 9443

## Table of Contents

### [Authentication](authentication.md)

### [Organizations](organizations.md)

- [Create an organization](organizations.md#create-an-organization)
- [List organizations](organizations.md#list-organizations)
- [Update an organization](organizations.md#update-an-organization)
- [Get an organization](organizations.md#get-an-organization)
- [Delete an organization](organizations.md#delete-an-organization)

### [Identity Providers](identity-providers.md)

- [Create an identity provider](identity-providers.md#create-an-identity-provider)
- [Update an identity provider](identity-providers.md#update-an-identity-provider)
- [Get an identity provider](identity-providers.md#get-an-identity-provider)
- [Delete an identity provider](identity-providers.md#delete-an-identity-provider)

### [Organization Content](organization-content.md)

- [Upload organization layout content](organization-content.md#upload-organization-layout-content)
- [Replace organization layout content](organization-content.md#replace-organization-layout-content)
- [Get a single organization layout asset](organization-content.md#get-a-single-organization-layout-asset)
- [Delete organization layout content](organization-content.md#delete-organization-layout-content)
- [List organization layout assets by file type](organization-content.md#list-organization-layout-assets-by-file-type)

### [Providers](providers.md)

- [Create a provider](providers.md#create-a-provider)
- [Update a provider](providers.md#update-a-provider)
- [Get providers](providers.md#get-providers)
- [Delete a provider](providers.md#delete-a-provider)

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

### [Subscription Policies](subscription-policies.md)

- [Create subscription policies](subscription-policies.md#create-subscription-policies)
- [Upsert subscription policies](subscription-policies.md#upsert-subscription-policies)
- [Get a subscription policy](subscription-policies.md#get-a-subscription-policy)
- [Delete a subscription policy](subscription-policies.md#delete-a-subscription-policy)

### [Labels](labels.md)

- [Create labels](labels.md#create-labels)
- [Upsert labels](labels.md#upsert-labels)
- [List labels](labels.md#list-labels)
- [Delete labels](labels.md#delete-labels)

### [Applications](applications.md)

- [Create an application](applications.md#create-an-application)
- [List applications](applications.md#list-applications)
- [Update an application](applications.md#update-an-application)
- [Get application details](applications.md#get-application-details)
- [Delete an application](applications.md#delete-an-application)
- [Import an application](applications.md#import-an-application)
- [Create an application for the authenticated user's organization](applications.md#create-an-application-for-the-authenticated-users-organization)
- [Update an application for the authenticated user](applications.md#update-an-application-for-the-authenticated-user)
- [Delete an application for the authenticated user](applications.md#delete-an-application-for-the-authenticated-user)
- [Reset application throttle policy](applications.md#reset-application-throttle-policy)

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

### [App Key Mapping](app-key-mapping.md)

- [Create application key mapping](app-key-mapping.md#create-application-key-mapping)
- [Get application key mappings](app-key-mapping.md#get-application-key-mappings)

### [Views](views.md)

- [Create a view](views.md#create-a-view)
- [List views](views.md#list-views)
- [Update a view](views.md#update-a-view)
- [Get a view](views.md#get-a-view)
- [Delete a view](views.md#delete-a-view)

### [Application Keys](application-keys.md)

- [Generate OAuth keys for a control-plane application](application-keys.md#generate-oauth-keys-for-a-control-plane-application)
- [Generate an OAuth access token](application-keys.md#generate-an-oauth-access-token)
- [Revoke OAuth keys](application-keys.md#revoke-oauth-keys)
- [Update OAuth keys](application-keys.md#update-oauth-keys)
- [Clean up OAuth key artifacts](application-keys.md#clean-up-oauth-key-artifacts)

### [Billing](billing.md)

- [Get billing usage data](billing.md#get-billing-usage-data)
- [List payment methods](billing.md#list-payment-methods)
- [Add billing engine keys](billing.md#add-billing-engine-keys)
- [Update billing engine keys](billing.md#update-billing-engine-keys)
- [Delete billing engine keys](billing.md#delete-billing-engine-keys)
- [Get billing engine keys](billing.md#get-billing-engine-keys)
- [Get billing profile information](billing.md#get-billing-profile-information)
- [List subscriptions for billing](billing.md#list-subscriptions-for-billing)
- [Create a checkout session](billing.md#create-a-checkout-session)
- [Register a Stripe checkout session](billing.md#register-a-stripe-checkout-session)
- [Cancel a paid subscription](billing.md#cancel-a-paid-subscription)
- [Get subscription billing status](billing.md#get-subscription-billing-status)
- [Create an organization billing portal session](billing.md#create-an-organization-billing-portal-session)
- [Create a subscription billing portal session](billing.md#create-a-subscription-billing-portal-session)

### [Usage](usage.md)

- [Get subscription usage](usage.md#get-subscription-usage)

### [Invoices](invoices.md)

- [List invoices](invoices.md#list-invoices)
- [Get an invoice](invoices.md#get-an-invoice)
- [List invoices by subscription](invoices.md#list-invoices-by-subscription)
- [Get invoice PDF link](invoices.md#get-invoice-pdf-link)
- [redirectHostedInvoice](invoices.md#redirecthostedinvoice)

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
- [List available key managers for developers](key-managers.md#list-available-key-managers-for-developers)
- [Get a key manager](key-managers.md#get-a-key-manager)
- [Update a key manager](key-managers.md#update-a-key-manager)
- [Delete a key manager](key-managers.md#delete-a-key-manager)

### [Webhook Events](webhook-events.md)

- [List webhook events](webhook-events.md#list-webhook-events)
- [Get a webhook event](webhook-events.md#get-a-webhook-event)
- [Retry a failed webhook delivery](webhook-events.md#retry-a-failed-webhook-delivery)

### [Schemas](schemas.md)

