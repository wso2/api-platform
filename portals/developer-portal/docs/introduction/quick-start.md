# Quick Start

Get the Developer Portal running locally in a few minutes using Docker Compose.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/) installed
- `openssl` on your `PATH` (used by the setup script to generate certs and secrets)
- Ports 9543 (Developer Portal) and 9243 (Platform API) available

## Steps

### 1. Clone the repository

```bash
git clone https://github.com/wso2/api-platform.git
cd api-platform/portals/developer-portal/
```

### 2. Run the setup script

```bash
./scripts/setup.sh
```

This one-time script provisions everything the containers need to start:

- a self-signed TLS certificate under `resources/certificates/`
- the Developer Portal's encryption/session secrets and the shared JWT signing key, written to `api-platform.env`
- `configs/config-platform-api.toml` — the config for the Platform API sidecar that validates login credentials and issues signed tokens

It also prompts you for an **admin username and password**. Press Enter at the password prompt to have a strong one generated for you — it is printed once at the end, so copy it before continuing. The credentials are stored bcrypt-hashed in `api-platform.env`.

`config.toml`, which controls the Developer Portal itself, is already present in `configs/` — no copying needed.

> The script is idempotent: re-running it only fills in what's missing and never overwrites an existing value. To rotate a secret, remove it from `api-platform.env` (or delete `resources/certificates/` for the TLS cert) and re-run.

### 3. Start the portal

```bash
docker compose up
```

This starts the Developer Portal (SQLite by default). On first boot the database schema and a default organization (`default`) with a `default` view are created automatically.

### 4. Open the portal

Navigate to:

```
https://localhost:9543/default/views/default
```

Sign in with the admin username and password you set when running `./scripts/setup.sh`.

You should see the default API catalog page. It stays empty until you add APIs — either seed the bundled samples (next step) or publish your own (the step after).

### 5. Seed sample APIs (optional)

The fastest way to see a populated catalog is to deploy the bundled sample APIs and MCP servers:

```bash
./scripts/seed-samples.sh
```

This deploys everything under `samples/` into the `default` organization through the public REST API (the portal has no built-in seeding logic). It prompts for the admin username and password you set in step 2 — or set `ADMIN_USERNAME` / `ADMIN_PASSWORD` to skip the prompt. Safe to re-run: samples that already exist (matched by name and version) are skipped.

> Requires `curl`, `jq`, and `zip` on your `PATH`. The portal must be running (step 3). Set `DEVPORTAL_URL` / `PLATFORM_API_URL` to override the defaults (`https://localhost:9543` / `https://localhost:9243`).

Refresh the catalog page and the sample APIs appear. To publish an API of your own instead, continue below.

### 6. Publish your first API

Create an API manifest file and an OpenAPI definition, then upload them:

```yaml
# api.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha2
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
# Use the admin credentials you set when running ./scripts/setup.sh.
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=<admin-username>&password=<admin-password>" | jq -r .token)

# Publish the API (the token's org_handle claim scopes this to the "default" org)
curl -sk -X POST "https://localhost:9543/api/v0.9/apis" \
  -H "Authorization: Bearer $TOKEN" \
  -F "metadata=@api.yaml;type=application/yaml" \
  -F "definition=@openapi.yaml;type=application/yaml"
```

Refresh the portal — the Ping API now appears in the catalog. Click it to view the documentation and try-out console.

## What was just created?

| Resource | Value |
|---|---|
| Organization | `default` |
| Default view | `default` |
| Portal URL | `https://localhost:9543/default/views/default` |
| Admin credentials | Set when you ran `./scripts/setup.sh` (stored bcrypt-hashed in `api-platform.env`) |
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
