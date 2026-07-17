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

### 2. Create configuration files

Copy both sample configuration files:

```bash
mkdir -p configs
cp configs/config.toml.example configs/config.toml
cp configs/config-platform-api-template.toml configs/config-platform-api.toml
```

`config.toml` controls the Developer Portal itself. `config-platform-api.toml` configures the Platform API sidecar that validates login credentials and issues signed tokens. The default credentials in the example file are `admin` / `admin`.

### 3. Start the portal

```bash
docker compose up
```

This starts the Developer Portal (SQLite by default). On first boot the database schema and a default organization (`default`) with a `default` view are created automatically.

### 4. Open the portal

Navigate to:

```
https://localhost:3000/default/views/default
```

Sign in with `admin` / `admin` (the credentials defined in `configs/config-platform-api.toml`).

You should see the default API catalog page, empty until you publish an API (next step) or run `./scripts/seed-samples.sh` to deploy a set of ready-made sample APIs/MCPs.

### 5. Publish your first API

Create an API manifest file and an OpenAPI definition, then upload them:

```yaml
# api.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: RestApi

metadata:
  name: ping-api-v1.0

spec:
  type: REST
  displayName: Ping API
  version: v1.0
  description: Sample HTTP echo/probe API. Requires API key authentication. No subscription plans.
  status: PUBLISHED
  referenceID: ping-api-v1.0

  tags:
    - ping
    - api-key

  labels:
    - default

  subscriptionPlans: []

  visibility: PUBLIC
  visibleGroups: []

  businessInformation:
    businessOwner: Platform Owner
    businessOwnerEmail: support@example.com
    technicalOwner: API Team
    technicalOwnerEmail: architecture@example.com

  endpoints:
    sandboxUrl: http://localhost:8080/ping
    productionUrl: http://localhost:8080/ping
```

```yaml
# openapi.yaml
openapi: 3.0.1
info:
  title: Ping API
  version: 1.0.0
  description: |
    HTTP echo/probe API secured with an API key (`X-API-Key` header).
    Use this API to inspect requests, test connectivity, and probe status codes.
    No subscription plans are required — just an API key.
servers:
  - url: /ping
security:
  - ApiKeyHeader: []
components:
  securitySchemes:
    ApiKeyHeader:
      type: apiKey
      in: header
      name: X-API-Key
  schemas:
    PingResponse:
      type: object
      description: Response returned by the ping/echo service
      additionalProperties: true

paths:
  /get:
    get:
      summary: Echo a GET request
      description: Returns the query parameters and headers sent with the request.
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PingResponse'

  /post:
    post:
      summary: Echo a POST request
      description: Echoes the posted JSON body back in the response.
      requestBody:
        required: false
        content:
          application/json:
            schema:
              type: object
              additionalProperties: true
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PingResponse'

  /status/{code}:
    get:
      summary: Return a specific HTTP status code
      description: Returns the given HTTP status code — useful for testing error handling.
      parameters:
        - name: code
          in: path
          required: true
          schema:
            type: integer
            format: int32
      responses:
        '200':
          description: Proxy response (actual status depends on the `code` path parameter)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PingResponse'

```

Scripts and CLI tools authenticate with a Bearer token obtained directly from the Platform API. Get one once, then reuse it until it expires:

```bash
# Get a token from the Platform API (runs alongside the devportal).
# Login uses the file-based credentials from the Platform API config.
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=admin&password=admin" | jq -r .token)

# Publish the API (the token's org_handle claim scopes this to the "default" org)
curl -sk -X POST "https://localhost:3000/api/v0.9/apis" \
  -H "Authorization: Bearer $TOKEN" \
  -F "api=@api.yaml;type=application/yaml" \
  -F "apiDefinition=@openapi.yaml;type=application/yaml"
```

Refresh the portal — the Ping API now appears in the catalog. Click it to view the documentation and try-out console.

## What was just created?

| Resource | Value |
|---|---|
| Organization | `default` |
| Default view | `default` |
| Portal URL | `https://localhost:3000/default/views/default` |
| Admin credentials | `admin` / `admin` (Platform API — see `configs/config-platform-api.toml`) |
| Sample API | `Ping API` visible in the catalog |

## Next steps

| Goal | Where to go |
|---|---|
| Customize org name and IdP | [Manage Organizations](../administer/manage-organizations.md) |
| Add a filtered view for a different audience | [Manage Views](../administer/manage-views.md) |
| Register and publish your first API | [Publish APIs](../publish-apis/publishing-apis.md) |
| Set up subscription plans | [Subscription Plans](../administer/subscription-plans.md) |
| Customize the portal look and feel | [Theming](../administer/theming/org-level-theming.md) |
| Notify your gateway of key/subscription changes | [Webhook Integration](../administer/webhook-integration.md) |
