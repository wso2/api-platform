# Core Concepts

This page explains the key building blocks of the Developer Portal and how they relate to each other.

## Organization

An **organization** is the top-level multi-tenant unit. Each organization gets its own branded space in the portal, and its APIs, applications, subscriptions, and users are isolated from other organizations.

The organization name appears in every portal URL:

```
https://<host>/<orgName>/views/<viewName>
```

Users are automatically routed to their organization when they sign in, based on a claim in their identity provider (IdP) token.

## View

A **view** is a filtered, branded subset of an organization's APIs. An organization can have multiple views — for example, one for internal developers and one for external partners — each showing only the APIs tagged with the relevant labels.

Each view has its own URL:

```
https://<host>/<orgName>/views/<viewName>
```

Views can have their own layout (HTML/CSS template) for independent branding.

## Layout

A **layout** is a custom Handlebars template that defines the structure and styling of a view's pages (home, API listing, API detail). Layouts let you give each view its own branding, navigation, and page structure beyond what the theme color settings allow.

## API

An **API** is an entry in the portal catalog. It represents an API that developers can discover and subscribe to. The portal supports:

| Type | Format |
|---|---|
| REST | OpenAPI (Swagger) YAML/JSON |
| Async | AsyncAPI YAML/JSON |
| GraphQL | GraphQL schema SDL |
| SOAP | WSDL/XML |

Each API can have its own landing page content, documentation sections, icon, and banner image. APIs are tagged with labels to control which views they appear in.

## Subscription Plan

A **subscription plan** (also called a subscription policy) is a named usage tier that controls how much of an API a developer can consume. Plans are attached to APIs during publishing, and developers choose a plan when subscribing.

Plans can define:
- Rate limits (requests per minute/hour)
- Quota (requests per day/month)

Example plans: `Free`, `Basic`, `Gold`, `Enterprise`.

> **Note:** When `generateDefaultSubPolicies: true` is set in the config (the default), four standard plans (`Bronze`, `Silver`, `Gold`, `Unlimited`) are automatically created for every new organization.

## Application

An **application** is a logical container — representing a mobile app, web app, device, or script — that a developer creates in the portal. Applications are used to generate OAuth2 consumer key/secret pairs for OAuth2-secured APIs.

A developer can have multiple applications with independent OAuth2 credentials. For example, a developer might have a `MyApp-Production` application and a `MyApp-Staging` application with separate credentials.

> **Note:** Applications are not required for API subscriptions or API key generation. Subscriptions are made directly to an API, and API keys are bound to an API — not to an application.

## Subscription

A **subscription** is a developer's access grant to a specific API under a chosen subscription plan. The plan determines the developer's rate limits and quota for that API.

Subscriptions are made directly to an API — no application is involved. Once subscribed, the developer can invoke the API under the terms of the chosen plan.


## API Key

An **API key** is a simple token bound to a specific API, used to authenticate requests to APIs that use API key-based authentication. API keys are generated per API — not per application or per subscription.

API keys can be:
- **Generated** — create a new key for an API
- **Regenerated** — rotate the key (invalidates the old key)
- **Revoked** — permanently invalidate the key

Key lifecycle events (generate, regenerate, revoke) are delivered in real-time to the API Gateway via [webhooks](../administer/gateway-integration.md) so the gateway can enforce access immediately.

## OAuth2 Credentials

For APIs that use OAuth2, developers generate a **consumer key** and **consumer secret** for their application. These credentials are used to obtain access tokens from the [key manager](../administer/key-manager-integration.md).

## Key Manager

A **key manager** is the OAuth2 authorization server configured for an organization. It issues consumer key/secret pairs and validates access tokens presented to the API Gateway. You can configure one or more key managers per organization.

## API Workflow

An **API workflow** is a published, multi-step sequence of API calls defined in [Arazzo format](https://spec.openapis.org/arazzo/latest.html). Workflows are authored by admins and published to the portal for both human developers and AI agents to discover and follow.

Workflows appear in the **API Workflows** section of the portal and are also exposed via machine-readable endpoints (`llms.txt`, `api-workflows.md`) for AI agent consumption.

## Webhook Subscriber

A **webhook subscriber** is an external system — typically the API Gateway — that receives real-time event notifications from the portal. When a developer generates an API key or changes a subscription, the portal fires a signed HTTP POST to all configured subscribers.

See [Gateway Integration](../administer/gateway-integration.md) for configuration details.
