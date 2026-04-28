# Gateway API flow — demo (Gateway, HTTPRoute, Service)

This walkthrough exercises the operator path that uses **Kubernetes Gateway API** resources to:

1. Deploy a **platform gateway** runtime (Helm release) from a **`Gateway`** (`gateway.networking.k8s.io`).
2. Push an API definition to **gateway-controller** from an **`HTTPRoute`**, using a **`Service`** as the upstream backend.

It complements the **`APIGateway` + `RestApi`** CRD flow documented elsewhere.

**Related:** For **HTTPRoute policies** using **`APIPolicy` CRs** (`spec.targetRef` for API-level, or rule `ExtensionRef` when `targetRef` is omitted), see [`../gateway-api-httproute-policies-demo/README.md`](../gateway-api-httproute-policies-demo/README.md) (apply after this demo’s Gateway and backend are ready).

## Prerequisites

- **cert-manager** installed in the cluster (see [Install cert-manager](#install-cert-manager) below). The gateway chart creates a **Certificate** and (for this demo) a namespace-scoped **Issuer**; those CRDs require cert-manager.
- **Gateway Operator** installed from [`operator-helm-chart`](../../operator-helm-chart) (or equivalent), with RBAC including `gateway.networking.k8s.io` (Helm chart includes this in `templates/_helpers.tpl`).
- **Gateway API CRDs** present in the cluster (from a cloud add-on, Traefik CRD release, `installStandardCRDs: true` on a truly empty cluster, or `kubectl apply` of upstream manifests). You do **not** need duplicate CRDs owned by this chart if another release already installed them.
- **`GatewayClass` name** must match one of the operator’s **`gateway_api.gateway_class_names`** values. By default the Helm chart sets **`wso2-api-platform`** in the operator `ConfigMap` (see `values.yaml` → `gatewayApi.managedGatewayClassNames`).
- Operator **`config.yaml`** must point at a valid **gateway Helm chart** (`gateway.helm.chartName` / `chartVersion`) and a mounted **`helm_values_file_path`** (the chart mounts `gateway_values.yaml` at `/config/gateway_values.yaml` by default).
- For gateway chart `1.0.0`, this demo sets `gateway.developmentMode: true` in the per-Gateway values override to avoid mandatory `gateway.controller.encryptionKeys` setup during local testing.
- The operator **deep-merges** this ConfigMap into the operator’s default `gateway_values.yaml`, so you only need overrides here (most defaults come from the operator chart / baked `gateway_values.yaml`).
- For demo stability, this uses a per-Gateway values override at **`gateway.config.controller.auth`** so management REST auth is deterministic.
- **HTTPS:** `02a-gateway-values-configmap.yaml` turns on **`gateway.controller.tls`** with **cert-manager**. The controller mounts the issued **listener** TLS secret and the xDS translator can build the HTTPS Envoy listener. The **gateway-runtime** **Service** exposes **8443** (HTTPS) and **8080** (HTTP). The Kubernetes **Gateway** here still uses an **HTTP :80** listener for the Gateway API contract in this demo; data-plane TLS is on Envoy.

## Install cert-manager

Install cert-manager once per cluster (namespace `cert-manager`):

```bash
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true
```

Wait until cert-manager is running before applying the demo gateway resources:

```bash
kubectl get pods -n cert-manager
```

You should see `cert-manager`, `cert-manager-cainjector`, and Helm often names the webhook `cert-manager-webhook-*` in `Running` state.

## What gets created

| Step | Resource | Role |
|------|----------|------|
| 1 | `GatewayClass` | Defines the class name your `Gateway` references (not reconciled by this operator, but must exist). |
| 2 | `Gateway` | Triggers operator: Helm install `{metadata.name}-gateway`, register controller Service in memory. This demo also sets `gateway.api-platform.wso2.com/api-selector` to a **k8s-only label selector** so the Gateway does not compete for **`RestApi`** objects bound to an **`APIGateway`** in mixed runs. |
| 3 | `Deployment` + `Service` | Demo HTTP backend (in-cluster URL used as API upstream). |
| 4 | `HTTPRoute` | Mapped to `APIConfigData` and **POST/PUT** to gateway-controller **`/api/management/v0.9/rest-apis`**. |

Helm also creates **Issuer** + **Certificate** (per `02a` values) in **`gateway-api-demo`**, and mounts the resulting **Secret** on **gateway-controller** for listener cert material used when generating xDS.

## Policies (HTTPRoute)

Policies use the same logical shape as **`RestApi`** (`name`, `version` required; optional `params`, `executionCondition`) and are merged into the payload before **`/api/management/v0.9/rest-apis`** is called.

**Recommended:** use **`APIPolicy`** CRs (`gateway.api-platform.wso2.com/v1alpha1`):

- **Policies list:** **`spec.policies`** is a non-empty array of policy instances (same fields as RestApi embedded `policies`).
- **API-level:** set **`spec.targetRef`** to the HTTPRoute (`gateway.networking.k8s.io`, kind `HTTPRoute`) — all `spec.policies` entries are merged into `APIConfigData.policies`.
- **Rule scope:** omit **`spec.targetRef`** on the `APIPolicy` and reference it from **`spec.rules[].filters`** with **`type: ExtensionRef`**, `group: gateway.api-platform.wso2.com`, `kind: APIPolicy`, `name: <metadata.name>` — all `spec.policies` entries apply to that rule’s operations.

Full manifests: [`gateway-api-httproute-policies-demo`](../gateway-api-httproute-policies-demo/README.md).

Malformed policy configuration can surface as **`ResolvedRefs=False`** / **`Invalid`** on the HTTPRoute. Ensure policy **names/versions** exist in your gateway deployment.

## Apply (order matters)

Use namespace **`gateway-api-demo`** (or change `namespace` consistently in every file, including `02a` `dnsNames` if you rename the namespace).

```bash
cd kubernetes/helm/resources/gateway-api-operator-demo
# 0. Install cert-manager (see above) if not already present.

kubectl apply -f 00-namespace.yaml
kubectl apply -f 01-gatewayclass.yaml
kubectl apply -f 02-gateway.yaml
kubectl apply -f 03-backend.yaml
# Wait until Helm workloads for the gateway are Ready (see Verification).
kubectl apply -f 04-httproute.yaml
kubectl apply -f 04-02-httproute.yaml 
```

## Verification

1. **cert-manager issued the gateway listener cert** (namespace `gateway-api-demo`):

   ```bash
   kubectl get certificate,issuer -n gateway-api-demo
   kubectl describe certificate -n gateway-api-demo platform-gw-gateway-controller-tls
   ```

2. **Gateway status** (Gateway API conditions):

   ```bash
   kubectl get gateway platform-gw -n gateway-api-demo -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
   ```

   Expect **Accepted** and **Programmed** to become **True** after the chart deploys and Deployments become ready. (With **`yq`**: `kubectl get gateway ... -o yaml | yq '.status.conditions'`.)

3. **Helm release** (release name `platform-gw-gateway`):

   ```bash
   helm list -n gateway-api-demo
   kubectl get deploy,svc,pods -n gateway-api-demo -l 'app.kubernetes.io/instance=platform-gw-gateway'
   ```

4. **Envoy HTTPS (data plane)** — self-signed cert; use `-k` or your CA bundle:

   ```bash
   kubectl run curl-https --rm -it --restart=Never --image=curlimages/curl -n gateway-api-demo -- \
     curl -skS "https://platform-gw-gateway-gateway-runtime.gateway-api-demo.svc.cluster.local:8443/"
   ```

5. **Operator logs** (no repeated `forbidden` on `gateways` / `httproutes`; successful sync messages).

6. **HTTPRoute parent status** (route programmed / accepted after REST sync):

   ```bash
   kubectl get httproute hello-api -n gateway-api-demo -o yaml
   ```

7. **Invoke the routed API with `curl`** (through gateway-runtime Service):

   ```bash
   curl --request GET \
     --url https://localhost:8443/hello-context/hello \
     --header 'Accept: application/json' -k
   ```

   Expected response body:

   ```text
   hello from gateway api demo
   ```

8. **REST API on gateway-controller** (optional, from a debug pod or curl job that can reach the controller **REST** Service):

   ```bash
   kubectl -n gateway-api-demo get svc -l app.kubernetes.io/component=controller,app.kubernetes.io/instance=platform-gw-gateway
   # GET rest-apis shows handle gateway-api-demo-hello-api (default handle) or your annotation override
   ```

   Auth follows the same basic-auth rules as **`RestApi`** (gateway `values.yaml` auth config or defaults).

## Customizing Helm values per Gateway

To use different Helm values for this `Gateway` than the operator’s global default file, create a **ConfigMap** in **`gateway-api-demo`** with key **`values.yaml`** (not `gateway_values.yaml`) and set annotation:

```yaml
metadata:
  annotations:
    gateway.api-platform.wso2.com/helm-values-configmap: "<configmap-name>"
```

See [GATEWAY_API_IMPLEMENTATION_NOTES](../../../gateway-operator/docs/GATEWAY_API_IMPLEMENTATION_NOTES.md) for all annotations.

## Troubleshooting

| Symptom | Check |
|--------|--------|
| `Gateway` never Programmed | `kubectl describe gateway platform-gw -n gateway-api-demo`; operator logs; Helm chart pull / image pull; `GatewayClass` name must match `managedGatewayClassNames`. |
| Helm fails with `no matches for kind "Certificate"` / `"Issuer"` | Install [cert-manager](#install-cert-manager) and wait for its pods to be Ready. |
| Certificate stays `Issuing` | `kubectl describe certificate -n gateway-api-demo platform-gw-gateway-controller-tls`; check Issuer events; cert-manager webhook must be healthy. |
| Controller log: failed to read certificate file / initial xDS snapshot failed | Listener secret must exist and `gateway.controller.tls.enabled` must be **true** so `/app/listener-certs` is mounted (this demo keeps cert-manager enabled). |
| `GET /api/admin/v0.9/health` **401** from gateway-controller, probes failing | Probes must use **`port: admin`** (admin server **9092**), not **`rest`** (**9090**). REST port runs Gin + basic auth; standalone gateway chart defaults already use `admin`. Upgrade **operator-helm-chart** so the `…-gateway-values` ConfigMap matches (or override `gateway.controller.deployment.{livenessProbe,readinessProbe}` accordingly). |
| Helm fails with `gateway.controller.encryptionKeys must be enabled when gateway.developmentMode is false` | For demo runs, keep `gateway.developmentMode: true` in `02a-gateway-values-configmap.yaml`; for production-like setups, configure `gateway.controller.encryptionKeys` secret instead. |
| `HTTPRoute` fails with `403 {"error":"forbidden"}` from gateway-controller | Ensure per-Gateway values override uses `gateway.config.controller.auth` for gateway chart `1.0.0`. Re-apply `02a-gateway-values-configmap.yaml` and reconcile `Gateway`. |
| `HTTPRoute` stuck, parent not accepted | Parent `Gateway` must be same namespace or `parentRefs` namespace set; registry only after Gateway sync succeeds. |
| `RestApi` deploys to the Kubernetes `Gateway` release instead of `APIGateway` | Ensure this demo `Gateway` has annotation `gateway.api-platform.wso2.com/api-selector` set (from `02-gateway.yaml`) and **`RestApi`** labels use a different **`gateway.api-platform.wso2.com/restapi-target`** value than this Gateway’s selector. Re-apply `02-gateway.yaml` and wait for reconcile. |
| REST 401 to gateway-controller | Align basic auth in gateway Helm values with what the operator sends (see auth helper / gateway ConfigMap). |
| Wrong API paths | Set annotation `gateway.api-platform.wso2.com/context` on the HTTPRoute; paths must satisfy `APIConfigData` validation rules. |
| `gateway-runtime` pod restarts ~1 minute after start (`SIGTERM` in logs) | Often **liveness** before Envoy/xDS is healthy, or **Helm values** merged incorrectly. Use an up-to-date operator build and gateway chart; ensure initial xDS snapshot succeeds (TLS + certs above). |
| `gateway-runtime` **Running** but **0/1 Ready** | If probes use `health-check.sh`, Envoy **`/ready`** may lag xDS; operator defaults may override probes. Reconcile Helm values. |
| Edited `platform-gw-values` ConfigMap but nothing happens | The operator fingerprints values onto the Gateway as `gateway.api-platform.wso2.com/last-helm-values-hash`. Remove that annotation on the `Gateway` (or change the Gateway spec) so Helm runs again, or use a versioned ConfigMap name + update the Gateway annotation. |

## Files

- `00-namespace.yaml` — `gateway-api-demo`
- `01-gatewayclass.yaml` — class `wso2-api-platform`
- `02a-gateway-values-configmap.yaml` — per-Gateway Helm values (`auth`, `developmentMode`, **cert-manager** listener TLS + SANs for in-cluster HTTPS)
- `02-gateway.yaml` — listener + `allowedRoutes` + annotation to use `platform-gw-values`
- `03-backend.yaml` — `ghcr.io/wso2/api-platform/sample-service` Deployment + ClusterIP Service (port **9080**, same image as integration tests)
- `04-httproute.yaml` — `PathPrefix /hello`, GET → backend Service; annotations set `api-version`, **`context`** (omit or leave blank to use API context **`/`**), `display-name`, and optional `project-id` for the generated API payload
