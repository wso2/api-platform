# API Content and Docs

Beyond the API definition file (OpenAPI, AsyncAPI, etc.), you can enrich each API's portal page with a landing page, images, and documentation sections. This content is what developers see when they open an API in the catalog.

## Content ZIP Structure

All API content is uploaded as a single ZIP file. The ZIP must follow this directory structure:

```
my-api-content.zip
├── web/                    # landing page and images (optional)
│   ├── api-content.hbs     # landing page — Handlebars template
│   │   OR
│   ├── api-content.md      # landing page — Markdown
│   ├── api-icon.png        # API icon shown in the catalog card
│   └── <other-images>      # images referenced from the landing page
└── docs/                   # documentation pages (optional)
    ├── overview.md
    ├── getting-started.md
    ├── faq.md
    └── HowTo/              # subdirectories are supported
        └── guide.md
```

At least one of `web/` or `docs/` must be present in the ZIP.

## Upload API Content

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

```bash
curl -X POST \
  "http://localhost:3000/devportal/v1/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "apiContent=@my-api-content.zip" \
  -F 'imageMetadata={"api-icon":"api-icon.png"}'
```

To update existing content, use `PUT`:

```bash
curl -X PUT \
  "http://localhost:3000/devportal/v1/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "apiContent=@my-api-content-v2.zip" \
  -F 'imageMetadata={"api-icon":"api-icon.png"}'
```

| Form field | Description |
|---|---|
| `apiContent` | ZIP file containing `web/` and/or `docs/` directories |
| `imageMetadata` | JSON object mapping image tag names to filenames within `web/`. The `api-icon` tag sets the catalog card icon |
| `docMetadata` | JSON array for external documentation links (see [External Doc Links](#external-doc-links)) |

## Landing Page (`web/`)

The `web/` directory holds the API landing page and any images it references.

### Handlebars Template (`.hbs`)

A Handlebars template gives you full control over the landing page HTML. Images uploaded in the same `web/` folder are accessible via `apiMetadata`:

```hbs
<!-- web/api-content.hbs -->
<section class="api-overview-section">
  <div class="api-overview">
    <h1>Order API</h1>
    <p>Create and manage customer orders.</p>
    <img src="{{apiMetadata.apiImageMetadata.banner}}" alt="Banner" />
  </div>
</section>
```

Available Handlebars variables:

| Variable | Description |
|---|---|
| `{{apiMetadata.name}}` | API display name |
| `{{apiMetadata.version}}` | API version string |
| `{{apiMetadata.description}}` | API description |
| `{{apiMetadata.type}}` | API type (`REST`, `SOAP`, `MCP`, `WS`, `WEBSUB`, `GRAPHQL`) |
| `{{apiMetadata.apiImageMetadata.<tag>}}` | URL of an image uploaded with the given tag name |

### Markdown (`.md`)

For simpler landing pages, a Markdown file works without any templating:

```markdown
<!-- web/api-content.md -->
## Overview

The Order API lets you create, retrieve, update, and cancel customer orders.

## Use Cases

- **E-commerce checkout** — create an order after payment is confirmed
- **Order tracking** — retrieve order status for customer-facing apps
```

## Documentation Pages (`docs/`)

Files placed in `docs/` appear as pages in the API's **Documentation** tab. Any Markdown file inside `docs/` (including subdirectories) becomes a documentation page. The file name is used as the page title.

```
docs/
├── overview.md          → "Overview" page
├── getting-started.md   → "Getting Started" page
├── faq.md               → "FAQ" page
└── HowTo/
    └── guide.md         → "Guide" page under "HowTo"
```

Example doc page:

```markdown
<!-- docs/getting-started.md -->
# Getting Started

## Authentication

Include your API key in the `X-API-Key` header on every request.

## Example

```bash
curl https://api.example.com/orders \
  -H "X-API-Key: <your-key>"
```
```

## External Doc Links

To link to externally hosted documentation (rather than uploaded files), pass `docMetadata` as a JSON string:

```bash
curl -X POST \
  "http://localhost:3000/devportal/v1/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "apiContent=@my-api-content.zip" \
  -F 'docMetadata=[{"name":"External Guide","url":"https://docs.example.com/guide","type":"LINK"}]'
```

## Get API Content

```bash
curl http://localhost:3000/devportal/v1/apis/{apiId}/assets \
  -H "Authorization: Bearer $TOKEN"
```

## Related

- [Publishing APIs](publishing-apis.md) — register the API entry before uploading content
- [API-Level Theming](../administer/theming/api-level-theming.md) — customize CSS for the API landing page
