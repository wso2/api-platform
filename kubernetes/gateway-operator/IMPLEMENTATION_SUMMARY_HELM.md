# Gateway Operator - Helm Deployment Implementation Summary

## Overview

Successfully refactored the Gateway Operator to support Helm-based deployment instead of direct Kubernetes manifest templates. The implementation maintains backward compatibility with the existing template-based approach.

## Changes Made

### 1. New Helm Client Package (`internal/helm/client.go`)

Created a comprehensive Helm client wrapper that provides:
- **InstallOrUpgrade**: Installs new releases or upgrades existing ones
- **Uninstall**: Removes Helm releases cleanly
- **Values Management**: Parses values from YAML strings or files
- **Release Management**: Checks for existing releases and handles lifecycle

Key features:
- Automatic detection of existing releases
- Configurable timeouts and wait options
- Support for local charts and chart repositories
- Proper error handling and logging

### 2. Configuration Updates (`internal/config/config.go`)

Added new configuration fields to `GatewayConfig`:
```go
UseHelm                bool   // Enable Helm-based deployment (default: true)
HelmChartPath          string // Path to Helm chart
HelmChartName          string // Chart name
HelmChartVersion       string // Chart version
HelmValuesTemplatePath string // Path to values template
```

Environment variables:
- `GATEWAY_USE_HELM` - Enable/disable Helm deployment
- `GATEWAY_HELM_CHART_PATH` - Helm chart location
- `GATEWAY_HELM_CHART_NAME` - Chart name
- `GATEWAY_HELM_CHART_VERSION` - Chart version
- `GATEWAY_HELM_VALUES_TEMPLATE_PATH` - Values template path

### 3. Helm Values Template (`internal/controller/resources/gateway-helm-values.yaml.tmpl`)

Created a comprehensive values template that:
- Maps Gateway spec to Helm chart values
- Supports image configuration (repository and tag separation)
- Configures resource limits and requests
- Sets control plane connection details
- Manages storage configuration
- Includes sensible defaults

Template variables:
- Gateway name, replicas
- Controller and router images (split into repository + tag)
- Control plane host, port, token
- Storage type and paths
- Resource requirements for both controller and router
- Image pull policies

### 4. Helm Values Data Structure (`internal/k8sutil/helm_values_data.go`)

New data structure for Helm values:
```go
type GatewayHelmValuesData struct {
    GatewayName               string
    Replicas                  int32
    ControllerImageRepository string
    ControllerImageTag        string
    RouterImageRepository     string
    RouterImageTag            string
    ControlPlaneHost          string
    ControlPlanePort          string
    // ... more fields
}
```

Includes helper function `ParseImageReference` to split image references into repository and tag components.

### 5. Controller Modifications (`internal/controller/gateway_controller.go`)

**Reconciliation Flow**:
```
applyGatewayManifest()
    ├─ if UseHelm:
    │   └─ deployGatewayWithHelm()
    │       ├─ buildHelmValuesData()
    │       ├─ RenderTemplate()
    │       └─ helmClient.InstallOrUpgrade()
    └─ else:
        └─ deployGatewayWithTemplate() [legacy]
```

**Deletion Flow**:
```
deleteGatewayResources()
    ├─ if UseHelm:
    │   └─ deleteGatewayWithHelm()
    │       └─ helmClient.Uninstall()
    └─ else:
        └─ Rely on owner references [legacy]
```

New functions:
- `deployGatewayWithHelm()`: Handles Helm-based deployment
- `buildHelmValuesData()`: Builds values from Gateway
- `deleteGatewayWithHelm()`: Uninstalls Helm releases
- `deployGatewayWithTemplate()`: Legacy template-based deployment

### 6. Template Rendering Utility (`internal/k8sutil/manifest.go`)

Added `RenderTemplate()` function:
- Reads template files
- Supports custom template functions (toYaml, nindent)
- Executes templates with provided data
- Returns rendered content as string

### 7. Dependencies (`go.mod`)

Added required dependencies:
- `helm.sh/helm/v3 v3.16.3` - Helm Go SDK
- `k8s.io/cli-runtime v0.29.2` - Kubernetes CLI runtime
- Updated `sigs.k8s.io/controller-runtime` to v0.19.4 for compatibility

### 8. Documentation (`HELM_DEPLOYMENT.md`)

Comprehensive documentation covering:
- Architecture and components
- Configuration options
- Usage examples
- Migration guide
- Troubleshooting
- Development guidelines

## Key Design Decisions

### 1. Backward Compatibility
- Kept existing template-based deployment as fallback
- Configuration flag (`UseHelm`) to switch between modes
- No breaking changes to existing Gateway CRD

### 2. Release Naming Convention
- Helm releases named: `{gateway-name}-gateway`
- Example: Gateway "my-gateway" → Release "my-gateway-gateway"
- Ensures uniqueness and clear association

### 3. Values Template Architecture
- Separate template file for flexibility
- Go template syntax for familiarity
- Supports dynamic configuration from Gateway spec
- Can be customized without code changes

### 4. Error Handling
- Proper error propagation through controller
- Detailed logging at each step
- Graceful fallback mechanisms
- Status updates on failures

### 5. Resource Management
- Helm manages resource lifecycle
- No manual cleanup needed for Helm deployments
- Release history maintained by Helm
- Support for rollbacks through Helm CLI

## Testing Recommendations

### Unit Tests
1. Test `ParseImageReference()` with various image formats
2. Test Helm values data building from Gateway
3. Test template rendering with different configurations
4. Mock Helm client for controller tests

### Integration Tests
1. Deploy gateway with Helm enabled
2. Verify Helm release creation
3. Update gateway spec and verify upgrade
4. Delete gateway and verify uninstall
5. Test migration from template to Helm deployment

### E2E Tests
1. Full gateway lifecycle with Helm
2. Multiple gateways in different namespaces
3. Gateway with custom resource requirements
4. Gateway with different storage configurations

## Migration Path

### For New Deployments
- Default to Helm-based deployment
- Use provided chart at `../helm/gateway-helm-chart`
- No additional configuration needed

### For Existing Deployments
1. Set `GATEWAY_USE_HELM=true`
2. Restart operator
3. Existing gateways migrate on next reconciliation
4. Operator creates Helm releases for existing deployments

### Rollback Option
- Set `GATEWAY_USE_HELM=false` to revert
- Useful for debugging or compatibility issues
- Template-based deployment still fully supported

## Future Enhancements

1. **Chart Repository Support**
   - Install from remote Helm repositories
   - Automatic version updates

2. **Custom Values Override**
   - Allow users to provide custom values in Gateway
   - Merge with generated values

3. **Multi-Chart Deployments**
   - Support installing dependencies
   - Manage multiple charts for complex deployments

4. **Chart Testing**
   - Integrate `helm test` in CI/CD
   - Validate chart deployments automatically

5. **Release Rollback**
   - Automatic rollback on failures
   - Integration with Gateway status

## Files Created/Modified

### New Files
- `internal/helm/client.go` - Helm client implementation
- `internal/k8sutil/helm_values_data.go` - Helm values data structures
- `internal/controller/resources/gateway-helm-values.yaml.tmpl` - Values template
- `HELM_DEPLOYMENT.md` - Comprehensive documentation

### Modified Files
- `internal/config/config.go` - Added Helm configuration
- `internal/controller/gateway_controller.go` - Added Helm deployment logic
- `internal/k8sutil/manifest.go` - Added RenderTemplate function
- `go.mod` - Added Helm dependencies

## Build Status

✅ Successfully compiles without errors
✅ All dependencies resolved
✅ No breaking changes to existing code
✅ Backward compatible with template-based deployment

## Next Steps

1. **Testing**: Run comprehensive tests with Helm deployment enabled
2. **Documentation**: Update main README with Helm deployment info
3. **Examples**: Create example Gateway resources
4. **CI/CD**: Update build pipeline to test Helm deployments
5. **Helm Chart**: Ensure gateway Helm chart is production-ready
