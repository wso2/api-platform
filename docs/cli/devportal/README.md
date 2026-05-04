# DevPortal CLI Reference

This guide covers the DevPortal-related commands currently implemented under `cli/src/cmd/devportal`.

Available command groups:

- `ap devportal`
- `ap devportal rest-api`
- `ap devportal org`

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

### `ap devportal org add`

Creates an organization using a JSON request payload file.

```shell
ap devportal org add --file <organization.json> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal org add -f organization.json
ap devportal org add -f organization.json --display-name my-portal --platform eu
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

Expected payload shape for `ap devportal org add` and `ap devportal org edit`:

```json
{
  "orgName": "Acme",
  "organizationIdentifier": "acme",
  "businessOwner": "Jane Doe"
}
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

### `ap devportal rest-api publish`

Publishes a DevPortal artifact zip to a DevPortal organization.

```shell
ap devportal rest-api publish [--file <zip-path>] --org <org-id> [--display-name <devportal-name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap devportal rest-api publish --org org_1
ap devportal rest-api publish -f fooapi/build/devportal.zip --org org_1
ap devportal rest-api publish -f fooapi/build/devportal.zip --org org_1 --display-name my-portal --platform eu
ap devportal rest-api publish -f fooapi/build/devportal.zip --org org_1 --insecure
```

Behavior:

- If `--file` is omitted, the command looks for `./devportal.zip` in the current directory.
- If neither `--file` nor `./devportal.zip` is available, the command returns an error.
- If `--display-name` is provided, the named DevPortal is used.
- If `--display-name` is provided without `--platform`, the command looks in the `default` platform.
- If `--display-name` is not provided, the command uses the active DevPortal of the resolved platform.
- `--insecure` skips TLS certificate verification for local or self-signed HTTPS endpoints.

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

## Related Commands

- `ap platform add`
- `ap platform use`
- `ap devportal use`
- `ap apiproject init`
