# Gateway API feature — implementation notes

This document is a **short maintainer index** for where code and behaviour live. The **single self-contained** write-up (problem statement, configuration, reconcilers, annotations, RBAC, demo, troubleshooting—intended for GitHub Discussions without depending on other repo docs) is **GITHUB_DISCUSSION_GATEWAY_API.md** in this folder.

## Goals

- **Gateway (standard API):** Same *infrastructure* role as `APIGateway` — deploy the platform gateway via Helm, discover the gateway-controller **Service**, register it in the in-memory **GatewayRegistry** (no dependency on an `APIGateway` CR for this path).
- **HTTPRoute:** Same *API* role as `RestApi` — build an `api.yaml`-compatible payload (`APIConfigData`) and call gateway-controller **REST** (`POST`/`PUT` `/rest-apis`, `DELETE` `/rest-apis/{handle}`).
- **Service / APIPolicy / Secret:** Not reconciled as APIs themselves; **HTTPRoute** resolution plus **watches** on **Service**, **`APIPolicy`**, and **Secret** enqueue affected routes when backends, policy CRs, or referenced Secret data change.
- **`APIPolicy` CR** (`gateway.api-platform.wso2.com/v1alpha1`, plural **`apipolicies`**): Gateway API–only policy attachment; **not** used by `RestApi` / `APIGateway` reconciliation. RestApi continues to embed `Policy` inline on the CR spec.

## Prerequisites (cluster)

- **Gateway API CRDs:** The operator Helm chart ships Gateway API **standard channel v1.3.0** YAML under **`files/gateway-api-standard/`** and only applies them when **`gatewayApi.installStandardCRDs`** is **`true`** (`templates/gateway-api-crds.yaml`). Defaults to **`false`** because many clusters already have Gateway API (or a prior Helm release managed the same CRDs); installing again causes **server-side apply conflicts** on fields such as `metadata.annotations.gateway.networking.k8s.io/bundle-version` and `spec.versions`. WSO2 CRDs (**`APIGateway`**, **`RestApi`**, **`APIPolicy`**) live in **`crds/`** (and the Helm chart **`crds/`** directory) and are installed with the chart.
  - For a **greenfield** cluster with no Gateway API: `helm install ... --set gatewayApi.installStandardCRDs=true`.
  - If CRDs already exist (including when another Helm release owns them, e.g. **`traefik-crd`** in `kube-system`): keep **`installStandardCRDs=false`**. Turning it on will fail with *cannot be imported into the current release / invalid ownership metadata* because Helm will not adopt CRDs from another release.
  - If CRDs already exist: keep the default `false` and use the cluster’s existing Gateway API version (ensure it is compatible with operator `sigs.k8s.io/gateway-api`).
- CRD files included: `gatewayclasses`, `gateways`, `httproutes`, `referencegrants`, `grpcroutes` (GRPCRoute not reconciled by the operator today).
- To upgrade bundled CRDs, replace the files under **`files/gateway-api-standard/`** from a newer `standard-install.yaml` release and bump the operator `go.mod` dependency to match.
- Create a **GatewayClass** whose name matches operator configuration (see below).

**Hands-on demo:** Manifests live under `kubernetes/helm/resources/gateway-api-operator-demo/` (same steps are summarized in **GITHUB_DISCUSSION_GATEWAY_API.md**).

## Configuration

| Mechanism | Purpose |
| --------- | ------- |
| `gateway_api.gateway_class_names` in operator `config.yaml` | List of `spec.gatewayClassName` values the operator **owns**. The operator Helm chart writes this from **`gatewayApi.managedGatewayClassNames`** in `values.yaml`. |
| Code / merge default | `wso2-api-platform` when the key is absent after config merge (`internal/config/config.go`). |
| `GATEWAY_API_GATEWAY_CLASS_NAMES` | Comma-separated env override of that list. |

**Method:** `OperatorConfig.ManagedGatewayClass(name string)` returns whether a class is managed.

**Helm / registry** for Kubernetes `Gateway` uses the same `gateway.*` Helm settings as `APIGateway` (`internal/config` → `GatewayConfig`; chart name, version, values file, registry credentials, etc.).

## Main code layout

| Area | Location |
| ---- | -------- |
| Gateway API scheme registration | `cmd/main.go` (`gatewayv1.AddToScheme`, `apiv1.AddToScheme`) |
| **`APIPolicy` CRD types** | `api/v1alpha1/policy_types.go` |
| Kubernetes `Gateway` reconciler | `internal/controller/k8s_gateway_controller.go` |
| `HTTPRoute` reconciler | `internal/controller/httproute_controller.go` |
| Service / `APIPolicy` / Secret → HTTPRoute enqueue | `internal/controller/httproute_enqueue.go` |
| HTTPRoute → `APIConfigData` mapping | `internal/controller/httproute_mapper.go` |
| Policy loading (APIPolicy, ConfigMap, annotations, operation map) | `internal/controller/httproute_policies.go` |
| **`params` `valueFrom` → Secret resolution** (before REST) | `internal/controller/httproute_policy_params_resolve.go` |
| Annotation / label keys | `internal/controller/gateway_api_annotations.go` |
| Shared Helm install/uninstall | `internal/helmgateway/deploy.go` |
| Shared manifest/registry helpers | `internal/controller/gateway_infra.go` |
| REST payload + HTTP calls | `internal/gatewayclient/` |
| Registry extensions (`HelmValuesConfigMapName`, `FromGatewayAPI`) | `internal/registry/gateway_registry.go` |
| Auth: ConfigMap vs `APIGateway` | `internal/auth/auth_helper.go` (`GetAuthSettingsForRegistryGateway`, `GetDeploymentAuthFromConfigMap`) |
| `RestApi` path (refactored to shared client) | `internal/controller/restapi_controller.go` |

## Annotations

### `Gateway` (gateway.networking.k8s.io)

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
| `gateway.api-platform.wso2.com/api-policies` | Inline JSON/YAML list of `Policy` objects for `APIConfigData.policies`. **Ignored** if `api-policies-configmap` is set. If **both** are unset, API-level policies may be loaded from labeled **`APIPolicy`** CRs (see below). |
| `gateway.api-platform.wso2.com/api-policies-configmap` | ConfigMap name (namespace = HTTPRoute namespace); data key `policies.yaml` \| `policies.yml` \| `policies.json`; **overrides** inline `api-policies` and **skips** ApiLevel `APIPolicy` list loading. |
| `gateway.api-platform.wso2.com/operation-policies` | Map keyed by `METHOD:/path` → policy list for that derived `Operation` (see `HTTPRouteOperationPolicyKey` in code). |
| `gateway.api-platform.wso2.com/operation-policies-configmap` | ConfigMap name (namespace = HTTPRoute namespace); data key `operation-policies.yaml` \| `operation-policies.yml` \| `operation-policies.json`; **overrides** inline `operation-policies`. Map entries append after policies from rule filters / `APIPolicy` ExtensionRefs. |

### `APIPolicy` CR (`gateway.api-platform.wso2.com/v1alpha1`)

Recommended way to attach policies for **HTTPRoute**-backed APIs (demo: `kubernetes/helm/resources/gateway-api-httproute-policies-demo/`).

| Field / label | Meaning |
| ------------- | ------- |
| `spec.targetRef` | **Shared** reference to the target **`HTTPRoute`** (`group: gateway.networking.k8s.io`, `kind: HTTPRoute`, `name`, optional `namespace`). |
| `spec.policies` | Non-empty array of **`Policy`**-shaped entries (`name`, `version`, optional `executionCondition`, `params`) — same logical shape as `RestApi.spec.policies`. |
| Label `gateway.api-platform.wso2.com/policy-scope: ApiLevel` | When set, **all** `spec.policies` entries are merged into **`APIConfigData.policies`** (API-level). Multiple `APIPolicy` objects are ordered by **metadata.name**. |
| *(label omitted or other value)* | Not loaded as API-level; policies apply only when this object is referenced from **`HTTPRoute.spec.rules[].filters`** (`ExtensionRef`). |

**Per-rule attachment:** `HTTPRouteFilter` with `type: ExtensionRef`, `group: gateway.api-platform.wso2.com`, `kind: APIPolicy`, `name: <metadata.name>`. The referenced CR must exist in the HTTPRoute namespace and `spec.targetRef` must match that route. **All** entries in `spec.policies` are appended to the operations derived from that rule’s matches. Rules **without** `ExtensionRef` get **no** rule-scoped policies (API-level only, if any).

**Legacy:** `ExtensionRef` to **core** `ConfigMap` (`group: ""`, `kind: ConfigMap`) with `policies.yaml` \| … is still supported for rule-level lists.

### `params` and Secrets (`valueFrom`)

Policy `params` may use a nested object that is **only** `{ "valueFrom": { "name", "valueKey" [, "namespace"] } }` (same namespace as the HTTPRoute unless `namespace` is set). Before **`DeployRestAPI`**, the operator **resolves** these to **string** values by reading **`Secret.data[valueKey]`**, so gateway-controller receives the same JSON types as inline `RestApi` policies (e.g. `subscriptionKeyHeader` as a string, not an object). Missing Secret or key → **transient** reconcile error.

## Reconciler behaviour (short)

### Kubernetes `Gateway`

1. Ignore resources whose `spec.gatewayClassName` is not in the managed list.
2. Finalizer: `gateway.api-platform.wso2.com/k8s-gateway-finalizer`.
3. **Install/upgrade** Helm via `helmgateway.InstallOrUpgrade` (release name `{metadata.name}-gateway`, same as `APIGateway`).
4. **Register** controller endpoint via `registerGatewayInRegistry` (discovery by labels `app.kubernetes.io/instance` + `component=controller`).
5. Wait for Deployments ready (`evaluateGatewayDeploymentsReady`); requeue on failure.
6. Patch **`Gateway.status.conditions`**: `Accepted` and `Programmed` (Gateway API condition types).
7. **Deletion:** `registry.Unregister`, `helmgateway.Uninstall`, remove finalizer.

### `HTTPRoute`

1. Resolve **parent** `Gateway` from `spec.parentRefs` (`Kind` `Gateway`, `Group` `gateway.networking.k8s.io` or unset/`Kind` default handling as implemented).
2. Load parent Gateway; confirm managed **gatewayClassName**.
3. Finalizer: `gateway.api-platform.wso2.com/httproute-finalizer`.
4. **Registry lookup** by parent `namespace/name` (not label-based `RestApi` matching).
5. Build `APIConfigData` (policies from annotations, ConfigMaps, **`APIPolicy`**, rule **`ExtensionRef`s**, operation maps — see `httproute_mapper.go` / `httproute_policies.go`).
6. **`resolveAPIConfigPolicyParamsSecrets`** — replace `params` `valueFrom` blobs with string values from **Secrets** (`httproute_policy_params_resolve.go`).
7. Serialize → YAML via `gatewayclient.BuildRestAPIYAML` (`apiVersion` `gateway.api-platform.wso2.com/v1alpha1`, `Kind` `RestApi`).
8. Auth: `GetAuthSettingsForRegistryGateway` (Helm values ConfigMap on `GatewayInfo` if set, else `APIGateway` CR with same name if present).
9. `RestAPIExists` + `DeployRestAPI`; update **`status.parents`** entry with `ControllerName` `gateway.api-platform.wso2.com/gateway-operator`.
10. **Deletion:** `DeleteRestAPI` for the handle, then remove finalizer.

### Watches (enqueue `HTTPRoute` reconcile)

| Watch | Behaviour |
| ----- | --------- |
| **Service** | On create/update/delete, list **`HTTPRoute`s** and enqueue those whose **backendRefs** reference that Service (namespace + name). |
| **`APIPolicy`** | On create/update/delete, enqueue the **`HTTPRoute`** named in **`spec.targetRef`** (when group/kind indicate HTTPRoute). Ensures policy CR edits redeploy without mutating the route. |
| **Secret** | On create/update/delete (predicate skips **`kubernetes.io/service-account-token`**), list **`APIPolicy`** cluster-wide; if any **`spec.policies[].params`** JSON references the Secret via **`valueFrom`** (see `apiPolicyReferencesSecret` / tree walk), enqueue that policy’s target HTTPRoute. Ensures credential rotation triggers redeploy. |

`SetupWithManager` wires all three in `httproute_controller.go`.

## RBAC

ClusterRole in `config/rbac/role.yaml` (generated from kubebuilder markers on **`httproute_controller.go`**) includes:

- `gateway.networking.k8s.io` **gateways** and **httproutes** (including **status** / **finalizers**), **referencegrants**
- Core **services**, **configmaps**, **secrets** (`get` / `list` / `watch`; Secret resolution and Secret watch)
- `gateway.api-platform.wso2.com` **apipolicies** (and **apipolicies/status**) for `APIPolicy` informer and status patches
- **`APIGateway` / `RestApi`** rules come from other controllers; HTTPRoute path does not add RestApi reconciliation for `APIPolicy`.

## Coexistence and naming

- Registry key is **`namespace/name`** of the logical gateway **CR** (`APIGateway` name or Kubernetes `Gateway` name — **not** the Helm release name).
- **Collision risk:** An `APIGateway` and a Kubernetes `Gateway` with the **same** `metadata.name` and `metadata.namespace` will overwrite the same registry slot; avoid duplicate names if both models are used in one namespace.

## MVP limitations (intentional)

- **HTTPRoute → APIConfigData:** Geared toward **HTTP** routes; single-backend assumptions in places (first resolving **Service** `backendRef` drives `upstream.main.url`).
- Advanced route **filters** (rewrite, redirect, weighted backends), **GRPCRoute**, and rich **TLS/vhost** mapping into `APIConfigData` are out of scope unless extended.
- Cross-namespace backends without **ReferenceGrant** are not fully modelled; errors should surface via reconcile / status where possible.

## Testing

- Mapper / policy validation: `internal/controller/httproute_mapper_test.go`
- Enqueue / Secret reference detection: `internal/controller/httproute_enqueue_test.go`
- `valueFrom` resolution: `internal/controller/httproute_policy_params_resolve_test.go`
- Run: `GOWORK=off go test ./...` from `kubernetes/gateway-operator` (repo **`go.work`** may not list this module).

## Further reading

- Operator-wide config and env vars: **CONFIGURATION.md** (Kubernetes Gateway API section).
- Original design intent: internal planning doc *Gateway API reconciliation plan* (not shipped in repo).
