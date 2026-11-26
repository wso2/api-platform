# Two gateways with admin passwords from Secrets

This demo installs the gateway operator, creates two `APIGateway`s with different admin users, and deploys one `RestApi` to each:

| Gateway | Namespace | Admin user | Password Secret | API |
|---|---|---|---|---|---|
| `gateway-1` | `gw1` | `gw1-admin` | `gateway-1-admin` | `api-1` at `/api1` |
| `gateway-2` | `gw2` | `gw2-admin` | `gateway-2-admin` | `api-2` at `/api2` |

Each password reaches its gateway-controller as the Secret key mounted read-only as a file.

Each password stays in its Secret. The operator passes only the Secret reference to the gateway Helm chart, and reads the Secret itself to call each gateway's management API. The password never appears in Helm values, the gateway ConfigMap, or the Helm release. Changing a Secret rolls that gateway's controller pod.

Each `APIGateway` selects APIs by its own label (`gateway.api-platform.wso2.com/target`), so an API reaches only one gateway.

## Prerequisites

- A cluster with cert-manager installed: the gateway chart issues the controller's TLS certificate through it.
- An operator build that supports `APIGateway.spec.adminCredentials` (served by `apiVersion: gateway.api-platform.wso2.com/v2alpha1`; the operator converts it to and from the stored `v1` object with its conversion webhook), with the matching CRDs applied (`kubectl apply --server-side --force-conflicts -f kubernetes/helm/operator-helm-chart/crds/`; `--force-conflicts` because the operator owns the APIGateway CRD's `spec.conversion`; the apply briefly puts back a placeholder webhook Service, and `v1alpha1`/`v2alpha1` requests can fail until the operator restores its own, see the operator chart README), and a gateway chart that supports `gateway.config.controller.auth.basic.users[].passwordSecretRef` (wso2/api-platform#4423). The chart declares it in its `Chart.yaml` annotation `gateway.api-platform.wso2.com/features`; the operator refuses to deploy a Secret-backed password with a chart that does not.
- A default StorageClass, so the gateway-controller database survives pod restarts. With `gateway.controller.persistence.enabled=false` the database is an `emptyDir`, so the APIs are lost when a password rotation rolls the controller pod.

## Steps

### 1. Install the operator

```bash
helm upgrade --install gateway-operator kubernetes/helm/operator-helm-chart \
  --namespace gateway-operator --create-namespace \
  --set gateway.values.gateway.controller.encryptionKeys.enabled=true \
  --set gateway.values.gateway.controller.encryptionKeys.secretName=gateway-encryption-keys \
  --wait
```

If `gateway-operator` is already installed, this command upgrades it, which changes your existing setup in two ways. The simplest way to avoid both is to run the demo on a separate cluster. Otherwise:

- **Values the command does not set go back to the chart defaults.** A release limited by `watchNamespaces`, for example, would then watch all namespaces, with cluster-wide RBAC. Add `--reset-then-reuse-values` (Helm 3.14 or later) to keep the existing values. If the kept `watchNamespaces` is not empty, add `gw1` and `gw2` to it and keep its entries, for example `--set 'watchNamespaces={production,gw1,gw2}'`. Otherwise the operator ignores the demo gateways and the wait in step 3 times out.
- **The two `encryptionKeys` flags change the base values every gateway inherits** unless it overrides them in its own `configRef`. Existing gateways would then look for a `gateway-encryption-keys` Secret in their own namespaces on their next redeploy, and fail to start where there is none. Leave the two flags out, and in step 2 create the key Secret under the `secretName` your installation already uses.

On a cluster without a LoadBalancer implementation (kind, for example), also set `--set gateway.values.gateway.gatewayRuntime.service.type=ClusterIP`. Otherwise the gateway install waits for an external IP until it times out.

To test an unreleased operator and chart locally (the setup this demo was verified on):

- **Operator image.** Build it with `make -C kubernetes/gateway-operator docker-build IMG=gateway-operator:dev` and load it into the cluster. Then set `image.repository=gateway-operator`, `image.tag=dev` and `image.pullPolicy=IfNotPresent`.
- **Gateway chart.** Package it with `helm package kubernetes/helm/gateway-helm-chart --version <version>` and push it to an OCI registry the operator can reach, for example a `registry:2` container on the kind network. Then set:
  - `gateway.helm.chartName=oci://<registry>/charts/gateway`
  - `gateway.helm.chartVersion=<version>`
  - `gateway.helm.plainHTTP=true`, if the registry is plain HTTP

### 2. Create the namespaces and Secrets

The namespaces `gw1`, `gw2` and `echo-backend` must not exist yet. The demo uses them for itself, and the clean-up deletes them with everything in them.

Step 1 runs from the repository root. The remaining steps use the manifests in this directory:

```bash
cd kubernetes/helm/resources/apigateway-admin-credentials-demo
kubectl apply -f 00-namespaces.yaml

for n in 1 2; do
  # Admin password: random, never written to a file.
  kubectl create secret generic gateway-$n-admin -n gw$n \
    --from-literal=password="$(openssl rand -base64 24)"

  # At-rest encryption key required by every gateway.
  key="$(mktemp)"; openssl rand 32 > "$key"
  kubectl create secret generic gateway-encryption-keys -n gw$n \
    --from-file=default-aesgcm256-v1.bin="$key"
  rm -f "$key"
done

kubectl apply -f 02-backend.yaml
```

### 3. Create the two gateways

```bash
kubectl apply -f 01-apigateways.yaml
# Waits for the two demo gateways only, not other gateways in the cluster.
kubectl wait -f 01-apigateways.yaml --for=condition=Programmed --timeout=10m
```

### 4. Deploy API 1 to gateway-1 and API 2 to gateway-2

```bash
kubectl apply -f 03-restapis.yaml
kubectl wait -f 03-restapis.yaml --for=condition=Programmed --timeout=3m
kubectl get restapi -A -o jsonpath='{range .items[*]}{.metadata.name}: {.status.conditions[?(@.type=="Programmed")].message}{"\n"}{end}'
# api-1: Successfully deployed to gateway gw1/gateway-1
# api-2: Successfully deployed to gateway gw2/gateway-2
```

## Verify

**Each gateway accepts only its own admin user.**

The checks run in one shell. `wait_port` waits until a port-forward accepts connections, so no check runs before its listener is up. It gives up after 30 seconds, or as soon as the port-forward exits (for example because the Service does not exist), and says which.

```bash
# wait_port PORT PID: wait up to 30 s for port-forward PID to accept connections on PORT.
wait_port() {
  for _ in $(seq 30); do
    kill -0 "$2" 2>/dev/null || { echo "port-forward on $1 exited" >&2; return 1; }
    curl -s -o /dev/null "localhost:$1" && return 0
    sleep 1
  done
  echo "port-forward on $1 not ready after 30 s" >&2; return 1
}

kubectl port-forward -n gw1 svc/gateway-1-gw-controller 19091:9090 & PF1=$!
kubectl port-forward -n gw2 svc/gateway-2-gw-controller 19092:9090 & PF2=$!
wait_port 19091 "$PF1" && wait_port 19092 "$PF2"
PW1=$(kubectl get secret gateway-1-admin -n gw1 -o jsonpath='{.data.password}' | base64 -d)
PW2=$(kubectl get secret gateway-2-admin -n gw2 -o jsonpath='{.data.password}' | base64 -d)

curl -s -o /dev/null -w '%{http_code}\n' -u "gw1-admin:$PW1" localhost:19091/api/management/v1/rest-apis   # 200
curl -s -o /dev/null -w '%{http_code}\n' -u "gw2-admin:$PW2" localhost:19091/api/management/v1/rest-apis   # 401
curl -s -o /dev/null -w '%{http_code}\n' -u "admin:admin"    localhost:19091/api/management/v1/rest-apis   # 401
curl -s -o /dev/null -w '%{http_code}\n' -u "gw2-admin:$PW2" localhost:19092/api/management/v1/rest-apis   # 200
curl -s -o /dev/null -w '%{http_code}\n' -u "gw1-admin:$PW1" localhost:19092/api/management/v1/rest-apis   # 401
```

**How the password is delivered.**

```bash
# A read-only Secret volume, no password env var, and the token in controller-secrets.toml
kubectl get deploy gateway-1-gw-controller -n gw1 -o jsonpath='{.spec.template.spec.volumes[?(@.name=="basic-auth-user-0")]}'
kubectl get cm gateway-1-gw-config -n gw1 -o jsonpath='{.data.controller-secrets\.toml}'
#   password = '{{ file "/secrets/gateway-controller/basic-auth/user-0/password" }}'

# Neither password is in a ConfigMap or the Helm release
kubectl get cm -n gw1 -o yaml | grep -cF -- "$PW1"                # 0
helm get values gateway-1-gw -n gw1 -a | grep -cF -- "$PW1"       # 0
```

**API traffic.**

```bash
kubectl port-forward -n gw1 svc/gateway-1-gw-gateway-runtime 18081:8080 & PF3=$!
kubectl port-forward -n gw2 svc/gateway-2-gw-gateway-runtime 18082:8080 & PF4=$!
wait_port 18081 "$PF3" && wait_port 18082 "$PF4"
curl -s -o /dev/null -w '%{http_code}\n' localhost:18081/api1/   # 200
curl -s -o /dev/null -w '%{http_code}\n' localhost:18081/api2/   # 404
curl -s -o /dev/null -w '%{http_code}\n' localhost:18082/api2/   # 200
curl -s -o /dev/null -w '%{http_code}\n' localhost:18082/api1/   # 404
```

**Rotate a password.**

The operator redeploys gateway-1 and its controller pod rolls. The roll ends the management port-forward, which was connected to the old pod, so reconnect before checking the passwords.

```bash
OLD_PW1=$PW1
GEN=$(kubectl get deploy gateway-1-gw-controller -n gw1 -o jsonpath='{.metadata.generation}')
kubectl create secret generic gateway-1-admin -n gw1 \
  --from-literal=password="$(openssl rand -base64 24)" --dry-run=client -o yaml | kubectl apply -f -

# Wait up to 2 minutes for the operator to roll the controller, then for the new pod.
# If it does not, show the gateway's conditions and Events, which name the reason.
redeployed=
for _ in $(seq 60); do
  [ "$(kubectl get deploy gateway-1-gw-controller -n gw1 -o jsonpath='{.metadata.generation}')" != "$GEN" ] && { redeployed=1; break; }
  sleep 2
done
if [ -n "$redeployed" ]; then
  kubectl rollout status deploy/gateway-1-gw-controller -n gw1 --timeout=5m
else
  echo "gateway-1 was not redeployed within 2 minutes" >&2
  kubectl get apigateway gateway-1 -n gw1 -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}: {.message}{"\n"}{end}'
  kubectl get events -n gw1 --field-selector involvedObject.kind=APIGateway,involvedObject.name=gateway-1
fi

kill "$PF1"
kubectl port-forward -n gw1 svc/gateway-1-gw-controller 19091:9090 & PF1=$!
wait_port 19091 "$PF1"
PW1=$(kubectl get secret gateway-1-admin -n gw1 -o jsonpath='{.data.password}' | base64 -d)

curl -s -o /dev/null -w '%{http_code}\n' -u "gw1-admin:$OLD_PW1" localhost:19091/api/management/v1/rest-apis   # 401
curl -s -o /dev/null -w '%{http_code}\n' -u "gw1-admin:$PW1"     localhost:19091/api/management/v1/rest-apis   # 200
```

Later API changes reach the gateway with the new password.

## Clean up

```bash
kubectl delete -f 03-restapis.yaml
kubectl delete -f 01-apigateways.yaml
kubectl delete -f 02-backend.yaml -f 00-namespaces.yaml

# Only if you installed the operator for this demo in step 1; skip it when you used an
# existing installation. Helm leaves the CRDs in place (see the operator chart README).
helm uninstall gateway-operator -n gateway-operator
```
