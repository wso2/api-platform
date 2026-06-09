# API Project CLI Reference

This guide covers the API project commands currently implemented under `cli/src/cmd/apiproject`.

## Commands

### `ap apiproject init`

Initializes a new API project in the current working directory.

```shell
ap apiproject init --display-name <name> --type <rest> --version <version> --context <context> [--no-interactive]
```

Examples:

```shell
ap apiproject init --display-name foo-api --type rest --version 1.0 --context /foo
ap apiproject init
```

Behavior:

- If `--no-interactive` is not used, the command prompts for any missing values.
- The project directory is created under the current directory using the `display-name` value.
- The directory name cannot be empty and cannot contain path separators.
- API type validation accepts `rest` and `soap`, but project scaffolding is currently implemented only for `rest`.

## Generated Project Structure

Running `ap apiproject init --display-name FooAPI --type rest --version 1.0.0 --context /petstore` creates:

```text
FooAPI/
├── .api-platform/
│   └── config.yaml
├── api.yaml
├── gateway.yaml
├── definition.yaml
├── docs/
└── tests/
```

## Generated Files

### `.api-platform/config.yaml`

Created with default project file paths:

```yaml
version: 1.0.0

filePaths:
  deploymentArtifact: ./gateway.yaml
  apiMetadata: ./api.yaml
  apiDefinition: ./definition.yaml
  docs: ./docs
  tests: ./tests

governanceRulesets: []

autoSync:
  gatewayArtifactFromDefinition: true
```

### `api.yaml`

- Contains API metadata (an `Api` management-plane CR) used when publishing to the DevPortal.
- The resource name is derived from the display name and version.
- `spec.referenceID` links this metadata to the API deployed on the gateway. After you deploy `gateway.yaml`, set `referenceID` to the gateway's returned API ID (see the [end-to-end workflow](#end-to-end-workflow-deploy-to-the-gateway-and-publish-to-the-devportal)).

A populated example:

```yaml
apiVersion: management.api-platform.wso2.com/v1
kind: Api
metadata:
  name: "echo-api-v2.0"
spec:
  type: REST
  displayName: Echo API
  version: v2.0
  description: Sample HTTP echo/probe API. Requires API key authentication. No subscription plans.
  provider: WSO2
  referenceID: echo-api-v2.0

  tags:
    - ping
    - api-key

  labels:
    - default

  subscriptionPolicies:
    - Gold
    - Bronze

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

### `gateway.yaml`

- Contains a `RestApi` gateway deployment artifact (the CR that `ap gateway apply` deploys).
- A starter version is generated with `displayName`, `version`, `context`, a sample backend URL, and default `GET`, `POST`, `PUT`, and `DELETE` operations on `/*`. Edit it to describe your real upstream, policies, and operations.
- For more complete, ready-to-use `RestApi` samples, see [`gateway/examples/sample-echo-api.yaml`](../../../gateway/examples/sample-echo-api.yaml) and [`gateway/examples/petstore-api.yaml`](../../../gateway/examples/petstore-api.yaml).

A populated example:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: echo-api-v2.0
  annotations:
    gateway.api-platform.wso2.com/artifact-id: a1b2c3d4-e000-4000-8000-000000000000
    gateway.api-platform.wso2.com/project-id: a1b2c3d4-e000-4000-8000-111111111111
spec:
  displayName: Echo API
  version: v2.0
  context: /postman-echo
  upstream:
    main:
      url: https://postman-echo.com
  policies:
    - name: api-key-auth
      version: v1
      params:
        key: X-API-Key
        in: header
    - name: cors
      version: v1
      params:
        allowedOrigins:
          - https://localhost:3000
        allowedMethods:
          - GET
          - POST
          - OPTIONS
  subscriptionPlans:
    - Gold
    - Bronze
  operations:
    - method: GET
      path: /get
    - method: POST
      path: /post
    - method: GET
      path: /status/{code}
```

### `definition.yaml`

- Contains a starter OpenAPI 3.0.3 document.
- Includes `GET`, `POST`, `PUT`, `DELETE`, and `OPTIONS` operations on `/*`.

## Notes

- If the target project directory already exists, the command fails.
- Resource names are normalized to a safe lower-case form using the display name and version.
- SOAP project scaffolding is not yet implemented, even though `soap` is accepted during initial validation.

## End-to-end workflow: deploy to the gateway and publish to the DevPortal

An API project is the single source for two destinations: the **gateway** (where the API is deployed and served, from `gateway.yaml`) and the **DevPortal** (where it is published for consumers, from `api.yaml` + the built artifact). The typical flow ties them together through the gateway-assigned API ID:

### 1. Initialize the project

```shell
ap apiproject init --display-name echo-api --type rest --version v2.0 --context /postman-echo
cd echo-api
```

This scaffolds `api.yaml`, `gateway.yaml`, `definition.yaml`, the `docs/` and `tests/` directories, and `.api-platform/config.yaml`.

### 2. Describe the API in `gateway.yaml`

Edit `gateway.yaml` to point at your real upstream and to declare the policies, operations, and subscription plans the gateway should enforce. Use [`gateway/examples/sample-echo-api.yaml`](../../../gateway/examples/sample-echo-api.yaml) and [`gateway/examples/petstore-api.yaml`](../../../gateway/examples/petstore-api.yaml) as references.

### 3. Deploy the API to the gateway

```shell
ap gateway apply -f gateway.yaml
```

This creates (or updates) the `RestApi` on the active gateway. The response includes the gateway-assigned API **ID** — copy it. (You can re-read it any time with `ap gateway rest-api get --display-name "Echo API" --version v2.0`.) See [`ap gateway apply`](../gateway/README.md#ap-gateway-apply) for details.

### 4. Wire the gateway ID into `api.yaml`

Set `spec.referenceID` in `api.yaml` to the gateway API ID from step 3. This links the DevPortal API metadata to the API actually deployed on the gateway:

```yaml
spec:
  referenceID: <gateway-api-id-from-step-3>
```

### 5. Build the DevPortal artifact

```shell
ap devportal build
```

This bundles `api.yaml`, the API definition, docs, and content into `build/devportal.zip` (one zip per configured DevPortal). For options and behavior, see [`ap devportal build`](../devportal/README.md#ap-devportal-build).

### 6. Publish the API to the DevPortal

```shell
ap devportal rest-api publish -f build/devportal.zip --org <org-id>
```

This uploads the built artifact to the DevPortal organization. See [`ap devportal rest-api publish`](../devportal/README.md#ap-devportal-rest-api-publish).

> **Tip:** Before steps 3–6, make sure you have selected the target gateway and DevPortal (`ap gateway use`, `ap devportal use`). The CLI uses the active gateway/DevPortal of the active platform unless you pass `--gateway` / `--display-name` and `--platform`.

## Related Commands

- `ap gateway apply` — deploy `gateway.yaml` to a gateway (see the [Gateway CLI reference](../gateway/README.md))
- `ap devportal build` — build the DevPortal artifact (see the [DevPortal CLI reference](../devportal/README.md#ap-devportal-build))
- `ap devportal rest-api publish` — publish the artifact to a DevPortal organization
