# Gateway Image Build - Implementation Plan

## Command Structure

```bash
apipctl gateway image build \
  --image-tag <image-tag> \
  [--path <directory>] \
  [--image-repository <image-repository>] \
  [--gateway-builder <gateway-builder-image>] \
  [--gateway-controller-base-image <gateway-controller-base-image>] \
  [--router-base-image <router-base-image>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--offline] \
  [--output-dir <output_dir>]
```

### Required Flags
- `--image-tag`: Docker image tag for the gateway build

### Optional Flags & Defaults
- `--path` / `-p`: Current directory (`.`) - Directory containing `policy-manifest.yaml` and `policy-manifest-lock.yaml`
- `--image-repository`: `ghcr.io/wso2/api-platform`
- `--gateway-builder`: `ghcr.io/wso2/api-platform/gateway-builder:0.2.0`
- `--gateway-controller-base-image`: Uses default from gateway-builder image
- `--router-base-image`: Uses default from gateway-builder image
- `--platform`: Uses host platform
- `--push`: `false`
- `--no-cache`: `false`
- `--offline`: `false`
- `--output-dir`: No output (empty)

### Directory Structure Requirements
The `--path` flag must point to a directory containing:
- `policy-manifest.yaml` (required in both online and offline modes)
- `policy-manifest-lock.yaml` (required in offline mode, generated in online mode)

## Policy Manifest Format

```yaml
version: v1/alpha1
versionResolution: minor  # Optional root-level default
policies:
  - name: BasicAuth
    version: v1.0.0
    versionResolution: exact  # Override root-level
  - name: BasicAuth
    version: v1.0.1
    versionResolution: exact
  - name: BasicAuth
    version: v1.0.2
    versionResolution: minor
  - name: MyCustomPolicy
    version: v1.0.0
    filePath: ./my-custom-policy/v1.0.0  # Local policy
```

### Policy Types
1. **Hub Policies**: No `filePath` specified → fetched from PolicyHub
2. **Local Policies**: Has `filePath` → loaded from local filesystem

## Policy Manifest Lock Format

```yaml
version: v1/alpha1
policies:
  - name: BasicAuth
    version: v1.0.0
    checksum: sha256:abc123...
    source: hub
  - name: BasicAuth
    version: v1.0.8  # Resolved version
    checksum: sha256:def456...
    source: hub
  - name: MyCustomPolicy
    version: v1.0.0
    checksum: sha256:ghi789...
    source: local
```

## File Locations

### Cache Structure
```
~/.apipctl/
├── cache/
│   └── policies/
│       ├── basic-auth-v1.0.0.zip
│       ├── jwt-auth-v0.1.0.zip
│       └── rate-limit-v1.5.2.zip
└── .temp/  # Temporary files (cleaned after operation)
```

### Policy Naming Convention
- Format: `<policy-name>-v<version>.zip`
- Policy name and version in kebab-case
- Example: `BasicAuth v1.0.0` → `basic-auth-v1.0.0.zip`

## Implementation Flow

### Online Mode (Default)

#### Step 1: Pre-flight Checks
- Check Docker availability (fail early if not present)
- Validate required flags

#### Step 2: Read Policy Manifest
- Load from `-f` flag or default `policy-manifest.yaml`
- Validate manifest structure

#### Step 3: Parse Policies
- Separate into:
  - **Local policies**: Have `filePath`
  - **Hub policies**: No `filePath`

#### Step 4: Process Local Policies
- For each local policy:
  - Verify path exists (file or folder)
  - If folder: zip contents to `.apipctl/.temp/<policy>-v<version>.zip`
  - If file: copy to temp location
  - Calculate SHA-256 checksum

#### Step 5: Resolve Hub Policies
- Send request to PolicyHub API:
  ```json
  [
    {
      "name": "rate-limiting",
      "version": "1.1.0",
      "versionResolution": "exact"
    },
    {
      "name": "jwt-authentication",
      "version": "2.1.2",
      "versionResolution": "patch"
    }
  ]
  ```
- Receive resolved versions with download URLs and checksums

#### Step 6: Download/Verify Hub Policies
- For each resolved policy:
  - Check if exists in `~/.apipctl/cache/policies/<policy>-v<version>.zip`
  - If exists:
    - Verify checksum matches
    - If match: skip download
    - If mismatch: warn user + re-download + replace cache
  - If not exists:
    - Download from PolicyHub
    - Verify checksum
    - Save to cache

#### Step 7: Generate Lock File
- Create `policy-manifest-lock.yaml` in the directory specified by `--path` (defaults to current directory)
- Include all policies (hub + local) with:
  - Resolved versions
  - SHA-256 checksums
  - Source (hub/local)

#### Step 8: Cleanup
- Remove `.apipctl/.temp/` contents
- Use defer to ensure cleanup on error

#### Step 9: Display Summary
- Show all flags with values (including defaults)
- List manifest file location
- List lock file location
- List loaded policies with sources

### Offline Mode (`--offline` flag)

#### Step 1: Pre-flight Checks
- Check Docker availability

#### Step 2: Read Manifest and Lock Files
- Read `policy-manifest.yaml` from the directory specified by `--path` (defaults to current directory)
- Read `policy-manifest-lock.yaml` from the same directory (must exist or fail)
- Fail with helpful error message if either file is not found

#### Step 3: Verify Hub Policies
- For each policy with `source: hub`:
  - Check `~/.apipctl/cache/policies/<policy>-v<version>.zip` exists
  - Verify SHA-256 checksum matches lock file
  - Fail if missing or mismatch

#### Step 4: Verify Local Policies
- For each policy with `source: local`:
  - Use the `filePath` from the manifest to locate the policy
  - Verify SHA-256 checksum matches lock file
  - Fail if missing or mismatch

#### Step 5: Display Summary
- Show all flags with values
- List manifest file location
- List lock file location
- List verified policies

## Error Handling

### Common Errors
- `--path` directory does not exist → Fail: "The specified path does not exist: <path>"
- `--path` points to a file → Fail: "The --path flag must point to a directory, not a file: <path>"
- Manifest file not found in directory → Fail: "policy-manifest.yaml not found in directory: <path>. Please ensure the manifest file exists in this location."

### Online Mode Errors
- Docker not available → Fail immediately
- Manifest file invalid YAML → Fail with clear parsing error
- PolicyHub unreachable → Fail (cannot resolve versions)
- Policy download fails → Fail
- Checksum mismatch → Warn + re-download
- No policies resolved successfully → Fail
- Lock file generation fails → Fail with write permission error

### Offline Mode Errors
- Docker not available → Fail immediately
- Lock file not found → Fail: "lock file 'policy-manifest-lock.yaml' not found in directory: <path>. Expected file: <full-path>. Please run the build command in ONLINE mode first (without --offline flag) to generate the lock file"
- Lock file corrupted → Fail: "failed to parse lock file at '<path>': <error>. The lock file may be corrupted. Try regenerating it by running in ONLINE mode (without --offline flag)"
- Policy not in cache → Fail: "Policy <name>-v<version> not found in cache. Run without --offline first."
- Checksum mismatch → Fail: "Checksum mismatch for <policy>. Cache may be corrupted."
- Local policy file not found → Fail with path where it was expected

## Utility Functions

### File Operations
- `calculateSHA256(filePath string) (string, error)`
- `zipDirectory(srcDir, destZip string) error`
- `findPoliciesFolders(rootDir string) ([]string, error)`
- `policyExists(policyName, version string, searchPaths []string) (string, error)`

### Policy Operations
- `parseManifest(manifestPath string) (*PolicyManifest, error)`
- `parseLockFile(lockPath string) (*PolicyLock, error)`
- `generateLockFile(policies []Policy, outputPath string) error`
- `downloadPolicy(url, destPath string) error`
- `verifyChecksum(filePath, expectedChecksum string) (bool, error)`

### PolicyHub Integration
- `resolveHubPolicies(policies []PolicyRequest) (*ResolveResponse, error)`
- `toKebabCase(name string) string`
- `formatPolicyFileName(name, version string) string`

### Display
- `displaySummary(flags map[string]string, policies []Policy, mode string)`
- `displayProgress(step int, total int, message string)`
