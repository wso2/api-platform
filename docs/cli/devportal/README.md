# DevPortal CLI Reference

This guide covers the DevPortal-related commands currently implemented under `cli/src/cmd/devportal`.

Available command groups:

- `ap devportal`
- `ap devportal rest-api`
- `ap devportal org`
- `ap devportal api-key`
- `ap devportal application`
- `ap devportal subscription`
- `ap devportal sub-plan`

## Prerequisites

- Add at least one DevPortal configuration before using commands that contact a DevPortal server.
- DevPortal configurations are stored in the CLI config file managed by `ap devportal add`.
- Commands that use an active DevPortal resolve the platform first, then the active DevPortal under that platform.

## Authentication

Supported DevPortal auth types:

- `basic`
- `oauth`
- `api-key`

Environment variables override credentials stored in the CLI config.

| Auth type | Environment variables |
| --- | --- |
| `basic` | `WSO2AP_DEVPORTAL_USERNAME`, `WSO2AP_DEVPORTAL_PASSWORD` |
| `oauth` | `WSO2AP_DEVPORTAL_TOKEN` |
| `api-key` | `WSO2AP_DEVPORTAL_API_KEY` |

## Connection Notes

- Commands that call the DevPortal API support `--insecure` when certificate verification must be skipped for local or self-signed HTTPS endpoints.
- When a command accepts `--display-name` and `--platform`, it resolves the DevPortal explicitly.
- When `--display-name` is provided without `--platform`, the command looks in the `default` platform.
- When `--display-name` is not provided, the command uses the active DevPortal in the resolved platform.

## Commands

### `ap devportal add`

Adds a DevPortal configuration to the CLI config file.

```shell
ap devportal add --display-name <name> --server <server-url> --auth <basic|oauth|api-key> [--platform <platform>] [--no-interactive]
```

Examples:

```shell
ap devportal add
ap devportal add --display-name my-portal --server https://devportal.example.com --auth basic
ap devportal add --display-name my-portal --server https://devportal.example.com --auth oauth
ap devportal add --display-name my-portal --server https://devportal.example.com --auth api-key
ap devportal add --display-name my-portal --platform eu --server https://devportal.example.com --auth api-key --no-interactive
```

Notes:

- Interactive mode prompts for missing values.
- Supplying credentials as flags is supported, but interactive mode or environment variables are preferred.
- If credentials are omitted, runtime commands expect the corresponding environment variables.

### `ap devportal list`

Lists DevPortal configurations for a platform.

```shell
ap devportal list [--platform <platform>]
```

Example:

```shell
ap devportal list
ap devportal list --platform eu
```

Notes:

- If `--platform` is omitted, the current platform is used.
- The active DevPortal is marked in the output table.

### `ap devportal remove`

Removes a DevPortal configuration from a platform.

```shell
ap devportal remove --display-name <name> [--platform <platform>]
```

Example:

```shell
ap devportal remove --display-name my-portal
```

### `ap devportal use`

Sets the active DevPortal for a platform.

```shell
ap devportal use --display-name <name> [--platform <platform>]
```

Example:

```shell
ap devportal use --display-name my-portal
ap devportal use --display-name my-portal --platform eu
```

Notes:

- If `--platform` is omitted, the current platform is used.
- The command reports whether credentials will come from environment variables or the stored config.

### `ap devportal current`

Shows the active DevPortal for a platform.

```shell
ap devportal current [--platform <platform>]
```

Example:

```shell
ap devportal current
```

### `ap devportal health`

Calls the DevPortal health endpoint using the active DevPortal for the resolved platform.

```shell
ap devportal health [--platform <platform>]
```

Example:

```shell
ap devportal health
ap devportal health --platform eu
```

### `ap devportal build`

Builds DevPortal deployment artifacts from an API project.

```shell
ap devportal build [-f <api-project-directory>]
```

Examples:

```shell
ap devportal build
ap devportal build -f /path/to/project
```

Behavior:

- If `-f` is omitted, the current directory is treated as the API project root.
- The command expects an API project with a `.api-platform/config.yaml`.
- If `devportals` configuration is missing in the API project config, a default `devportal/` structure is created and added to the config.
- The `build/` directory is cleaned before new artifacts are written.
- One zip is generated per configured DevPortal entry.

Generated artifact names:

- `default` DevPortal config: `build/devportal.zip`
- named DevPortal config: `build/devportal_<name>.zip`

## Apply Command

### `ap devportal apply`

**Creates** or **updates** a DevPortal resource from a **single file**, aligning with `ap gateway apply` and `ap ai-workspace apply`. Because a project can contain multiple DevPortal resources, you point `-f` at the **exact file** — a YAML CR or a built REST API artifact zip — not at the project directory.

```shell
ap devportal apply -f <file> [--org <org-id>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

The target endpoint is selected from the resource **kind**. For a YAML CR the top-level `kind` is read directly; for a `.zip` the `kind` is read from the artifact's `devportal.yaml`.

For kinds that are addressable by their handle (`metadata.name`) and expose a `PUT` — `Organization` and `RestApi` — apply first **checks whether the resource exists** (`GET {resource}/{handle}`, like `ap gateway apply`) and then **updates** it (`PUT`, reported as `updated`) or **creates** it (`POST`, reported as `applied`). Subscription plans have no per-plan `PUT` — their publish endpoint upserts, and `SubscriptionPolicyList` is a bulk upload — so they are always `POST`ed.

| Kind (source) | Input | Create / update | `--org` |
| --- | --- | --- | --- |
| `Organization` (YAML CR) | `.yaml` | `GET /organizations/{name}` → `PUT /organizations/{name}` or `POST /organizations` | not required |
| `SubscriptionPolicy` / `SubscriptionPolicyList` (YAML CR) | `.yaml` | `POST .../subscription-policies` (server upsert) | **required** |
| `RestApi` (artifact `devportal.yaml`) | `.zip` | `GET .../apis/{name}` → `PUT .../apis/{name}` or `POST .../apis` | **required** |

Examples:

```shell
# Organization (no --org; the file itself identifies the org handle)
ap devportal apply -f org.yaml

# Subscription plan(s) — single (kind: SubscriptionPolicy) or bulk (kind: SubscriptionPolicyList)
ap devportal apply -f sub_plan.yaml --org org_1

# REST API from a built artifact zip (kind: RestApi read from devportal.yaml)
ap devportal apply -f build/devportal.zip --org org_1

# Target a specific devportal without relying on the active one
ap devportal apply -f org.yaml --display-name my-portal --platform eu
```

Notes:

- `--file` is required and must be an **exact file** (a `.yaml`/`.yml` CR or a `.zip` artifact), not a directory.
- `--org` is required for the org-scoped kinds (`RestApi`, `SubscriptionPolicy`/`SubscriptionPolicyList`) and is not needed for `Organization`.
- A `RestApi` must be supplied as a built `.zip` (from `ap devportal build`); a CR must be supplied as YAML.
- On success it prints a `Status`/`Message`/`ID` summary — the message reports `applied` (created) or `updated` — followed by the server response body.
- This replaces the former `ap devportal org add`, `ap devportal sub-plan publish`, and `ap devportal rest-api publish` commands (which were create/publish-only).

## Organization Commands

These commands manage DevPortal organizations using the `/devportal/organizations` endpoints.

### `ap devportal org list`

Lists organizations in the selected DevPortal.

```shell
ap devportal org list [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal org list
ap devportal org list --display-name my-portal --platform eu
```

Behavior:

- Prints a table with `ORG_ID`, `ORG_NAME`, `BUSINESS_OWNER`, and `ORGANIZATION_IDENTIFIER`.

### `ap devportal org get`

Gets a single organization by ID.

```shell
ap devportal org get --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal org get --org org_1
ap devportal org get --org org_1 --display-name my-portal --platform eu
```

### Creating an organization

Organizations are created with the unified [`ap devportal apply`](#ap-devportal-apply) command from a `kind: Organization` YAML CR:

```shell
ap devportal apply -f org.yaml
```

Expected CR shape (`org.yaml`):

```yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: Organization

metadata:
  name: ACME        # read as the organization handle

spec:
  displayName: acme # read as the organization display name; all other fields are read from spec
  organizationIdentifier: acme
  adminRole: admin
  subscriberRole: subscriber
  superAdminRole: superAdmin

  labels:
    - name: default
      displayName: Default

  views:
    - name: default
      displayName: Default View
      labels:
        - default
```

### `ap devportal org edit`

Updates an organization using a JSON request payload file.

```shell
ap devportal org edit --org <org-id> --file <organization.json> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal org edit --org org_1 -f organization.json
ap devportal org edit --org org_1 -f organization.json --display-name my-portal --platform eu
```

### `ap devportal org delete`

Deletes an organization by ID.

```shell
ap devportal org delete --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal org delete --org org_1
ap devportal org delete --org org_1 --display-name my-portal --platform eu
```

Expected payload shape for `ap devportal org edit`:

`organization.json`
```json
{
  "orgName": "John",
  "businessOwner": "Jane Doe",
  "businessOwnerContact": "+1-202-555-0147",
  "businessOwnerEmail": "jane.doe@abc.example",
  "orgHandle": "johndoe",
  "roleClaimName": "roles",
  "groupsClaimName": "groups",
  "organizationClaimName": "organizationIdentifier",
  "organizationIdentifier": "JOHN",
  "adminRole": "admin",
  "subscriberRole": "subscriber",
  "superAdminRole": "superAdmin"
}
```

## Application Commands

These commands manage DevPortal applications using the `/devportal/organizations/{orgId}/applications` endpoints.

### `ap devportal application create`

Creates an application. Only `--name` and `--type` are required; `--description` is optional.

```shell
ap devportal application create --org <org-id> --name <name> --type <type> [--description <description>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal application create --org org_1 --name "Weather App" --type WEB
ap devportal application create --org org_1 --name "Weather App" --type WEB --description "Calls the Weather APIs"
ap devportal application create --org org_1 --name "Weather App" --type WEB --display-name my-portal --platform eu
```

Expected payload shape (`description` is omitted when `--description` is not provided):

```json
{
  "name": "Weather App",
  "type": "WEB",
  "description": "Calls the Weather APIs"
}
```

### `ap devportal application get`

Lists applications in an organization, or retrieves a single application when `--app-id` is provided.

```shell
ap devportal application get --org <org-id> [--app-id <app-id>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal application get --org org_1
ap devportal application get --org org_1 --app-id app_1
ap devportal application get --org org_1 --app-id app_1 --display-name my-portal --platform eu
```

### `ap devportal application update`

Updates an existing application. `--name` and `--type` are required in the body; `--description` is optional. (`edit` is accepted as an alias.)

```shell
ap devportal application update --org <org-id> --app-id <app-id> --name <name> --type <type> [--description <description>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal application update --org org_1 --app-id app_1 --name "Weather App" --type WEB
ap devportal application update --org org_1 --app-id app_1 --name "Weather App" --type WEB --description "Calls the Weather APIs"
```

### `ap devportal application delete`

Deletes an application by its application ID.

```shell
ap devportal application delete --org <org-id> --app-id <app-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal application delete --org org_1 --app-id app_1
ap devportal application delete --org org_1 --app-id app_1 --display-name my-portal --platform eu
```

## Subscription Commands

These commands manage DevPortal platform subscriptions using the `/devportal/organizations/{orgId}/api-platform-subscriptions` endpoints.

### `ap devportal subscription create`

Creates a platform subscription. Only the API ID is required; the subscription plan is optional.

```shell
ap devportal subscription create --org <org-id> --api-id <api-id> [--subscription-plan <plan-name>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal subscription create --org org_1 --api-id api_1
ap devportal subscription create --org org_1 --api-id api_1 --subscription-plan gold
ap devportal subscription create --org org_1 --api-id api_1 --subscription-plan gold --display-name my-portal --platform eu
```

Expected payload shape (`subscriptionPlanName` is omitted when `--subscription-plan` is not provided):

```json
{
  "apiId": "api_1",
  "subscriptionPlanName": "gold"
}
```

### `ap devportal subscription edit`

Updates a platform subscription status with flags.

```shell
ap devportal subscription edit --org <org-id> --sub-id <subscription-id> --status <status> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal subscription edit --org org_1 --sub-id sub_1 --status ACTIVE
ap devportal subscription edit --org org_1 --sub-id sub_1 --status ACTIVE --display-name my-portal --platform eu
```

Expected payload shape:

```json
{
  "status": "ACTIVE"
}
```

### `ap devportal subscription get`

Gets all platform subscriptions in an organization, or a single subscription when `--sub-id` is provided.

```shell
ap devportal subscription get --org <org-id> [--sub-id <subscription-id>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal subscription get --org org_1
ap devportal subscription get --org org_1 --sub-id sub_1
ap devportal subscription get --org org_1 --sub-id sub_1 --display-name my-portal --platform eu
```

### `ap devportal subscription delete`

Deletes a platform subscription by ID.

```shell
ap devportal subscription delete --org <org-id> --sub-id <subscription-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal subscription delete --org org_1 --sub-id sub_1
ap devportal subscription delete --org org_1 --sub-id sub_1 --display-name my-portal --platform eu
```

## REST API Commands

These commands manage API artifacts in a DevPortal organization.

### `ap devportal rest-api list`

Lists APIs in a DevPortal organization.

```shell
ap devportal rest-api list --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal rest-api list --org org_1
ap devportal rest-api list --org org_1 --display-name my-portal --platform eu
```

Behavior:

- Prints a table with `API_ID`, `API_HANDLE`, `API_NAME`, and `API_VERSION`.
- Use the `API_ID` value from the table with `ap devportal rest-api get`.

### `ap devportal rest-api get`

Gets a single API artifact from a DevPortal organization.

```shell
ap devportal rest-api get --org <org-id> --id <api-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal rest-api get --org org_1 --id api_1
ap devportal rest-api get --org org_1 --id api_1 --display-name my-portal --platform eu
```

### Publishing a REST API

A built REST API artifact zip is published with the unified [`ap devportal apply`](#ap-devportal-apply) command — `apply` reads `kind: RestApi` from the zip's `devportal.yaml` and routes it to the organization's `apis` endpoint:

```shell
ap devportal apply -f build/devportal.zip --org org_1
```

Build the zip first with [`ap devportal build`](#ap-devportal-build). Use `ap devportal rest-api get`/`list`/`edit`/`delete` (below) to manage an already-published API.

### `ap devportal rest-api edit`

Updates an existing API artifact in a DevPortal organization.

```shell
ap devportal rest-api edit [--file <zip-path>] --org <org-id> --id <api-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal rest-api edit --org org_1 --id api_1
ap devportal rest-api edit -f fooapi/build/devportal.zip --org org_1 --id api_1
ap devportal rest-api edit -f fooapi/build/devportal.zip --org org_1 --id api_1 --display-name my-portal --platform eu
```

Behavior:

- If `--file` is omitted, the command looks for `./devportal.zip` in the current directory.
- If neither `--file` nor `./devportal.zip` is available, the command returns an error.

### `ap devportal rest-api delete`

Deletes an API artifact from a DevPortal organization.

```shell
ap devportal rest-api delete --org <org-id> --id <api-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal rest-api delete --org org_1 --id api_1
ap devportal rest-api delete --org org_1 --id api_1 --display-name my-portal --platform eu
```

Current status:

- The command exists in the CLI surface, but the apply flow is not implemented yet.

## API Key Commands

These commands manage API keys using the `/devportal/organizations/{orgId}/api-keys` endpoints.

### `ap devportal api-key generate`

Generates an API key for an API. The plaintext secret is returned once in the response and is never persisted. Run without the required flags to be prompted interactively.

```shell
ap devportal api-key generate --org <org-id> --api-id <api-id> --name <key-name> [--expires-at <expiry>] [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
# Provide everything via flags
ap devportal api-key generate --org org_1 --api-id api_1 --name weather_prod_key

# Add an expiry (ISO-8601 with timezone, epoch seconds, or epoch milliseconds)
ap devportal api-key generate --org org_1 --api-id api_1 --name weather_prod_key --expires-at 2026-12-31T23:59:59Z

# Interactive mode (prompts for any missing org/api-id/name/expiry)
ap devportal api-key generate

# Skip prompts and fail if a required flag is missing
ap devportal api-key generate --org org_1 --api-id api_1 --name weather_prod_key --no-interactive
```

Notes:

- `--name` is the API key name and must match `^[a-z0-9][a-z0-9_-]{0,127}$` (lowercase letters, numbers, `_`, and `-`). The CLI validates this before sending the request.
- `--expires-at` is optional.
- `generate` is the only API key command with interactive mode.

### `ap devportal api-key get`

Lists API keys for an API.

```shell
ap devportal api-key get --org <org-id> --api-id <api-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal api-key get --org org_1 --api-id api_1
ap devportal api-key get --org org_1 --api-id api_1 --display-name my-portal --platform eu
```

### `ap devportal api-key regenerate`

Regenerates the secret for an existing API key. The old secret is invalidated at connected gateways and the new plaintext secret is returned once.

```shell
ap devportal api-key regenerate --org <org-id> --api-key-id <api-key-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal api-key regenerate --org org_1 --api-key-id key_1
ap devportal api-key regenerate --org org_1 --api-key-id key_1 --display-name my-portal --platform eu
```

### `ap devportal api-key revoke`

Revokes an existing API key. Connected gateways immediately reject requests carrying the revoked key.

```shell
ap devportal api-key revoke --org <org-id> --api-key-id <api-key-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal api-key revoke --org org_1 --api-key-id key_1
ap devportal api-key revoke --org org_1 --api-key-id key_1 --display-name my-portal --platform eu
```

## Subscription Plan Commands

These commands manage subscription plans using the `/devportal/organizations/{orgId}/subscription-policies` endpoint.

### Publishing subscription plans

Subscription plans are published with the unified [`ap devportal apply`](#ap-devportal-apply) command from a YAML CR — either a single plan (`kind: SubscriptionPolicy`) or a bulk list (`kind: SubscriptionPolicyList` with an `items` array). `--org` is required.

```shell
ap devportal apply -f sub_plan_gold.yaml --org org_1     # single plan
ap devportal apply -f sub_plans.yaml --org org_1         # bulk list
```

The CLI validates the CR locally before upload: `kind` must be `SubscriptionPolicy` or `SubscriptionPolicyList`, and each plan must have `metadata.name`.

Single plan CR shape (`sub_plan_gold.yaml`):

```yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: SubscriptionPolicy
metadata:
  name: Gold
spec:
  displayName: Gold Plan
  billingPlan: FREE
  type: requestcount
  requestCount: 5000
  description: Allows 5000 requests per minute
  refId: cp-plan-gold           # optional external reference
```

Multiple plans in one file (`sub_plans.yaml`):

```yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: SubscriptionPolicyList
items:
  - metadata:
      name: Gold
    spec:
      displayName: Gold Plan
      billingPlan: FREE
      type: requestcount
      requestCount: 5000
  - metadata:
      name: Unlimited
    spec:
      displayName: Unlimited
      billingPlan: FREE
      type: requestcount
      requestCount: -1
```

Notes:

- `type` accepts `requestcount` or `eventcount`. Use `-1` for an unlimited request/event count.

### `ap devportal sub-plan list`

Lists all subscription plans in an organization.

```shell
ap devportal sub-plan list --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal sub-plan list --org org_1
ap devportal sub-plan list --org org_1 --display-name my-portal --platform eu
```

Notes:

- Calls `GET /o/<org-id>/devportal/v1/subscription-policies` (operationId `listSubscriptionPolicies`).

### `ap devportal sub-plan get`

Gets a single subscription plan by its policy ID.

```shell
ap devportal sub-plan get --policy-id <policy-id> --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal sub-plan get --policy-id plan_1 --org org_1
ap devportal sub-plan get --policy-id plan_1 --org org_1 --display-name my-portal --platform eu
```

Notes:

- The spec path parameter is `policyIdOrName`; this command always treats the supplied value as the policy ID.

### `ap devportal sub-plan delete`

Deletes a subscription plan by its policy ID.

```shell
ap devportal sub-plan delete --policy-id <policy-id> --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal sub-plan delete --policy-id plan_1 --org org_1
ap devportal sub-plan delete --policy-id plan_1 --org org_1 --display-name my-portal --platform eu
```

Notes:

- The spec path parameter is `policyIdOrName`; this command always treats the supplied value as the policy ID.
- A successful delete returns `204 No Content`.

## Related Commands

- `ap platform add`
- `ap platform use`
- `ap devportal use`
- `ap apiproject init`
