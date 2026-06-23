# AI-Workspace CLI Reference

This guide covers the AI-Workspace commands currently implemented under `cli/src/cmd/aiws`.

Available command group:

- `ap ai-ws`

The `add`, `list`, `remove`, `use`, and `current` commands manage AI-Workspace **server connections** stored in the CLI config file (the same per-platform config used by `ap gateway` and `ap devportal`). The `build` command is different: it works inside an **API project** and generates an LLM proxy creation payload from the project's artifacts.

## Prerequisites

- Add at least one AI-Workspace configuration before using commands that target a specific workspace connection.
- AI-Workspace connections are stored in the CLI config file managed by `ap ai-ws add`.
- Commands that use an active AI-Workspace resolve the platform first, then the active AI-Workspace under that platform.

## Authentication

Supported AI-Workspace auth types:

- `basic`
- `oauth`
- `api-key` (default)

Environment variables override credentials stored in the CLI config.

| Auth type | Environment variables |
| --- | --- |
| `basic` | `WSO2AP_AIWORKSPACE_USERNAME`, `WSO2AP_AIWORKSPACE_PASSWORD` |
| `oauth` | `WSO2AP_AIWORKSPACE_TOKEN` |
| `api-key` | `WSO2AP_AIWORKSPACE_API_KEY` |

## Connection Notes

- When a command accepts `--display-name` and `--platform`, it resolves the AI-Workspace explicitly.
- When `--display-name` is provided without `--platform`, the command looks in the `default` platform.
- When `--display-name` is not provided, the command uses the active AI-Workspace in the resolved platform.

## Connection Commands

### `ap ai-ws add`

Adds an AI-Workspace configuration to the CLI config file.

```shell
ap ai-ws add --display-name <name> --server <server-url> --auth <basic|oauth|api-key> [--platform <platform>] [--no-interactive]
```

Examples:

```shell
ap ai-ws add
ap ai-ws add --display-name my-workspace --server https://ai-workspace.example.com --auth basic
ap ai-ws add --display-name my-workspace --server https://ai-workspace.example.com --auth oauth
ap ai-ws add --display-name my-workspace --server https://ai-workspace.example.com --auth api-key
ap ai-ws add --display-name my-workspace --platform eu --server https://ai-workspace.example.com --auth api-key --no-interactive
```

Notes:

- Interactive mode prompts for missing values.
- Supplying credentials as flags is supported, but interactive mode or environment variables are preferred.
- If credentials are omitted, runtime commands expect the corresponding environment variables.

### `ap ai-ws list`

Lists AI-Workspace configurations for a platform.

```shell
ap ai-ws list [--platform <platform>]
```

Examples:

```shell
ap ai-ws list
ap ai-ws list --platform eu
```

Notes:

- If `--platform` is omitted, the current platform is used.
- The active AI-Workspace is marked in the output table.

### `ap ai-ws remove`

Removes an AI-Workspace configuration from a platform.

```shell
ap ai-ws remove --display-name <name> [--platform <platform>]
```

Example:

```shell
ap ai-ws remove --display-name my-workspace
```

### `ap ai-ws use`

Sets the active AI-Workspace for a platform.

```shell
ap ai-ws use --display-name <name> [--platform <platform>]
```

Examples:

```shell
ap ai-ws use --display-name my-workspace
ap ai-ws use --display-name my-workspace --platform eu
```

Notes:

- If `--platform` is omitted, the current platform is used.
- The command reports whether credentials will come from environment variables or the stored config.

### `ap ai-ws current`

Shows the active AI-Workspace for a platform.

```shell
ap ai-ws current [--platform <platform>]
```

Example:

```shell
ap ai-ws current
```

## `ap ai-ws build`

Generates an **LLM proxy**, **LLM provider**, or **MCP proxy** creation payload (JSON) from an API project's artifacts. The payload shape is selected by the `kind` declared in `metadata.yaml`/`runtime.yaml`:

- `kind: LlmProxy` → matches the `POST /llm-proxies` request body (`LLMProxy` schema).
- `kind: LlmProvider` → matches the `POST /llm-providers` request body (`LLMProvider` schema).
- `kind: Mcp` → MCP proxy creation payload (capabilities + policies).

Any other kind is rejected.

```shell
ap ai-ws build [-f <project-directory>] [-o <file.json | directory>] [--use-spec]
```

Examples:

```shell
# Build using the current directory as the project root
ap ai-ws build

# Build from a specific project directory
ap ai-ws build -f /path/to/project

# Write the payload to a specific directory
ap ai-ws build -o build/

# Write the payload to a specific file
ap ai-ws build -o build/openai.json

# Fold the OpenAPI spec (definition.yaml) into the payload
ap ai-ws build --use-spec
```

### What it reads

The command expects an API project containing `.api-platform/config.yaml`, and reads the `ai-workspaces` section of that file:

```yaml
ai-workspaces:
  - name: dev
    portalRoot: .
    filePaths:                  # paths relative to portalRoot
      metadata: ./metadata.yaml
      runtime: ./runtime.yaml
      definition: ./definition.yaml   # required for LlmProvider; opt-in for LlmProxy via --use-spec
```

For each configured entry, the build:

- Resolves `metadata`, `runtime`, and `definition` relative to that entry's `portalRoot` (defaults: `./metadata.yaml`, `./runtime.yaml`, `./definition.yaml`; `portalRoot` defaults to `.`, the project root).
- Requires `metadata.yaml` and `runtime.yaml` to exist.
- Requires the `kind` declared in `metadata.yaml` and `runtime.yaml` to match; otherwise the build fails with a kind-mismatch error.
- Requires `metadata.name` to match between `metadata.yaml` and `runtime.yaml`; otherwise the build fails with a name-mismatch error.
- Handles `definition.yaml` (the OpenAPI spec) by kind — see [The OpenAPI spec](#the-openapi-spec---use-spec) below.
- If no `ai-workspaces` section exists, a single `default` entry (`portalRoot: .`) is created in the project config and used.

All resolved paths are constrained to the project directory; a path that escapes the project root fails the build for that entry.

### What it generates

One JSON file per configured AI-Workspace entry, written to the build output.

#### `LlmProxy`

| Payload field | Source |
| --- | --- |
| `id` | `metadata.yaml` → `metadata.name` |
| `name` | `metadata.yaml` → `metadata.name` |
| `version` | `metadata.yaml` → `spec.version` |
| `context` | `runtime.yaml` → `spec.context` |
| `provider` (`id`, `auth.{type,header,value}`) | `runtime.yaml` → `spec.provider` |
| `policies[]` (`name`, `version`, `paths[].{path,methods,params}`) | `runtime.yaml` → `spec.policies` |
| `openapi` | content of `definition.yaml` with `--use-spec`, otherwise empty |
| `vhost` | always empty (filled in at publish time) |
| `projectId` | intentionally omitted |

#### `LlmProvider`

| Payload field | Source |
| --- | --- |
| `id` | `metadata.yaml` → `metadata.name` |
| `name` | `metadata.yaml` → `metadata.name` |
| `version` | `metadata.yaml` → `spec.version` |
| `context` | `runtime.yaml` → `spec.context` |
| `template` | `runtime.yaml` → `spec.template` |
| `upstream` (`main.{url,auth}`) | `runtime.yaml` → `spec.upstream` |
| `accessControl` (`mode`, `exceptions[]`) | `runtime.yaml` → `spec.accessControl` |
| `security` (`apiKey.{key,in}`) | the `api-key-auth` policy in `runtime.yaml` → `spec.policies` |
| `rateLimiting` | the `*-ratelimit` policies (see below) |
| `policies[]` (`name`, `version`, `paths[].{path,methods,params}`) | every other `runtime.yaml` → `spec.policies` entry (i.e. not `api-key-auth` or `*-ratelimit`) |
| `openapi` | content of `definition.yaml` (**required** for providers) |

**rateLimiting mapping.** Each policy whose name ends with `-ratelimit` becomes a rate-limiting dimension, selected by name:

| Policy name | Dimension | Value source (`params`) |
| --- | --- | --- |
| `advanced-ratelimit` | `request` | `quotas[].limits[].{limit,duration}` |
| `token-based-ratelimit` | `token` | `totalTokenLimits[].{count,duration}` |
| `llm-cost-based-ratelimit` | `cost` | `budgetLimits[].{amount,duration}` |

Each dimension is placed under `consumerLevel` when the policy params carry `consumerBased: true` (or, for `advanced-ratelimit`, when the quota `name` starts with `consumer`); otherwise under `providerLevel`. Durations like `1h`/`3h` are parsed into `{duration, unit}` reset windows.

A limit whose path is `/*` is applied as a `global` limit for its scope; a limit on a specific path is applied `resourceWise`, keyed by that path (with any `/*` limits in the same scope folded into the resourceWise `default`).

> Not yet emitted: `modelProviders` (no source defined in the project artifacts yet).

#### `Mcp`

| Payload field | Source |
| --- | --- |
| `id` | `metadata.yaml` → `metadata.name` |
| `name` | `metadata.yaml` → `metadata.name` |
| `version` | `metadata.yaml` → `spec.version` |
| `context` | `runtime.yaml` → `spec.context` |
| `mcpSpecVersion` | `runtime.yaml` → `spec.specVersion` |
| `upstream` (`main.{url,auth}`) | `runtime.yaml` → `spec.upstream` |
| `policies[]` (`name`, `version`, `params`) | `runtime.yaml` → `spec.policies` (auth/authz/etc.) |
| `capabilities` (`prompts`, `resources`, `tools`) | `definition.yaml` (**required** for MCP) |
| `description` | empty |

`definition.yaml` for an MCP proxy holds `prompts`, `resources`, and `tools`. `prompts` and `tools` are passed through unchanged; `resources` are trimmed to `uri`, `name`, and `mimeType` (any inline `text`/`blob` content is dropped). `projectId` is omitted and injected at publish time.

### The OpenAPI spec (`--use-spec`)

The `definition.yaml` is handled by kind:

- **`LlmProvider`** — **required**: `definition.yaml` must exist (the build errors otherwise) and is always folded into `openapi`. `--use-spec` is not needed.
- **`Mcp`** — **required**: `definition.yaml` must exist; its `prompts`/`resources`/`tools` populate `capabilities`. `--use-spec` is not needed.
- **`LlmProxy`** — **opt-in**: `openapi` is empty by default even when `definition.yaml` exists; pass `--use-spec` to fold it in. A missing `definition.yaml` with `--use-spec` leaves the field empty (no error).

### Output location and artifact names (`-o`)

The artifact file name is taken from the **ai-workspace config `name`** in `.api-platform/config.yaml` (not from `metadata.name`).

- **No `-o`** — payloads are written to the project `build/` directory as `build/<config-name>.json`.
- **`-o <directory>`** (a path without a `.json` extension, e.g. `-o build/`) — payloads are written to that directory as `<directory>/<config-name>.json`. Missing directories are created.
- **`-o <file.json>`** (a path ending in `.json`) — the single payload is written to exactly that file. Missing parent directories are created.

Behavior notes:

- An existing output file is overwritten.
- A `.json` file target can only hold one payload, so `-o <file.json>` with **multiple** `ai-workspaces` configurations is an error — use a directory instead.
- With a directory target and multiple configurations, one file per config is produced (`<config-name>.json`).

## Get Commands

These commands retrieve artifacts from the AI workspace resolved from the CLI config (`--display-name`/`--platform`, or the active AI workspace). With `--id` a single artifact is fetched; without it all artifacts are listed, with optional `--limit`/`--offset` pagination. The full JSON response is printed.

The scoping query parameter differs by resource:

- **LLM providers** are scoped by `organizationId` (`--org`).
- **LLM/MCP proxies** are scoped by `projectId` (`--project-id`) when listing; fetching a single proxy by `--id` takes only the id path parameter (no org/project query).

### `ap ai-ws llm-provider list`

Lists all LLM providers in an organization (`GET /llm-providers?organizationId={org}`, operationId `listLLMProviders`).

```shell
ap ai-ws llm-provider list --org <org-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-ws llm-provider get`

```shell
# List all LLM providers
ap ai-ws llm-provider get --org <org-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]

# Get a single LLM provider
ap ai-ws llm-provider get --org <org-id> --id <provider-id>
```

### `ap ai-ws llm-proxy get`

```shell
# List all LLM proxies in a project (GET /llm-proxies?projectId={project})
ap ai-ws llm-proxy get --project-id <project-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]

# Get a single LLM proxy (GET /llm-proxies/{id})
ap ai-ws llm-proxy get --id <proxy-id>
```

### `ap ai-ws mcp-proxy get`

```shell
# List all MCP proxies in a project (GET /mcp-proxies?projectId={project})
ap ai-ws mcp-proxy get --project-id <project-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]

# Get a single MCP proxy (GET /mcp-proxies/{id})
ap ai-ws mcp-proxy get --id <proxy-id>
```

Notes:

- `llm-provider list` and `llm-provider get` both list providers when no `--id` is given; `list` is the dedicated list-all command (`--org` required), while `get` additionally fetches a single provider with `--id`.
- For `llm-provider get`, `--org` is required. For `llm-proxy`/`mcp-proxy get`, `--project-id` is required only when listing; fetching a single proxy needs just `--id`.
- `--limit` and `--offset` apply only when listing.
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Delete Commands

These commands delete an artifact by its identifier (`DELETE /{resource}/{id}`). The artifact is identified solely by `--id` — no organization or project scoping is required — and a successful delete (`204 No Content`) prints a confirmation line.

### `ap ai-ws llm-provider delete`

```shell
ap ai-ws llm-provider delete --id <provider-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-ws llm-proxy delete`

```shell
ap ai-ws llm-proxy delete --id <proxy-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-ws mcp-proxy delete`

```shell
ap ai-ws mcp-proxy delete --id <proxy-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Notes:

- `--id` is required for all delete commands.
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Push Commands

These commands push a payload JSON (produced by `ap ai-ws build`) to the AI workspace server resolved from the CLI config (`--display-name`/`--platform`, or the active AI workspace). The resource `id` in the request URL is taken from the payload's `id` field, and credentials come from the configured auth type (see [Authentication](#authentication)).

### `ap ai-ws llm-provider push`

Creates/updates an LLM provider with `PUT /api-proxy/api/v1/llm-providers/{id}?organizationId={org}`. The JSON file is sent as the request body unchanged.

```shell
ap ai-ws llm-provider push -f <payload.json> --org <org-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap ai-ws llm-provider push -f build/wso2-claude.json --org 99089a17-72e0-4dd8-a2f4-c8dfbb085295
ap ai-ws llm-provider push -f build/wso2-claude.json --org <org-id> --display-name my-workspace --platform eu
```

### `ap ai-ws llm-proxy push`

Creates an LLM proxy with `POST /api-proxy/api/v1/llm-proxies/{id}?organizationId={org}`. The supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-ws llm-proxy push -f <payload.json> --org <org-id> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap ai-ws llm-proxy push -f build/wso2-openai-proxy.json --org <org-id> --project-id 550e8400-e29b-41d4-a716-446655440000
```

### `ap ai-ws mcp-proxy push`

Creates an MCP proxy with `POST /api-proxy/api/v1/mcp-proxies?organizationId={org}`. Like the LLM proxy, the supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-ws mcp-proxy push -f <payload.json> --org <org-id> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap ai-ws mcp-proxy push -f build/bijira-mcp-everything.json --org <org-id> --project-id 019ecf0e-8237-7153-96d5-bb3934e2c313
```

Notes:

- `--file` and `--org` are required for all push commands; `--project-id` is also required for LLM proxies and MCP proxies.
- By default a concise summary line is printed, including the artifact `id` (the value other commands need for `--id`). Pass `--output json` (or `-o json`) to print the full server response instead (useful for piping to `jq`).
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Edit Commands

These commands update an existing artifact on the AI workspace by sending its payload JSON with a `PUT` request to the id-scoped resource path (`/{resource}/{id}?organizationId={org}`). The resource `id` is taken from the payload's `id` field, and the AI workspace and credentials are resolved exactly like the push commands.

### `ap ai-ws llm-provider edit`

Updates an existing LLM provider with `PUT /api-proxy/api/v1/llm-providers/{id}?organizationId={org}`. The JSON file is sent as the request body unchanged.

```shell
ap ai-ws llm-provider edit -f <payload.json> --org <org-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-ws llm-proxy edit`

Updates an existing LLM proxy with `PUT /api-proxy/api/v1/llm-proxies/{id}?organizationId={org}`. The supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-ws llm-proxy edit -f <payload.json> --org <org-id> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-ws mcp-proxy edit`

Updates an existing MCP proxy with `PUT /api-proxy/api/v1/mcp-proxies/{id}?organizationId={org}`. Like the LLM proxy, the supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-ws mcp-proxy edit -f <payload.json> --org <org-id> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Notes:

- `--file` and `--org` are required for all edit commands; `--project-id` is also required for LLM proxies and MCP proxies.
- The payload must contain the `id` of the artifact to update; it identifies the resource in the request URL.
- By default a concise summary line is printed, including the artifact `id` (the value other commands need for `--id`). Pass `--output json` (or `-o json`) to print the full server response instead (useful for piping to `jq`).
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Related Commands

- `ap platform add`
- `ap platform use`
- `ap ai-ws use`
- `ap ai-ws build`
- `ap project init`
