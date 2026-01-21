# Gateway Configuration Reference

## Overview

The APIGateway CRD supports custom Helm configuration through ConfigMap references. This allows you to override the default Helm values on a per-gateway basis.

## Configuration Options

### Option 1: Using Default Configuration (Backward Compatible)

If you don't specify `configRef`, the operator will use the default mounted configuration:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: default-gateway
spec:
  apiSelector:
    scope: Cluster
  # No configRef - uses operator's default configuration
```

### Option 2: Using Custom ConfigMap

You can provide a ConfigMap with custom Helm values:

```yaml
---
# Create a ConfigMap with your custom Helm values
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-gateway-config
  namespace: default
data:
  values.yaml: |
    replicaCount: 3

    resources:
      limits:
        cpu: 2000m
        memory: 2Gi

    service:
      type: LoadBalancer
      port: 9090

---
# Reference the ConfigMap in your Gateway
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: custom-gateway
  namespace: default
spec:
  apiSelector:
    scope: Cluster

  # Reference your custom ConfigMap
  configRef:
    name: my-gateway-config
```

## ConfigMap Requirements

1. **Key Name**: The ConfigMap must contain a key named `values.yaml`
2. **Namespace**: The ConfigMap must be in the same namespace as the APIGateway
3. **Format**: The content must be valid YAML for Helm values

## How It Works

1. When an APIGateway is created/updated:
   - If `configRef` is specified:
     - The operator reads the ConfigMap
     - Uses the `values.yaml` content as Helm values
   - If `configRef` is NOT specified:
     - The operator uses the default mounted configuration file

2. The ConfigMap values override the default Helm chart values

3. Any changes to the ConfigMap require updating the APIGateway (e.g., adding an annotation) to trigger reconciliation

## Use Cases

### Production Gateway with High Availability

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prod-gateway-config
data:
  values.yaml: |
    replicaCount: 3

    resources:
      limits:
        cpu: 2000m
        memory: 4Gi
      requests:
        cpu: 1000m
        memory: 2Gi

    autoscaling:
      enabled: true
      minReplicas: 3
      maxReplicas: 10

    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
        - topologyKey: kubernetes.io/hostname
```

### Development Gateway with Minimal Resources

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dev-gateway-config
data:
  values.yaml: |
    replicaCount: 1

    resources:
      limits:
        cpu: 500m
        memory: 512Mi
      requests:
        cpu: 250m
        memory: 256Mi
```

### Gateway with External Database

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-postgres-config
data:
  values.yaml: |
    storage:
      type: postgres
      host: postgres.database.svc.cluster.local
      port: 5432
      database: gateway_prod

    # Reference existing secret for DB credentials
    existingSecret: postgres-credentials
```

## Best Practices

1. **Version Control**: Store your ConfigMaps in Git alongside your Gateway manifests

2. **Naming Convention**: Use descriptive names like `{gateway-name}-config`

3. **Environment Separation**: Use different ConfigMaps for different environments:
   - `gateway-dev-config`
   - `gateway-staging-config`
   - `gateway-prod-config`

4. **Validation**: Test your Helm values locally before applying:
   ```bash
   helm template <chart> -f values.yaml
   ```

5. **Documentation**: Add comments in your ConfigMap to explain custom settings

## Troubleshooting

### ConfigMap Not Found

```
Error: failed to get ConfigMap default/my-config: configmap "my-config" not found
```

**Solution**: Ensure the ConfigMap exists in the same namespace as the APIGateway.

### Missing values.yaml Key

```
Error: ConfigMap default/my-config does not contain 'values.yaml' key
```

**Solution**: Add a `values.yaml` key to your ConfigMap data.

### Invalid YAML

```
Error: failed to parse values YAML: yaml: line 5: ...
```

**Solution**: Validate your YAML syntax using a YAML validator or `yamllint`.

## Examples

See the [gateway-custom-config.yaml](../config/samples/gateway-custom-config.yaml) file for complete examples.
