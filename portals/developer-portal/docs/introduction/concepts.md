# Core Concepts

This page explains the key building blocks of the Developer Portal and how they relate to each other.

## Organization

An **organization** is the top-level multi-tenant unit. Each organization gets its own branded space in the portal, and its APIs, applications, subscriptions, and users are isolated from other organizations.

The organization's handle appears in every portal URL:

```
https://<host>/<orgHandle>/views/<viewName>
```

Users are automatically routed to their organization when they sign in, based on a claim in their identity provider (IdP) token.

## View

A **view** is a filtered, branded subset of an organization's APIs. An organization can have multiple views — for example, one for internal developers and one for external partners — each showing only the APIs tagged with the relevant labels.

Each view has its own URL:

```
https://<host>/<orgHandle>/views/<viewName>
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

A **subscription plan** is a named usage tier that controls how much of an API a developer can consume. Plans are attached to APIs during publishing, and developers choose a plan when subscribing.

Plans can define rate limits — a request count (or event count, for async APIs) per time window (minute, hour, day, or month).

> **Note:** When `generateDefaultSubPlans: true` is set in the config (the default), five standard plans (`Bronze`, `Silver`, `Gold`, `Unlimited`, `AsyncUnlimited`) are automatically created for every new organization.

## Application

An **application** is a logical container — representing a mobile app, web app, device, or script — that a developer creates in the portal. For OAuth2-secured APIs, an application holds the client ID(s) that link to OAuth applications created directly in a key manager; the portal never generates or stores consumer key/secret pairs.

A developer can have multiple applications, each with independent OAuth2 client IDs. For example, a developer might have a `MyApp-Production` application and a `MyApp-Staging` application linked to separate OAuth applications in the key manager.

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

Key lifecycle events (generate, regenerate, revoke) are published in real-time as [webhooks](../administer/webhook-integration.md) to your configured subscriber endpoint(s), so whatever system you have listening — typically a handler in front of your API Gateway — can enforce access immediately.

## OAuth2 Credentials

For APIs that use OAuth2, developers create an OAuth application directly in a [key manager](../administer/key-manager-integration.md), then link the resulting **client ID** to an application in the portal. The portal never sees or stores the client secret — it's supplied by the developer each time they generate an access token, and the portal proxies that token request to the key manager.

## Key Manager

A **key manager** is an external OAuth2 authorization server configured for an organization. Developers create and own their OAuth applications there; the portal only stores a reference to the client ID and proxies `client_credentials` token requests to the key manager's token endpoint. You can configure one or more key managers per organization.

## API Workflow

An **API workflow** is a published, multi-step sequence of API calls defined in [Arazzo format](https://spec.openapis.org/arazzo/latest.html). Workflows are authored by admins and published to the portal for both human developers and AI agents to discover and follow.

Workflows appear in the **API Workflows** section of the portal and are also exposed via machine-readable endpoints (`llms.txt`, `api-workflows.md`) for AI agent consumption.

## Webhook Subscriber

A **webhook subscriber** is an HTTPS endpoint you register to receive real-time event notifications from the portal. When a developer generates an API key or changes a subscription, the portal fires a signed HTTP POST to all matching subscribers; what the subscriber does with that event — e.g. propagating the change to an API Gateway — is entirely up to whatever you run behind that endpoint.

See [Webhook Integration](../administer/webhook-integration.md) for configuration details.
