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

A workflow has two independent controls:

- **Status** (`Draft` / `Published`) — whether the workflow exists anywhere outside the admin console. Draft workflows are not shown in the portal UI or any agent-facing surface.
- **Agent Visibility** (`Visible` / `Hidden`) — for a published workflow, whether it's additionally exposed to AI agents via `llms.txt` and `api-workflows.md`. A published workflow always appears in the portal's API Workflows section for human users regardless of this setting; setting it to **Hidden** only excludes it from agent-facing surfaces.

This lets you publish a workflow for human users in the portal before deciding it's ready to be surfaced to AI agents.

## Related

- [Consuming API Workflows](../discover-apis/api-workflows.md) — how developers and agents discover and use workflows
- [LLM Instructions](llm-instructions.md) — configure the portal identity shown to AI agents
- [AI Agent Discovery](../discover-apis/ai-agent-discovery.md) — how agents navigate the portal
