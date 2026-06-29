# LLM Providers

An LLM Provider represents a connection to an upstream LLM service. Provider credentials are stored in the Platform API and delivered to gateway runtimes over the control-plane connection — consumer applications never see the upstream API keys.

## Supported Providers

| Provider | Notes |
|----------|-------|
| OpenAI | Requires an OpenAI API key |
| Anthropic | Requires an Anthropic API key |
| Azure OpenAI | Requires endpoint URL + API key |
| Azure AI Foundry | Requires endpoint URL + API key |
| Google Gemini | Requires a Gemini API key |
| Mistral | Requires a Mistral API key |

## Creating a Provider

1. Navigate to **LLM Providers** in the left sidebar.
2. Click **Add Provider**.
3. Select the provider type.
4. Enter the required credentials (API key, endpoint URL where applicable).
5. Click **Save**.

The provider is stored but not yet active on any gateway. Continue to the **Deploy** step to make it available.

## Configuring a Provider

After creation, the provider detail page exposes several tabs:

### Connection

Update the API key or endpoint URL. Changes take effect on the next deployment to affected gateways.

### Models

Control which models from this provider are exposed to consumers. By default, all models available from the provider are accessible. You can restrict the list to specific model IDs.

### Security

Configure inbound authentication for consumers calling this provider through the gateway:

- **API key location** — header or query parameter
- **Header / parameter name** — custom name (default: `api-key`)

### Rate Limiting

Apply token-based, cost-based, or basic request-count rate limits at the provider level. Limits apply across all consumers of this provider.

### Guardrails

Attach guardrail policies (Semantic Prompt Guard, PII Masking, Azure Content Safety, etc.) to inspect or transform requests and responses at the provider level.

## Deploying a Provider

1. Open the provider and click **Deploy**.
2. Select one or more target gateways.
3. Click **Deploy** — the Platform API pushes the provider configuration to the selected gateways.

A provider must be deployed to at least one gateway before consumers can use it.

## Managing a Provider

- **Edit** — update credentials, model list, security, or policy settings.
- **Undeploy** — remove the provider configuration from a gateway without deleting the provider record.
- **Delete** — permanently removes the provider. Deployed instances are removed from all gateways.
