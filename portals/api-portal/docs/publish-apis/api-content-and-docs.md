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

`web/` and `docs/` can sit inside one wrapper folder (for example `my-api/web/…`), but a lone `web/` or `docs/` is never treated as a wrapper.

Every image in `web/` fills the slot named by its file name without the extension: `api-icon.png` (or `.jpg`) is the thumbnail, and `banner.png` is `apiImageMetadata.banner` in a landing-page template.

## Add or Replace API Content

| Method | What it does | Success |
|---|---|---|
| `PUT /apis/{apiId}/assets` | Replaces **all** content with the ZIP: documents, links, landing page and images. Anything not in the ZIP is removed. An empty ZIP with no `docMetadata` links removes all content. The API definition is never changed. | `201` if the API had no content and the upload adds some, otherwise `200` (including an empty upload) |
| `POST /apis/{apiId}/assets` | Adds new content only. Nothing existing is changed or removed. If any file, image slot or link already exists, the whole request is rejected with `409` listing every clash, and nothing is stored. An empty ZIP is rejected with `400`. | `201` |

MCP servers use the same operations under `/mcp-servers/{mcpServerId}/assets`.

To change a single existing file, delete it and add the new version, or replace all content with `PUT`.

The **Upload content** button in the settings wizard uses `PUT`, so it also replaces all of the API's content.

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=<admin-username>&password=<admin-password>" | jq -r .token)
> ```

```bash
curl -k -X POST \
  "https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "content=@my-api-content.zip" \
  -F 'imageMetadata={"api-icon":"api-icon.png"}'
```

To replace all existing content, use `PUT`:

```bash
curl -k -X PUT \
  "https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "content=@my-api-content-v2.zip" \
  -F 'imageMetadata={"api-icon":"api-icon.png"}'
```

| Form field | Description |
|---|---|
| `content` | ZIP file containing `web/` and/or `docs/` directories |
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
curl -k -X POST \
  "https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "content=@my-api-content.zip" \
  -F 'docMetadata=[{"name":"External Guide","url":"https://docs.example.com/guide","type":"LINK"}]'
```

## Get API Content

Retrieving content requires both a `type` (`web`, `document`, `image`, or `link`) and a `fileName` query parameter to select a single stored file:

```bash
curl -k "https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets?type=document&fileName=getting-started.md" \
  -H "Authorization: Bearer $TOKEN"
```

## Related

- [Publishing APIs](publishing-apis.md) — register the API entry before uploading content
- [API-Level Theming](../administer/theming/api-level-theming.md) — customize CSS for the API landing page
