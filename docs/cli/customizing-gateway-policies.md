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
  [--output-dir <output_dir>] \
  [--go-toolchain <gotoolchain>]
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
  # Optional: GOTOOLCHAIN for the gateway-builder container (defaults to "auto").
  # Accepts any Go GOTOOLCHAIN value, e.g. "auto", "go1.26.5" (pin a version),
  # or "go1.26.2+auto" (minimum version, upgrade if a policy requires newer).
  goToolchain: auto
  # Optional: override base images
  images:
    builder: "internal-registry.company.com/wso2/gateway-builder:1.0.5" # Optional: override base image
    controller: "internal-registry.company.com/wso2/gateway-controller:1.0.5" # Optional: override base image
    runtime: "internal-registry.company.com/wso2/gateway-runtime:1.0.5" # Optional: override base image
policies:
  - name: api-key-auth
    filePath: api-key-auth-v0.1.0 # Local
  - name: respond
    gomodule: github.com/wso2/gateway-controllers/policies/respond@v0.1.0 # Hub
```

Policies defined with a `filePath` are treated as **local policies**:

- These policies are not managed through Go Modules.

All other policies are treated as **PolicyHub policies**:


Even an output directory is not explicitly specified, the built images are cached by default. On subsequent executions, the cache is cleared and replaced with newly built images. The cache location is displayed in the command output.

### Non-Self-Explanatory Flags

- `--name` (`-n`): Sets the gateway name. If not provided, the current directory name is used by default.
- `--path`: Specifies the directory containing `build.yaml`. By default, this is the current working directory.
- `push`: Pushes the built images to Docker.
- `no-cache`: Disables Docker build cache usage.
- `--platform`: Specifies the target platform. By default, the host platform is used.
- `--go-toolchain`: Sets `GOTOOLCHAIN` for the gateway-builder container, controlling which Go toolchain compiles the policies. Defaults to `auto`, which downloads and uses a newer Go toolchain when a policy requires one (e.g. a policy whose `go.mod` declares a newer Go version than the builder ships). Accepts any Go `GOTOOLCHAIN` value: `auto`, a pinned version like `go1.26.5`, or a minimum-with-upgrade form like `go1.26.2+auto`. When provided, this flag takes precedence over `gateway.goToolchain` in `build.yaml`.
