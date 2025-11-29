# Gateway Operator Configuration Guide

This document explains how to configure the gateway-operator using YAML configuration files and environment variables.

## Overview

The gateway-operator uses a layered configuration approach with the following priority (highest to lowest):

1. **Environment Variables** - Runtime overrides
2. **YAML Configuration File** - Primary configuration source
3. **Default Values** - Fallback when not specified

This approach is similar to the gateway-controller, using the [koanf](https://github.com/knadh/koanf) library for configuration management.

## Configuration File

### Location

By default, the operator looks for configuration at:
```
config/config.yaml
```

You can specify a custom location using the `--config` flag:
```bash
./bin/manager --config /path/to/custom-config.yaml
```

### Structure

```yaml
# Gateway deployment configuration
gateway:
  namespace: "gateway-system"
  controlPlaneHost: "http://platform-api:3001"
  manifestPath: ""  # Leave empty when using Helm
  
  helm:
    enabled: true
    chartName: "oci://ghcr.io/your-org/gateway-helm-chart"
    chartVersion: "0.1.0"
    valuesFilePath: "/etc/gateway-operator/values/values.yaml"
    releaseNamePrefix: "gateway"

# Reconciliation behavior
reconciliation:
  enabled: true
  interval: 300  # seconds

# Logging configuration
logging:
  level: "info"   # debug, info, warn, error
  format: "json"  # json, console
```

## Configuration Fields

### Gateway Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `gateway.namespace` | string | `gateway-system` | Namespace where gateway resources are deployed |
| `gateway.controlPlaneHost` | string | `http://platform-api:3001` | Control plane API endpoint |
| `gateway.manifestPath` | string | `""` | Path to K8s manifest (legacy mode, leave empty for Helm) |

### Helm Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `gateway.helm.enabled` | bool | `true` | Enable Helm-based deployment |
| `gateway.helm.chartName` | string | (required) | Helm chart reference (OCI or repo/chart) |
| `gateway.helm.chartVersion` | string | `0.1.0` | Chart version to install |
| `gateway.helm.valuesFilePath` | string | (required) | Path to Helm values.yaml file |
| `gateway.helm.releaseNamePrefix` | string | `gateway` | Prefix for Helm release names |

### Reconciliation Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `reconciliation.enabled` | bool | `true` | Enable automatic reconciliation |
| `reconciliation.interval` | int | `300` | Reconciliation interval in seconds |

### Logging Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `logging.level` | string | `info` | Log level (debug/info/warn/error) |
| `logging.format` | string | `json` | Log format (json/console) |

## Environment Variable Overrides

Any configuration field can be overridden using environment variables with the following naming convention:

```
<SECTION>_<FIELD>
```

For nested fields, use underscores:

```
<SECTION>_<SUBSECTION>_<FIELD>
```

### Examples

```bash
# Override namespace
export GATEWAY_NAMESPACE="prod-gateway-system"

# Override Helm chart version
export GATEWAY_HELM_CHART_VERSION="0.2.0"

# Override values file path
export GATEWAY_HELM_VALUES_FILE_PATH="/custom/path/values.yaml"

# Override log level
export LOG_LEVEL="debug"

# Override reconciliation interval
export RECONCILIATION_INTERVAL="600"
```

## Deployment Scenarios

### 1. Development with Local Config

```bash
# Use default config/config.yaml
./bin/manager

# Or specify custom config
./bin/manager --config config/dev-config.yaml
```

### 2. Docker Deployment

Mount the configuration file as a volume:

```yaml
services:
  gateway-operator:
    image: gateway-operator:latest
    volumes:
      - ./config/config.yaml:/app/config/config.yaml:ro
      - ./values/values.yaml:/etc/gateway-operator/values/values.yaml:ro
    command: ["--config", "/app/config/config.yaml"]
```

### 3. Kubernetes Deployment

Use ConfigMaps for configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-operator-config
  namespace: gateway-operator-system
data:
  config.yaml: |
    gateway:
      namespace: "gateway-system"
      controlPlaneHost: "http://platform-api:3001"
      helm:
        enabled: true
        chartName: "oci://ghcr.io/your-org/gateway-helm-chart"
        chartVersion: "0.1.0"
        valuesFilePath: "/etc/gateway-operator/values/values.yaml"
    reconciliation:
      enabled: true
      interval: 300
    logging:
      level: "info"
      format: "json"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-helm-values
  namespace: gateway-operator-system
data:
  values.yaml: |
    # Your Helm chart values here
    replicaCount: 2
    image:
      repository: your-registry/gateway
      tag: "1.0.0"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway-operator
  namespace: gateway-operator-system
spec:
  template:
    spec:
      containers:
      - name: manager
        image: gateway-operator:latest
        args:
          - --config=/etc/operator/config.yaml
        volumeMounts:
          - name: config
            mountPath: /etc/operator
            readOnly: true
          - name: helm-values
            mountPath: /etc/gateway-operator/values
            readOnly: true
      volumes:
        - name: config
          configMap:
            name: gateway-operator-config
        - name: helm-values
          configMap:
            name: gateway-helm-values
```

### 4. Environment Variable Override (Production)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway-operator
spec:
  template:
    spec:
      containers:
      - name: manager
        image: gateway-operator:latest
        env:
          # Override specific settings
          - name: GATEWAY_NAMESPACE
            value: "production-gateway"
          - name: LOG_LEVEL
            value: "warn"
          - name: GATEWAY_HELM_CHART_VERSION
            value: "1.2.3"
          - name: RECONCILIATION_INTERVAL
            value: "600"
        volumeMounts:
          - name: config
            mountPath: /etc/operator
            readOnly: true
        args:
          - --config=/etc/operator/config.yaml
```

## Configuration Validation

The operator validates configuration on startup. If validation fails, the operator will exit with an error message.

Common validation errors:

- **Missing chart name**: `gateway.helm.chartName` is required when Helm is enabled
- **Missing values file path**: `gateway.helm.valuesFilePath` is required when Helm is enabled
- **Invalid log level**: Must be one of: debug, info, warn, error
- **Invalid log format**: Must be one of: json, console

## Migration from Environment-Only Configuration

If you were previously using only environment variables:

1. Create `config/config.yaml` with default values
2. Move static configuration to the YAML file
3. Keep environment-specific overrides as environment variables
4. Update deployment manifests to mount the config file

### Before (Environment Variables Only)

```bash
export GATEWAY_NAMESPACE="gateway-system"
export GATEWAY_CONTROL_PLANE_HOST="http://platform-api:3001"
export GATEWAY_HELM_ENABLED="true"
export GATEWAY_HELM_CHART_NAME="oci://ghcr.io/your-org/gateway-helm-chart"
export GATEWAY_HELM_CHART_VERSION="0.1.0"
export GATEWAY_HELM_VALUES_FILE_PATH="/etc/values/values.yaml"
./bin/manager
```

### After (YAML + Environment Overrides)

**config/config.yaml:**
```yaml
gateway:
  namespace: "gateway-system"
  controlPlaneHost: "http://platform-api:3001"
  helm:
    enabled: true
    chartName: "oci://ghcr.io/your-org/gateway-helm-chart"
    chartVersion: "0.1.0"
    valuesFilePath: "/etc/values/values.yaml"
```

**Runtime:**
```bash
# Only override what changes between environments
export GATEWAY_HELM_CHART_VERSION="0.2.0"
./bin/manager --config config/config.yaml
```

## Best Practices

1. **Use YAML for static configuration**: Store stable settings in the config file
2. **Use environment variables for dynamic values**: Override per-environment settings
3. **Version control your config**: Commit `config/config.yaml` to your repository
4. **Separate Helm values**: Keep Helm chart values in a separate `values.yaml` file
5. **Validate before deployment**: Test configuration changes locally before deploying
6. **Document custom settings**: Add comments to your config file explaining custom values

## Troubleshooting

### Configuration Not Loading

Check the operator logs for:
```
unable to load configuration
```

Common causes:
- File not found at specified path
- Invalid YAML syntax
- Permission denied reading config file

### Environment Variables Not Working

Ensure environment variable names match the expected format:
```bash
# Correct
export GATEWAY_NAMESPACE="test"

# Incorrect (case matters)
export gateway_namespace="test"
```

### Values File Not Found

Ensure the `valuesFilePath` points to a valid, readable file:
```bash
# Check if file exists and is readable
ls -la /etc/gateway-operator/values/values.yaml
```

## Related Documentation

- [Helm Deployment Guide](HELM_DEPLOYMENT.md)
- [Simplified Helm Usage](SIMPLIFIED_HELM.md)
- [Gateway Controller Configuration](../../gateway/gateway-controller/pkg/config/config.go) (reference implementation)
