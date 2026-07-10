# Syncing Gateway-Created AI Artifacts to the AI Workspace

## Overview

When you create an AI artifact directly on the AI Gateway — an **LLM Provider Template**, **LLM Provider**, **LLM Proxy**, or **MCP Proxy** — it is automatically synced up to the **AI Workspace**, where it appears as a **read-only** copy you can view and monitor.

This is the reverse of the usual top-down flow:

| | You create it in… | The AI Workspace copy is… |
|---|---|---|
| **Top-down** | the AI Workspace, then it's pushed to the gateway | editable — you own it |
| **This guide (bottom-up)** | the gateway, then it's synced up to the AI Workspace | read-only — the gateway owns it |

Because the gateway owns these artifacts, they keep serving traffic even if the AI Workspace is temporarily unavailable, and any change you make on the gateway is synced up automatically.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Enabling the sync](#enabling-the-sync)
3. [How the sync works](#how-the-sync-works)
4. [Supported artifacts](#supported-artifacts)
5. [Create the artifacts on the gateway](#create-the-artifacts-on-the-gateway)
6. [View them in the AI Workspace](#view-them-in-the-ai-workspace)
7. [What you can and can't change in the AI Workspace](#what-you-can-and-cant-change-in-the-ai-workspace)
8. [Updating and deleting](#updating-and-deleting)
9. [If the AI Workspace is temporarily unavailable](#if-the-ai-workspace-is-temporarily-unavailable)
10. [Immutable gateways](#immutable-gateways)
11. [Troubleshooting](#troubleshooting)

---

## Prerequisites

- A gateway that is registered with, and can reach, your AI Workspace.
- Syncing enabled on the gateway (see [Enabling the sync](#enabling-the-sync) — it is on by default).
- For **LLM Proxies** and **MCP Proxies**, which belong to a project: the project they reference must already exist in your organization in the AI Workspace.

---

## Enabling the sync

Syncing is controlled by a single gateway setting, `deployment_sync_enabled`, which is **on by default**. It controls syncing in both directions between the gateway and the AI Workspace.

**File:** `config.toml`

```toml
[controller.controlplane]
gateway_name = "default"
insecure_skip_verify = true

# Sync artifacts with the AI Workspace (on by default).
deployment_sync_enabled = true
```

Restart the gateway after changing the setting. When it is turned off, the gateway neither syncs its artifacts up nor receives artifacts from the AI Workspace.

---

## How the sync works

When you create or update an artifact on the gateway:

```
Create on the gateway ─┬─▶ takes effect immediately (starts serving traffic)
                       └─▶ synced to the AI Workspace ─▶ appears as a read-only copy
```

The sync happens automatically in the background — you don't trigger it. A few things to know:

- **Matched by name.** Each artifact is identified by the name you give it (`metadata.name`). Re-creating an artifact with the same name on the gateway updates the same AI Workspace copy instead of creating a duplicate.
- **References use names.** An LLM Provider names its template, and an LLM Proxy names its provider. Create them in order — **template → provider → proxy** — so each reference resolves. MCP Proxies stand on their own.
- **Most recent deployment wins.** If the same artifact is deployed on more than one gateway, the AI Workspace shows the version from the most recent deployment.

---

## Supported artifacts

Four AI artifact kinds sync from the gateway to the AI Workspace. You create them through the gateway's management API, under the base path `/api/management/v1` (default port `9090`):

| Kind | Management API endpoint | Belongs to a project? |
|------|-------------------------|-----------------------|
| `LlmProviderTemplate` | `/api/management/v1/llm-provider-templates` | No (organization level) |
| `LlmProvider` | `/api/management/v1/llm-providers` | No (organization level) |
| `LlmProxy` | `/api/management/v1/llm-proxies` | Yes |
| `Mcp` | `/api/management/v1/mcp-proxies` | Yes |

All manifests use `apiVersion: gateway.api-platform.wso2.com/v1`. Project-scoped kinds name their project in an annotation:

```yaml
metadata:
  annotations:
    "gateway.api-platform.wso2.com/project-id": "Project 1"
```

### Calling the management API

The examples below use these conventions:

- **Base URL:** `http://localhost:9090/api/management/v1`
- **Content type:** `Content-Type: text/yaml` (the API also accepts JSON)
- **Auth:** HTTP Basic, using a user configured under `[[controller.auth.basic.users]]` in `config.toml`. Pass your own credentials — don't hard-code them:
  ```bash
  export GW_USER='<username>' GW_PASSWORD='<password>'
  ```
- **Body:** `--data-binary '@<file>.yaml'` uploads a manifest file as-is.

The manifests below are the ready-to-run samples in [`gateway/examples/`](../../gateway/examples). Run the `curl` commands from that directory so the `@<file>.yaml` paths resolve.

---

## Create the artifacts on the gateway

This walkthrough builds a complete **LLM Proxy** together with the artifacts it depends on — an **LLM Provider Template** and an **LLM Provider** — then adds a standalone **MCP Proxy**. Each `curl` creates the artifact on the gateway; it starts serving immediately and is synced to the AI Workspace.

Create them in dependency order so each reference resolves:

```
LlmProviderTemplate ──(spec.template)──▶ LlmProvider ──(spec.provider.id)──▶ LlmProxy
```

> The LLM Proxy and MCP Proxy reference **`Project 1`** — make sure that project exists in your organization in the AI Workspace first.

### Step 1 — Create the LLM Provider Template

The provider references a template by name, so create the template first. `llm-provider-template.yaml`:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: my-llm-provider-template
spec:
  displayName: OpenAI
  promptTokens:     { location: payload, identifier: $.usage.inputTokens }
  completionTokens: { location: payload, identifier: $.usage.outputTokens }
  totalTokens:      { location: payload, identifier: $.usage.totalTokens }
  # ... see gateway/examples/llm-provider-template.yaml for the full manifest
```

```bash
curl --location 'http://localhost:9090/api/management/v1/llm-provider-templates' \
  --header 'Content-Type: text/yaml' \
  --user "$GW_USER:$GW_PASSWORD" \
  --data-binary '@llm-provider-template.yaml'
```

### Step 2 — Create the LLM Provider

The provider links to the template above via `spec.template`. `llm-provider.yaml`:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: my-llm-provider
spec:
  displayName: WSO2 My LLM Provider
  version: v1.0
  context: /openai-dp-1
  template: my-llm-provider-template   # ← must match the template's metadata.name
  vhost: api.my-llm-provider.local
  upstream:
    url: https://httpbin.org/anything/v1
    auth: { type: api-key, header: Authorization, value: api_key_abc123 }
  # ... accessControl + policies omitted; see gateway/examples/llm-provider.yaml
```

```bash
curl --location 'http://localhost:9090/api/management/v1/llm-providers' \
  --header 'Content-Type: text/yaml' \
  --user "$GW_USER:$GW_PASSWORD" \
  --data-binary '@llm-provider.yaml'
```

### Step 3 — Create the LLM Proxy

The proxy belongs to a project (the `project-id` annotation) and links to the provider via `spec.provider.id`. `llm-proxy.yaml`:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProxy
metadata:
  name: wso2con-assistant
  annotations:
    "gateway.api-platform.wso2.com/project-id": "Project 1"   # ← must be an existing project
spec:
  displayName: WSO2 Con Assistant
  version: v1.0
  context: "/project-1/assistant"
  provider:
    id: my-llm-provider   # ← must match the provider's metadata.name
    auth: { header: X-API-Key, type: api-key, value: adminfoobar }
  # ... policies omitted; see gateway/examples/llm-proxy.yaml
```

```bash
curl --location 'http://localhost:9090/api/management/v1/llm-proxies' \
  --header 'Content-Type: text/yaml' \
  --user "$GW_USER:$GW_PASSWORD" \
  --data-binary '@llm-proxy.yaml'
```

### Step 4 — Create an MCP Proxy

An MCP Proxy also belongs to a project but stands on its own (no template or provider prerequisite). `mcp-proxy.yaml`:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Mcp
metadata:
  name: everything-mcp-v1.0
  annotations:
    "gateway.api-platform.wso2.com/project-id": "Project 1"
spec:
  displayName: Everything
  version: v1.0
  context: "/project-1/everything"
  specVersion: "2025-06-18"
  upstream:
    url: https://.../mcp-everything-server/v1.0
    auth: { header: X-Api-Key, type: header, value: admin }
  # ... policies omitted; see gateway/examples/mcp-proxy.yaml
```

```bash
curl --location 'http://localhost:9090/api/management/v1/mcp-proxies' \
  --header 'Content-Type: text/yaml' \
  --user "$GW_USER:$GW_PASSWORD" \
  --data-binary '@mcp-proxy.yaml'
```

---

## View them in the AI Workspace

The gateway syncs each artifact up automatically. Shortly after you create them, all four appear in the AI Workspace as **read-only** copies. To find them:

1. Open the **AI Workspace** for the organization your gateway is registered with.
2. Locate each artifact in the sidebar — the copy keeps the same name you gave it on the gateway:

   | Artifact | Where to find it in the AI Workspace | Name |
   |----------|--------------------------------------|------|
   | LLM Provider Template | **Settings → LLM Provider Templates** | `my-llm-provider-template` |
   | LLM Provider | **LLM → LLM Providers** | `my-llm-provider` |
   | LLM Proxy | **LLM → App LLM Proxies** (under **Project 1**) | `wso2con-assistant` |
   | MCP Proxy | **MCP → MCP Proxies** (under **Project 1**) | `everything-mcp-v1.0` |

3. Open any of them to browse the full configuration. It opens in a read-only view — the edit and deploy actions are unavailable because the gateway owns the artifact.

If an artifact hasn't appeared after a short wait, see [Troubleshooting](#troubleshooting).

---

## What you can and can't change in the AI Workspace

A gateway-created artifact is **read-only** in the AI Workspace because the gateway owns it. "Read-only" applies to anything the gateway uses to run the artifact — everything else stays editable.

**You _can_ change things that don't affect how the gateway runs the artifact** (these stay in the AI Workspace only):

- Its description and display name
- Documentation and API (OpenAPI) definitions
- For an **LLM Provider Template**: its connection details (endpoint URL, auth type/header), logo, and OpenAPI spec

**You _can't_ change what the gateway uses to run the artifact.** Make those changes on the gateway instead — they sync up automatically. This includes:

- Upstreams, the auth/routing used to serve traffic, and policies
- An LLM Provider Template's token-tracking settings
- Deploying, redeploying, or undeploying the artifact
- Deleting it while it is still deployed on a gateway (undeploy it from all gateways first)

The AI Workspace simply won't offer the actions it can't perform, and will decline an edit that would change how the gateway runs the artifact.

---

## Updating and deleting

| On the gateway you… | In the AI Workspace… |
|---------------------|----------------------|
| **Update** the artifact | the read-only copy refreshes automatically |
| **Delete** the artifact | the copy is kept (not removed) and shown as no longer deployed on that gateway, preserving a record of it |

To re-sync an artifact after a hiccup, re-apply it on the gateway with the same definition.

---

## If the AI Workspace is temporarily unavailable

Syncing is resilient. If the AI Workspace can't be reached when you create or change an artifact:

- The artifact still takes effect on the gateway and keeps serving traffic.
- The gateway retries the sync automatically.
- When the connection is restored, everything that hasn't synced yet is pushed up — no manual action needed.

You can create artifacts on a gateway while it is disconnected, and they reconcile up on their own once it reconnects. This applies to all four artifact kinds.

---

## Immutable gateways

Some gateways run in **immutable** mode, where artifacts are loaded from on-disk configuration at startup rather than created through the management API (see [Immutable Gateway](../gateway/immutable-gateway.md)).

The sync behaves exactly the same for these gateways: artifacts loaded from files are synced up to the AI Workspace just like ones created through the management API, with the same read-only copies and the same automatic reconciliation — no extra configuration. An immutable, file-driven gateway is still fully visible in the AI Workspace.

---

## Troubleshooting

### An artifact I created on the gateway doesn't appear in the AI Workspace

- **Syncing is turned off.** Set `deployment_sync_enabled = true` in the gateway's `config.toml` and restart the gateway.
- **The AI Workspace can't be reached.** The artifact still works on the gateway; the sync retries automatically and catches up once the connection is restored. Check that the gateway is connected to the AI Workspace.
- **The project doesn't exist** (LLM Proxy or MCP Proxy). These belong to a project. Create the project named in the artifact's `project-id` annotation in your organization, then re-apply the artifact on the gateway:
  ```yaml
  metadata:
    annotations:
      "gateway.api-platform.wso2.com/project-id": "Project 1"
  ```
- **A referenced artifact isn't there yet.** An LLM Provider needs its template, and an LLM Proxy needs its provider. Create them in order (template → provider → proxy); the dependent artifact catches up on its own once the one it references has synced.

### I can't edit, deploy, or delete a gateway-created artifact in the AI Workspace

This is expected — the gateway owns it, so it is read-only in the AI Workspace.

- Make configuration and deployment changes on the **gateway**; they sync up automatically.
- You can still edit runtime-neutral details (description, display name, documentation, OpenAPI definitions, and — for LLM Provider Templates — connection details and logo).
- To delete it from the AI Workspace, first undeploy it from **all** gateways it was deployed to, then delete.

See [What you can and can't change](#what-you-can-and-cant-change-in-the-ai-workspace) for the full list.
