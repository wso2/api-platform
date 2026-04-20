# HTTPRoute policies demo (APIPolicy CR)

This is a **separate** manifest set from [`../gateway-api-operator-demo`](../gateway-api-operator-demo). It shows the **APIPolicy** custom resource (`gateway.api-platform.wso2.com/v1alpha1`) for Gateway API HTTPRoutes:

1. **API-level policies** — `APIPolicy` objects with **`spec.targetRef`** set to the **HTTPRoute** and one or more entries in **`spec.policies`** (same shape as RestApi embedded policies). All entries are merged into `APIConfigData.policies` (ordered by `APIPolicy` name, then list order).
2. **Rule / resource scope** — `APIPolicy` objects **without** **`spec.targetRef`** are not loaded as API-level; reference them only from **`spec.rules[].filters`** with **`type: ExtensionRef`**. **All** entries in **`spec.policies`** on the referenced `APIPolicy` apply to operations derived from that rule’s matches.
3. **Secret-backed params** — `02` + `03` add a **`Secret`**, an `APIPolicy` whose **`params`** use nested **`valueFrom`** (`name` / `valueKey`), and a second **`HTTPRoute`** so you can validate Secret watch → HTTPRoute re-reconcile (patch the Secret data and confirm the operator redeploys).

There are **no** policy ConfigMaps or inline policy annotations in this demo (policies are attached only via `APIPolicy`).

When **`spec.policies[].params`** embed **`valueFrom`** (e.g. `name` / `valueKey`, optional `namespace`), the operator watches those **Secrets** and re-reconciles the target **HTTPRoute** when referenced Secret data changes. `ServiceAccount` token secrets are ignored by the watch.

## Prerequisites

1. Complete the base Gateway API demo through a **Programmed** `Gateway` and the **`hello-backend`** Service — see [`gateway-api-operator-demo/README.md`](../gateway-api-operator-demo/README.md) through `03-backend.yaml` (`04-httproute.yaml` is optional).
2. **Install the APIPolicy CRD** (e.g. operator Helm chart `crds/gateway.api-platform.wso2.com_apipolicies.yaml`) if not already installed.
3. **Policy names** in `spec.policies` are placeholders; use policies your **gateway-controller** defines.

All resources use namespace **`gateway-api-demo`**.

## ExtensionRef → APIPolicy

In a **rule**, use:

```yaml
filters:
  - type: ExtensionRef
    extensionRef:
      group: gateway.api-platform.wso2.com
      kind: APIPolicy
      name: <apipolicy-metadata-name>
```

The referenced `APIPolicy` must exist in the HTTPRoute namespace and **`spec.policies`** must be a non-empty array. For rule-attached policies, **omit** **`spec.targetRef`** on the `APIPolicy`. If **`targetRef`** is set, it must match **that** HTTPRoute (`group: gateway.networking.k8s.io`, `kind: HTTPRoute`, `name` matching the route).

## Apply

```bash
cd kubernetes/helm/resources/gateway-api-httproute-policies-demo

kubectl apply -f 00-apipolicies.yaml
kubectl apply -f 01-httproute.yaml
# Optional: secret-backed policy flow (second REST API / HTTPRoute)
kubectl apply -f 02-secret-and-apipolicy.yaml
kubectl apply -f 03-httproute-secret-policy.yaml
```

REST handles (annotations):

- Base flow: `gateway-api-demo-hello-apipolicy`
- Secret flow: `gateway-api-demo-hello-apipolicy-secrets`

## Verification

1. HTTPRoute status:

```bash
kubectl get httproute -n gateway-api-demo hello-apipolicy-demo -o yaml
kubectl get httproute -n gateway-api-demo hello-apipolicy-secrets-demo -o yaml
kubectl get apipolicy,secret -n gateway-api-demo
```

2. Operator logs and optional gateway-controller **`GET /rest-apis/{handle}`** payload.

3. Curl (HTTPS, self-signed):

```bash
curl --request GET \
  --url https://localhost:8443/hello-policies-context/hello-policies \
  --header 'Accept: application/json' -k
```

Same HTTPRoute, path **without** rule-level `ExtensionRef` (still has **API-level** policies from the `APIPolicy` with **`spec.targetRef`**):

```bash
curl --request GET \
  --url https://localhost:8443/hello-policies-context/hello-policies-plain \
  --header 'Accept: application/json' -k
```

Secret flow (HTTPS, self-signed):

```bash
curl --request GET \
  --url https://localhost:8443/hello-secrets/hello-secrets \
  --header 'Accept: application/json' -k
```

**Validate Secret watch:** change `subscriptionKey` in `Secret/httproute-demo-policy-credentials` (e.g. `kubectl edit secret -n gateway-api-demo httproute-demo-policy-credentials` or patch `stringData`). The operator resolves **`params.valueFrom`** to string values before calling gateway-controller, and should re-reconcile **`hello-apipolicy-secrets-demo`** without editing the HTTPRoute.

## Files

| File | Purpose |
|------|---------|
| `00-apipolicies.yaml` | API-level `APIPolicy` (`targetRef` → `hello-apipolicy-demo`) + rule-scoped `APIPolicy` (no `targetRef`). |
| `01-httproute.yaml` | HTTPRoute with **ExtensionRef** to the rule-scoped `APIPolicy` and optional `project-id` annotation propagated to payload metadata. |
| `02-secret-and-apipolicy.yaml` | `Secret` + rule `APIPolicy` with **`params.valueFrom`** (no `targetRef`; referenced from `03` HTTPRoute only). |
| `03-httproute-secret-policy.yaml` | Second HTTPRoute; **ExtensionRef** to the secret-backed `APIPolicy`. |

## Policy attachment (operator)

Only **`APIPolicy`** is supported for Gateway API policy attachment.
