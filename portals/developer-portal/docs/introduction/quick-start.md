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
mkdir -p configs && cp sample.config.yaml configs/config.yaml
```

### 3. Start the portal

```bash
docker compose up
```

Docker Compose starts the Developer Portal (SQLite by default). On first boot the database schema and a default organization (`default`) with a `default` view are created automatically.

### 4. Open the portal

Navigate to:

```
https://localhost:3000/default/views/default
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
# Get the default org UUID
ORG_ID=$(curl -sk -u admin:admin https://localhost:3000/organizations | jq -r '.[0].orgID')

# Create the API
curl -sk -X POST "https://localhost:3000/o/$ORG_ID/devportal/v1/apis" \
  -u admin:admin \
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
