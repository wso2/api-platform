# Data Plane → Control Plane Artifact Sync Guide

## Overview

When you create an AI artifact directly on the AI Gateway — an **LLM Provider Template**, **LLM Provider**, **LLM Proxy**, or **MCP Proxy** — the gateway **automatically pushes it up to the Platform API / AI Workspace** (the cloud control plane), where it appears in a **read-only** state.

### Understanding the Two Sync Directions

Artifacts can flow in two directions, distinguished by **where the artifact is created and which side owns it**:

#### **Top-Down (Control Plane → Gateway)**

The **AI Workspace (control plane) is the source of truth**. You author an artifact in the AI Workspace and the platform pushes it down to the gateway for execution.

- Control plane owns and edits the artifact
- Gateway receives and runs it
- **Origin:** `control_plane`

#### **Data Plane → Control Plane (Gateway → AI Workspace)**

The **gateway is the source of truth**. Artifacts created directly on the gateway (via the Gateway Controller management API, or loaded from disk on an immutable gateway) are automatically synced **up** to the AI Workspace so it becomes aware of them.

- Gateway owns the artifact; the AI Workspace holds a read-only copy
- The copy appears in the AI Workspace with `readOnly: true`
- **Origin:** `gateway_api`
- **Sync:** Automatic and tracked (`pending` → `success`/`failed`), with retries
- **Continues working:** the artifact stays available on the gateway even if the control plane is unavailable

#### **Key Differences**

| Aspect | Top-Down | Data Plane → Control Plane              |
|--------|----------|-----------------------------------------|
| **Initiation** | AI Workspace (control plane) | Gateway (management API / file load)    |
| **Direction** | Control plane → Gateway | Gateway → AI Workspace                  |
| **Source of Truth** | Control plane | Gateway                                 |
| **State in AI Workspace** | Editable | Read-only (`readOnly: true`)            |
| **Origin** | `control_plane` | `gateway_api`                           |
| **Status Tracking** | Managed by control plane | Tracked on the gateway (`cpSyncStatus`) |
| **Failure Handling** | Depends on control plane | Artifact works locally; sync retries    |

---

**This guide focuses on the Data Plane → Control Plane flow** for AI artifacts, with automatic sync to the AI Workspace.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Configuration](#configuration)
3. [How It Works](#how-it-works)
4. [Supported Artifacts](#supported-artifacts)
5. [Deploying an Artifact and Watching It Sync](#deploying-an-artifact-and-watching-it-sync)
6. [Read-Only Mode in the Control Plane](#read-only-mode-in-the-control-plane)
7. [Updates and Deletes](#updates-and-deletes)
8. [Sync on Connect and Reconnect](#sync-on-connect-and-reconnect)
9. [Immutable Gateways](#immutable-gateways)
10. [Sync Status Tracking](#sync-status-tracking)
11. [LLM Provider Template Syncing](#llm-provider-template-syncing)
12. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Gateway Controller Requirements

- Gateway controller running locally or remotely

### Sync Requirements

- A Platform API / AI Workspace (cloud) control plane the gateway is registered with
- Network connectivity between the gateway controller and the control plane
- `deployment_sync_enabled = true` on the gateway (the default)
- For project-scoped artifacts (LLM Proxy, MCP Proxy): the referenced **project must already exist** in your organization in the control plane

---

## Configuration

The entire flow is controlled by a single gateway setting.

| Setting | Default | Controls |
|---------|---------|----------|
| `deployment_sync_enabled` | `true` | **Both** directions of sync: DP→CP push **and** CP→DP pull |

**File:** `config.toml`

```toml
[controller.server]
gateway_id = "gateway-1"
port = 9090

[controller.controlplane]
gateway_name = "default"
insecure_skip_verify = true

# Enable two-way artifact/deployment sync with the control plane.
# DP -> CP push (this guide) and CP -> DP pull are both gated by this flag.
deployment_sync_enabled = true

[controller.auth.basic]
enabled = true

[[controller.auth.basic.users]]
username = "admin"
password = "admin"
roles = ["admin"]
```

Restart the gateway after changing the setting.

> There is no separate switch on the control plane — it simply accepts what the gateway pushes. Turn the feature on or off entirely from the gateway. When `false`, the gateway neither pushes its artifacts up nor pulls deployments down.

---

## How It Works

### Sync Behavior

When you create or update an AI artifact on the gateway:

1. **The artifact is applied to the gateway immediately**
   - Available for serving traffic
   - Keys/policies are enforced
   - Requests are processed

2. **The artifact is pushed up to the control plane (if sync is enabled and the control plane is reachable)**
   - Created in the control plane with origin `gateway_api`, **read-only**
   - Sync status tracked on the gateway: `pending` → `success`/`failed`
   - Automatic retries (up to 5 attempts) with exponential backoff on failure

### Origin: how the control plane tells artifacts apart

| Origin | Meaning | Editable in the AI Workspace? |
|--------|---------|-------------------------------|
| `control_plane` | Authored in the AI Workspace (top-down) | Yes |
| `gateway_api` | Created on the gateway and pushed up | No (read-only) |

Origin is what prevents a gateway-created artifact from being recreated or overwritten by the control plane, and what drives read-only behaviour.

### Identity and references

- The control plane assigns its **own** identifier to a gateway-originated artifact; it does not reuse the gateway's internal ID. Artifacts are matched to their existing control-plane copy by **handle** (`metadata.name`), so recreating an artifact on the gateway updates the same record instead of creating a duplicate.
- Cross-references between artifacts are by **handle**, not ID:
  - LLM Provider → template via `spec.template`
  - LLM Proxy → provider via `spec.provider.id`
  - The referenced artifact must already exist in the control plane for the reference to resolve. The gateway pushes artifacts in dependency order (templates → providers → proxies → MCP proxies) so this resolves automatically.

### Conflict resolution — "last deployment wins"

The same artifact can be deployed to multiple gateways, and pushes can arrive out of order. The control plane chooses its working copy with a **last-in-wins** rule based on **deployment time** (each push carries the gateway's deployment time in UTC):

- The push with the **most recent** deployment time defines the control plane's working copy (configuration + metadata).
- An **older** push is treated as stale and does **not** overwrite the working copy.
- Even a stale push still updates the **per-gateway deployment status**, so the control plane always shows accurately *where* the artifact is deployed — only the shared working copy is guarded by the deployment-time check.

### Deployment Flow

```
┌────────────────────────────────────────────────────────┐
│  AI artifact created on the gateway                    │
│ (POST /llm-providers, /llm-proxies, /mcp-proxies, ...) │
│  — or loaded from disk on an immutable gateway         │
└───────────────────────┬────────────────────────────────┘
                        │ (Origin = "gateway_api")
                        ▼
              ┌─────────────────────┐
              │ Apply on gateway    │
              │ cpSyncStatus:       │
              │ pending             │
              └─────────┬───────────┘
                        │
            ┌───────────┴───────────┐
            ▼                       ▼
     ┌─────────────┐        ┌───────────────────┐
     │ Gateway     │        │ Control plane     │
     │ ready       │        │ reachable?        │
     │ (serving    │        └─────────┬─────────┘
     │  traffic)   │           ┌──────┴───────┐
     └─────────────┘           ▼              ▼
                        ┌─────────────┐  ┌───────────────┐
                        │ Push up     │  │ Stays pending │
                        │ → read-only │  │ retried on    │
                        │   copy in   │  │ reconnect     │
                        │AI Workspace │  └───────────────┘
                        │ status:     │
                        │ success     │
                        └──────────── ┘
```

---

## Supported Artifacts

The following AI artifact kinds participate in the DP→CP flow:

| Kind (`kind`) | Gateway endpoint | Project-scoped |
|---------------|------------------|----------------|
| `LlmProviderTemplate` | `/llm-provider-templates` | No (org-level) |
| `LlmProvider` | `/llm-providers` | No (org-level) |
| `LlmProxy` | `/llm-proxies` | Yes |
| `Mcp` | `/mcp-proxies` | Yes |

All use `apiVersion: gateway.api-platform.wso2.com/v1`. Project-scoped kinds carry the project in a metadata annotation:

```yaml
metadata:
  annotations:
    "gateway.api-platform.wso2.com/project-id": "Project 4"
```

---

## Deploying an Artifact and Watching It Sync

This walkthrough uses an LLM Provider. The pattern (create on gateway → it appears read-only in the AI Workspace) is identical for every supported kind.

### Step 1 — Create the template it depends on

An LLM Provider references a template by handle, so create the template first.

`my-llm-provider-template.json`:

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "LlmProviderTemplate",
  "metadata": { "name": "my-llm-provider-template" },
  "spec": {
    "displayName": "OpenAI Template",
    "promptTokens":     { "location": "payload", "identifier": "$.usage.inputTokens" },
    "completionTokens": { "location": "payload", "identifier": "$.usage.outputTokens" },
    "totalTokens":      { "location": "payload", "identifier": "$.usage.totalTokens" }
  }
}
```

```bash
curl -X POST http://localhost:9090/llm-provider-templates \
  -H "Content-Type: application/json" \
  -u <user>:<password> \
  -d @my-llm-provider-template.json
```

### Step 2 — Create the LLM Provider

`openai-provider.json`:

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1",
  "kind": "LlmProvider",
  "metadata": { "name": "openai-dp" },
  "spec": {
    "displayName": "OpenAI DP",
    "version": "v1.0",
    "context": "/openai-dp",
    "template": "my-llm-provider-template",
    "upstream": {
      "url": "https://api.openai.com/v1",
      "auth": { "type": "api-key", "header": "Authorization", "value": "sk-..." }
    }
  }
}
```

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/json" \
  -u <user>:<password> \
  -d @openai-provider.json
```

**Response:**

```json
{
  "uuid": "provider-uuid-on-gateway",
  "displayName": "OpenAI DP",
  "version": "v1.0",
  "origin": "gateway_api",
  "createdAt": "2026-06-26T10:30:00Z"
}
```

### Step 3 — Watch the sync status on the gateway

```bash
curl -X GET http://localhost:9090/llm-providers/openai-dp \
  -u <user>:<password> | jq '{origin, cpSyncStatus}'
```

**Before sync completes:**

```json
{ "origin": "gateway_api", "cpSyncStatus": "pending" }
```

**After sync completes:**

```json
{ "origin": "gateway_api", "cpSyncStatus": "success" }
```

### Step 4 — Verify it appears read-only in the AI Workspace

The artifact now shows up in the AI Workspace with `readOnly: true`. Fetched via the Platform API, the artifact response carries:

```json
{
  "name": "openai-dp",
  "origin": "gateway_api",
  "readOnly": true
}
```

You can view everything about it, but cannot edit its configuration or deploy it to other gateways from the control plane.

---

## Read-Only Mode in the Control Plane

Gateway-originated artifacts (origin `gateway_api`) are **read-only** in the AI Workspace. This protects the gateway's role as the source of truth.

**Blocked in the control plane:**

| Action | Result |
|--------|--------|
| Edit the artifact's configuration or metadata | Rejected — **HTTP 403** |
| Deploy the artifact to other gateways from the control plane | Rejected — **HTTP 403** |
| Delete while still deployed on any gateway | Rejected — **HTTP 409** |

### What you *can* add to a read-only artifact

Read-only does not mean frozen. You can still enrich a gateway-originated artifact in the AI Workspace with information that **does not affect how it runs on the gateway**:

- **Documentation** — add and manage docs for the artifact.
- **OpenAPI / API definitions** — attach or update the artifact's API definition.

These additions live entirely in the control plane and are never pushed back to the gateway, so they cannot change runtime behaviour. Everything that *would* change runtime behaviour (configuration, upstreams, policies, deployment targets) stays read-only and is managed on the gateway.

---

## Updates and Deletes

| Event on the gateway | Effect in the control plane |
|----------------------|-----------------------------|
| Artifact **updated** | Change pushed up; the read-only copy is refreshed (subject to last-in-wins). `cpSyncStatus` resets to `pending`, then `success`. |
| Artifact **deleted** | The artifact is **not** removed from the AI Workspace. It remains visible, marked **undeployed** on the gateway it was removed from, preserving a record of it. |

To re-trigger a sync for an artifact (for example after a transient failure), simply update it on the gateway with the same definition — this resets `cpSyncStatus` to `pending` and pushes again.

---

## Sync on Connect and Reconnect

Pushing artifacts up is **resilient to the control plane being temporarily unavailable**:

1. **On every create/update**, while the control plane is reachable, the gateway pushes the affected artifact immediately.
2. **If a push fails** (control plane down/unreachable), the gateway retries automatically — up to **5 attempts** with exponential backoff.
3. If it still cannot push, the artifact is marked **`pending`/`failed`** on the gateway rather than lost.
4. **On the next connect or reconnect**, the gateway gathers everything still pending or failed and pushes it again.

The practical effect: you can create artifacts on a gateway while it is disconnected from the control plane, and they reconcile up automatically once the connection is (re)established — no manual intervention.

> This reconnect retry covers **all** supported kinds — LLM Provider Templates, LLM Providers, LLM Proxies and MCP Proxies. See [LLM Provider Template syncing](#llm-provider-template-syncing) for notes specific to templates.

---

## Immutable Gateways

Some gateways run in **immutable** mode, where artifacts are not created via the management API at runtime but are loaded from on-disk configuration at startup (see [Immutable Gateway](../gateway/immutable-gateway.md)).

The DP→CP flow behaves identically for these gateways: artifacts loaded from files at startup are pushed up to the control plane exactly as artifacts created via the management API are. You get the same read-only copies in the AI Workspace, the same last-in-wins conflict resolution, and the same connect/reconnect reconciliation — with no extra configuration. An immutable, file-driven gateway is therefore still fully visible in the AI Workspace.

---

## Sync Status Tracking

The gateway tracks the push status of each gateway-originated artifact in its `cpSyncStatus` field.

| Status | Meaning | Action |
|--------|---------|--------|
| `pending` | Queued/awaiting push to the control plane | Wait for automatic sync |
| `success` | Successfully pushed to the control plane | None needed |
| `failed` | Push failed after 5 retries | Check connectivity; retried on next reconnect, or update the artifact to retry now |

```bash
# Check sync status for any gateway-originated artifact
curl -X GET http://localhost:9090/llm-providers/openai-dp \
  -u <user>:<password> | jq '{origin, cpSyncStatus}'
```

A push that cannot be satisfied for non-transient reasons fails for that **one** artifact and is logged, without blocking others pushed alongside it. The most common cause is a **missing project** for a project-scoped artifact (see Troubleshooting).

---

## LLM Provider Template syncing

LLM Provider Templates sync just like the other kinds: they are pushed on create/update, tracked with a `cpSyncStatus`, and **included in the connect/reconnect retry**. A template created or changed while the control plane is unavailable is reconciled automatically once the gateway reconnects — no manual intervention required.

**Ordering note:** LLM Providers reference their template **by handle**, so a template must exist in the control plane before the providers that depend on it can resolve the reference. The gateway pushes artifacts in dependency order (templates first, then providers, then proxies), so this resolves automatically in normal operation. If a provider push ever reports an unresolved template reference, it will succeed on the next retry once the template has synced.

---

## Troubleshooting

### Issue: Artifact Not Syncing to the Control Plane

**Symptom:** `cpSyncStatus` stays `pending` or shows `failed`.

#### 1. Sync disabled

```bash
# Check the setting (config.toml [controller.controlplane]) or env var
echo $controller_controlplane_deployment_sync_enabled
```

**Fix:** set `deployment_sync_enabled = true` and restart the gateway.

#### 2. Control plane not reachable

The artifact works locally but cannot be pushed. It stays `pending`/`failed` and is retried on reconnect.

**Fix:** verify network connectivity and that the gateway is registered/connected to the control plane. Once reconnected, all pending/failed artifacts are pushed automatically.

#### 3. Project does not exist (project-scoped artifacts)

**Symptom:** an LLM Proxy or MCP Proxy fails to sync while org-level artifacts (templates, providers) succeed.

**Fix:** create the referenced project in your organization in the AI Workspace, then update the artifact on the gateway to retry. Confirm the annotation value matches an existing project:

```yaml
metadata:
  annotations:
    "gateway.api-platform.wso2.com/project-id": "Project 4"
```

#### 4. Unresolved cross-reference

**Symptom:** an LLM Provider or LLM Proxy fails to sync.

**Cause:** its referenced template/provider is not yet in the control plane (e.g. it is still being synced — see [LLM Provider Template syncing](#llm-provider-template-syncing)).

**Fix:** the dependent artifact succeeds on the next retry once the referenced artifact (matched by handle) has synced. No manual action is normally required; if needed, update the dependent artifact on the gateway to re-push it.

### Issue: Cannot Edit / Deploy / Delete the Artifact in the AI Workspace

**Symptom:** edits or deploys return **403**; delete returns **409**.

**Cause:** this is expected — the artifact is `gateway_api` origin and therefore **read-only** in the control plane.

**Fix:**
- Make configuration changes on the **gateway**; they sync up automatically.
- You may still add **documentation** and **OpenAPI definitions** in the AI Workspace.
- To delete it from the control plane, first undeploy it on **all** gateways it was deployed to, then delete.
