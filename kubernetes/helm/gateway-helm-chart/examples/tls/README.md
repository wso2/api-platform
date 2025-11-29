# TLS Certificate Examples

This directory contains example configurations for TLS certificate management.

## Prerequisites

### Option 1: Use Built-in cert-manager (Recommended for New Installations)

The chart includes cert-manager as a subchart. It will be installed automatically:

```bash
helm install gateway ../gateway-helm-chart
```

### Option 2: Use Existing cert-manager

If you already have cert-manager installed in your cluster:

```bash
helm install gateway ../gateway-helm-chart \
  --set cert-manager.enabled=false
```

### Option 3: Manual cert-manager Installation

If you prefer to install cert-manager separately:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.3/cert-manager.yaml

helm install gateway ../gateway-helm-chart \
  --set cert-manager.enabled=false
```

## Example 1: Self-Signed Certificate (Development)

The chart includes a default self-signed Issuer for development:

```bash
helm install gateway ../gateway-helm-chart \
  --set gateway.controller.tls.enabled=true \
  --set gateway.controller.tls.certificateProvider=cert-manager
```

This creates:
- cert-manager (if not already installed)
- A self-signed Issuer in the release namespace
- A Certificate resource
- A Secret with the TLS certificate

## Example 2: Let's Encrypt (Production)

Create a Let's Encrypt Issuer (or ClusterIssuer):

```yaml
# letsencrypt-issuer.yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt-prod
  namespace: default  # Must match the release namespace
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod-account-key
    solvers:
    - http01:
        ingress:
          class: nginx
```

Install the gateway:

```bash
kubectl apply -f letsencrypt-issuer.yaml

helm install gateway ../gateway-helm-chart \
  --set gateway.controller.tls.enabled=true \
  --set gateway.controller.tls.certificateProvider=cert-manager \
  --set gateway.controller.tls.certManager.issuerRef.name=letsencrypt-prod \
  --set gateway.controller.tls.certManager.issuerRef.kind=Issuer \
  --set gateway.controller.tls.certManager.createIssuer=false \
  --set gateway.controller.tls.certManager.commonName=api.example.com \
  --set gateway.controller.tls.certManager.dnsNames[0]=api.example.com
```

Or use a values file (see `values-letsencrypt.yaml`).

## Example 3: Existing Certificate Secret

If you already have a TLS certificate:

```bash
# Create secret from certificate files
kubectl create secret tls gateway-tls \
  --cert=server.crt \
  --key=server.key

# Install gateway
helm install gateway ../gateway-helm-chart \
  --set gateway.controller.tls.enabled=true \
  --set gateway.controller.tls.certificateProvider=secret \
  --set gateway.controller.tls.secret.name=gateway-tls
```

## Example 4: Custom Upstream CA Certificates

For backends using self-signed or corporate CA certificates:

```bash
# Create secret with CA certificates
kubectl create secret generic upstream-ca-certs \
  --from-file=backend1-ca.crt \
  --from-file=backend2-ca.crt

# Install gateway with upstream certs
helm install gateway ../gateway-helm-chart \
  --set gateway.controller.upstreamCerts.enabled=true \
  --set gateway.controller.upstreamCerts.secretName=upstream-ca-certs
```

## Verification

Check certificate status:

```bash
# Check Certificate resource
kubectl get certificate

# Check Secret created by cert-manager
kubectl get secret <release-name>-controller-tls

# View certificate details
kubectl describe certificate <release-name>-controller-tls
```

Test HTTPS connection:

```bash
# Port forward to router HTTPS port
kubectl port-forward svc/<release-name>-router 8443:8443

# Test (with self-signed cert, use -k to skip verification)
curl -k https://localhost:8443/
```
