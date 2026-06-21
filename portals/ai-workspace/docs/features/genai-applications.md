# GenAI Applications

A GenAI Application represents an actual AI application or agent in your organization. Applications group multiple API keys under a named entity, making it easy to track token usage, request counts, and cost at the application level — independent of which LLM proxy or provider the keys are attached to.

## Concepts

- **Application** — a named group representing a real app or agent (e.g. "Customer Support Bot", "Code Review Agent").
- **API key** — a credential tied to an application. An application can have multiple API keys (e.g. one per deployment environment).
- **Usage tracking** — requests made with any of the application's API keys are aggregated under that application in the Insights dashboard.

## Creating an Application

1. Navigate to **Applications** in the left sidebar.
2. Click **New Application**.
3. Enter a name and optional description.
4. Click **Create**.

## Generating API Keys

1. Open the application.
2. Under the **API Keys** tab, click **Generate Key**.
3. Copy the key immediately — it is only shown once.

API keys expire after **90 days**. Generate a new key before the current one expires to avoid service interruption.

## Associating Keys with Proxies

When configuring an LLM Proxy (or provider) in the gateway, you can configure it to validate API keys against the application registry. Requests carrying a key that belongs to an application are attributed to that application in usage metrics.

## Viewing Application Usage

Navigate to **Insights** and filter by application to see:

- Total requests
- Token consumption (input + output tokens)
- Estimated cost
- Error rates
- Guardrail trigger counts

## Managing an Application

- **Edit** — update name or description.
- **Revoke key** — invalidate a specific API key without affecting other keys.
- **Delete** — permanently removes the application and all its API keys. Usage history is retained in the analytics backend.
