# Gateway Image Build - Implementation Plan

## Command Structure

```bash
ap gateway image build \
  [--name <gateway-name>] \
  [--path <directory>] \
  [--repository <image-repository>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--output-dir <output_dir>]
```

### Optional Flags & Defaults
- `--name`: Gateway name (defaults to directory name)
- `--path` / `-p`: Current directory (`.`) - Directory containing `build.yaml`
- `--repository`: `ghcr.io/wso2/api-platform` - Docker image repository
- `--platform`: Uses host platform - Target platform (e.g., linux/amd64)
- `--push`: `false` - Push image to registry after build
- `--no-cache`: `false` - Build without using cache
- `--output-dir`: No output (empty) - Output directory for build artifacts

### Directory Structure Requirements
The `--path` flag must point to a directory containing:
- `build.yaml` (required)

Or execute in manifest location

Note: The `build.yaml` file is internally renamed to `policy-manifest.yaml` when copied to the Docker build workspace.

## Policy Manifest Format

### Basic Example
```yaml
version: v1
gateway:
  version: "1.0.4"  # Required: Gateway version for the build
policies:
  - name: api-key-auth
    filePath: api-key-auth-v0.1.0 # Local Policy
  - name: respond
    gomodule: github.com/wso2/gateway-controllers/policies/respond@v0.1.0 # Hub Policy
```

### With Custom Images (Optional)
```yaml
version: v1
gateway:
  version: "1.0.4"    # Required: Gateway version
  images:             # Optional: Custom image paths
    builder: "internal-registry.company.com/wso2/gateway-builder:1.0.4"
    controller: "internal-registry.company.com/wso2/gateway-controller:1.0.4"
    router: "internal-registry.company.com/wso2/gateway-router:1.0.4"
policies:
  - name: api-key-auth
    filePath: api-key-auth-v0.1.0
```

### Required Fields
- `gateway.version`: Gateway version for the build

### Optional Fields
- `gateway.images.builder`: Custom gateway builder image (defaults to `ghcr.io/wso2/api-platform/gateway-builder:<version>`)
- `gateway.images.controller`: Custom gateway controller base image (defaults to `ghcr.io/wso2/api-platform/gateway-controller:<version>`)
- `gateway.images.router`: Custom router base image (defaults to `ghcr.io/wso2/api-platform/gateway-router:<version>`)

Each image path can be specified independently. If not provided, the default image path will be constructed using the `gateway.version`.

### Policy Types
1. **Hub Policies**: No `filePath` specified → fetched from PolicyHub
2. **Local Policies**: Has `filePath` → loaded from local filesystem

## File Locations

### Cache Structure
```
~/.ap/
├── cache/
│   └── policies/
│       ├── basic-auth-v1.0.0.zip
│       ├── jwt-auth-v0.1.1.zip
│       └── rate-limit-v1.5.2.zip
└── .temp/  # Temporary files (cleaned after operation)
```
