# Customizing the Gateway by Adding and Removing Policies

## CLI Command

```shell
ap gateway image build \
  [--name <gateway-name>] \
  [--path <gateway-project-dir>] \
  [--repository <image-repository>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--output-dir <output_dir>]
```

## Sample Command

```shell
ap gateway image build
```

## Additional Notes for Users

Use `ap gateway image build --help` to view detailed usage information for this command.

Docker is a prerequisite for executing this command. If Docker is not available, the command will validate this at the beginning and fail immediately.

A `build.yaml` file is mandatory. By default, the CLI expects this file to be present in the current working directory unless a specific --path is provided.

### Sample `build.yaml`

```yaml
version: v1
gateway:
  version: 1.0.4
  # Optional: override base images
  images:
    builder: "internal-registry.company.com/wso2/gateway-builder:1.0.5" # Optional: override base image
    controller: "internal-registry.company.com/wso2/gateway-controller:1.0.5" # Optional: override base image
    runtime: "internal-registry.company.com/wso2/gateway-runtime:1.0.5" # Optional: override base image
policies:
  - name: api-key-auth
    filePath: api-key-auth-v0.1.0 # Local
  - name: respond
    gomodule: github.com/wso2/gateway-controllers/policies/respond@v1.1.0 # Hub
    checksum: h1:cgoKDMdixHqGygwuIPkbSsFGfjlANoUPynrUVd/ZSBA=
```

Policies defined with a `filePath` are treated as **local policies**:

- These policies are not managed through Go Modules.

All other policies are treated as **PolicyHub policies**:

- A `gomodule` entry must name an exact version (`@v1.1.0` or a pseudo-version, not `@v1`
  or `@latest`) and carry a `checksum:` with the module's `h1:` hash. Get it with
  `go mod download -json <module>@<version>` (the `Sum` field); it is the same value Go
  records in `go.sum`. The build fails if the downloaded module does not match.
- A `pipPackage` that points at a git repository must pin a full commit SHA, for example
  `github.com/wso2/gateway-controllers/policies/prompt-compressor@<40-hex commit SHA>`.
- A `pipPackage` from a package index must use an exact `==` version and carry
  `checksum: sha256:<hex>` of the wheel.

See "Policy pinning in `build.yaml`" in [gateway/README.md](../../gateway/README.md) for how
to bump a pinned policy.


Even an output directory is not explicitly specified, the built images are cached by default. On subsequent executions, the cache is cleared and replaced with newly built images. The cache location is displayed in the command output.

### Non-Self-Explanatory Flags

- `--name` (`-n`): Sets the gateway name. If not provided, the current directory name is used by default.
- `--path`: Specifies the directory containing `build.yaml`. By default, this is the current working directory.
- `push`: Pushes the built images to Docker.
- `no-cache`: Disables Docker build cache usage.
- `--platform`: Specifies the target platform. By default, the host platform is used.
