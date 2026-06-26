# LLM Proxies

An LLM Proxy is an optional application-facing endpoint layered on top of an LLM Provider. Each proxy has its own API key, allowing you to isolate consumers (teams, apps, agents) from one another and apply per-proxy controls — guardrails, rate limits, authentication settings — without affecting other consumers of the same provider.

## Why Use Proxies?

Without proxies, all consumers share the same provider endpoint and credentials. With proxies you can:

- Give each app or team its own API key and revoke it independently.
- Apply different guardrails or rate limits to different consumers.
- Track usage and cost per proxy (and per application) separately.
- Expose only a subset of provider models to a particular consumer.

## Creating a Proxy

1. Navigate to **LLM Proxies** in the left sidebar.
2. Click **New Proxy**.
3. Select the LLM Provider the proxy should sit in front of.
4. Enter a name and optional description.
5. Click **Create**.

## Configuring a Proxy

After creation, the proxy detail page exposes:

### Overview

Displays the proxy endpoint URL, the API key (generated on creation, shown once), and connection details for supported SDKs.

### Security

Configure inbound authentication:

- **API key location** — header or query parameter
- **Header / parameter name** — custom name (default: `api-key`)

API keys for proxies expire after **90 days**. New keys can be generated at any time from the overview tab.

### Guardrails

Attach guardrail policies specific to this proxy. Guardrails set on the parent provider also apply; proxy-level guardrails are additive.

### Rate Limiting

Set token-based, cost-based, or basic rate limits scoped to this proxy. Applied after any provider-level limits.

### Resources

Control which models from the parent provider are exposed through this proxy. Restricts the model list to a subset of what the provider makes available.

## Deploying a Proxy

1. Open the proxy and click **Deploy**.
2. Select one or more target gateways.
3. Click **Deploy**.

The parent provider must already be deployed to the same gateway(s).

## Connecting an Application

Once deployed, share the proxy's endpoint URL and API key with the consuming application. The endpoint is OpenAI-compatible — applications using the OpenAI SDK can point their `base_url` at the proxy endpoint.

Example (Python):

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://<gateway-host>/v1",
    api_key="<proxy-api-key>",
)
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}],
)
```

## Managing a Proxy

- **Edit** — update name, description, security settings, guardrails, or rate limits.
- **Rotate API key** — invalidates the old key and generates a new one.
- **Undeploy** — removes the proxy from a gateway without deleting the proxy record.
- **Delete** — permanently deletes the proxy and removes it from all gateways.
