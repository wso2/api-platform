# Feature: WSO2 management resources as operator CRDs (APIGateway path)

This document is the **implementation plan** for extending the gateway-operator so the **WSO2 `APIGateway` / gateway-controller REST flow** reconciles additional kinds from the gateway-controller **management API** — not only `RestApi`. It parallels the architectural pattern described for `RestApi` elsewhere; it does **not** apply to Kubernetes **Gateway API** (`Gateway`, `HTTPRoute`, `APIPolicy`).

Related maintainer docs in this folder:

- **GITHUB_DISCUSSION_MANAGEMENT_RESOURCES_CRDs.md** — Discussion-style narrative (problem, audience, use cases, boundaries); paste into GitHub **Discussions** like **`GITHUB_DISCUSSION_GATEWAY_API.md`**.
- **GATEWAY_API_IMPLEMENTATION_NOTES.md** — Gateway API (`Gateway` / `HTTPRoute`) behaviour and layout (**`GITHUB_DISCUSSION_GATEWAY_API.md`** is the Discussion companion).
- This file — **`RestApi`**-adjacent additions: LLM/MCP/secrets/certificates/API keys/subscriptions, all driven by **`APIGateway` + `GatewayRegistry` + `/api/management/v0.9`** only.

---

## Overview

Extend the gateway-operator (**APIGateway flow only**) so it manages **all chosen top-level** management-api kinds:

`LlmProviderTemplate`, `LlmProvider`, `LlmProxy`, `Mcp`, `Secret`, `Certificate`, `ApiKey`, `SubscriptionPlan`, `Subscription`

**Note:** Gateway-controller also exposes **`/websub-apis`** in its management OpenAPI, but the operator does **not** ship a **`WebSubApi`** CRD: WebSub is not fully supported end-to-end on the gateway product surface, so GitOps for WebSub APIs is out of scope here.

…using the **same** selector → tracker → push-to-gateway-controller pattern that `RestApi` uses today (`internal/controller/restapi_controller.go`, `internal/gatewayclient/`, `internal/registry/`).

Implementation work is tracked as the checklist below.

### Checklist

- [ ] Add v1alpha1 Go types for the kinds above, plus `SecretValueSource`; regenerate deepcopy and CRDs.
- [ ] Refactor `internal/gatewayclient` (generic resource client, path constants); bespoke clients for Certificate, Subscription, SubscriptionPlan, ApiKey (nested).
- [ ] Extract decision/tracker/retry logic from `restapi_controller.go` into a reusable `generic_reconciler.go` for new kinds.
- [ ] Per-kind controllers (finalizers, status, gateway selector, `APIGateway` watch).
- [ ] Resolve `SecretValueSource` (`value` vs `valueFrom.secretKeyRef`) against core `Secret` objects.
- [ ] Persist gateway-issued UUIDs in `status` for Subscription, SubscriptionPlan, Certificate.
- [ ] Register reconcilers in `cmd/main.go` and add types to the scheme.
- [ ] RBAC markers, regenerate `role` / Helm `clusterrole`, emit CRDs under Helm `crds/`.
- [ ] Example CR YAML under `gateway/examples/` where missing.
- [ ] Unit tests: payload builders, `SecretValueSource`, ApiKey parent paths, UUID status round-trip.

---

## Background

Today the operator has a single managed kind **`RestApi`** (`api/v1alpha1/restapi_types.go`) reconciled by **`restapi_controller.go`**. That reconciler:

1. Watches `RestApi` CRs and the `APIGateway` CR (selector match).
2. Picks the matching gateway from `registry.GatewayRegistry` (`internal/registry/gateway_registry.go`).
3. Builds a YAML payload via `gatewayclient.BuildRestAPIYAML` (`internal/gatewayclient/yaml_payload.go`).
4. `POST` / `PUT` / `DELETE` against the gateway-controller management API under `/api/management/v0.9/rest-apis/...` (`internal/gatewayclient/rest_api.go`).
5. Uses an in-memory **`APITracker`** for retry/backoff and finalizer-based cleanup.

The same pattern applies to further resources defined in **`gateway/gateway-controller/api/management-openapi.yaml`** (source of truth for paths and schemas). Generated Go models live in **`gateway/gateway-controller/pkg/api/management/generated.go`**.

The Kubernetes Gateway API flow (`internal/controller/k8s_gateway_controller.go`, `httproute_controller.go`, …) stays **unchanged**.

---

## Resource → endpoint map

Rough mapping from new operator concepts to gateway-controller REST paths (`ManagementAPIBasePath` = `/api/management/v0.9`; see `internal/gatewayclient/rest_api.go`).

| Operator concern | Typical management path segment |
| ---------------- | ------------------------------- |
| LlmProvider | `/llm-providers/{name}` |
| LlmProviderTemplate | `/llm-provider-templates/{name}` |
| LlmProxy | `/llm-proxies/{name}` |
| Mcp | `/mcp-proxies/{name}` |
| Secret | `/secrets/{name}` |
| Certificate | `/certificates/{id}` (certificate upload shape differs from apiVersion/kind envelopes) |
| ApiKey | `/<parent>/<parentName>/api-keys/<name>` (parents: rest-apis, llm-providers, llm-proxies) |
| SubscriptionPlan | `/subscription-plans/{planId}` |
| Subscription | `/subscriptions/{subscriptionId}` |

**Variants:**

- **`Certificate`** — uses `CertificateUploadRequest`-style payloads; gateway returns a UUID `id`.
- **`ApiKey`** — nested under a parent resource; modeled with **`spec.parentRef`** `{ kind, name }`.
- **`SubscriptionPlan`** / **`Subscription`** — JSON request/response; gateway assigns UUIDs that must land in **`status.id`** so updates/deletes target the correct row.

(A Mermaid diagram of the same map lived in the original plan; ASCII table is enough here for grepability.)

---

## Design highlights

1. **Mirror gateway-controller specs** — Each new CRD `Spec` aligns with `*ConfigData` / `*Request` in `generated.go`, with kubebuilder validation. Polymorphic or free-form blobs (e.g. upstream unions, policy params) use `*runtime.RawExtension` + `PreserveUnknownFields` where needed (same spirit as `Policy.Params` on `RestApi`).

2. **Sensitive value sourcing** — `Secret.spec.value`, optional `ApiKey` material, optional `Certificate` PEM accept:
   ```go
   type SecretValueSource struct {
       Value     *string                       `json:"value,omitempty"`
       ValueFrom *corev1.SecretKeySelector     `json:"valueFrom,omitempty"`
   }
   ```
   Controllers resolve `valueFrom` at reconcile time before calling the REST API.

3. **Generalise the gateway HTTP client** — Factor `Deploy` / `Exists` / `Delete` patterns out of **`rest_api.go`** behind path + handle; thin typed wrappers per kind; nested **`ApiKey`** client; specialised JSON clients for subscriptions and certificate flows.

4. **Generic reconciler helper** — Extract generation/tracker/retry/backoff/status updates from **`RestApiReconciler`** into a reusable module; new kinds plug adapters. **Migrating `RestApi` itself** onto the generic helper can be a follow PR to keep the first rollout reviewable.

5. **UUID-keyed kinds** — After first successful **`POST`**, persist returned id in **`status.id`** for Subscription, SubscriptionPlan, Certificate.

6. **Gateway selection** — Reuse **`FindMatchingGateways(namespace, labels)`**. Generalise enqueue-from-**`APIGateway`** helpers so selector logic is not **`RestApi`\-only**.

7. **Out of bounds** — Do not touch `k8s_gateway_*`, `httproute_*`, listener/infrastructure overlays, or **`gateway-controller`** server code for this feature tranche.

---

## Concrete deliverables

### 1. New CRD types — `api/v1alpha1/`

One Go file per kind (names illustrative):

| File | Purpose |
| ---- | ------- |
| `llmprovidertemplate_types.go` | Template CRD |
| `llmprovider_types.go` | Provider CRD |
| `llmproxy_types.go` | LLM proxy CRD |
| `mcp_types.go` | MCP proxy CRD |
| `secret_types.go` | Secret CRD (`SecretValueSource` for values) |
| `certificate_types.go` | Certificate CRD (PEM via `SecretValueSource`; **`status.id`**) |
| `apikey_types.go` | ApiKey CRD (`parentRef` + optional key material via `SecretValueSource`) |
| `subscriptionplan_types.go` | Plan CRD (`SubscriptionPlanCreateRequest` shape + **`status.id`**) |
| `subscription_types.go` | Subscription CRD (`SubscriptionCreateRequest` shape + token via `SecretValueSource` + **`status.id`**) |

Register each type with **`SchemeBuilder`** in **`init()`** (same pattern as `restapi_types.go`).

Regenerate **`zz_generated.deepcopy.go`** and CRD YAML (**`controller-gen`**).

### 2. Gateway client — `internal/gatewayclient/`

- `resource_client.go` — generic existence/deploy/delete by resource path.
- `paths.go` — base path fragments + ApiKey parent map.
- `apikey_client.go`, `certificate_client.go`, `subscription_client.go`, `subscription_plan_client.go` — non-standard bodies or responses where needed.
- YAML builders parallel to **`BuildRestAPIYAML`** for envelope-shaped kinds `{ apiVersion, kind, metadata, spec }`.

### 3. Controllers — `internal/controller/`

- `generic_reconciler.go` (shared tracker/finalizers/status semantics).
- One reconciler source file per kind (or small groupings), each watching its CRD + **`APIGateway`** (predicate mirroring **`restapi_controller.go`** enqueue pattern).
- **ApiKey** resolves **`parentRef`** and builds nested URLs.
- Subscription / plan / certificate reconcilers honour **`status.id`**.

### 4. Wiring — `cmd/main.go`

Register each reconciler **`SetupWithManager`** alongside **`NewRestApiReconciler`**.

### 5. RBAC and Helm

- **`+kubebuilder:rbac`** for each plural resource + subresources + finalizers + **`secrets` get** where resolution is needed.
- Regenerate **`config/rbac/role.yaml`** and align **`kubernetes/helm/operator-helm-chart/templates/clusterrole.yaml`** as in existing practice.
- Ship CRD YAML files under **`kubernetes/helm/operator-helm-chart/crds/`** per plural.

### 6. Examples — `gateway/examples/`

Add samples for kinds missing them (e.g. secret, certificate, subscription-plan, subscription, api-key); existing **`llm-*.yaml`**, **`mcp-proxy.yaml`** should align once CRDs land.

### 7. Tests

- Payload marshal tests (golden or structural).
- `SecretValueSource` resolution (happy path + missing Secret).
- ApiKey parent path matrix.
- UUID persistence for the three UUID-keyed kinds.

---

## Out of scope (this feature branch)

- **Gateway API** controllers and **`APIPolicy`** path (they remain orthogonal).
- **`RestApi`** migration onto the generic reconciler (**follow-up PR**).

