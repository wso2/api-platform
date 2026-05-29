# Managing API Workflows

As a portal admin, you can create, edit, publish, and control the visibility of API Workflows. Workflows are multi-step, Arazzo-format sequences of API calls that developers and AI agents can follow to accomplish a specific goal.

## Creating a Workflow

1. In the Developer Portal, navigate to **Admin Settings** (`/<orgName>/views/<viewName>/admin/settings`).
2. Open the **API Workflows** section.
3. Click **Create**.
4. Fill in the workflow details:

| Field | Description |
|---|---|
| **Name** | A short, task-oriented name (e.g., "Place an Order", "Register a Webhook") |
| **Description** | One to two sentences explaining the goal of the workflow and when to use it |
| **Arazzo Specification** | The machine-readable workflow definition in [Arazzo format](https://spec.openapis.org/arazzo/latest.html) |
| **Agent Prompt** | Natural language guidance for AI agents on when and how to invoke the workflow |
| **Agent Visibility** | Controls whether the workflow is surfaced to AI agents. Set to **Visible** to include it in `llms.txt` and `api-workflows.md`, or **Hidden** to exclude it from agent-facing surfaces while keeping it accessible to human users in the portal |

5. Click **Save as Draft** to save without publishing, or **Publish** to make the workflow live immediately.

## Editing a Workflow

1. Navigate to **API Workflows** in the admin settings.
2. Select the workflow you want to edit.
3. Make your changes in the editor.
4. Click **Save** to update a published workflow, or **Save as Draft** to unpublish it while editing.

## Publishing and Unpublishing

Workflows must be explicitly published to become visible to consumers. Draft workflows are only visible within the admin console.

**To publish:**
1. Open the workflow in the admin settings.
2. Click **Publish**.

**To unpublish:**
1. Open the workflow.
2. Click **Unpublish** (or **Save as Draft**). The workflow is removed from the portal and from `llms.txt` / `api-workflows.md` immediately.

## Controlling Visibility

Visibility is controlled independently for human users and AI agents.

### Portal UI Visibility

Determines whether the workflow appears in the Developer Portal for human users.

- **Visible** — appears in the portal's API Workflows section and in search results
- **Hidden** — not shown in the portal UI; still accessible via direct URL

### Agent Visibility

Determines whether the workflow is exposed to AI agents via machine-readable surfaces.

- **Visible** — included in `llms.txt` and `api-workflows.md`
- **Hidden** — excluded from agent-facing surfaces; human users can still see it in the portal UI

This allows workflows to be tailored for specific audiences — for example, exposing a workflow to agents before it is ready for the developer-facing portal, or vice versa.

## Related

- [Consuming API Workflows](../discover-apis/api-workflows.md) — how developers and agents discover and use workflows
- [LLM Instructions](llm-instructions.md) — configure high-level guidance for AI agents
- [AI Agent Discovery](../discover-apis/ai-agent-discovery.md) — how agents navigate the portal
