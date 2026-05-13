# \[Feature\] Gateway-controller management resources as WSO2 CRDs (APIGateway path)

> **Suggested GitHub discussion title:** `WSO2 CRDs for LLM, MCP, secrets, certificates, API keys, and subscriptions — APIGateway + gateway-controller REST` — paste the body below into the api-platform repo **Discussions** (e.g. category **Ideas** or **General**).

For the **engineering plan and checklist**, see **`FEATURE_MANAGEMENT_RESOURCES_CRDs.md`** in this folder.

---

## Summary

Extend the **gateway-operator** so **`APIGateway`**-backed flows can reconcile **more than `RestApi`**. Each new CRD mirrors a resource exposed by the gateway-controller **management API** (OpenAPI in `gateway/gateway-controller/api/management-openapi.yaml`; base URL prefix `/api/management/v0.9`).

**In scope:**

- **`LlmProvider`**, **`LlmProviderTemplate`**, **`LlmProxy`** — AI provider/template/proxy configuration (YAML payloads aligned with gateway-controller schemas).
- **`Mcp`** (MCP proxy) — MCP proxy runtime configuration.
- **`Secret`** — platform secrets stored by gateway-controller (**not** a replacement for core Kubernetes `Secret` objects; optional `valueFrom` to **read from** core Secrets when populating payloads).
- **`Certificate`** — certificate upload; gateway assigns a **`id`** (UUID); operator persists **`status.id`** after first sync.
- **`ApiKey`** — nested under a **parent** resource (`RestApi`, `LlmProvider`, or `LlmProxy`) via **`spec.parentRef`**; URLs look like **`/…/api-keys/…`**.
- **`SubscriptionPlan`** and **`Subscription`** — quota/billing-aligned objects; gateway assigns **`id`**; operator keeps **`status.id`** for updates and deletes.

**Out of scope (unchanged):**

- Kubernetes **Gateway API** — **`gateway.networking.k8s.io` `Gateway`**, **`HTTPRoute`**, **`APIPolicy`** reconcilers (`k8s_gateway_*`, `httproute_*`) are **not** extended here. Same split as **`GITHUB_DISCUSSION_GATEWAY_API.md`** vs **`RestApi`**; this proposal is **`APIGateway` + `/api/management/v0.9`** only.

---

## Problem statement

Today only **`RestApi`** is a first-class WSO2 CR that the operator syncs to gateway-controller (**`POST`/`PUT`/`DELETE`** under **`/rest-apis/…`**). Operators who deploy **LLM routers**, **MCP proxies**, platform **secrets**, **certificates**, **API keys**, and **subscriptions** currently apply those configs outside Kubernetes (manual REST, Helm hooks, CI) or duplicate logic. Bringing them under the **same selection, retry, status, and finalizer** semantics as **`RestApi`** reduces drift and makes GitOps coherent.

---

## Who is this for?

- **Platform teams** managing the API Platform gateway and wanting every management-API capability from Git + RBAC-reviewed CRDs.
- **API / AI teams** provisioning **LLM** and **MCP** proxies next to **`RestApi`** workloads.
- **Operators** running **`APIGateway`** with **`APISelector`** (cluster / namespaces / labels) who expect new resource types to follow the **same gateway binding rules** as **`RestApi`**.

---

## Why does this matter?

- **Single reconciliation model** — Finalizers, backoff, **`Programmed`**-style conditions, and **GatewayRegistry** lookup mirror **`restapi_controller.go`** behaviour.
- **One management surface** — The operator continues to speak **only** gateway-controller REST; no alternate control plane paths for those features.
- **Clear boundary** — Standard Gateway API (**`GITHUB_DISCUSSION_GATEWAY_API.md`**) stays orthogonal to this **`APIGateway`**-centric extension.

---

## Use cases

| Use case | What you do |
| -------- | ----------- |
| Deploy an LLM provider from Git | **`LlmProvider`** CR; operator **`POST`/`PUT`** to **`/llm-providers/{handle}`**. |
| Register an MCP proxy | **`Mcp`** CR → **`/mcp-proxies/{handle}`**. |
| Store a TLS bundle for the gateway | **`Certificate`** CR; after create, **`status.id`** identifies the uploaded cert for **`PUT`/`DELETE`**. |
| Platform secret usable in policies/upstreams | **`Secret`** CR under gateway-controller management **`/secrets/…`**; optional **`valueFrom`** to pull bytes from core **`Secret`**. |
| Issue an API key for a **`RestApi` or LLM resource | **`ApiKey`** with **`spec.parentRef`** (`kind`, `name`); REST path nests under parent. |
| Define throttling/plan and bind a subscriber token | **`SubscriptionPlan`** + **`Subscription`** CRs; persist gateway **`id`** on **`status`**. |

---

## Goals (technical)

- **`APIGateway` selection parity** — New kinds use **`registry.GatewayRegistry.FindMatchingGateways(namespace, labels)`** the same way **`RestApi`** does (see **`APISelector`** on **`APIGateway.spec`**).
- **Payload fidelity** — Spec fields align with **`gateway/gateway-controller/pkg/api/management/generated.go`** (`*ConfigData`, request DTOs). Polymorphic or schemaless fragments use **`runtime.RawExtension`** where needed (similar spirit to **`Policy.params`** on **`RestApi`**).
- **Secrets in payloads** — Support **inline bytes** **or** **`valueFrom.secretKeyRef`** resolving to plaintext before **`POST`/`PUT`**.
- **UUID lifecycles** — **`Certificate`**, **`SubscriptionPlan`**, **`Subscription`**: persist returned **`id`** in **`status`** so subsequent reconcile uses **`/{id}`** paths.
- **Nested API keys** — **`ApiKey`** CR carries **`spec.parentRef`**; controller validates parent existence and prefixes REST paths (**`rest-apis`**, **`llm-providers`**, **`llm-proxies`**).

---

## Prerequisites (cluster and assumptions)

- A deployed **gateway-operator** that already registers gateways from **`APIGateway`** (**or equivalent registry population** as today for **`RestApi`**).
- Gateway-controller reachable at the **`Service`** the operator resolves (same endpoint resolution semantics as **`RestApi`** today).
- **RBAC**: operator needs **`get`** on **`Secret`** wherever **`valueFrom`** is used.

**Not required:** Installing Kubernetes Gateway API CRDs (**`GITHUB_DISCUSSION_GATEWAY_API.md`**) solely for these WSO2 CRDs.

---

## Gateway selection (`APIGateway` and labels)

Behaviour matches **`RestApi`** today:

| Mechanism | Role |
| --------- | ---- |
| **`spec.apiSelector`** on **`APIGateway`** | **Cluster**, **Namespaced**, or **LabelSelector** scope — decides which namespaces / labels qualify. |
| **Labels on workload CRs** | Used under **LabelSelector** scope (**`gateway_registry.go`**). |

---

## Reconciler behaviour (high level)

1. Watch the new CR kind **and** **`APIGateway`** (enqueue when gateways change selection, same pattern as **`restapi_controller.go`** enqueue from gateway events).
2. **Finalizer** per kind (distinct name per resource type).
3. Resolve gateway endpoint → **basic auth** (same **`GetAuthSettingsForRegistryGateway`** pattern as **`RestApi`**).
4. Build payload (YAML envelope for **`apiVersion`/`kind`/`metadata`/`spec`** kinds where the management API expects it; JSON for subscriptions where OpenAPI specifies JSON-only).
5. **Existence probe** (**`GET`**) where supported → **`POST`** or **`PUT`**; **`DELETE`** on CR removal.
6. **Retry / backoff** using the same **`APITracker`**-style in-memory semantics as **`RestApi`** (factored toward a generic helper implementation-side).
7. **`status`**: conditions (**`Accepted`**, **`Programmed`**) analogous to **`RestApi`**; extra fields **`status.id`** for UUID-backed resources.

---

## Nested **`ApiKey` parents**

| `parentRef.kind` (planned) | Management API parent segment |
| -------------------------- | ----------------------------- |
| `RestApi` | `/rest-apis/{parent}` |
| `LlmProvider` | `/llm-providers/{parent}` |
| `LlmProxy` | `/llm-proxies/{parent}` |

Child path suffix: **`/api-keys`**; key by API key **`name`/handle**.

---

## RBAC

Per-kind rules extend the operator RBAC bundle: verbs on each new **`gateway.api-platform.wso2.com`** plural resource (`get`, `list`, `watch`, `update`, `patch`), subresources **`status`** and **`finalizers`**, and core **`secrets` `get`** when **`valueFrom`** resolution is implemented. Consolidated in **`config/rbac/role.yaml`** and propagated to the Helm **`clusterrole`** (same rollout model as **`RestApi`**).

---

## Coexistence

- **`RestApi`** and new kinds can share a namespace; **`APIGateway`** selection applies per CR type independently (each reconciler filters with the same **`FindMatchingGateways`** inputs).
- **Kubernetes `Gateway` / `HTTPRoute`** workloads do **not** automatically create these CRs. The Gateway API Discussion (**`GITHUB_DISCUSSION_GATEWAY_API.md`**) covers **`HTTPRoute`** → **`RestApi`‑equivalent** payloads on **`/rest-apis`** only for now. Driving LLM/MCP/etc. CRs from **`HTTPRoute`** would need a separate design.

---

## MVP limitations

- Ordering dependencies (e.g. **`LlmProvider`** before **`LlmProxy`** references it) must be enforced by GitOps conventions or surface as reconcile errors until richer graph ordering exists.
- **Subscription** payloads may omit fields the gateway resolves server-side — exact validation matches gateway-controller behaviour.
- **Large PEM / secret values**: prefer **`valueFrom`** to avoid etcd-unfriendly manifests.

---

## Main code layout (anticipated)

| Area | Location |
| ---- | -------- |
| New CRDs | `api/v1alpha1/*_types.go` (per kind files) |
| Shared REST helpers | `internal/gatewayclient/` (generalised **`Deploy`/`Exists`/`Delete`**, path constants) |
| Reconcilers | `internal/controller/*_controller.go` (per kind; optionally shared **`generic_reconciler.go`**) |
| Existing **`RestApi`** reference implementation | `internal/controller/restapi_controller.go` |
| Registry / endpoint | `internal/registry/gateway_registry.go` |
| Auth | `internal/auth/auth_helper.go` |
| Maintainer plan index | **`FEATURE_MANAGEMENT_RESOURCES_CRDs.md`** |
| Gateway API (orthogonal) | **`GITHUB_DISCUSSION_GATEWAY_API.md`** |

