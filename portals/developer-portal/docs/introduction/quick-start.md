# Quick Start

Get the Developer Portal running locally in a few minutes using Docker Compose.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/) installed
- Port 3000 available

## Steps

### 1. Clone the repository

```bash
git clone https://github.com/wso2/api-platform.git
cd api-platform/portals/developer-portal/
```

### 2. Create a configuration file

Copy the sample configuration and set the minimum required values:

```bash
cp sample_config.yaml config.yaml
```

Open `config.yaml` and set:

```yaml
baseUrl: "http://localhost:3000"

db:
  host: "localhost"       # or "postgres" when using Docker Compose
  port: 5432
  database: "devportal"
  username: "postgres"
  password: "yourpassword"

advanced:
  http: true              # use plain HTTP for local dev

defaultAuth:
  users:
    - username: "admin"
      password: "admin"
      roles:
        - "admin"
      orgClaimName: "ACME"
      organizationIdentifier: "ACME"
```

> **Note:** The `defaultAuth` block enables built-in local users for exploration. Remove it before deploying to production and configure a real [identity provider](../administer/manage-organizations.md#identity-provider-configuration).

### 3. Start the portal

```bash
docker compose up
```

Docker Compose starts PostgreSQL and the Developer Portal. On first boot the database schema and a default organization (`ACME`) with a `default` view are created automatically.

### 4. Open the portal

Navigate to:

```
http://localhost:3000/ACME/views/default
```

Sign in with `admin` / `admin` (or the credentials you set in `defaultAuth`).

You should see the default API catalog page.

### 5. Publish your first API

Create an API manifest file and an OpenAPI definition, then upload them:

```yaml
# api.yaml
apiVersion: devportal.wso2.com/v1
kind: RestApi

metadata:
  name: ping-api-v1

spec:
  type: REST
  displayName: Ping API
  version: v1.0
  description: A simple health-check API.
  status: PUBLISHED
  labels:
    - default
  visibility: PUBLIC
  endpoints:
    productionUrl: https://httpbin.org/get
```

```yaml
# openapi.yaml
openapi: "3.0.0"
info:
  title: Ping API
  version: v1.0
paths:
  /ping:
    get:
      summary: Health check
      responses:
        "200":
          description: OK
```

```bash
curl -X POST "http://localhost:3000/organizations/ACME/apis" \
  -u admin:admin \
  -F "api=@api.yaml" \
  -F "apiDefinition=@openapi.yaml;type=application/yaml"
```

Refresh the portal — the Ping API now appears in the catalog. Click it to view the documentation and try-out console.

> **Tip:** For `orgId` you can use the org handle (`ACME`) or the UUID returned when the organization was created.

## What was just created?

| Resource | Value |
|---|---|
| Organization | `ACME` |
| Default view | `default` |
| Portal URL | `http://localhost:3000/ACME/views/default` |
| Admin credentials | `admin` / `admin` (local auth) |
| Sample API | `Ping API` visible in the catalog |

## Next steps

| Goal | Where to go |
|---|---|
| Customize org name and IdP | [Manage Organizations](../administer/manage-organizations.md) |
| Add a filtered view for a different audience | [Manage Views](../administer/manage-views.md) |
| Register and publish your first API | [Publish APIs](../publish-apis/publishing-apis.md) |
| Set up subscription plans | [Subscription Plans](../administer/subscription-plans.md) |
| Customize the portal look and feel | [Theming](../administer/theming/org-level-theming.md) |
| Connect to the API Gateway | [Gateway Integration](../administer/gateway-integration.md) |
