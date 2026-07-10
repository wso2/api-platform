# Gateway CLI Reference

This guide covers the Gateway-related commands implemented under `cli/src/cmd/gateway`.

Available command groups:

- `ap gateway` — manage gateway connections (add, list, remove, use, current, health)
- `ap gateway apply` — create or update a resource from a file
- `ap gateway image` — build gateway images
- `ap gateway rest-api` — manage REST APIs on a gateway
- `ap gateway rest-api api-key` — manage API keys for a REST API
- `ap gateway mcp` — manage MCP proxies on a gateway
- `ap gateway subscription-plan` — manage subscription plans on a gateway
- `ap gateway subscription` — manage subscriptions on a gateway

> **Note:** Each command supports the `--help` flag for detailed usage information.

The basic connection-management commands (`add`, `list`, `remove`, `use`, `current`, `health`) are documented in the [CLI reference](../reference.md). This guide focuses on the commands that operate against a gateway.

## Prerequisites

- You must first add and/or select a gateway in the CLI using the gateway connection commands (`ap gateway add` / `ap gateway use`). See the [CLI reference](../reference.md).
- Commands that contact a gateway resolve the platform first, then the gateway under that platform (see [Gateway selection](#gateway-selection)).
- Credentials for a gateway can come from either the gateway configuration (added with `ap gateway add`) or from environment variables. **Environment variables take precedence** over the stored configuration.

## Authentication

Supported gateway auth types:

- `none`
- `basic`
- `bearer`

Environment variables override credentials stored in the CLI config.

| Auth type | Environment variables |
| --- | --- |
| `none` | _(no credentials required)_ |
| `basic` | `WSO2AP_GW_USERNAME`, `WSO2AP_GW_PASSWORD` |
| `bearer` | `WSO2AP_GW_TOKEN` |

## Gateway selection

All commands that contact a gateway accept two optional selection flags:

- `--platform <platform>` — the platform to resolve. Defaults to the active platform.
- `--gateway <display-name>` — the gateway to use. Defaults to the active gateway for the resolved platform.

Resolution semantics:

- When neither flag is given, the command uses the **active gateway** in the **active platform**.
- When `--platform` is given without `--gateway`, the command uses the active gateway in that platform.
- When `--gateway` is given, the command uses that named gateway in the resolved platform.

> **Note:** The `health` command also routes to the gateway-controller admin API. When a gateway was added with a separate `--admin-server`, that URL is used for `health`; otherwise the management `--server` URL is reused.

## Apply Command

### `ap gateway apply`

Creates or updates a gateway resource (REST API, MCP proxy, etc.) from a YAML or JSON file. The command reads `kind` and `metadata.name` from the file, checks whether the resource already exists, and then creates (`POST`) or updates (`PUT`) it.

```shell
ap gateway apply --file <path> [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway apply --file petstore-api.yaml
ap gateway apply -f petstore-api.json
ap gateway apply -f mcp-proxy.yaml --platform eu --gateway prod
```

Notes:

- Supported kinds: `RestApi` and `Mcp`.
- JSON input is converted to YAML before being sent.
- Ready-to-use sample CRs live in [`gateway/examples`](../../../gateway/examples): [`sample-echo-api.yaml`](../../../gateway/examples/sample-echo-api.yaml) and [`petstore-api.yaml`](../../../gateway/examples/petstore-api.yaml) (`RestApi`), and [`mcp-proxy.yaml`](../../../gateway/examples/mcp-proxy.yaml) (`Mcp`).

## REST API Commands

These commands manage REST APIs using the `/rest-apis` management endpoints. To create or update a REST API, use [`ap gateway apply`](#ap-gateway-apply) with a `kind: RestApi` file.

### `ap gateway rest-api list`

Lists REST APIs deployed on the gateway.

```shell
ap gateway rest-api list [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway rest-api list
ap gateway rest-api list --platform eu --gateway prod
```

Behavior:

- Prints a table with `ID`, `DISPLAY_NAME`, `VERSION`, `CONTEXT`, `STATE`, and `CREATED_AT`.

### `ap gateway rest-api get`

Retrieves a single REST API by ID, or by name and version.

```shell
ap gateway rest-api get --id <id> [--format <json|yaml>] [--platform <platform>] [--gateway <display-name>]
ap gateway rest-api get --display-name <name> --version <version> [--format <json|yaml>] [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway rest-api get --id sample-1 --format yaml
ap gateway rest-api get --display-name "PetStore API" --version v1.0 --format json
```

Notes:

- `--display-name` here is the **API** name (not the gateway). When using `--display-name`, `--version` is required.
- `--format` defaults to `yaml`.

### `ap gateway rest-api delete`

Deletes a REST API by ID.

```shell
ap gateway rest-api delete --id <id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway rest-api delete --id sample-1
```

## API Key Commands

These commands manage API keys for a REST API using the `/rest-apis/{id}/api-keys` endpoints.

### `ap gateway rest-api api-key create`

Generates a new API key from an `ApiKey` custom resource file (YAML or JSON). The parent REST API is taken from `spec.parentRef.name` and the key name from `metadata.name`. The plaintext key is returned once in the response.

```shell
ap gateway rest-api api-key create --file <api-key.yaml> [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway rest-api api-key create -f api-key.yaml
ap gateway rest-api api-key create -f api-key.json --platform eu --gateway prod
```

`ApiKey` CR shape (`api-key.yaml`):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: ApiKey
metadata:
  name: petstore-key-acme
spec:
  parentRef:
    kind: RestApi
    name: petstore-api-v1.0
  expiresIn:
    duration: 30
    unit: days
```

Notes:

- `metadata.name` becomes the API key name.
- `spec.parentRef.kind` must be `RestApi` (or omitted) for this command.
- All other `spec` fields (e.g. `apiKey`, `expiresIn`, `expiresAt`) are forwarded as the request body.
- Sample CR: [`gateway/examples/api-key.yaml`](../../../gateway/examples/api-key.yaml).

### `ap gateway rest-api api-key list`

Lists API keys for a REST API.

```shell
ap gateway rest-api api-key list --id <rest-api-id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway rest-api api-key list --id reading-list-api-v1.0
```

Behavior:

- Prints a table with `NAME`, `DISPLAY_NAME`, `API_ID`, `STATUS`, `CREATED_AT`, and `EXPIRES_AT`. The plaintext key value is only returned by `create`/`regenerate`.

### `ap gateway rest-api api-key regenerate`

Regenerates an API key value, replacing the previous one. The new plaintext key is returned once.

```shell
ap gateway rest-api api-key regenerate --id <rest-api-id> --key-name <name> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway rest-api api-key regenerate --id reading-list-api-v1.0 --key-name my-production-key
```

### `ap gateway rest-api api-key update`

Updates an existing API key with a new name.

```shell
ap gateway rest-api api-key update --id <rest-api-id> --key-name <current-name> --name <new-name> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway rest-api api-key update --id reading-list-api-v1.0 --key-name old-key-name --name new-key-name
```

### `ap gateway rest-api api-key revoke`

Revokes an API key so it can no longer be used for authentication.

```shell
ap gateway rest-api api-key revoke --id <rest-api-id> --key-name <name> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway rest-api api-key revoke --id reading-list-api-v1.0 --key-name my-production-key
```

## MCP Commands

These commands manage MCP proxies using the `/mcp-proxies` management endpoints. To create or update an MCP proxy, use [`ap gateway apply`](#ap-gateway-apply) with a `kind: Mcp` file.

### `ap gateway mcp list`

Lists MCP proxies deployed on the gateway.

```shell
ap gateway mcp list [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway mcp list
```

Behavior:

- Prints a table with `ID`, `DISPLAY_NAME`, `VERSION`, `CONTEXT`, `STATE`, and `CREATED_AT`.

### `ap gateway mcp get`

Retrieves a single MCP proxy by ID, or by name and version.

```shell
ap gateway mcp get --id <id> [--format <json|yaml>] [--platform <platform>] [--gateway <display-name>]
ap gateway mcp get --display-name <name> --version <version> [--format <json|yaml>] [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway mcp get --id sample-id --format json
ap gateway mcp get --display-name my-mcp --version 1.0.0 --format json
```

### `ap gateway mcp delete`

Deletes an MCP proxy by ID.

```shell
ap gateway mcp delete --id <id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway mcp delete --id sample-id
```

### `ap gateway mcp generate`

Generates an MCP configuration from a running MCP server. This command talks to the MCP server URL directly and does not require an active gateway.

```shell
ap gateway mcp generate --server <server> [--output <path>] [--header "Name: Value"]
```

Example:

```shell
ap gateway mcp generate --server http://localhost:3001/mcp --output target
```

## Subscription Plan Commands

These commands manage subscription plans using the `/subscription-plans` management endpoints. A subscription plan defines rate limits and access tiers for API subscriptions.

### `ap gateway subscription-plan create`

Creates a subscription plan from a `SubscriptionPlan` custom resource file (YAML or JSON). The resource `spec` is sent to the gateway management API.

```shell
ap gateway subscription-plan create --file <subscription-plan.yaml> [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway subscription-plan create -f subscription-plan.yaml
ap gateway subscription-plan create -f subscription-plan.json --platform eu --gateway prod
```

`SubscriptionPlan` CR shape (`subscription-plan.yaml`):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: SubscriptionPlan
metadata:
  name: bronze-1k-per-min
spec:
  planName: Bronze
  status: ACTIVE
  stopOnQuotaReach: true
  throttleLimitCount: 1000
  throttleLimitUnit: Min
```

Notes:

- `spec.planName` is required.
- `spec.throttleLimitUnit` accepts `Min`, `Hour`, `Day`, or `Month`.
- The `spec` fields are forwarded as the request body; `metadata.name` is the local CR handle.
- Sample CR: [`gateway/examples/subscription-plan.yaml`](../../../gateway/examples/subscription-plan.yaml).

### `ap gateway subscription-plan list`

Lists subscription plans on the gateway.

```shell
ap gateway subscription-plan list [--platform <platform>] [--gateway <display-name>]
```

Behavior:

- Prints a table with `ID`, `PLAN_NAME`, `BILLING_PLAN`, `THROTTLE_LIMIT`, `STATUS`, and `CREATED_AT`.

### `ap gateway subscription-plan get`

Gets a subscription plan by ID.

```shell
ap gateway subscription-plan get --id <plan-id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway subscription-plan get --id gold-plan
```

### `ap gateway subscription-plan update`

Updates a subscription plan. Only the flags you provide are sent, so partial updates are supported.

```shell
ap gateway subscription-plan update --id <plan-id> \
  [--plan-name <name>] [--billing-plan <plan>] [--stop-on-quota-reach] \
  [--throttle-limit-count <count>] [--throttle-limit-unit <Min|Hour|Day|Month>] \
  [--expiry-time <iso-8601>] [--status <ACTIVE|INACTIVE>] \
  [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway subscription-plan update --id gold-plan --throttle-limit-count 2000 --throttle-limit-unit Hour
ap gateway subscription-plan update --id gold-plan --status INACTIVE
```

### `ap gateway subscription-plan delete`

Deletes a subscription plan by ID.

```shell
ap gateway subscription-plan delete --id <plan-id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway subscription-plan delete --id gold-plan
```

## Subscription Commands

These commands manage subscriptions using the `/subscriptions` management endpoints. A subscription binds an application to a REST API under a subscription plan.

### `ap gateway subscription create`

Creates a subscription from a `Subscription` custom resource file (YAML or JSON). The resource `spec` is sent to the gateway management API.

```shell
ap gateway subscription create --file <subscription.yaml> [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway subscription create -f subscription.yaml
ap gateway subscription create -f subscription.json --platform eu --gateway prod
```

`Subscription` CR shape (`subscription.yaml`):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Subscription
metadata:
  name: petstore-acme-bronze
spec:
  apiId: petstore-api-v1.0
  subscriptionPlanId: bronze-1k-per-min
  status: ACTIVE
  subscriptionToken: a-strong-token-of-at-least-36-characters
```

Notes:

- `spec.apiId` is required.
- `spec.subscriptionToken` must be a plain string. Secret references (`valueFrom`) are an operator-only feature and are not resolved by the CLI.
- Sample CR: [`gateway/examples/subscription.yaml`](../../../gateway/examples/subscription.yaml).

### `ap gateway subscription list`

Lists subscriptions on the gateway, with optional filtering.

```shell
ap gateway subscription list [--api-id <id>] [--application-id <id>] [--status <ACTIVE|INACTIVE|REVOKED>] [--platform <platform>] [--gateway <display-name>]
```

Examples:

```shell
ap gateway subscription list
ap gateway subscription list --api-id reading-list-api-v1.0 --status ACTIVE
```

Behavior:

- Prints a table with `ID`, `API_ID`, `APPLICATION_ID`, `PLAN_ID`, `STATUS`, and `CREATED_AT`.

### `ap gateway subscription get`

Gets a subscription by ID.

```shell
ap gateway subscription get --id <subscription-id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway subscription get --id sub-1
```

### `ap gateway subscription update`

Updates a subscription's status.

```shell
ap gateway subscription update --id <subscription-id> --status <ACTIVE|INACTIVE|REVOKED> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway subscription update --id sub-1 --status REVOKED
```

### `ap gateway subscription delete`

Deletes a subscription by ID.

```shell
ap gateway subscription delete --id <subscription-id> [--platform <platform>] [--gateway <display-name>]
```

Example:

```shell
ap gateway subscription delete --id sub-1
```

## Image Commands

### `ap gateway image build`

Builds a gateway image from a gateway project directory.

```shell
ap gateway image build \
  [--name <gateway-name>] \
  [--path <gateway-project-dir>] \
  [--repository <image-repository>] \
  [--version <gateway-version>] \
  [--gateway-builder <gateway-builder-image>] \
  [--gateway-controller-base-image <gateway-controller-base-image>] \
  [--router-base-image <router-base-image>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--offline] \
  [--output-dir <output_dir>]
```

Example:

```shell
ap gateway image build
```

For more information about customizing gateway policies during a build, refer to [this document](../customizing-gateway-policies.md).

## Related Commands

- `ap gateway add`
- `ap gateway use`
- `ap gateway current`
- `ap platform use`
