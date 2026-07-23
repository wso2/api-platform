# Design Mode

Design mode lets you develop and preview API page layouts and org-level themes without a running PostgreSQL database or an identity provider. The portal starts with sample data loaded from disk and serves all pages anonymously — login, API key generation, and subscription management are disabled.

## When to Use Design Mode

Use design mode when you want to:

- Build or customise the portal layout, CSS, header, and footer
- Iterate on Handlebars templates for API overview pages
- Work on a new API page design without needing a full portal stack running

## Enable Design Mode

In `configs/config.toml`, add or update the `[design_mode]` section:

```toml
[design_mode]
enabled = true
api_samples_path = "./samples/apis/"          # directory of sample API definitions
mcp_samples_path = "./samples/mcps/"          # directory of sample MCP server definitions
subscription_plans_path = "./samples/subscriptionPlans.yaml"
applications_path = "./samples/applications.yaml"
path_to_layout = "./src/defaultContent/"      # Handlebars templates and static assets
```

Then start the portal normally:

```bash
npm start
```

Visit **http://localhost:9543/views/default**.

> The portal always starts on plain HTTP in design mode — no TLS certificate setup required.

## Configuration Options

| Option (TOML) | Default | Description |
|--------|---------|-------------|
| `enabled` | `false` | Set to `true` to activate design mode |
| `api_samples_path` | `./samples/apis/` | Directory of sample REST/GraphQL/SOAP/WS API definitions |
| `mcp_samples_path` | `./samples/mcps/` | Directory of sample MCP server definitions |
| `subscription_plans_path` | `./samples/subscriptionPlans.yaml` | YAML file with org-level subscription plan details |
| `applications_path` | `./samples/applications.yaml` | YAML file with sample applications shown on the Applications page |
| `path_to_layout` | `./src/defaultContent/` | Layout directory used to render pages (see [Customising the Theme](#customising-the-theme)) |

All options can also be set via environment variables:

```
APIP_DP_DESIGNMODE_ENABLED=true
APIP_DP_DESIGNMODE_APISAMPLESPATH=./my-samples/apis/
APIP_DP_DESIGNMODE_MCPSAMPLESPATH=./my-samples/mcps/
APIP_DP_DESIGNMODE_SUBSCRIPTIONPLANSPATH=./my-samples/plans.yaml
APIP_DP_DESIGNMODE_APPLICATIONSPATH=./my-samples/applications.yaml
APIP_DP_DESIGNMODE_PATHTOLAYOUT=./my-layout/
```

## Customising the Theme

`path_to_layout` is your working theme directory. It mirrors the structure used by the production theme ZIP upload, so a theme built here can be deployed to production without any conversion.

### What can be customised

| Asset | Path in `path_to_layout` | Effect |
|-------|------------------------|--------|
| Page shell | `layout/main.hbs` | Controls the outer HTML, `<head>`, nav frame |
| Header | `partials/header.hbs` | Top navigation bar |
| Footer | `partials/footer.hbs` | Footer content |
| Home page content | `pages/home/partials/home.hbs` | Home page body section |
| API landing body | `pages/api-landing/partials/api-content.hbs` | Default body for API overview pages |
| API listing extras | `pages/apis/partials/apis-md.hbs` | Markdown injected above the API list |
| API landing extras | `pages/api-landing/partials/api-landing-md.hbs` | Markdown injected on API overview pages |
| Stylesheet | `styles/main.css` | Portal-wide CSS (can `@import` other CSS files) |
| Images | `images/` | Custom logo and other images |

You only need to include the files you want to override — everything else falls back to the defaults in `src/defaultContent/` automatically.

### Getting started

Copy only the files you want to change from `src/defaultContent/`:

```bash
mkdir -p my-theme/partials my-theme/styles my-theme/images
cp src/defaultContent/partials/header.hbs my-theme/partials/
cp src/defaultContent/styles/main.css my-theme/styles/
```

Then point `path_to_layout` at your directory:

```toml
[design_mode]
enabled = true
path_to_layout = "./my-theme/"
```

Reload the browser after editing any file — changes appear immediately without a server restart.

> **Note on `main.css`:** This file must be copied in full, not replaced with just `:root` overrides. It contains both the CSS variables and the actual rule definitions (buttons, layout, containers). The other CSS files (`home.css`, `header.css`, etc.) referenced in its `@import` statements are served from the defaults automatically and do not need to be copied.

### Colour-only theme example

`samples/layouts/green-theme/` shows the minimal change needed: a full copy of `main.css` with only the `:root` colour palette swapped. Use it as a starting point:

```bash
cp -r samples/layouts/green-theme/ my-theme/
```

> **Note:** JavaScript files cannot be added as theme assets. Portal behaviour is controlled by the portal's own scripts in `src/scripts/`. Only CSS, Handlebars templates, and images are customisable.

### Per-API custom layout

To customise the layout of a specific API's overview page, add an `api-content.hbs` file inside that API's `web/` directory under `api_samples_path`:

```
samples/apis/my-api-v1.0/
├── api.yaml
├── definition.yml
├── docs/
└── web/
    └── api-content.hbs   ← custom HBS for this API's overview body
```

### Deploying to production

When the theme is ready, zip the `path_to_layout` directory and upload it to the portal. The ZIP structure is identical to what design mode uses, so no conversion is needed.

**Build the ZIP:**

```bash
# from inside your path_to_layout directory
zip -r my-theme.zip layout/ partials/ styles/ images/
```

**Option 1 — Admin UI:** Click **Settings** in the sidebar, open the **Views** tab, then in the **Upload View Content** card select the target view, choose the ZIP file, and click **Upload**.

**Option 2 — curl:** Replace `{viewName}` with the view name (e.g. `default`). The examples use a `$TOKEN` variable — get one first:
```bash
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=<admin-username>&password=<admin-password>" | jq -r .token)
```

Initial upload:

```bash
curl -X POST "http://localhost:9543/api/v0.9/views/{viewName}/layout" \
  -H "Authorization: Bearer $TOKEN" \
  -F "zipFile=@my-theme.zip"
```

Update an existing layout:

```bash
curl -X PUT "http://localhost:9543/api/v0.9/views/{viewName}/layout" \
  -H "Authorization: Bearer $TOKEN" \
  -F "zipFile=@my-theme.zip"
```

Revert to the default layout:

```bash
curl -X DELETE "http://localhost:9543/api/v0.9/views/{viewName}/layout/template" \
  -H "Authorization: Bearer $TOKEN"
```

## Sample APIs and MCP Servers

APIs and MCP servers live in separate directories. The directory name becomes the handle used in the URL.

```
samples/
├── apis/                          # REST, GraphQL, SOAP, WS APIs  →  /views/default/apis
│   ├── reading-list-api-v1.0/
│   │   ├── api.yaml
│   │   ├── definition.yml
│   │   └── docs/
│   └── …
└── mcps/                          # MCP servers  →  /views/default/mcps
    ├── travel-assistant-mcp-v1/
    │   ├── api.yaml
    │   ├── schemaDefinition.yaml
    │   └── docs/
    └── …
```

### `api.yaml` Format (REST API)

```yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha2
kind: RestApi

metadata:
  name: my-api-v1.0          # used as the URL handle: /views/default/api/my-api-v1.0

spec:
  type: REST                  # REST | WS | GRAPHQL | SOAP | WEBSUB
  displayName: My API
  version: v1.0
  description: A short description shown on the API card.
  status: PUBLISHED

  tags:
    - payments

  subscriptionPlans:       # leave empty [] if no subscription plans
    - Gold
    - Silver

  endpoints:
    sandboxUrl: http://localhost:8080/my-api
    productionUrl: https://api.example.com/my-api
```

### `api.yaml` Format (MCP Server)

```yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha2
kind: MCP

metadata:
  name: my-mcp-v1.0

spec:
  type: MCP
  displayName: My MCP Server
  version: 1.0.0
  description: MCP server exposing tools for AI agents.
  status: PUBLISHED

  tags:
    - mcp

  subscriptionPlans:
    - Gold

  endpoints:
    productionUrl: https://mcp.example.com
```

The `schemaDefinition.yaml` alongside `api.yaml` defines the tools, resources, and prompts exposed by the server:

```yaml
- type: TOOL
  name: search_flights
  description: Search for available flights between two cities.
  inputSchema:
    type: object
    properties:
      origin: { type: string }
      destination: { type: string }
```

See the [built-in samples](../../samples/apis/) for complete examples.

### Live Reload

The portal re-reads API definitions from disk on every page request. Edit `api.yaml` or any doc file, then reload the browser — no server restart needed.

## Sample Applications

The Applications page is available in design mode and shows entries from `applications_path`. The format follows the same Kubernetes-style manifest used across all sample files:

```yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha2
kind: ApplicationList
items:

  - metadata:
      name: my-app

    spec:
      displayName: My App
      description: A short description shown on the application card.
```

`metadata.name` becomes the application ID. Creating new applications, viewing individual application details, and managing keys are not available in design mode.

See `samples/applications.yaml` for a ready-made example.

## What Is Disabled in Design Mode

| Feature | Status |
|---------|--------|
| Login / IDP authentication | Disabled — all pages are served anonymously |
| API subscriptions | Disabled |
| Creating / deleting applications | Disabled — Applications page is read-only |
| Individual application details | Disabled |
| API key generation | Disabled |
| Database | Not required — no connection is attempted |
| TLS / HTTPS | Not used — server always starts on plain HTTP |

## Turning Design Mode Off

Set `enabled = false` (or remove the section entirely) and restart:

```toml
[design_mode]
enabled = false
```

The portal returns to production mode, requiring a database and (if configured) an IDP.
