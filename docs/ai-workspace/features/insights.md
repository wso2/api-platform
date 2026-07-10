# Insights

The Insights page links to your [Moesif](https://www.moesif.com) analytics workspace, where you can view usage trends, request activity, latency, and AI consumer behavior for traffic flowing through your gateways.

## How It Works

1. **Configure your gateway** with a Moesif Application ID (see below). The gateway runtime publishes telemetry — requests, tokens, latency, guardrail events — directly to Moesif.
2. **Open Insights** in the AI Workspace sidebar. The page shows a button that opens your Moesif workspace (`https://www.moesif.com/wrap/basic`) in a new tab.
3. **View analytics** in Moesif — all data is there, not in the AI Workspace itself.

The AI Workspace does not embed or proxy Moesif content. It simply provides the link.

## Enabling Moesif on a Gateway

Set the `MOESIF_KEY` environment variable on your gateway runtime deployment.

Example (Docker Compose):

```yaml
environment:
  MOESIF_KEY: "your-moesif-app-id"
```

Once set, the gateway publishes events to Moesif automatically. No changes to the AI Workspace or Platform API are needed.

## What Moesif Tracks

With the gateway Moesif integration active, your Moesif workspace shows:

- Request and response traffic (volume, latency, error rates)
- Token usage by model and provider
- Estimated LLM cost
- Guardrail policy triggers
- Per-application and per-consumer breakdowns

Filtering and dashboarding is done entirely within Moesif.
