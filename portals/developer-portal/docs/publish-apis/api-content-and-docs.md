# API Content and Docs

Beyond the API definition file (OpenAPI, AsyncAPI, etc.), you can enrich each API's portal page with a landing page, documentation sections, and images. This content is what developers see when they open an API in the catalog.

## Landing Page Content

The landing page is the first thing a developer sees when they open an API. It can be a Markdown document or a Handlebars template.

### Upload Landing Page Content

```bash
# Markdown
curl -X POST \
  "http://localhost:3000/organizations/{orgId}/apis/{apiId}/content" \
  -u admin:admin \
  -F "apiContent=@overview.md;type=text/markdown"
```

```bash
# Handlebars template
curl -X POST \
  "http://localhost:3000/organizations/{orgId}/apis/{apiId}/content" \
  -u admin:admin \
  -F "apiContent=@overview.hbs;type=text/x-handlebars-template"
```

### Update Landing Page Content

```bash
curl -X PUT \
  "http://localhost:3000/organizations/{orgId}/apis/{apiId}/content" \
  -u admin:admin \
  -F "apiContent=@updated-overview.md;type=text/markdown"
```

### Delete Landing Page Content

```bash
curl -X DELETE \
  "http://localhost:3000/organizations/{orgId}/apis/{apiId}/content" \
  -u admin:admin
```

### Markdown Example

A well-structured landing page helps developers quickly understand the API:

```markdown
## Overview

The Order API lets you create, retrieve, update, and cancel customer orders.

## Use Cases

- **E-commerce checkout** — create an order after payment is confirmed
- **Order tracking** — retrieve order status for customer-facing apps
- **Order management** — update or cancel orders from admin tools

## Quick Start

1. [Subscribe](../../consume-an-api/subscriptions.md) to the Order API
2. Generate an [API key](../../consume-an-api/consume-with-api-key.md) or [OAuth2 credentials](../../consume-an-api/consume-with-oauth2.md)
3. Create your first order:

```bash
curl -X POST https://api.example.com/orders/v1/orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"customerId": "cust_123", "items": [{"sku": "PROD-001", "quantity": 2}]}'
```

## Rate Limits

| Plan | Requests/hour |
|---|---|
| Bronze | 500 |
| Gold | 5000 |
| Unlimited | No limit |
```

### Handlebars Variables

If using a `.hbs` template, these variables are available:

| Variable | Description |
|---|---|
| `{{apiName}}` | API display name |
| `{{apiVersion}}` | API version string |
| `{{baseUrl}}` | Portal base URL |
| `{{apiDescription}}` | API description |
| `{{apiContext}}` | API base path |

---

## Documentation Sections

In addition to the landing page, you can upload additional documentation files (guides, tutorials, changelog, etc.) that appear as sections in the API's **Documentation** tab.

Documentation sections can be Markdown files uploaded alongside or after the API definition.

---

## API Images

Upload an icon and banner image to make the API visually distinct in the catalog.

### Upload an Icon

```bash
curl -X POST \
  "http://localhost:3000/organizations/{orgId}/apis/{apiId}/content" \
  -u admin:admin \
  -F "apiIcon=@icon.png;type=image/png"
```

Supported formats: `image/png`, `image/jpeg`, `image/svg+xml`.

Recommended icon size: 128×128 pixels.

## Get API Content

```bash
curl http://localhost:3000/organizations/{orgId}/apis/{apiId}/content \
  -u admin:admin
```

## Related

- [Publishing APIs](publishing-apis.md) — register the API entry first
- [API-Level Theming](../administer/theming/api-level-theming.md) — customize CSS for the API landing page
