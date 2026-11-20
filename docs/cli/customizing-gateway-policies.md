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

A container engine is a prerequisite for executing this command: Docker by default, or Podman or Buildah. The command checks that the selected engine is available at the beginning and fails immediately if it is not.

To build with Podman or Buildah instead of Docker, pass `--container-engine`, or set the `WSO2AP_CONTAINER_ENGINE` environment variable, for example in a CI job. The flag takes precedence over the environment variable.

```shell
ap gateway image build --container-engine podman
```

```shell
export WSO2AP_CONTAINER_ENGINE=buildah
ap gateway image build
```

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
    gomodule: github.com/wso2/gateway-controllers/policies/respond@v0.1.0 # Hub
```

Policies defined with a `filePath` are treated as **local policies**:

- These policies are not managed through Go Modules.

All other policies are treated as **PolicyHub policies**:


Even an output directory is not explicitly specified, the built images are cached by default. On subsequent executions, the cache is cleared and replaced with newly built images. The cache location is displayed in the command output.

### Non-Self-Explanatory Flags

- `--name` (`-n`): Sets the gateway name. If not provided, the current directory name is used by default.
- `--path`: Specifies the directory containing `build.yaml`. By default, this is the current working directory.
- `push`: Pushes the built images to the image repository.
- `no-cache`: Disables the container engine's build cache.
- `--platform`: Specifies the target platform. By default, the host platform is used. With `--platform`, the images are always pushed: Docker uses `docker buildx`, and Podman and Buildah build a manifest list and push it.
- `--container-engine`: Selects the container engine: `docker` (default), `podman` or `buildah`. Can also be set with the `WSO2AP_CONTAINER_ENGINE` environment variable.
