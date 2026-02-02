# Gateway Image Build - Implementation Plan

## Command Structure

```bash
ap gateway image build \
  --image-tag <image-tag> \
  [--path <directory>] \
  [--image-repository <image-repository>] \
  [--gateway-builder <gateway-builder-image>] \
  [--gateway-controller-base-image <gateway-controller-base-image>] \
  [--router-base-image <router-base-image>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--output-dir <output_dir>]
```

### Required Flags
- `--image-tag`: Docker image tag for the gateway build

### Optional Flags & Defaults
- `--path` / `-p`: Current directory (`.`) - Directory containing `build.yaml`
- `--image-repository`: `ghcr.io/wso2/api-platform`
- `--gateway-builder`: `ghcr.io/wso2/api-platform/gateway-builder:latest`
- `--gateway-controller-base-image`: Uses default from gateway-builder image
- `--router-base-image`: Uses default from gateway-builder image
- `--platform`: Uses host platform
- `--push`: `false`
- `--no-cache`: `false`
- `--output-dir`: No output (empty)

### Directory Structure Requirements
The `--path` flag must point to a directory containing:
- `build.yaml` (required)

Or execute in manifest location

Note: The `build.yaml` file is internally renamed to `policy-manifest.yaml` when copied to the Docker build workspace.

## Policy Manifest Format

```yaml
version: v1
policies:
  - name: api-key-auth
    filePath: api-key-auth-v0.1.0 # Local Policy
  - name: respond
    gomodule: github.com/wso2/gateway-controllers/policies/respond@v0.1.0 # Hub Policy
```

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
