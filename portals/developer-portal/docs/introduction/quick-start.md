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
mkdir -p configs && cp sample_config.yaml configs/config.yaml
```

<!-- Open `config.yaml` and set:

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

> **Note:** The `defaultAuth` block enables built-in local users for exploration. Remove it before deploying to production and configure a real [identity provider](../administer/manage-organizations.md#identity-provider-configuration). -->

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
apiVersion: devportal.api-platform.wso2.com/v1
kind: RestApi

metadata:
  name: ping-api-v1.0

spec:
  type: REST
  displayName: Ping API
  version: v1.0
  description: Sample HTTP echo/probe API. Requires API key authentication. No subscription plans.
  provider: WSO2
  status: PUBLISHED
  gatewayType: wso2/api-platform
  referenceID: ping-api-v1.0

  tags:
    - ping
    - api-key

  labels:
    - default

  subscriptionPolicies: []

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

```bash
curl -sk -X POST "https://localhost:3000/devportal/organizations/1ba42a09-45c0-40f8-a1bf-e4aa7cde1575/apis" \           ✔
   -u admin:admin \
   -F "api=@api.yaml;type=application/yaml" \
   -F "apiDefinition=@openapi.yaml;type=application/yaml" -k
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
