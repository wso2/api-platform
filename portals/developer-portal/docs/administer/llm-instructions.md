# LLM Instructions

LLM Instructions allow portal admins to provide AI agents with high-level context about the developer portal, what APIs are available, how the portal is organized, and any conventions or constraints agents should follow when interacting with it.

These instructions are published as part of the portal's `llms.txt` file, which is the primary entry point for AI agents discovering the portal's capabilities.

## What Are LLM Instructions?

LLM Instructions are free-form natural language text written by the portal admin. They appear at the top of `llms.txt` and are read by agents before they navigate the rest of the portal's content.

Use this space to tell agents:

- What the portal covers and what kind of APIs it exposes
- How APIs are organized (by product area, team, lifecycle stage, etc.)
- Any authentication conventions agents should be aware of
- Which workflows are the recommended starting points for common tasks
- Any limitations or usage policies agents should respect

Well-written LLM Instructions reduce agent errors and improve the quality of AI-assisted integrations by giving agents the orientation they need upfront, rather than leaving them to infer context from individual API specs.

## Configuring LLM Instructions

LLM Instructions are configured per view, from the organization's admin settings.

1. Sign in to the Developer Portal as an admin.
2. Navigate to **Settings** (`/<orgName>/settings`).
3. Select **LLM Instructions** from the sidebar.
4. If your organization has more than one view, the **View-scoped setting** banner at the top shows which view you're currently editing — use the pill selector to switch views. Each view has its own LLM Instructions.
5. Use the **Portal is AI-discoverable** toggle to control whether `llms.txt` and the agent-facing content endpoints are served for this view at all. When turned off, those endpoints return `404` to agents — turning it off doesn't just hide the setting, it takes the portal out of agent discovery entirely.
6. Enter a **Portal name** and **Description** to orient agents.
7. Click **Publish** — this is always available, even when AI-discoverable is turned off, so you can save that you've intentionally disabled discovery for this view.

Changes take effect immediately — the updated instructions are reflected in `llms.txt` as soon as you save.

## Previewing the Output

To verify how your instructions appear to agents, fetch the portal's `llms.txt` directly:

```bash
curl https://<host>/<orgName>/views/<viewName>/llms.txt
```

Your LLM Instructions will appear at the top of the file, followed by the portal's API and workflow index.

## Example Instructions

```
This portal exposes the Acme Commerce Platform APIs.

APIs are organized by domain:
- Orders: create and manage customer orders
- Inventory: product catalog and stock management
- Payments: payment processing and refunds
- Notifications: email and SMS delivery

All APIs use OAuth2 bearer token authentication. Obtain a token from
the /token endpoint using your consumer key and secret.

Recommended starting workflow: "Place an Order" — covers the end-to-end
flow from product lookup through order confirmation.

Rate limits apply per subscription plan. Use the Gold plan for
production workloads.
```

## Related

- [AI Agent Discovery](../discover-apis/ai-agent-discovery.md) — how agents navigate the portal
- [Manage API Workflows](managing-api-workflows.md) — publish workflows for agent consumption
