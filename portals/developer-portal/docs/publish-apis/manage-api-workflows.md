# Manage API Workflows (Admin)

API Workflows are multi-step sequences of API calls that help developers accomplish a complete task — for example, "Place an Order" or "Register a Webhook". Workflows are defined in [Arazzo format](https://spec.openapis.org/arazzo/latest.html) and published in the portal for both human developers and AI agents to discover and follow.

For the portal UI guide, see [Managing API Workflows](../administer/managing-api-workflows.md).

This page covers the admin API endpoints for managing workflows programmatically.

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

## Create a Workflow

API workflow requests are JSON. The `apiWorkflowDefinition` field contains the Arazzo content as an inline object.

```json
// workflow.json
{
  "name": "Place an Order",
  "handle": "place-an-order",
  "description": "End-to-end flow for creating and confirming a customer order",
  "agentPrompt": "Use this workflow when a user wants to place a new order. It covers product lookup, cart creation, and order submission.",
  "agentVisibility": "VISIBLE",
  "contentType": "ARAZZO",
  "apiWorkflowDefinition": {
    "arazzo": "1.0.0",
    "info": {
      "title": "Place an Order",
      "version": "1.0"
    },
    "sourceDescriptions": [
      {
        "name": "OrderAPI",
        "url": "/orgs/acme/apis/order-api/spec",
        "type": "openapi"
      }
    ],
    "workflows": [
      {
        "workflowId": "place-order",
        "summary": "Create and confirm a customer order",
        "steps": [
          {
            "stepId": "lookup-product",
            "operationId": "getProduct",
            "parameters": [
              {"name": "productId", "in": "query", "value": "$inputs.productId"}
            ]
          },
          {
            "stepId": "create-order",
            "operationId": "createOrder",
            "requestBody": {
              "payload": {
                "customerId": "$inputs.customerId",
                "items": [
                  {"sku": "$steps.lookup-product.outputs.sku", "quantity": "$inputs.quantity"}
                ]
              }
            }
          }
        ]
      }
    ]
  }
}
```

```bash
curl -X POST \
  "http://localhost:3000/api/v0.9/views/{viewName}/api-workflows" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @workflow.json
```

| Field | Description |
|---|---|
| `name` | Short, task-oriented workflow name |
| `handle` | URL-safe identifier for the workflow (auto-derived from `name` if omitted) |
| `description` | One to two sentences describing what the workflow accomplishes |
| `agentPrompt` | Natural language guidance for AI agents on when/how to use this workflow |
| `agentVisibility` | `VISIBLE` — included in `llms.txt`/`api-workflows.md`; `HIDDEN` — excluded from agent surfaces |
| `contentType` | `ARAZZO` for Arazzo workflows; `MD` for Markdown-based workflows |
| `apiWorkflowDefinition` | Inline Arazzo specification object (when `contentType` is `ARAZZO`) |
| `markdownContent` | Markdown string (when `contentType` is `MD`) |

## List Workflows

```bash
curl http://localhost:3000/api/v0.9/views/{viewName}/api-workflows \
  -H "Authorization: Bearer $TOKEN"
```

## Get a Workflow

```bash
curl http://localhost:3000/api/v0.9/views/{viewName}/api-workflows/{apiWorkflowId} \
  -H "Authorization: Bearer $TOKEN"
```

## Update a Workflow

```json
// workflow-update.json
{
  "name": "Place an Order",
  "description": "Updated description",
  "agentVisibility": "VISIBLE"
}
```

```bash
curl -X PUT \
  "http://localhost:3000/api/v0.9/views/{viewName}/api-workflows/{apiWorkflowId}" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @workflow-update.json
```

## Delete a Workflow

```bash
curl -X DELETE \
  "http://localhost:3000/api/v0.9/views/{viewName}/api-workflows/{apiWorkflowId}" \
  -H "Authorization: Bearer $TOKEN"
```

## Generate an Agent Prompt with AI

The portal can generate a suggested agent prompt for a workflow using AI:

```json
// generate-prompt.json
{
  "name": "Place an Order",
  "description": "End-to-end flow for creating and confirming a customer order",
  "orgHandle": "acme",
  "viewName": "default",
  "handle": "place-an-order",
  "apis": [
    {"name": "Order API", "version": "v1.0"}
  ]
}
```

```bash
curl -X POST \
  "http://localhost:3000/api/v0.9/views/{viewName}/api-workflows/generate-prompt" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @generate-prompt.json
```

The response contains a suggested `agentPrompt` you can use as-is or refine before saving.

## Related

- [Managing API Workflows (Admin UI)](../administer/managing-api-workflows.md) — portal UI guide for workflow management
- [Consuming API Workflows](../discover-apis/api-workflows.md) — how developers and agents use workflows
