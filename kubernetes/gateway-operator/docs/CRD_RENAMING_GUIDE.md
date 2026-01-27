# CRD Renaming Guide

This document provides a step-by-step guide for renaming a Custom Resource Definition (CRD) in this Kubernetes operator project.

## Overview

Renaming a CRD involves changes across multiple files and requires careful coordination to ensure all references are updated consistently. This guide outlines the process used to rename the `Gateway` CRD to `APIGateway`.

## Prerequisites

- Go development environment
- `controller-gen` tool (installed via `make` targets)
- Understanding of Kubernetes CRDs and kubebuilder

## Step-by-Step Process

### 1. Update API Types (`api/v1alpha1/`)

1. **Rename the types file**:
   - `gateway_types.go` → `apigateway_types.go`

2. **Update struct names in the types file**:
   - Rename the main struct (e.g., `Gateway` → `APIGateway`)
   - Rename the list struct (e.g., `GatewayList` → `APIGatewayList`)
   - Update the struct comment to reflect the new name
   - Keep helper types (e.g., `GatewaySpec`, `GatewayStatus`) as they are internal

3. **Update the `init()` function**:
   ```go
   func init() {
       SchemeBuilder.Register(&APIGateway{}, &APIGatewayList{})
   }
   ```

### 2. Update PROJECT File

Update the `PROJECT` file to reflect the new kind:

```yaml
- api:
    crdVersion: v1
    namespaced: true
  domain: gateway.api-platform.wso2.com
  kind: APIGateway  # Changed from Gateway
  path: github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1
  version: v1alpha1
```

### 3. Update Controller (`internal/controller/`)

1. **Rename the controller file**:
   - `gateway_controller.go` → `apigateway_controller.go`

2. **Update kubebuilder RBAC markers**:
   ```go
   //+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways,verbs=get;list;watch;create;update;patch;delete
   //+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways/status,verbs=get;update;patch
   //+kubebuilder:rbac:groups=gateway.api-platform.wso2.com,resources=apigateways/finalizers,verbs=update
   ```

3. **Update finalizer names**:
   - Rename the finalizer constant (e.g., `gatewayFinalizerName` → `apigatewayFinalizerName`)
   - Update the finalizer string value (e.g., `"gateway.api-platform.wso2.com/gateway-finalizer"` → `"gateway.api-platform.wso2.com/apigateway-finalizer"`)
   - **Important**: This is critical for proper cleanup during CR deletion

4. **Update type references**:
   - Change all `*apiv1.Gateway` → `*apiv1.APIGateway`
   - Change all `apiv1.GatewayList` → `apiv1.APIGatewayList`

5. **Update log messages and comments** for clarity

### 4. Update main.go (`cmd/main.go`)

Update error messages and controller references to reflect the new CRD name.

### 5. Regenerate Manifests

Run the following commands to regenerate CRD and RBAC manifests:

```bash
# Disable go.work if present to avoid module resolution issues
GOWORK=off make manifests generate
```

This will:
- Generate new CRD YAML in `config/crd/bases/`
- Regenerate `zz_generated.deepcopy.go`
- Update `config/rbac/role.yaml`

### 6. Clean Up Old CRD Files

Delete the old CRD file:
```bash
rm config/crd/bases/gateway.api-platform.wso2.com_gateways.yaml
```

### 7. Update RBAC Files (`config/rbac/`)

1. **Rename RBAC files**:
   - `gateway_editor_role.yaml` → `apigateway_editor_role.yaml`
   - `gateway_viewer_role.yaml` → `apigateway_viewer_role.yaml`

2. **Update RBAC file contents**:
   - Update role names (e.g., `gateway-editor-role` → `apigateway-editor-role`)
   - Update resource references (e.g., `gateways` → `apigateways`)

3. **Update `kustomization.yaml`** to reference new file names

### 8. Update Sample Files (`config/samples/`)

1. **Rename sample files**:
   - `api_v1_gateway.yaml` → `api_v1_apigateway.yaml`

2. **Update sample file contents**:
   - Change `kind: Gateway` → `kind: APIGateway`

3. **Update `kustomization.yaml`** to reference new file names

### 9. Update Helm Chart CRDs

Copy the newly generated CRD to the Helm chart:
```bash
cp config/crd/bases/gateway.api-platform.wso2.com_apigateways.yaml \
   ../helm/operator-helm-chart/crds/
rm ../helm/operator-helm-chart/crds/gateway.api-platform.wso2.com_gateways.yaml
```

### 10. Update Documentation (`docs/`)

Update all documentation files that reference the old CRD name:
- Update `kind:` references in YAML examples
- Update kubectl commands (e.g., `kubectl get gateway` → `kubectl get apigateway`)
- Update prose references to the CRD

### 11. Update README.md

Update the main README with new CRD references and commands.

### 12. Update CI/CD Workflows (`.github/workflows/`)

Update workflow files that create or interact with the CRD:
- Update `kind:` in kubectl apply commands
- Update kubectl wait/get/describe commands
- Update variable names and comments

### 13. Verify Changes

1. **Build the operator**:
   ```bash
   GOWORK=off go build ./...
   ```

2. **Run tests**:
   ```bash
   GOWORK=off go test ./...
   ```

3. **Verify CRD generation**:
   - Check that the new CRD file exists and has correct names
   - Verify plural name, kind, and listKind are correct

## Files Changed Summary

| Category | Files |
|----------|-------|
| API Types | `api/v1alpha1/gateway_types.go` → `apigateway_types.go` |
| Controller | `internal/controller/gateway_controller.go` → `apigateway_controller.go` |
| CRD | `config/crd/bases/gateway.api-platform.wso2.com_gateways.yaml` → `*_apigateways.yaml` |
| RBAC | `config/rbac/gateway_*_role.yaml` → `apigateway_*_role.yaml` |
| Samples | `config/samples/api_v1_gateway.yaml` → `api_v1_apigateway.yaml` |
| Helm | `kubernetes/helm/operator-helm-chart/crds/*` |
| Docs | `docs/*.md` |
| Workflows | `.github/workflows/operator-integration-test.yml` |

## Important Notes

1. **No backward compatibility**: This guide assumes no backward compatibility is needed. If you need to support both old and new CRD names during migration, additional steps are required.

2. **Regeneration**: Always run `make manifests generate` after changing type definitions to ensure CRD and deepcopy files are in sync.

3. **go.work issues**: If you encounter module resolution errors with controller-gen, use `GOWORK=off` prefix.

4. **Testing**: Always run tests after renaming to catch any missed references.

5. **Search comprehensively**: Use grep to find all references to the old name before finalizing changes.

## Useful Commands

```bash
# Find all references to old CRD name
grep -rn "kind: Gateway" --include="*.yaml" --include="*.yml" .
grep -rn "Gateway" --include="*.go" .

# Regenerate manifests
GOWORK=off make manifests generate

# Build and test
GOWORK=off go build ./...
GOWORK=off go test ./...
```
