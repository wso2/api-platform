# Gateway API feature — implementation notes

This document is a **short maintainer index** for where code and behaviour live. The **single self-contained** write-up (problem statement, configuration, reconcilers, annotations, RBAC, demo, troubleshooting—intended for GitHub Discussions without depending on other repo docs) is **GITHUB_DISCUSSION_GATEWAY_API.md** in this folder.

## Goals

- **Gateway (standard API):** Same *infrastructure* role as `APIGateway` — deploy the platform gateway via Helm, discover the gateway-controller **Service**, register it in the in-memory **GatewayRegistry** (no dependency on an `APIGateway` CR for this path).
- **HTTPRoute:** Same *API* role as `RestApi` — build an `api.yaml`-compatible payload (`APIConfigData`) and call gateway-controller **REST** (`POST`/`PUT` `/api/management/v0.9/rest-apis`, `DELETE` `/api/management/v0.9/rest-apis/{handle}`).
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
| Policy loading (APIPolicy) | `internal/controller/httproute_policies.go` |
| **`params.valueFrom` → Secret / ConfigMap resolution** (before REST) | `internal/controller/httproute_policy_params_resolve.go` |
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

#### `spec.infrastructure` → gateway-runtime Service

`Gateway.spec.infrastructure` is the Gateway API's standard place for "metadata to apply to any resources created in response to this Gateway". The operator honors it as follows, overlayed onto the gateway-runtime Service:

| Source on the `Gateway` | Applied override |
| ----------------------- | ---------------- |
| `spec.infrastructure.annotations["gateway.api-platform.wso2.com/service-type"]` | `gateway.gatewayRuntime.service.type` (must be one of `ClusterIP`, `NodePort`, `LoadBalancer`, `ExternalName`; reconcile fails fast on invalid values) |
| `spec.infrastructure.annotations` (all other keys) | Deep-merged into `gateway.gatewayRuntime.service.annotations` (e.g. `service.beta.kubernetes.io/aws-load-balancer-*` flows through naturally) |
| `spec.infrastructure.labels` | Deep-merged into `gateway.gatewayRuntime.service.labels` |

Example:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
spec:
  infrastructure:
    annotations:
      gateway.api-platform.wso2.com/service-type: LoadBalancer
      service.beta.kubernetes.io/aws-load-balancer-type: external
      service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
    labels:
      team: platform
```

The reserved `service-type` key is consumed by the operator and **not** copied to the Service's `metadata.annotations`.

#### `spec.listeners[]` → Helm router and Service ports

The operator reads `Gateway.spec.listeners[]` and automatically overrides the Helm values the deployed gateway runtime uses, both for Envoy's listener bindings and for the `gateway-runtime` Kubernetes `Service` that sits in front of it:

| Listener match | Applied overrides |
| -------------- | ----------------- |
| First listener with `protocol: HTTP` | `gateway.config.router.listener_port: <port>` **and** `gateway.gatewayRuntime.service.ports.http: <port>` |
| First listener with `protocol: HTTPS` | `gateway.config.router.https_enabled: true`, `gateway.config.router.https_port: <port>` **and** `gateway.gatewayRuntime.service.ports.https: <port>` |
| No listener with `protocol: HTTPS` | `gateway.config.router.https_enabled: false` (HTTPS is turned off; existing `https_port` / `service.ports.https` are left untouched) |
| Listeners with any other protocol (TCP, UDP, TLS) | Ignored |

Keeping the router port and the Service port in sync is required: the gateway-runtime Service template uses `port` as the implicit `targetPort`, so the Service `port` has to equal the port Envoy actually binds to inside the pod. By applying both keys from a single source (`spec.listeners[].port`), the Gateway CR becomes the sole source of truth for addressing.

The overlay is applied **after** the ConfigMap overlay, so Gateway listener ports always win. Non-port values in the Helm values ConfigMap (replicas, image, controller config, TLS/cert wiring, etc.) are preserved.

### `HTTPRoute`

| Annotation | Meaning |
| ---------- | ------- |
| `gateway.api-platform.wso2.com/api-version` | `APIConfigData.Version` (default `v1.0`). |
| `gateway.api-platform.wso2.com/context` | Overrides API **context** path. |
| `gateway.api-platform.wso2.com/display-name` | Overrides display name (default: route `metadata.name`). |
| `gateway.api-platform.wso2.com/project-id` | User-defined metadata; **all** `HTTPRoute` annotations are copied verbatim into the gateway-controller `api.yaml` payload under `metadata.annotations` (same keys as on the route). |
| `gateway.api-platform.wso2.com/api-handle` | REST handle for `/api/management/v0.9/rest-apis/{handle}` (default: `{namespace}-{name}` with `/` stripped). |
| *(no HTTPRoute policy annotations)* | Policy attachment is via `APIPolicy` only (API-level when `spec.targetRef` is set; rule-scope via `ExtensionRef` when `targetRef` is omitted). |

### `APIPolicy` CR (`gateway.api-platform.wso2.com/v1alpha1`)

Recommended way to attach policies for **HTTPRoute**-backed APIs (demo: `kubernetes/helm/resources/gateway-api-httproute-policies-demo/`).

| Field | Meaning |
| ----- | ------- |
| `spec.targetRef` | Optional. When set, **all** `spec.policies` entries are merged into **`APIConfigData.policies`** (API-level) for the named **`HTTPRoute`** (`group: gateway.networking.k8s.io`, `kind: HTTPRoute`, `name`, optional `namespace`). Multiple `APIPolicy` objects are ordered by **metadata.name**. When omitted, the CR is **not** loaded as API-level; use rule **`ExtensionRef`** only. |
| `spec.policies` | Non-empty array of **`Policy`**-shaped entries (`name`, `version`, optional `executionCondition`, `params`) — same logical shape as `RestApi.spec.policies`. |

**Per-rule attachment:** `HTTPRouteFilter` with `type: ExtensionRef`, `group: gateway.api-platform.wso2.com`, `kind: APIPolicy`, `name: <metadata.name>`. The referenced CR must exist in the HTTPRoute namespace. If the `APIPolicy` has **`spec.targetRef`**, it must match that route; if **`targetRef`** is omitted, the policy is rule-attached only. **All** entries in `spec.policies` are appended to the operations derived from that rule’s matches. Rules **without** `ExtensionRef` get **no** rule-scoped policies (API-level only, if any).

### `params` and external values (`valueFrom`)

Policy `params` may reference Kubernetes `Secret` or `ConfigMap` data using the same shape as PodSpec `env[].valueFrom`:

```yaml
params:
  subscriptionKeyHeader:
    valueFrom:
      secretKeyRef:
        name: demo-creds
        key: subscriptionKey
        # namespace: <optional; defaults to the APIPolicy namespace>
  region:
    valueFrom:
      configMapKeyRef:
        name: demo-config
        key: region
```

**Rules (enforced by the resolver in `httproute_policy_params_resolve.go`):**

- The `valueFrom` object must contain **exactly one** of `secretKeyRef` or `configMapKeyRef`; unknown sibling keys are rejected.
- The selected ref must have non-empty `name` and `key`; unknown fields on the ref are rejected.
- `namespace` is optional and defaults to the **APIPolicy** namespace (the operator only resolves within the same cluster; cross-namespace is permitted by the resolver but callers are responsible for authorization).
- `secretKeyRef` reads from `Secret.data[key]`.
- `configMapKeyRef` reads from `ConfigMap.data[key]`, falling back to `ConfigMap.binaryData[key]` (the bytes are inlined as a UTF-8 string).
- A missing Secret / ConfigMap or missing key yields a **transient** reconcile error (requeue).

Before **`DeployRestAPI`**, the operator **resolves** each `valueFrom` to a single **string** value replacing the `{ valueFrom: {...} }` object in the JSON tree, so gateway-controller sees the same JSON types as inline `RestApi` policies (e.g. `subscriptionKeyHeader` as a string, not an object).

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
5. Build `APIConfigData` (policies from **`APIPolicy`** CRs and rule **`ExtensionRef`s** — see `httproute_mapper.go` / `httproute_policies.go`).
6. **`resolveAPIConfigPolicyParamsValueFrom`** — replace `params.valueFrom.secretKeyRef` / `configMapKeyRef` blobs with string values from **Secrets** / **ConfigMaps** (`httproute_policy_params_resolve.go`).
7. Serialize → YAML via `gatewayclient.BuildRestAPIYAML` (`apiVersion` `gateway.api-platform.wso2.com/v1alpha1`, `Kind` `RestApi`).
8. Auth: `GetAuthSettingsForRegistryGateway` (Helm values ConfigMap on `GatewayInfo` if set, else `APIGateway` CR with same name if present).
9. `RestAPIExists` + `DeployRestAPI`; update **`status.parents`** entry with `ControllerName` `gateway.api-platform.wso2.com/gateway-operator`.
10. **Deletion:** `DeleteRestAPI` for the handle, then remove finalizer.

### Watches (enqueue `HTTPRoute` reconcile)

| Watch | Behaviour |
| ----- | --------- |
| **Service** | On create/update/delete, list **`HTTPRoute`s** and enqueue those whose **backendRefs** reference that Service (namespace + name). |
| **`APIPolicy`** | On create/update/delete, enqueue the **`HTTPRoute`** named in **`spec.targetRef`** when set; otherwise enqueue HTTPRoutes in the same namespace that reference this policy via rule **`ExtensionRef`**. Ensures policy CR edits redeploy without mutating the route. |
| **Secret** | On create/update/delete (predicate skips **`kubernetes.io/service-account-token`**), list **`APIPolicy`** cluster-wide; if any **`spec.policies[].params`** JSON references the Secret via **`valueFrom.secretKeyRef`** (see `apiPolicyReferencesValueFrom` / tree walk), enqueue the affected HTTPRoute(s) (via **`targetRef`** or **`ExtensionRef`**). Ensures credential rotation triggers redeploy. |
| **ConfigMap** | Symmetric to the **Secret** watch: on create/update/delete, enqueue HTTPRoutes whose APIPolicy `params` reference this ConfigMap via **`valueFrom.configMapKeyRef`**. Enables live reload of non-sensitive policy inputs (e.g. model pricing JSON, regional config). |

`SetupWithManager` wires all four in `httproute_controller.go`.

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
