# LLM Provider Templates

An LLM Provider Template is a reusable blueprint for connecting to an upstream LLM service. A template captures everything needed to talk to a provider — the endpoint URL, inbound authentication, the OpenAPI specification of the provider's API, and the token/model mappings used for usage tracking — so that an actual [LLM Provider](llm-providers.md) can be created from it without re-entering the same configuration each time.

Templates come in two types:

| Type | `managedBy` | Description |
|------|-------------|-------|
| **Built-in** | `wso2` | Shipped with the product and seeded automatically (OpenAI, Azure OpenAI, Azure AI Foundry, AWS Bedrock, Anthropic, Mistral, Gemini, …). Read-only — cannot be edited or deleted, but can be disabled. |
| **Custom** | `customer` (default) | Created by you, either from scratch or by branching a new version off a built-in template. Fully editable and deletable. |

## How Templates Are Identified

Each template carries three identifiers. Understanding them explains how the listing and versioning behave:

| Field | Meaning |
|-------|---------|
| `id` | Unique identifier of **one specific version** (e.g. `openai`, `deep-seek-test66-v-2-0`). |
| `groupId` | Stable identifier shared by **every version of the same family**. Family-wide reads (listing all versions, fetching a version by number) are keyed by this; edit/enable/delete act on a specific `id` (handle). |
| `version` | The semantic version of that entry, `vMAJOR.MINOR` (e.g. `v1.0`, `v2.0`). |


## Listing

Open **LLM Provider Templates** from **Settings**. The listing shows two sections:

- **LLM Provider Templates** (custom) — templates you created.
- **Built-in Templates** — the WSO2-shipped templates.

By default the listing shows the **latest version of each family, per kind**. This means a family that began as a built-in template and was later branched into a custom version appears in **both** sections — the original built-in version stays under *Built-in Templates*, and the custom line (its newer versions) appears under *LLM Provider Templates*.

Disabled templates are shown dimmed. Card names and descriptions are truncated so cards stay uniform.

## Creating a Custom Template

1. Navigate to **Settings → LLM Provider Templates**.
2. Click **Create**.
3. Enter a **name** and (optionally) a description.
4. Provide the connection details on the **Connection** tab — the upstream **endpoint URL**, the inbound **auth** settings, and the provider's **OpenAPI specification** (by URL or by upload).
5. Set the **token and model mappings** on the **Token Mapping** tab.
6. Click **Save**.

A newly created custom template starts at version **v1.0** with `managedBy = customer`.

## Configuring a Template

Opening a template shows its overview, a version selector, and the following tabs.

### Overview

Shows the template's logo (fetched from the configured **logo URL**), description, current version, and **Last updated** time. From here you can:

- **Download YAML** — export the template as a gateway-ready manifest.
- **Enable / Disable** the current version.
- **Delete** the current version (custom only). There is no separate "delete entire template" action — deletion is always per version, and deleting a template's last remaining version removes the whole family. See [Lifecycle Rules](#lifecycle-rules).

> Built-in templates show a reduced overview: the spec upload/URL section and the per-resource overrides are hidden, since built-in templates are read-only.

### Connection

Defines how the gateway reaches the upstream service:

- **Endpoint URL** — the upstream base URL.
- **OpenAPI specification** — supply it by **URL** (then click **Fetch** to load it) or by **upload**. The URL and the upload are mutually exclusive; the loaded spec is rendered here and on the overview.
- **Authentication** — the inbound auth type, header/parameter name, and value prefix.

### Token Mapping

Configures how token usage and model identifiers are extracted from provider requests and responses:

- **Default (Global)** mappings — prompt tokens, completion tokens, total tokens, remaining tokens, request model, and response model.
- **Per-resource overrides** — override the token/model mappings for individual API resources when a provider differs per endpoint.

## Versioning

Templates are **immutable per version** — editing a published configuration is done by creating a new version, not by mutating the existing one. This keeps any provider already built from an earlier version stable.

- Versions use the `vMAJOR.MINOR` format and the version is **mandatory** on creation.
- Use the **version selector** on the template page to switch between versions. The dropdown lists versions in **descending order** and selects the **latest** by default.
- **Create new version** branches a new version from the current family. The newest version becomes `isLatest`; the previous latest is demoted.
- Branching a new version off a **built-in** template produces a **custom** version (`managedBy = customer`) under the same family — this is how a built-in becomes customizable without altering the read-only original.

## Lifecycle Rules

Enable/disable and deletion follow these rules:

| Action | Built-in | Custom |
|--------|----------|--------|
| Edit | ✗ (read-only) | ✓ |
| Enable | ✓ | ✓ |
| Disable | ✓ | ✓, unless a provider was created from that version |
| Delete a version | ✗ | ✓, unless a provider was created from that version |


Additional constraints:

- **Disabling** a version is allowed only when no provider was created from that version.
- **Deleting** a version is blocked while any provider depends on it. For example, if a provider was created from `v1.0`, you may delete `v2.0` but not `v1.0`.
- A version is never silently removed while in use — the API returns `409 Conflict`.

## Using a Template to Create a Provider

When adding an [LLM Provider](llm-providers.md), you select a template and a version:

1. Pick a template from the picker.
2. If the template has **more than one version**, a version-selection dialog appears (versions listed newest-first). If only one version exists, it is selected automatically.
3. Confirm — the provider is created from that **specific version's** configuration: its endpoint, auth, and token mappings are copied from the chosen version.

## Exporting and Deploying

Use **Download YAML** on the overview to export the template as a manifest. The manifest carries the family's `groupId` and version, so a template exported from the AI Workspace behaves the same as a built-in template when applied to a gateway.

## API Reference

All endpoints are under `/api/v0.9/llm-provider-templates` and require a valid JWT; results are scoped to the caller's organization.

> **Identifier convention:** all routes are flat — there are **no `/versions/...` subpaths**. The `{id}` path parameter is always a **handle** (one specific version, e.g. `openai`, `deep-seek-test66-v-2-0`). Family-wide reads (listing or fetching a version by number) are done on the collection route with a `query` parameter carrying a `groupId`.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/llm-provider-templates` | Create a custom template (starts at v1.0). |
| POST | `/llm-provider-templates/copy` | Create a new version by copying an existing one (see [Create a New Version](#create-a-new-version)). |
| GET | `/llm-provider-templates` | List templates. Default: latest of each family. See [List Query Parameters](#list-query-parameters). |
| GET | `/llm-provider-templates/{id}` | Get one specific template version by handle (not "latest"). |
| PUT | `/llm-provider-templates/{id}` | Update the version identified by the handle (custom only). |
| PATCH | `/llm-provider-templates/{id}` | Enable or disable the version identified by the handle. |
| DELETE | `/llm-provider-templates/{id}` | Delete the version identified by the handle — blocked while a provider references it. Deleting the family's last version removes the family. |

### List Query Parameters

The `query` parameter carries `key:value` pairs joined by `&`. Supported keys: `groupId`, `version`, `latest`.

| Parameter | Default | Max | Description |
|-----------|---------|-----|-------------|
| `limit` | 20 | 100 | Maximum number of results. |
| `offset` | 0 | — | Number of results to skip. |
| `query=latest:true` | — | — | Return only the latest version of each family. |
| `query=groupId:{groupId}` | — | — | Return **all** versions of a single family. |
| `query=groupId:{groupId}&version:{version}` | — | — | Return one specific version of a family. |

---

## Related

- [LLM Providers](llm-providers.md) — create a runnable provider from a template.
- [LLM Proxies](llm-proxies.md) — application-facing endpoints on top of a provider.
- [Secrets Management](secrets-management.md) — how upstream credentials are stored and referenced.
