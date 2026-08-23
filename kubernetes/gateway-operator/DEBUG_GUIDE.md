# Gateway Operator — Debug Guide

Run the operator from your host (VS Code debugger) against a real cluster. All commands run from `kubernetes/gateway-operator/`.

## Prerequisites

1. Go toolchain + VS Code with the Go extension.
2. `kubectl` pointing at the target cluster (the operator uses your current kubeconfig context).
3. A reachable cluster (kind / k3s / colima / minikube / remote).

## Steps

1. **Confirm your cluster context:**
   ```bash
   kubectl config current-context && kubectl cluster-info
   ```

2. **Install the Gateway API standard CRDs** (the operator watches `Gateway`, `GatewayClass`, `HTTPRoute`, `ReferenceGrant`). Skip if already present:
   ```bash
   kubectl get crd gateways.gateway.networking.k8s.io >/dev/null 2>&1 || \
     kubectl apply -f ../helm/operator-helm-chart/files/gateway-api-standard/standard-crds.yaml
   ```
   > Uses the in-repo bundle (v1.5.1) the operator compiles against.

3. **Install the operator's own CRDs** (`gateway.api-platform.wso2.com`):
   ```bash
   make install
   ```
   > Regenerates the CRD manifests (controller-gen) and applies them via kustomize. Installs only this operator's CRDs — the Gateway API set in step 2 is separate.
   > Fallback (no regeneration / avoids client-side apply limits): `kubectl apply --server-side -f config/crd/bases/`

4. **Verify all CRDs are registered:**
   ```bash
   kubectl get crd | grep -E 'gateway.api-platform.wso2.com|gateway.networking.k8s.io'
   ```

5. **Launch the debugger:** VS Code → Run and Debug → **Gateway Operator**.
   Already wired in `.vscode/launch.json`: `-config config/config.yaml`, `GATEWAY_HELM_CHART_PATH` (local `gateway-helm-chart`), `GATEWAY_HELM_VALUES_FILE_PATH` (`config/gateway_values.yaml`). Leader election is off and no webhooks are registered, so no certs/lease are needed.

6. **Confirm a clean start:** you should see `Starting workers` for every controller and **no** `no matches for kind` / cache-sync-timeout errors. Health probe → `:8081`, metrics → `:8080` (keep these local ports free).

7. **Provision the mandatory AES-256 encryption key Secret** — at-rest encryption is mandatory; the chart **refuses to render** and the controller won't start without it (no dev bypass). Create it in the namespace where the gateway deploys:
   ```bash
   NS=default   # the namespace your APIGateway deploys into
   kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -
   openssl rand 32 > default-aesgcm256-v1.bin
   kubectl create secret generic gateway-encryption-keys \
     --from-file=default-aesgcm256-v1.bin=default-aesgcm256-v1.bin -n "$NS"
   rm -f default-aesgcm256-v1.bin
   ```
   Then **enable `encryptionKeys`** (disabled by default) so the chart renders — pick one:
   - **Globally** (recommended for debug — every gateway inherits it): edit the operator default `config/gateway_values.yaml` (the `GATEWAY_HELM_VALUES_FILE_PATH` base) under `gateway.controller`:
     ```yaml
     encryptionKeys:
       enabled: true
       secretName: gateway-encryption-keys   # the Secret above; data key: default-aesgcm256-v1.bin
     ```
   - **Per gateway**: put the same block in that `APIGateway`'s `configRef` ConfigMap (overrides the default; lets each gateway use its own key).

   Either way the Secret must exist **in each gateway namespace** — the operator doesn't create it.

8. **Trigger a reconcile** (set breakpoints in `internal/controller/apigateway_controller.go` `Reconcile` first). Use the ready-made two-gateway example — it enables `encryptionKeys` and documents the per-namespace Secret in its header:
   ```bash
   # create the encryption Secret in gw-120 and gw-121 first (see the file header), then:
   kubectl apply -f config/samples/api_v1_apigateway_multiversion.yaml
   ```
   > The shipped `config/samples/api_v1_apigateway.yaml` does **not** enable `encryptionKeys`, so it fails the at-rest-encryption check as-is — enable it (step 7) before applying, or use the multiversion sample above.

## Notes

- **`encryptionKeys must be enabled`** in operator logs (`Max retries exceeded ... at-rest encryption is mandatory`) = the Secret is missing in the gateway's namespace, or the values don't set `encryptionKeys.enabled=true` + `secretName`. It's per-namespace, so each gateway needs its own Secret. (Host-run gateway debug uses a file instead — see `gateway/DEBUG_GUIDE.md`.)
- After changing API types (`api/**`): run `make manifests` to regenerate `config/crd/bases/`, then re-apply via step 3.
- Cleanup: `kubectl delete -f config/samples/api_v1_apigateway_multiversion.yaml`; remove CRDs with `kubectl delete -f config/crd/bases/`.
