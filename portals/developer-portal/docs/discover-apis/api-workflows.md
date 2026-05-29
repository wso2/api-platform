# API Workflows

API Workflows are published, multi-step sequences of API calls that help you accomplish a complete task. Instead of figuring out which APIs to call and in what order, a workflow provides a vetted, step-by-step guide — for both human developers and AI agents.

## How Human Developers Discover Workflows

Workflows are available in the **API Workflows** section of the Developer Portal, accessible from the main navigation.

Each workflow is listed with its name and a short description. Clicking a workflow opens a detail view that includes:

- **Description** — what the workflow does and when to use it
- **Steps** — the ordered sequence of API calls, inputs, and expected outputs
- **Arazzo Specification** — the machine-readable workflow definition, available to download or copy

### Following a Workflow

1. Navigate to **API Workflows** in the Developer Portal sidebar.
2. Select the workflow that matches your goal.
3. Follow the steps in order, using the inputs and outputs described at each step.
4. If the workflow includes an Arazzo specification, download it to use with Arazzo-compatible tools.

## How AI Agents Discover Workflows

Agents that interact with the Developer Portal can discover workflows through two machine-readable endpoints.

### `llms.txt`

The portal's `llms.txt` file provides a structured index of everything the portal exposes to AI agents, including a list of all published workflows with agent visibility set to **Visible**.

```
GET /{orgName}/views/{viewName}/llms.txt
```

Each workflow entry in `llms.txt` includes its name, description, and a link to its agent prompt and Arazzo specification.

### `api-workflows.md`

A dedicated Markdown document lists all agent-visible workflows in a format optimized for LLM consumption. Each entry includes:

- Workflow name and handle
- Agent prompt — natural language guidance on when and how to invoke the workflow
- Link to the Arazzo specification

```
GET /{orgName}/views/{viewName}/api-workflows.md
```

### Fetching a Specific Workflow

Once an agent identifies a relevant workflow, it fetches the Arazzo specification directly:

| Resource | Endpoint |
|---|---|
| Arazzo specification | `GET /{orgName}/views/{viewName}/api-workflows/{handle}/arazzo.json` |

### Typical Agent Flow

1. Fetch `llms.txt` to get an overview of available APIs and workflows.
2. Read `api-workflows.md` to find a workflow that matches the current task.
3. Retrieve the Arazzo specification for the selected workflow.
4. Execute the workflow steps using the API credentials already obtained.

## Workflow Visibility

Not all published workflows are visible to all consumers. Admins control visibility independently for human users and AI agents.

| Workflow Setting | Visible in portal UI | Appears in `llms.txt` / `api-workflows.md` |
|---|---|---|
| Fully visible | Yes | Yes |
| Agent visibility hidden | Yes | No |
| Portal UI hidden | No | Yes |

## Related

- [Managing API Workflows (Admin)](../publish-apis/manage-api-workflows.md) — create and publish workflows
- [AI Agent Discovery](ai-agent-discovery.md) — full overview of machine-readable discovery endpoints
