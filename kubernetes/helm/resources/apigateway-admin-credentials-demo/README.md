# Two gateways with admin passwords from Secrets

This demo installs the gateway operator, creates two `APIGateway`s with different admin users, and deploys one `RestApi` to each:

| Gateway | Namespace | Admin user | Password Secret | Delivery (`source`) | API |
|---|---|---|---|---|---|
| `gateway-1` | `gw1` | `gw1-admin` | `gateway-1-admin` | `file`: Secret key mounted read-only into the gateway-controller | `api-1` at `/api1` |
| `gateway-2` | `gw2` | `gw2-admin` | `gateway-2-admin` | `env`: environment variable from the Secret | `api-2` at `/api2` |

Each password stays in its Secret. The operator passes only the Secret reference to the gateway Helm chart, and reads the Secret itself to call each gateway's management API. The password never appears in Helm values, the gateway ConfigMap, or the Helm release. Changing a Secret rolls that gateway's controller pod.

Each `APIGateway` selects APIs by its own label (`gateway.api-platform.wso2.com/target`), so an API reaches only one gateway.

## Prerequisites

- A cluster with cert-manager installed: the gateway chart issues the controller's TLS certificate through it.
- An operator build that supports `APIGateway.spec.adminCredentials` (served by `apiVersion: gateway.api-platform.wso2.com/v2alpha1`; the operator converts it to and from the stored `v1` object with its conversion webhook), with the matching CRDs applied (`kubectl apply --server-side -f kubernetes/helm/operator-helm-chart/crds/`), and a gateway chart that supports `gateway.config.controller.auth.basic.users[].passwordSecretRef` (wso2/api-platform#4423). The chart declares it in its `Chart.yaml` annotation `gateway.api-platform.wso2.com/features`; the operator refuses to deploy a Secret-backed password with a chart that does not.
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

On a cluster without a LoadBalancer implementation (kind, for example), also set `--set gateway.values.gateway.gatewayRuntime.service.type=ClusterIP`. Otherwise the gateway install waits for an external IP until it times out.

To test an unreleased operator and chart locally (the setup this demo was verified on):

- **Operator image.** Build it with `make -C kubernetes/gateway-operator docker-build IMG=gateway-operator:dev` and load it into the cluster. Then set `image.repository=gateway-operator`, `image.tag=dev` and `image.pullPolicy=IfNotPresent`.
- **Gateway chart.** Package it with `helm package kubernetes/helm/gateway-helm-chart --version <version>` and push it to an OCI registry the operator can reach, for example a `registry:2` container on the kind network. Then set:
  - `gateway.helm.chartName=oci://<registry>/charts/gateway`
  - `gateway.helm.chartVersion=<version>`
  - `gateway.helm.plainHTTP=true`, if the registry is plain HTTP

### 2. Create the namespaces and Secrets

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
kubectl wait apigateway --all -A --for=condition=Programmed --timeout=10m
```

### 4. Deploy API 1 to gateway-1 and API 2 to gateway-2

```bash
kubectl apply -f 03-restapis.yaml
kubectl wait restapi --all -A --for=condition=Programmed --timeout=3m
kubectl get restapi -A -o jsonpath='{range .items[*]}{.metadata.name}: {.status.conditions[?(@.type=="Programmed")].message}{"\n"}{end}'
# api-1: Successfully deployed to gateway gw1/gateway-1
# api-2: Successfully deployed to gateway gw2/gateway-2
```

## Verify

**Each gateway accepts only its own admin user.**

```bash
kubectl port-forward -n gw1 svc/gateway-1-gw-controller 19091:9090 &
kubectl port-forward -n gw2 svc/gateway-2-gw-controller 19092:9090 &
PW1=$(kubectl get secret gateway-1-admin -n gw1 -o jsonpath='{.data.password}' | base64 -d)
PW2=$(kubectl get secret gateway-2-admin -n gw2 -o jsonpath='{.data.password}' | base64 -d)

curl -s -o /dev/null -w '%{http_code}\n' -u "gw1-admin:$PW1" localhost:19091/api/management/v1/rest-apis   # 200
curl -s -o /dev/null -w '%{http_code}\n' -u "gw2-admin:$PW2" localhost:19091/api/management/v1/rest-apis   # 401
curl -s -o /dev/null -w '%{http_code}\n' -u "admin:admin"    localhost:19091/api/management/v1/rest-apis   # 401
curl -s -o /dev/null -w '%{http_code}\n' -u "gw2-admin:$PW2" localhost:19092/api/management/v1/rest-apis   # 200
curl -s -o /dev/null -w '%{http_code}\n' -u "gw1-admin:$PW1" localhost:19092/api/management/v1/rest-apis   # 401
```

**How each password is delivered.**

```bash
# gateway-1 (file): a read-only Secret volume, no password env var, and the token in controller-secrets.toml
kubectl get deploy gateway-1-gw-controller -n gw1 -o jsonpath='{.spec.template.spec.volumes[?(@.name=="basic-auth-user-0")]}'
kubectl get cm gateway-1-gw-config -n gw1 -o jsonpath='{.data.controller-secrets\.toml}'
#   password = '{{ file "/secrets/gateway-controller/basic-auth/user-0/password" }}'

# gateway-2 (env): an env var from the Secret, and the token in config.toml
kubectl get deploy gateway-2-gw-controller -n gw2 -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="APIP_GW_CONTROLLER_AUTH_BASIC_USER_0_PASSWORD")]}'
kubectl get cm gateway-2-gw-config -n gw2 -o jsonpath='{.data.config\.toml}' | grep -A4 'controller.auth.basic\]'
#   password = '{{ env "APIP_GW_CONTROLLER_AUTH_BASIC_USER_0_PASSWORD" "" }}'

# Neither password is in a ConfigMap or the Helm release
kubectl get cm -n gw1 -o yaml | grep -cF -- "$PW1"                # 0
helm get values gateway-1-gw -n gw1 -a | grep -cF -- "$PW1"       # 0
```

**API traffic.**

```bash
kubectl port-forward -n gw1 svc/gateway-1-gw-gateway-runtime 18081:8080 &
kubectl port-forward -n gw2 svc/gateway-2-gw-gateway-runtime 18082:8080 &
curl -s -o /dev/null -w '%{http_code}\n' localhost:18081/api1/   # 200
curl -s -o /dev/null -w '%{http_code}\n' localhost:18081/api2/   # 404
curl -s -o /dev/null -w '%{http_code}\n' localhost:18082/api2/   # 200
curl -s -o /dev/null -w '%{http_code}\n' localhost:18082/api1/   # 404
```

**Rotate a password.**

```bash
kubectl create secret generic gateway-1-admin -n gw1 \
  --from-literal=password="$(openssl rand -base64 24)" --dry-run=client -o yaml | kubectl apply -f -
```

The operator redeploys gateway-1 and its controller pod rolls. The old password then gets `401`, the new one `200`, and later API changes reach the gateway with the new password.

## Clean up

```bash
kubectl delete -f 03-restapis.yaml
kubectl delete -f 01-apigateways.yaml
kubectl delete -f 02-backend.yaml -f 00-namespaces.yaml
```
