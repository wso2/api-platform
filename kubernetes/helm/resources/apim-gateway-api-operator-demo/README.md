# Gateway API + APIM control plane demo

This walkthrough exercises the operator path that uses **Kubernetes Gateway API** resources with an APIM control plane:

1. Deploy a **platform gateway** runtime (Helm release) from a **`Gateway`** (`gateway.networking.k8s.io`).
2. Configure APIM control plane connectivity via per-Gateway Helm values.
3. Push an API definition to **gateway-controller** from an **`HTTPRoute`**, using a **`Service`** as the upstream backend.

It complements the **`APIGateway` + `RestApi`** CRD flow documented elsewhere.

**Related:** For **HTTPRoute policies** using **`APIPolicy` CRs** (`spec.targetRef` for API-level, or rule `ExtensionRef` when `targetRef` is omitted), see [`../gateway-api-httproute-policies-demo/README.md`](../gateway-api-httproute-policies-demo/README.md) (apply after this demo’s Gateway and backend are ready).

## Prerequisites

- **Gateway Operator** installed from [`operator-helm-chart`](../../operator-helm-chart) (or equivalent), with RBAC including `gateway.networking.k8s.io`.
- **Gateway API CRDs** present in the cluster.
- Operator **`gateway_api.gateway_class_names`** includes **`wso2-api-platform-apim`** (used by this demo’s GatewayClass/Gateway).
- APIM control plane reachable from the cluster with a valid registration token.
- Operator **`config.yaml`** points to a valid gateway chart and a default `gateway_values.yaml`; this demo provides only per-Gateway overrides via ConfigMap.

## What gets created

| Step | Resource | Role |
|------|----------|------|
| 1 | `Namespace` | Isolates demo resources (`gateway-api-demo-apim`). |
| 2 | `GatewayClass` | Defines class `wso2-api-platform-apim` referenced by the `Gateway`. |
| 3 | `ConfigMap` | Per-Gateway Helm values override (`values.yaml`) for APIM host/port/token and development mode. |
| 4 | `Gateway` | Triggers operator Helm install `{metadata.name}-gateway` and references ConfigMap via `gateway.api-platform.wso2.com/helm-values-configmap`. |
| 5 | `Deployment` + `Service` | Demo backend (`hello-backend-apim`) used as upstream. |
| 6 | `HTTPRoute` | Mapped to `APIConfigData` and synced to gateway-controller `/rest-apis`. |

## Policies (HTTPRoute)

Policies use the same logical shape as **`RestApi`** (`name`, `version` required; optional `params`, `executionCondition`) and are merged into the payload before **`/rest-apis`** is called.

**Recommended:** use **`APIPolicy`** CRs (`gateway.api-platform.wso2.com/v1alpha1`):

- **Policies list:** **`spec.policies`** is a non-empty array of policy instances (same fields as RestApi embedded `policies`).
- **API-level:** set **`spec.targetRef`** to the HTTPRoute (`gateway.networking.k8s.io`, kind `HTTPRoute`) — all `spec.policies` entries are merged into `APIConfigData.policies`.
- **Rule scope:** omit **`spec.targetRef`** on the `APIPolicy` and reference it from **`spec.rules[].filters`** with **`type: ExtensionRef`**, `group: gateway.api-platform.wso2.com`, `kind: APIPolicy`, `name: <metadata.name>` — all `spec.policies` entries apply to that rule’s operations.

Full manifests: [`gateway-api-httproute-policies-demo`](../gateway-api-httproute-policies-demo/README.md).

Malformed policy configuration can surface as **`ResolvedRefs=False`** / **`Invalid`** on the HTTPRoute. Ensure policy **names/versions** exist in your gateway deployment.

## Apply (order matters)

Use namespace **`gateway-api-demo-apim`** (or change `namespace` consistently in every file).

```bash
cd kubernetes/helm/resources/apim-gateway-api-operator-demo

kubectl apply -f 00-namespace.yaml
kubectl apply -f 01-gatewayclass.yaml
kubectl apply -f 01-gateway-values-configmap.yaml
kubectl apply -f 02-gateway.yaml
kubectl apply -f 03-backend.yaml
# Wait until Helm workloads for the gateway are Ready (see Verification).
kubectl apply -f 04-httproute.yaml
```

## Verification

1. **Gateway status** (Gateway API conditions):

   ```bash
   kubectl get gateway platform-gw-apim -n gateway-api-demo-apim -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
   ```

   Expect `Accepted=True` and `Programmed=True` once Helm workloads are ready.

2. **Helm release and workloads** (release name `platform-gw-apim-gateway`):

   ```bash
   helm list -n gateway-api-demo-apim
   kubectl get deploy,svc,pods -n gateway-api-demo-apim -l 'app.kubernetes.io/instance=platform-gw-apim-gateway'
   ```

3. **HTTPRoute status** (parent + resolved refs):

   ```bash
   kubectl get httproute hello-api-apim -n gateway-api-demo-apim -o yaml
   ```

4. **Invoke the routed API** (through gateway-runtime Service):

   ```bash
   curl --request GET \
     --url https://localhost:8443/hello-context-apim/hello \
     --header 'Accept: application/json' -k
   ```

5. **Controller APIM token wiring** (important for this flow):

   ```bash
   kubectl logs deploy/platform-gw-apim-gateway-controller -n gateway-api-demo-apim | rg "control_plane_token_configured|Control plane token"
   ```

   Expect `control_plane_token_configured:true` and no repeated `Control plane token not configured, skipping connection`.

## Customizing Helm values per Gateway

To use different Helm values for this `Gateway` than the operator’s global default file, create a **ConfigMap** in **`gateway-api-demo-apim`** with key **`values.yaml`** (not `gateway_values.yaml`) and set annotation:

```yaml
metadata:
  annotations:
    gateway.api-platform.wso2.com/helm-values-configmap: "<configmap-name>"
```

See [GATEWAY_API_IMPLEMENTATION_NOTES](../../../gateway-operator/docs/GATEWAY_API_IMPLEMENTATION_NOTES.md) for all annotations.

## Troubleshooting

| Symptom | Check |
|--------|--------|
| `Gateway` never Programmed | `kubectl describe gateway platform-gw-apim -n gateway-api-demo-apim`; operator logs; Helm chart/image pull; `GatewayClass` must match operator `managedGatewayClassNames`. |
| Controller logs `control_plane_token_configured:false` | Ensure ConfigMap uses key `values.yaml` and path `gateway.controller.controlPlane.token.value` (camelCase `controlPlane`). Re-apply `01-gateway-values-configmap.yaml` and `02-gateway.yaml`. |
| Controller logs `Control plane token not configured, skipping connection` | Verify merged Helm values and deployment env var `APIP_GW_GATEWAY_REGISTRATION_TOKEN` in `platform-gw-apim-gateway-controller`. |
| APIM still unreachable | Verify `gateway.controller.controlPlane.host` and `port` values and network reachability from cluster/pods. |
| Helm fails with `gateway.controller.encryptionKeys must be enabled when gateway.developmentMode is false` | Keep `gateway.developmentMode: true` for demo, or configure `gateway.controller.encryptionKeys` for production-like setups. |
| `HTTPRoute` stuck, parent not accepted | Parent `Gateway` must be same namespace or `parentRefs` namespace set; registry only after Gateway sync succeeds. |
| `RestApi` deploys to Kubernetes `Gateway` release instead of `APIGateway` | Ensure this `Gateway` has `gateway.api-platform.wso2.com/api-selector` annotation and `RestApi` labels use a different `gateway.api-platform.wso2.com/restapi-target` value in mixed demos. |
| Wrong API paths | Set annotation `gateway.api-platform.wso2.com/context` on `HTTPRoute`; paths must satisfy `APIConfigData` validation rules. |
| Edited `custom-gateway-values-apim` but nothing changed | Reconcile the Gateway (re-apply `02-gateway.yaml` or update Gateway metadata) so operator reruns Helm with updated merged values. |

## Files

- `00-namespace.yaml` — namespace `gateway-api-demo-apim`
- `01-gatewayclass.yaml` — class `wso2-api-platform-apim`
- `01-gateway-values-configmap.yaml` — per-Gateway Helm values (`developmentMode`, `gateway.controller.controlPlane.*`, and `gateway.config.controller.controlplane.insecure_skip_verify`)
- `02-gateway.yaml` — `Gateway` `platform-gw-apim` + annotation to use `custom-gateway-values-apim`
- `03-backend.yaml` — `ghcr.io/wso2/api-platform/sample-service:latest` Deployment + ClusterIP Service `hello-backend-apim` (port `9080`)
- `04-httproute.yaml` — `HTTPRoute` `hello-api-apim` (`PathPrefix /hello`) with API metadata annotations (`api-version`, `context`, `display-name`)
