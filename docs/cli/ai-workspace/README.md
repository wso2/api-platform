# AI-Workspace CLI Reference

This guide covers the AI-Workspace commands currently implemented under `cli/src/cmd/aiws`.

Available command group:

- `ap ai-workspace`

The `add`, `list`, `remove`, `use`, and `current` commands manage AI-Workspace **server connections** stored in the CLI config file (the same per-platform config used by `ap gateway` and `ap devportal`). The `build` command is different: it works inside an **API project** and generates an LLM proxy creation payload from the project's artifacts.

## Prerequisites

- Add at least one AI-Workspace configuration before using commands that target a specific workspace connection.
- AI-Workspace connections are stored in the CLI config file managed by `ap ai-workspace add`.
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

### `ap ai-workspace add`

Adds an AI-Workspace configuration to the CLI config file.

```shell
ap ai-workspace add --display-name <name> --server <server-url> --auth <basic|oauth|api-key> [--platform <platform>] [--no-interactive]
```

Examples:

```shell
ap ai-workspace add
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth basic
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth oauth
ap ai-workspace add --display-name my-workspace --server https://ai-workspace.example.com --auth api-key
ap ai-workspace add --display-name my-workspace --platform eu --server https://ai-workspace.example.com --auth api-key --no-interactive
```

Notes:

- Interactive mode prompts for missing values.
- Supplying credentials as flags is supported, but interactive mode or environment variables are preferred.
- If credentials are omitted, runtime commands expect the corresponding environment variables.

### `ap ai-workspace list`

Lists AI-Workspace configurations for a platform.

```shell
ap ai-workspace list [--platform <platform>]
```

Examples:

```shell
ap ai-workspace list
ap ai-workspace list --platform eu
```

Notes:

- If `--platform` is omitted, the current platform is used.
- The active AI-Workspace is marked in the output table.

### `ap ai-workspace remove`

Removes an AI-Workspace configuration from a platform.

```shell
ap ai-workspace remove --display-name <name> [--platform <platform>]
```

Example:

```shell
ap ai-workspace remove --display-name my-workspace
```

### `ap ai-workspace use`

Sets the active AI-Workspace for a platform.

```shell
ap ai-workspace use --display-name <name> [--platform <platform>]
```

Examples:

```shell
ap ai-workspace use --display-name my-workspace
ap ai-workspace use --display-name my-workspace --platform eu
```

Notes:

- If `--platform` is omitted, the current platform is used.
- The command reports whether credentials will come from environment variables or the stored config.

### `ap ai-workspace current`

Shows the active AI-Workspace for a platform.

```shell
ap ai-workspace current [--platform <platform>]
```

Example:

```shell
ap ai-workspace current
```

## `ap ai-workspace build`

Generates an **LLM proxy**, **LLM provider**, or **MCP proxy** creation payload (JSON) from an API project's artifacts. The payload shape is selected by the `kind` declared in `metadata.yaml`/`runtime.yaml`. In `metadata.yaml` the kind carries a `Metadata` suffix (the ai-workspace metadata kind); `runtime.yaml` uses the bare kind:

- `kind: LlmProxyMetadata` (metadata.yaml) / `LlmProxy` (runtime.yaml) → matches the `POST /llm-proxies` request body (`LLMProxy` schema).
- `kind: LlmProviderMetadata` (metadata.yaml) / `LlmProvider` (runtime.yaml) → matches the `POST /llm-providers` request body (`LLMProvider` schema).
- `kind: McpMetadata` (metadata.yaml) / `Mcp` (runtime.yaml) → MCP proxy creation payload (capabilities + policies).

Any other kind is rejected. The `Metadata` suffix is stripped before the metadata and runtime kinds are matched, so the two files still refer to the same artifact (e.g. `LlmProxyMetadata` in metadata.yaml matches `LlmProxy` in runtime.yaml).

```shell
ap ai-workspace build [-f <project-directory>] [-o <file.json | directory>]
```

Examples:

```shell
# Build using the current directory as the project root
ap ai-workspace build

# Build from a specific project directory
ap ai-workspace build -f /path/to/project

# Write the payload to a specific directory
ap ai-workspace build -o build/

# Write the payload to a specific file
ap ai-workspace build -o build/openai.json
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
      definition: ./definition.yaml   # OpenAPI spec, required for all kinds
```

For each configured entry, the build:

- Resolves `metadata`, `runtime`, and `definition` relative to that entry's `portalRoot` (defaults: `./metadata.yaml`, `./runtime.yaml`, `./definition.yaml`; `portalRoot` defaults to `.`, the project root).
- Requires `metadata.yaml` and `runtime.yaml` to exist.
- Requires the `kind` declared in `metadata.yaml` and `runtime.yaml` to match once the metadata's `Metadata` suffix is stripped (e.g. `LlmProxyMetadata` vs `LlmProxy`); otherwise the build fails with a kind-mismatch error.
- Requires `metadata.name` to match between `metadata.yaml` and `runtime.yaml`; otherwise the build fails with a name-mismatch error.
- Requires `definition.yaml` (the OpenAPI spec) for every kind — see [The OpenAPI spec](#the-openapi-spec) below.
- If no `ai-workspaces` section exists, a single `default` entry (`portalRoot: .`) is created in the project config and used.

All resolved paths are constrained to the project directory; a path that escapes the project root fails the build for that entry.

#### Associating gateways (`metadata.yaml`)

Optionally list the gateways the artifact can be deployed to, with per-gateway configuration overrides, in an `associatedGateways` section **under `spec`** in `metadata.yaml`. This applies to all artifact kinds (`LlmProxyMetadata`, `LlmProviderMetadata`, `McpMetadata`). Each entry is keyed by the gateway `id`. The build extracts this list from `spec.associatedGateways` and copies it into the generated payload verbatim (entries without an `id` are dropped; the field is omitted entirely when absent):

```yaml
# metadata.yaml
kind: LlmProviderMetadata
metadata:
  name: wso2-claude-provider
spec:
  displayName: wso2 claude provider
  version: v1.0
  associatedGateways:
    - id: default
      configurations:
        host: prod-gw.example.com
```

`configurations` is a free-form object — the supported keys depend on the artifact type.

### What it generates

One JSON file per configured AI-Workspace entry, written to the build output.

#### `LlmProxy`

| Payload field | Source |
| --- | --- |
| `id` | `metadata.yaml` → `metadata.name` |
| `displayName` | `metadata.yaml` → `spec.displayName` |
| `version` | `metadata.yaml` → `spec.version` |
| `context` | `runtime.yaml` → `spec.context` |
| `description` | `runtime.yaml` → `spec.description` (defaults to `"No description provided for this proxy."` when absent) |
| `provider` (`id`, `auth.{type,header}`) | `runtime.yaml` → `spec.provider` (the auth secret `value` is **not** copied — the provider owns it) |
| `security` (`enabled`, `apiKey.{enabled,in,key}`) | the `api-key-auth` policy in `runtime.yaml` → `spec.globalPolicies` |
| `globalPolicies[]` (`name`, `version`, `params`) | every other `runtime.yaml` → `spec.globalPolicies` entry; `params` is copied verbatim (policy-specific, no fixed schema) |
| `operationPolicies[]` (`name`, `version`, `paths[].{path,methods,params}`) | `runtime.yaml` → `spec.operationPolicies`; each path's `params` is copied verbatim |
| `readOnly` | always `false` |
| `openapi` | content of `definition.yaml` (**required**) |
| `associatedGateways[]` (`id`, `configurations`) | `metadata.yaml` → `spec.associatedGateways` (omitted when absent) |
| `projectId` | intentionally omitted (injected by `push`/`edit` via `--project-id`) |

#### `LlmProvider`

| Payload field | Source |
| --- | --- |
| `id` | `metadata.yaml` → `metadata.name` |
| `name` | `metadata.yaml` → `metadata.name` |
| `version` | `metadata.yaml` → `spec.version` |
| `context` | `runtime.yaml` → `spec.context` |
| `template` | `runtime.yaml` → `spec.template` |
| `modelProviders[]` (`id`, `displayName`, `models[].{id,displayName}`) | derived from `spec.template` (see below); omitted for an unknown template |
| `upstream` (`main.{url,auth}`) | `runtime.yaml` → `spec.upstream` |
| `accessControl` (`mode`, `exceptions[]`) | `runtime.yaml` → `spec.accessControl` |
| `security` (`apiKey.{key,in}`) | the `api-key-auth` policy in `runtime.yaml` → `spec.policies` |
| `rateLimiting` | the `*-ratelimit` policies (see below) |
| `policies[]` (`name`, `version`, `paths[].{path,methods,params}`) | every other `runtime.yaml` → `spec.policies` entry (i.e. not `api-key-auth` or `*-ratelimit`) |
| `openapi` | content of `definition.yaml` (**required** for providers) |
| `associatedGateways[]` (`id`, `configurations`) | `metadata.yaml` → `spec.associatedGateways` (omitted when absent) |

**rateLimiting mapping.** Each policy whose name ends with `-ratelimit` becomes a rate-limiting dimension, selected by name:

| Policy name | Dimension | Value source (`params`) |
| --- | --- | --- |
| `advanced-ratelimit` | `request` | `quotas[].limits[].{limit,duration}` |
| `token-based-ratelimit` | `token` | `totalTokenLimits[].{count,duration}` |
| `llm-cost-based-ratelimit` | `cost` | `budgetLimits[].{amount,duration}` |

Each dimension is placed under `consumerLevel` when the policy params carry `consumerBased: true` (or, for `advanced-ratelimit`, when the quota `name` starts with `consumer`); otherwise under `providerLevel`. Durations like `1h`/`3h` are parsed into `{duration, unit}` reset windows.

A limit whose path is `/*` is applied as a `global` limit for its scope; a limit on a specific path is applied `resourceWise`, keyed by that path (with any `/*` limits in the same scope folded into the resourceWise `default`).

**modelProviders mapping.** When `spec.template` matches a known template, the build emits a single `modelProviders` entry keyed by the template name (`id` = `displayName` = the template), carrying that template's models (each model's `id` and `displayName` are the model identifier). Unknown templates emit no `modelProviders`. Supported templates and their models:

| Template | Models |
| --- | --- |
| `meta` | `us.meta.llama3-3-70b-instruct-v1:0`, `us.meta.llama4-maverick-17b-instruct-v1:0` |
| `openai` | `gpt-4o-mini`, `gpt-4.1-mini`, `o4-mini` |
| `anthropic` | `claude-3.5-sonnet`, `claude-3-opus` |
| `google-vertex` | `gemini-1.5-pro`, `gemini-1.5-flash` |
| `aws-bedrock` | `amazon.titan-text-premier`, `anthropic.claude-v2` |
| `mistralai` | `mistral-large-latest`, `mistral-small-latest`, `open-mixtral-8x22b` |

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
| `associatedGateways[]` (`id`, `configurations`) | `metadata.yaml` → `spec.associatedGateways` (omitted when absent) |
| `description` | empty |

`definition.yaml` for an MCP proxy holds `prompts`, `resources`, and `tools`. `prompts` and `tools` are passed through unchanged; `resources` are trimmed to `uri`, `name`, and `mimeType` (any inline `text`/`blob` content is dropped). `projectId` is omitted and injected at publish time.

### The OpenAPI spec

`definition.yaml` is **required for every kind** — the build errors if it is missing:

- **`LlmProvider`** — folded into `openapi`.
- **`LlmProxy`** — folded into `openapi`.
- **`Mcp`** — its `prompts`/`resources`/`tools` populate `capabilities`.

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

- **LLM providers** need no scoping parameter — the organization is derived from the auth token (`GET /llm-providers`, `GET /llm-providers/{id}`).
- **LLM/MCP proxies** are scoped by `projectId` (`--project-id`) when listing; fetching a single proxy by `--id` takes only the id path parameter (no org/project query).

### `ap ai-workspace llm-provider list`

Lists all LLM providers (`GET /llm-providers`, operationId `listLLMProviders`). The organization comes from the auth token, so no `--org` is needed.

```shell
ap ai-workspace llm-provider list [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace llm-provider get`

The organization comes from the auth token, so no `--org` is needed.

```shell
# List all LLM providers (GET /llm-providers)
ap ai-workspace llm-provider get [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]

# Get a single LLM provider (GET /llm-providers/{id})
ap ai-workspace llm-provider get --id <provider-id>
```

### `ap ai-workspace app-llm-proxy list`

Lists all LLM proxies in a project (`GET /llm-proxies?projectId={project}`, operationId `listLLMProxies`). `--project-id` is required.

```shell
ap ai-workspace app-llm-proxy list --project-id <project-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace app-llm-proxy get`

```shell
# List all LLM proxies in a project (GET /llm-proxies?projectId={project})
ap ai-workspace app-llm-proxy get --project-id <project-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]

# Get a single LLM proxy (GET /llm-proxies/{id})
ap ai-workspace app-llm-proxy get --id <proxy-id>
```

### `ap ai-workspace mcp-proxy list`

Lists all MCP proxies in a project (`GET /mcp-proxies?projectId={project}`, operationId `listMCPProxies`). `--project-id` is required.

```shell
ap ai-workspace mcp-proxy list --project-id <project-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace mcp-proxy get`

```shell
# List all MCP proxies in a project (GET /mcp-proxies?projectId={project})
ap ai-workspace mcp-proxy get --project-id <project-id> [--limit <n>] [--offset <n>] [--display-name <name>] [--platform <platform>] [--insecure]

# Get a single MCP proxy (GET /mcp-proxies/{id})
ap ai-workspace mcp-proxy get --id <proxy-id>
```

Notes:

- `llm-provider list` and `llm-provider get` both list providers when no `--id` is given; `list` is the dedicated list-all command, while `get` additionally fetches a single provider with `--id`. Neither needs `--org` (the organization is derived from the auth token).
- `llm-proxy`/`mcp-proxy` each have a dedicated `list` command (project-scoped, `--project-id` required) alongside `get`, which lists when no `--id` is given and fetches a single proxy with `--id`.
- For `llm-proxy`/`mcp-proxy get`, `--project-id` is required only when listing; fetching a single proxy needs just `--id`.
- `--limit` and `--offset` apply only when listing.
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Delete Commands

These commands delete an artifact by its identifier (`DELETE /{resource}/{id}`). The artifact is identified solely by `--id` — no organization or project scoping is required — and a successful delete (`204 No Content`) prints a confirmation line.

### `ap ai-workspace llm-provider delete`

```shell
ap ai-workspace llm-provider delete --id <provider-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace app-llm-proxy delete`

```shell
ap ai-workspace app-llm-proxy delete --id <proxy-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace mcp-proxy delete`

```shell
ap ai-workspace mcp-proxy delete --id <proxy-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Notes:

- `--id` is required for all delete commands.
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Push Commands

These commands push a payload JSON (produced by `ap ai-workspace build`) to the AI workspace server resolved from the CLI config (`--display-name`/`--platform`, or the active AI workspace). The organization is derived from the auth token, so **no `--org` flag is needed**. Credentials come from the configured auth type (see [Authentication](#authentication)).

### `ap ai-workspace llm-provider push`

Creates an LLM provider with `POST /api/v0.9/llm-providers` (operationId `createLLMProvider`). The JSON file is sent as the request body unchanged.

```shell
ap ai-workspace llm-provider push -f <payload.json> [--display-name <name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap ai-workspace llm-provider push -f build/wso2-claude.json
ap ai-workspace llm-provider push -f build/wso2-claude.json --display-name my-workspace --platform eu
```

### `ap ai-workspace app-llm-proxy push`

Creates an LLM proxy with `POST /api/v0.9/llm-proxies` (operationId `createLLMProxy`). The supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-workspace app-llm-proxy push -f <payload.json> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap ai-workspace app-llm-proxy push -f build/wso2-openai-proxy.json --project-id 550e8400-e29b-41d4-a716-446655440000
```

### `ap ai-workspace mcp-proxy push`

Creates an MCP proxy with `POST /api/v0.9/mcp-proxies` (operationId `createMCPProxy`). Like the LLM proxy, the supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-workspace mcp-proxy push -f <payload.json> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Examples:

```shell
ap ai-workspace mcp-proxy push -f build/bijira-mcp-everything.json --project-id 019ecf0e-8237-7153-96d5-bb3934e2c313
```

Notes:

- `--file` is required for all push commands; `--project-id` is also required for LLM proxies and MCP proxies. The organization is derived from the auth token, so no `--org` flag is needed.
- By default a structured result is printed (like `ap gateway apply`): `Status`, `Message`, `ID`, and — when known — `Organization`, `Project`, `Created At`, `Updated At`, and `State`. `Project` shows the `--project-id` you supplied (proxies/MCP); `Organization` is derived from the auth token so it only appears when the server echoes `organizationId`. Pass `--output json` (or `-o json`) to print the full server response instead (useful for piping to `jq`).
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Edit Commands

These commands update an existing artifact on the AI workspace by sending its payload JSON with a `PUT` request to the id-scoped resource path (`/{resource}/{id}`). The resource `id` is taken from the payload's `id` field, the organization is derived from the auth token (**no `--org` flag**), and the AI workspace and credentials are resolved exactly like the push commands.

### `ap ai-workspace llm-provider edit`

Updates an existing LLM provider with `PUT /api/v0.9/llm-providers/{id}` (operationId `updateLLMProvider`). The JSON file is sent as the request body unchanged.

```shell
ap ai-workspace llm-provider edit -f <payload.json> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace app-llm-proxy edit`

Updates an existing LLM proxy with `PUT /api/v0.9/llm-proxies/{id}` (operationId `updateLLMProxy`). The supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-workspace app-llm-proxy edit -f <payload.json> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

### `ap ai-workspace mcp-proxy edit`

Updates an existing MCP proxy with `PUT /api/v0.9/mcp-proxies/{id}` (operationId `updateMCPProxy`). Like the LLM proxy, the supplied `--project-id` is injected into the payload as `projectId` before it is sent.

```shell
ap ai-workspace mcp-proxy edit -f <payload.json> --project-id <project-id> [--display-name <name>] [--platform <platform>] [--insecure]
```

Notes:

- `--file` is required for all edit commands; `--project-id` is also required for LLM proxies and MCP proxies. The organization is derived from the auth token, so no `--org` flag is needed.
- The payload must contain the `id` of the artifact to update; it identifies the resource in the request URL.
- By default a structured result is printed (like `ap gateway apply`): `Status`, `Message`, `ID`, and — when known — `Organization`, `Project`, `Created At`, `Updated At`, and `State`. `Project` shows the `--project-id` you supplied (proxies/MCP); `Organization` is derived from the auth token so it only appears when the server echoes `organizationId`. Pass `--output json` (or `-o json`) to print the full server response instead (useful for piping to `jq`).
- `--insecure` skips TLS verification for local or self-signed HTTPS endpoints.

## Related Commands

- `ap platform add`
- `ap platform use`
- `ap ai-workspace use`
- `ap ai-workspace build`
- `ap project init`
