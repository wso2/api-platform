# LLM Provider Templates

An LLM Provider Template is a reusable blueprint for connecting to an upstream LLM service. It captures the endpoint URL, inbound authentication, the provider's OpenAPI specification, and the token/model mappings. So, an [LLM Provider](llm-providers.md) can be created from it without re-entering the same configuration.

## Template Types

| Type | Description |
|------|-------------|
| **Built-in** | Shipped with the product (OpenAI, Azure OpenAI, Azure AI Foundry, AWS Bedrock, Anthropic, Mistral, Gemini). Read-only — can only be enabled or disabled. |
| **Custom** | Created by you, from scratch or by creating a new version of a built-in template. Editable and deletable. |

## Viewing Templates

Navigate to **Settings → LLM Provider Templates**. The listing shows custom templates and built-in templates in separate sections, each showing the latest version of every template. Disabled templates are shown dimmed.

## Creating a Custom Template

1. Navigate to **Settings → LLM Provider Templates**.
2. Click **Create**.
3. Enter relevant details including **name** and the upstream **endpoint URL**.
4. Click **Create**.

The new template starts at version **v1.0**. Open it to complete the configuration.

> **Note:** A custom template must also be deployed to the gateway before providers created from it will work — see [Deploying a Custom Template](#deploying-a-custom-template).

## Configuring a Template

The template page shows an overview, a version selector, and the following tabs:

### Overview

Shows the template's logo, description, current version, and last-updated time. From here you can:

- **Download YAML** — export the current version as a gateway-ready manifest.
- **Enable / Disable** the current version (built-in templates only).
- **Delete** the current version (custom templates only).

### Connection

- **Endpoint URL** — the upstream base URL.
- **OpenAPI specification** — supply by **URL** (click **Fetch** to load it) or by **upload**.
- **Authentication** — the inbound auth type, header/parameter name, and value prefix.

### Token Mapping

- **Default (Global)** mappings — prompt, completion, total, and remaining tokens; request and response model.
- **Per-resource overrides** — override the mappings for individual API resources.

## Versioning

Template versions are immutable — instead of editing a published version, you create a new one, so providers built from earlier versions stay stable.

To create a new version:

1. Open the template and click the **version selector** (e.g. **v1.0**).
2. Click **Create new version**.
3. Enter the new version (e.g. `v2.0`) and adjust the configuration as needed.
4. Click **Create**.

Creating a new version of a **built-in** template produces a **custom** version.

## Deploying a Custom Template

Unlike built-in templates, which are already available on the gateway, a custom template must be deployed to the gateway manually:

1. Open the template's **Overview** and click **Download YAML**.
2. Apply the downloaded manifest to the target gateway.

A provider created from a custom template will only work as expected after the template has been deployed to the gateway serving that provider.

## Using a Template to Create a Provider

When adding an [LLM Provider](llm-providers.md):

1. Pick a template from the picker.
2. If the template has more than one version, select one and click **Continue**. A single version is selected automatically.
3. Enter the provider name and credentials — the provider copies the chosen version's endpoint, auth, and token mappings.

## Managing a Template

- **Edit** — update the configuration (custom templates only).
- **Enable / Disable** — turn a built-in template version on or off. This is supported for **built-in templates only**; disabling is blocked while a provider uses that version. Custom templates are removed by deleting them instead.
- **Delete** — remove a custom template version. Deletion is blocked while a provider was created from that version; deleting the last remaining version removes the whole template.

## Related

- [LLM Providers](llm-providers.md) — create a runnable provider from a template.
- [LLM Proxies](llm-proxies.md) — application-facing endpoints on top of a provider.
- [Secrets Management](secrets-management.md) — how upstream credentials are stored and referenced.
