# Design Mode

Design mode lets you develop and preview API page layouts without a running PostgreSQL database or an identity provider. The portal starts with sample API definitions loaded from disk and serves all pages anonymously — login, subscriptions, applications, and API keys are all disabled.

## When to Use Design Mode

Use design mode when you want to:

- Build or customise Handlebars templates for API overview pages
- Iterate on layout, CSS, and partials with a fast edit-and-reload workflow
- Work on a new API page design without needing a full portal stack running

## Enable Design Mode

In `configs/config.yaml`, add or update the `designMode` section:

```yaml
designMode:
  enabled: true
  samplesPath: "./samples/apis/"          # directory of sample API definitions
  pathToLayout: "./src/defaultContent/"   # Handlebars templates and static assets
```

Then start the portal normally:

```bash
npm start
```

Visit **http://localhost:3000/views/default**.

> The portal always starts on plain HTTP in design mode — no TLS certificate setup required.

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `false` | Set to `true` to activate design mode |
| `samplesPath` | `./samples/apis/` | Path to the directory of sample API definitions (see [Sample APIs](#sample-apis)) |
| `pathToLayout` | `./src/defaultContent/` | Path to the Handlebars template directory used to render pages |

All three options can also be set via environment variables:

```
DP_DESIGNMODE_ENABLED=true
DP_DESIGNMODE_SAMPLESPATH=./my-samples/
DP_DESIGNMODE_PATHTOLAYOUT=./my-layout/
```

## Sample APIs

Each subdirectory under `samplesPath` represents one API. The directory name becomes the API handle (used in the URL).

```
samples/apis/
├── ping-api-v1.0/
│   ├── api.yaml          # API metadata (display name, type, tags, endpoints, …)
│   ├── definition.yml    # OpenAPI 3.0 specification
│   └── docs/
│       └── overview.md   # Markdown content shown on the API landing page
├── booking-api-v1.0/
│   └── …
└── catalog-api-v1.0/
    └── …
```

### `api.yaml` Format

```yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: RestApi

metadata:
  name: my-api-v1.0          # used as the URL handle: /views/default/api/my-api-v1.0

spec:
  type: REST                  # REST | WS | GRAPHQL | SOAP | WEBSUB
  displayName: My API
  version: v1.0
  description: A short description shown on the API card.
  provider: WSO2
  status: PUBLISHED

  tags:
    - payments

  subscriptionPolicies:       # leave empty [] if no subscription plans
    - Gold
    - Silver

  endpoints:
    sandboxUrl: http://localhost:8080/my-api
    productionUrl: https://api.example.com/my-api
```

See the [built-in samples](../../../../samples/apis/) for complete examples including subscription plans and business information.

### Live Reload

The portal re-reads API definitions from disk on every page request. Edit `api.yaml` or `overview.md`, then reload the browser — no server restart needed.

## Customising Templates

Point `pathToLayout` at a directory that mirrors the structure of `src/defaultContent/`:

```
my-layout/
├── layout/
│   └── main.hbs          # outer page shell (header, sidebar, footer)
├── pages/
│   ├── home/
│   │   └── page.hbs
│   ├── apis/
│   │   └── page.hbs
│   ├── api-landing/
│   │   └── page.hbs
│   └── docs/
│       └── page.hbs
├── partials/
│   ├── header.hbs
│   └── sidebar.hbs
├── styles/
└── images/
```

Start by copying `src/defaultContent/` to your working directory:

```bash
cp -r src/defaultContent/ my-layout/
```

Then set `pathToLayout: "./my-layout/"` in `configs/config.yaml`.

## What Is Disabled in Design Mode

| Feature | Status |
|---------|--------|
| Login / IDP authentication | Disabled — all pages are served anonymously |
| API subscriptions | Disabled — subscribe buttons are hidden |
| Applications | Disabled — the Applications page is not registered |
| API key generation | Disabled — API Keys nav item is hidden |
| Database | Not required — no connection is attempted |
| TLS / HTTPS | Not used — server always starts on plain HTTP |

## Turning Design Mode Off

Set `enabled: false` (or remove the section entirely) and restart:

```yaml
designMode:
  enabled: false
```

The portal returns to production mode, requiring a database and (if configured) an IDP.
