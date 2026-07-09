# WSO2 API Platform Gateway

## Table of contents

| API channel | Implementation version | Mode     | Report                                          |
|-------------|------------------------|-------------|-------------------------------------------------|
| standard    | _(fill in tag)_        | default | `wso2-api-platform-<version>-report.yaml`  

## Steps to Reproduce

These steps build the WSO2 API Platform gateway images from source and run the
Gateway API conformance suite against them, as required for report submission.
Prerequisites: KinD, Helm, kubectl, Docker, `jq`, and a Go toolchain.

### 1. Clone the repository

```sh
git clone https://github.com/wso2/api-platform.git
cd api-platform
```

### 2. Build the gateway images from source

Build the gateway-controller and gateway-runtime images:

```sh
cd gateway
make build
```

Then build the gateway-operator image:

```sh
cd ../kubernetes/gateway-operator
make docker-build
cd ../..
```

This produces:

- `ghcr.io/wso2/api-platform/gateway-controller:1.2.0-SNAPSHOT`
- `ghcr.io/wso2/api-platform/gateway-runtime:1.2.0-SNAPSHOT`
- `ghcr.io/wso2/api-platform/gateway-operator:0.8.1-SNAPSHOT`

### 3. Create the KinD cluster with MetalLB and load the images

```sh
cd conformance-report
./kind/setup-kind.sh          # macOS + Colima: use ./kind/setup-colima.sh instead

for img in \
  ghcr.io/wso2/api-platform/gateway-controller:1.2.0-SNAPSHOT \
  ghcr.io/wso2/api-platform/gateway-runtime:1.2.0-SNAPSHOT \
  ghcr.io/wso2/api-platform/gateway-operator:0.8.1-SNAPSHOT; do
  kind load docker-image "$img" --name wso2-conformance
done
```

MetalLB gives the operator-provisioned gateway-runtime LoadBalancer Service a
routable address the suite can reach. See `README.md` for why this is needed and
for the macOS/Colima host-reachability details.

### 4. Install the Gateway API CRDs, operator, and GatewayClass

```sh
./install-wso2-gateway.sh
```

Installs cert-manager, the gateway operator using the locally built images (the
operator installs the bundled standard-channel Gateway API CRDs v1.5.1), and the
`wso2-api-platform` GatewayClass.

### 5. Run the conformance suite

```sh
./run-conformance.sh
```

Runs `go test` in the `runner/` Go module, which imports the upstream conformance
suite (`sigs.k8s.io/gateway-api/conformance`) as a dependency — the suite and its
embedded test manifests are pulled from the module cache, so no clone of the
gateway-api repo is needed. Writes `wso2-api-platform-<version>-report.yaml` here.

### 6. View the report

```sh
cat wso2-api-platform-*-report.yaml
```
