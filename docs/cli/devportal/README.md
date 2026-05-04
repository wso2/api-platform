# DevPortal CLI Reference

This guide covers the DevPortal-related commands currently implemented under `cli/src/cmd/devportal`.

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

## Related Commands

- `ap platform add`
- `ap platform use`
- `ap devportal use`
- `ap apiproject init`
