# \[Feature\] Kubernetes Gateway API support in Gateway Operator

> **Suggested GitHub discussion title:** `Kubernetes Gateway API (Gateway + HTTPRoute) support in Gateway Operator` — paste the body below into the api-platform repo **Discussions** (e.g. category **Ideas** or **General**).

---

## Summary

The **gateway-operator** reconciles standard **Kubernetes Gateway API** resources (`gateway.networking.k8s.io` **Gateway** and **HTTPRoute**) alongside the existing **`APIGateway`** and **`RestApi`** CRDs.

- **Gateway:** Same *infrastructure* role as **`APIGateway`** — deploy the platform gateway via Helm, discover the gateway-controller **Service**, register it in the in-memory **GatewayRegistry** (no dependency on an `APIGateway` CR for this path).
- **HTTPRoute:** Same *API* role as **`RestApi`** — build an `api.yaml`-compatible payload (`APIConfigData`) and call gateway-controller **REST** (`POST`/`PUT` `/rest-apis`, `DELETE` `/rest-apis/{handle}`).
- **Service / `APIPolicy` / Secret:** **HTTPRoute** resolution plus **watches** on **Service**, **`APIPolicy`**, and **Secret** enqueue routes when backends, policy CRs, or referenced Secret data change.
- **`APIPolicy` CR:** Gateway API–only (`gateway.api-platform.wso2.com/v1alpha1`); **`spec.policies`** array; optional **`spec.targetRef`** (**HTTPRoute**) for API-level merge; omit **`targetRef`** and attach via rule **`ExtensionRef`** for rule/resource scope. **`params.valueFrom`** is resolved from Secrets before gateway-controller REST. Does **not** change `RestApi` / `APIGateway` reconciliation.

---

## Problem statement

Teams adopting **Gateway API** as the cluster-native way to declare gateways and HTTP routes needed a first-class path to drive the **API Platform gateway** without requiring **`APIGateway` / `RestApi`** CRDs for the same flow. The operator previously only reconciled WSO2 custom resources; there was no bridge from **`Gateway` + `HTTPRoute`** to the existing gateway-controller deployment and API lifecycle.

---

## Who is this for?

- **Platform engineers** standardizing on Gateway API (GKE, EKS, shared gateways, GitOps).
- **API teams** who want HTTP routing and backend **`Service`** references expressed as **`HTTPRoute`** while reusing the same gateway runtime and controller APIs.
- **Operators** already running **`APIGateway`** who want parity for Gateway API–native manifests.

---

## Why does this matter?

- **One gateway stack**, two API shapes: custom CRDs **or** standard Gateway API, both backed by the same Helm release and registry.
- **Familiar primitives** for Kubernetes users (`GatewayClass`, `Gateway`, `HTTPRoute`) without giving up platform features (policies, xDS, gateway-controller REST).
- **Clear extension points** via annotations for Helm values, API handle, context, and optional control-plane hints.

---

## Use cases

| Use case | What you do |
| -------- | ----------- |
| Deploy platform gateway from Gateway API | Create a **`Gateway`** whose `gatewayClassName` is on the operator allowlist; optional per-Gateway Helm values via ConfigMap annotation. |
| Publish an API from a route | Create **`HTTPRoute`** with `parentRefs` → your **`Gateway`**; **`backendRefs`** → **`Service`**; operator builds and deploys **`RestApi`–equivalent** YAML to gateway-controller. |
| Refresh route when backends change | Operator watches **`Service`**; relevant **`HTTPRoute`s** re-queue when backend Services change. |
| Refresh route when policies or secrets change | Operator watches **`APIPolicy`** (target HTTPRoute) and **`Secret`** (when referenced from `APIPolicy` `params` via **`valueFrom`**). |
| Remove API with the route | Delete **`HTTPRoute`**; finalizer removes the REST API handle from gateway-controller. |

---

## Goals (technical)

- **Gateway (standard API):** Same *infrastructure* role as `APIGateway` — deploy the platform gateway via Helm, discover the gateway-controller **Service**, register it in **GatewayRegistry**.
- **HTTPRoute:** Same *API* role as `RestApi` — build `APIConfigData` and sync over gateway-controller REST.
- **Watches:** **`HTTPRoute`** controller re-queues routes when referenced **Services**, related **`APIPolicy`** objects, or **Secrets** referenced from policy params change (see implementation notes).

---

## Prerequisites (cluster and install)

### Gateway API CRDs

The operator Helm chart ships Gateway API **standard channel v1.3.0** YAML under **`files/gateway-api-standard/`** and only applies them when **`gatewayApi.installStandardCRDs`** is **`true`** (`templates/gateway-api-crds.yaml`). The default is **`false`** because many clusters already have Gateway API (or a prior Helm release managed the same CRDs); installing again causes **server-side apply conflicts** on fields such as `metadata.annotations.gateway.networking.k8s.io/bundle-version` and `spec.versions`. The WSO2 CRDs (`APIGateway`, `RestApi`, `APIPolicy`) remain in **`crds/`** and are always installed with the chart.

- **Greenfield** cluster with no Gateway API: install the operator with `gatewayApi.installStandardCRDs=true` (exact flag depends on your `helm install` / values).
- **CRDs already exist** (including when another Helm release owns them, e.g. a `*crd*` release in `kube-system`): keep **`installStandardCRDs=false`**. Turning it on can fail with *cannot be imported into the current release / invalid ownership metadata* because Helm will not adopt CRDs from another release.
- **CRDs already exist:** keep the default `false` and use the cluster’s existing Gateway API version (ensure it is compatible with the operator’s **`sigs.k8s.io/gateway-api`** dependency in `go.mod`).
- Bundled CRD kinds include: `gatewayclasses`, `gateways`, `httproutes`, `referencegrants`, `grpcroutes` (**GRPCRoute** is not reconciled by the operator today).
- To upgrade bundled CRDs: replace files under **`files/gateway-api-standard/`** from a newer upstream `standard-install.yaml` and bump the operator `go.mod` dependency to match.

Create a **GatewayClass** whose **`metadata.name`** matches operator configuration (default managed class: **`wso2-api-platform`**).

---

## Configuration

| Mechanism | Purpose |
| --------- | ------- |
| `gateway_api.gateway_class_names` in operator `config.yaml` | List of `spec.gatewayClassName` values the operator **owns**. The operator Helm chart writes this from **`gatewayApi.managedGatewayClassNames`** in `values.yaml`. |
| Code / merge default | `wso2-api-platform` when the key is absent after config merge (`internal/config/config.go`). |
| `GATEWAY_API_GATEWAY_CLASS_NAMES` | Comma-separated **environment** override of that list. |

**Resolution:** `OperatorConfig.ManagedGatewayClass(name string)` returns whether a class is managed.

Only **Gateway** objects whose `spec.gatewayClassName` is in this list are managed. **HTTPRoute** objects are processed when their parent **Gateway** uses one of these classes.

Example `config.yaml` fragment:

```yaml
gateway_api:
  gateway_class_names:
    - wso2-api-platform
```

Environment variable example: `GATEWAY_API_GATEWAY_CLASS_NAMES=wso2-api-platform,my-class`

**Helm / registry** for Kubernetes `Gateway` uses the same `gateway.*` Helm settings as `APIGateway` (`internal/config` → `GatewayConfig`: chart name, version, values file, registry credentials, etc.).

---

## Annotations

### `Gateway` (`gateway.networking.k8s.io`)

| Annotation | Meaning |
| ---------- | ------- |
| `gateway.api-platform.wso2.com/helm-values-configmap` | Name of a ConfigMap in the Gateway namespace whose data includes **`values.yaml`** (Helm values), analogous to `APIGateway.spec.configRef`. |
| `gateway.api-platform.wso2.com/api-selector` | Optional JSON for `APISelector` (same shape as on `APIGateway`) — which `RestApi` CRs logically associate with this deployment. |
| `gateway.api-platform.wso2.com/control-plane-host` | Optional; stored on `GatewayInfo.ControlPlaneHost` in the registry. |

If the Helm values ConfigMap annotation is **omitted**, the operator uses the default Helm values file from config (same pattern as `APIGateway` without `configRef`).

### `HTTPRoute`

| Annotation | Meaning |
| ---------- | ------- |
| `gateway.api-platform.wso2.com/api-version` | `APIConfigData.Version` (default `v1`). |
| `gateway.api-platform.wso2.com/context` | Overrides API **context** path. |
| `gateway.api-platform.wso2.com/display-name` | Overrides display name (default: route `metadata.name`). |
| `gateway.api-platform.wso2.com/api-handle` | REST handle for `/rest-apis/{handle}` (default: `{namespace}-{name}` with `/` stripped). |
| *(no HTTPRoute policy annotations)* | Policy attachment is via `APIPolicy` only (API-level when `spec.targetRef` is set; rule-scope via `ExtensionRef` when `targetRef` is omitted). |

**`APIPolicy` CR**: `spec.policies` → array of `Policy`-shaped entries. **API-level:** set `spec.targetRef` to the `HTTPRoute`. **Rule-level:** omit `spec.targetRef`; reference from `spec.rules[].filters` with `ExtensionRef` (`group: gateway.api-platform.wso2.com`, `kind: APIPolicy`).

**Sensitive params:** Nested `{ "valueFrom": { "name", "valueKey" [, "namespace"] } }` in `params` is resolved from **`Secret.data`** before **`DeployRestAPI`** so gateway-controller sees plain strings (same as `RestApi` inline policies).

---

## Reconciler behaviour

### Kubernetes `Gateway`

1. Ignore resources whose `spec.gatewayClassName` is not in the managed list.
2. Finalizer: `gateway.api-platform.wso2.com/k8s-gateway-finalizer`.
3. **Install/upgrade** Helm via `helmgateway.InstallOrUpgrade` (release name **`{metadata.name}-gateway`**, same pattern as `APIGateway`).
4. **Register** controller endpoint via `registerGatewayInRegistry` (discovery by labels `app.kubernetes.io/instance` + `component=controller`).
5. Wait for Deployments ready (`evaluateGatewayDeploymentsReady`); requeue on failure.
6. Patch **`Gateway.status.conditions`**: `Accepted` and `Programmed` (Gateway API condition types).
7. **Deletion:** `registry.Unregister`, `helmgateway.Uninstall`, remove finalizer.

### `HTTPRoute`

1. Resolve **parent** `Gateway` from `spec.parentRefs` (`Kind` `Gateway`, `Group` `gateway.networking.k8s.io` or unset / default handling as implemented).
2. Load parent Gateway; confirm managed **gatewayClassName**.
3. Finalizer: `gateway.api-platform.wso2.com/httproute-finalizer`.
4. **Registry lookup** by parent `namespace/name` (not label-based `RestApi` matching).
5. Build `APIConfigData` (**`APIPolicy`**, rule ExtensionRefs).
6. Resolve **`params.valueFrom`** using **Secrets** (replace with string values for gateway-controller).
7. Serialize → YAML via `gatewayclient.BuildRestAPIYAML` (`apiVersion` `gateway.api-platform.wso2.com/v1alpha1`, `Kind` `RestApi`).
8. **Auth:** `GetAuthSettingsForRegistryGateway` (Helm values ConfigMap on `GatewayInfo` if set, else `APIGateway` CR with same name if present).
9. `RestAPIExists` + `DeployRestAPI`; update **`status.parents`** with `ControllerName` **`gateway.api-platform.wso2.com/gateway-operator`**.
10. On success, **info** log: `HTTPRoute deployed to gateway` (includes handle and endpoint, as implemented).
11. **Deletion:** `DeleteRestAPI` for the handle, then remove finalizer.

### Watches (HTTPRoute controller)

- **Services:** On create/update/delete, list `HTTPRoute`s and enqueue those whose **backendRefs** reference that Service.
- **`APIPolicy`:** Enqueue the HTTPRoute in **`spec.targetRef`** when set; otherwise enqueue HTTPRoutes in the same namespace that reference this policy via rule **`ExtensionRef`**.
- **Secrets:** When a Secret changes (except service-account token type), enqueue HTTPRoutes affected by any **`APIPolicy`** whose `params` reference that Secret via **`valueFrom`** (target HTTPRoute or ExtensionRef-only policies).

---

## RBAC

ClusterRole rules include `gateway.networking.k8s.io` **gateways** and **httproutes** (including **status** and **finalizers**), core **services**, **configmaps**, **secrets**, and `gateway.api-platform.wso2.com` **apipolicies** as required for mapping, resolution, and watches (`config/rbac/role.yaml`).

---

## Coexistence and naming

- Registry key is **`namespace/name`** of the logical gateway **CR** (`APIGateway` name or Kubernetes `Gateway` name — **not** the Helm release name).
- **Collision risk:** An `APIGateway` and a Kubernetes `Gateway` with the **same** `metadata.name` and `metadata.namespace` share the same registry slot — avoid duplicate names if both models are used in one namespace.

---

## MVP limitations

- **HTTPRoute → APIConfigData:** Oriented to **HTTP** routes; single-backend assumptions in places (first resolving **Service** `backendRef` drives `upstream.main.url`).
- Advanced route **filters** (rewrite, redirect, weighted backends), **GRPCRoute**, and rich **TLS/vhost** mapping into `APIConfigData` are out of scope unless extended.
- Cross-namespace backends without **ReferenceGrant** are not fully modelled; errors surface via reconcile / status where possible.

---

## Main code layout

| Area | Location |
| ---- | -------- |
| Gateway API scheme registration | `cmd/main.go` (`gatewayv1.AddToScheme`) |
| Kubernetes `Gateway` reconciler | `internal/controller/k8s_gateway_controller.go` |
| `HTTPRoute` reconciler | `internal/controller/httproute_controller.go` |
| Service / APIPolicy / Secret → HTTPRoute enqueue | `internal/controller/httproute_enqueue.go` |
| HTTPRoute → `APIConfigData` mapping | `internal/controller/httproute_mapper.go` |
| Policy loading | `internal/controller/httproute_policies.go` |
| `valueFrom` Secret resolution | `internal/controller/httproute_policy_params_resolve.go` |
| **`APIPolicy` CRD** | `api/v1alpha1/policy_types.go` |
| Annotation keys | `internal/controller/gateway_api_annotations.go` |
| Shared Helm install/uninstall | `internal/helmgateway/deploy.go` |
| Shared manifest/registry helpers | `internal/controller/gateway_infra.go` |
| REST payload + HTTP calls | `internal/gatewayclient/` |
| Registry extensions (`HelmValuesConfigMapName`, `FromGatewayAPI`) | `internal/registry/gateway_registry.go` |
| Auth: ConfigMap vs `APIGateway` | `internal/auth/auth_helper.go` (`GetAuthSettingsForRegistryGateway`, `GetDeploymentAuthFromConfigMap`) |
| `RestApi` path (shared client) | `internal/controller/restapi_controller.go` |