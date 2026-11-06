# API Content and Docs

Beyond the API definition file (OpenAPI, AsyncAPI, etc.), you can enrich each API's portal page with a landing page, images, and documentation sections. This content is what developers see when they open an API in the catalog.

## Content ZIP Structure

All API content is uploaded as a single ZIP file, laid out like this:

```
my-api-content.zip
├── web/                    # landing page and images (optional)
│   ├── api-content.hbs     # landing page — Handlebars template
│   │   OR
│   ├── apiContent.md       # landing page — Markdown
│   ├── api-icon.png        # API icon shown in the catalog card
│   └── <other-images>      # images referenced from the landing page
└── docs/                   # documentation pages (optional)
    ├── overview.md
    ├── getting-started.md
    ├── faq.md
    └── HowTo/              # one category folder, no deeper
        └── guide.md
```

Each path in the ZIP gets one outcome, from the first rule that matches:

| Path | Outcome |
|---|---|
| Any path, when the ZIP breaks an extraction limit or holds an unsafe path | The whole upload is rejected |
| The only top-level entry is one folder other than `web/` or `docs/` (for example `my-api/web/…`) | The ZIP is read from inside that folder. A lone `web/` or `docs/` is never a wrapper |
| A part of the path starts with `.` (`.gitkeep`, `.git/`, `._A.md`, `.DS_Store`), or it is under `__MACOSX/` | Ignored |
| The same path appears more than once | Rejected |
| `web/apiContent.md` or `web/api-content.hbs` | The landing page. No other name is read |
| `web/<name>` ending in `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg` or `.ico` (any letter case) | An image |
| `docs/<name>.md` | A document of category `Other` |
| `docs/<Category>/<name>.md` | A document of that category |
| Anything else, such as a misspelled `doc/` (even when empty), `web/style.css`, `docs/guide.pdf` or `docs/HowTo/v1/a.md` | Rejected |

The `.md` extension is lowercase. If any path is rejected, the whole upload is rejected with `400` and nothing is stored. The response lists every rejected path with its reason:

```json
{
  "status": "error",
  "code": "COMMON_VALIDATION_ERROR",
  "message": "Bad Request",
  "errors": [
    { "message": "doc/: unknown top-level entry" },
    { "message": "docs/guide.pdf: documents must be Markdown (.md)" },
    { "message": "web/style.css: web/ holds only apiContent.md, api-content.hbs and images" }
  ]
}
```

The same rules apply to the `web/` and `docs/` content of a full API ZIP (the `artifact` field of `POST /apis` and `PUT /apis/{apiId}`). Its top level also holds the metadata file (`metadata.yaml`, `metadata.yml`, `api.yaml` or `mcp.yaml`) and the definition (`definition.yaml`, `definition.yml` or `definition.json`), one of each.

Every image in `web/` fills the slot named by its file name without the extension: `api-icon.png` (or `.jpg`) is the thumbnail, and `banner.png` is `apiImageMetadata.banner` in a landing-page template.

## Add, Replace or Delete API Content

| Method | What it does | Success |
|---|---|---|
| `PUT /apis/{apiId}/assets` | Replaces **all** content with the ZIP: documents, links, landing page and images. Anything not in the ZIP is removed. An empty ZIP with no `docMetadata` links removes all content. The API definition is never changed. | `201` if the API had no content and the upload adds some, otherwise `200` (including an empty upload) |
| `POST /apis/{apiId}/assets` | Adds new content only. Nothing existing is changed or removed. If any file, image slot or link already exists, the whole request is rejected with `409` listing every clash, and nothing is stored. An upload with no content, an empty ZIP and no `docMetadata` links, is rejected with `400`. | `201` |
| `DELETE /apis/{apiId}/assets?type=<type>[&fileName=<name>]` | Deletes one file, or all content of exactly that type. `type` must be `MARKETING`, `IMAGE` or `DOC_<Category>`; anything else is rejected with `400`. | `204`, or `404` if nothing matched |

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
| `imageMetadata` | Optional JSON object mapping an image slot to a file name within `web/`, overriding the slot taken from the file name. The `api-icon` slot sets the catalog card icon |
| `docMetadata` | Optional JSON array for external documentation links (see [External Doc Links](#external-doc-links)) |

Text in `imageMetadata` or `docMetadata` that isn't valid JSON is rejected with `400`.

An image takes the slot named by its file name without the extension (`logo.png` takes `logo`). The upload is rejected with `400` when:

- `imageMetadata` names a file that isn't in the upload.
- `imageMetadata` names the same file for more than one slot. Each image fills one slot.
- Two images would take the same slot, such as `logo.png` and `logo.jpg`. Name one of them in `imageMetadata` to choose which one gets the slot.

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
<!-- web/apiContent.md -->
## Overview

The Order API lets you create, retrieve, update, and cancel customer orders.

## Use Cases

- **E-commerce checkout** — create an order after payment is confirmed
- **Order tracking** — retrieve order status for customer-facing apps
```

## Documentation Pages (`docs/`)

Files placed in `docs/` appear as pages in the API's **Documentation** tab. Each Markdown (`.md`) file directly in `docs/` or in one category folder below it becomes a documentation page, following the [rules above](#content-zip-structure). The file name is used as the page title.

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

To link to externally hosted documentation (rather than uploaded files), pass `docMetadata` as a JSON string. Group links by document type; each `type` must be `DOC_<Category>` (for example `DOC_LINK`). A link is stored as `LINK_<displayName>` under that type:

```bash
curl -k -X POST \
  "https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets" \
  -H "Authorization: Bearer $TOKEN" \
  -F "content=@my-api-content.zip" \
  -F 'docMetadata=[{"type":"DOC_LINK","links":[{"displayName":"External Guide","url":"https://docs.example.com/guide"}]}]'
```

## Get API Content

Retrieving content requires both a `type` and a `fileName` query parameter to select a single stored file. `type` is matched exactly: `MARKETING` (landing-page files), `IMAGE`, `DOC_<Category>` (documents and links, for example `DOC_Other` for files directly in `docs/`), `API_DEFINITION` or `SCHEMA_DEFINITION`:

```bash
curl -k "https://localhost:9543/api-portal/api/v0.9/apis/{apiId}/assets?type=DOC_Other&fileName=getting-started.md" \
  -H "Authorization: Bearer $TOKEN"
```

## Related

- [Publishing APIs](publishing-apis.md) — register the API entry before uploading content
