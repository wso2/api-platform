# Customizing the Gateway by Adding and Removing Policies

## CLI Command

```shell
ap gateway image build \
  [--name <gateway-name>] \
  [--path <gateway-project-dir>] \
  [--repository <image-repository>] \
  [--version <gateway-version>] \
  [--gateway-builder <gateway-builder-image>] \
  [--gateway-controller-base-image <gateway-controller-base-image>] \
  [--router-base-image <router-base-image>] \
  [--push] \
  [--no-cache] \
  [--platform <platform>] \
  [--offline] \
  [--output-dir <output_dir>]
```

## Sample Command

```shell
ap gateway build
```

## Additional Notes for Users

Use `ap gateway build --help` to view detailed usage information for this command.

Docker is a prerequisite for executing this command. If Docker is not available, the command will validate this at the beginning and fail immediately.

A `policy_manifest.yaml` file is mandatory. By default, the CLI expects this file to be present in the current working directory unless a specific path is provided.

### Sample `policy_manifest.yaml`

```yaml
version: v1
versionResolution: minor
policies:
  - name: basic-auth
    version: v1.0.0
    versionResolution: exact
  - name: custom-auth
    version: v2.0.0
    filePath: ./sample.zip
```

Policies defined with a `filePath` are treated as **local policies**:

- These policies are not cached or managed through PolicyHub.

All other policies are treated as **PolicyHub policies**:

- They are cached locally and validated using PolicyHub checksums.
- If a policy is missing or the checksum does not match, it will be downloaded or replaced automatically.

Even an output directory is not explicitly specified, the built images are cached by default. On subsequent executions, the cache is cleared and replaced with newly built images. The cache location is displayed in the command output.

The `policy_manifest_lock.yaml` file will be automatically generated or updated with the resolved policies in the path. This file can be used later for offline mode if needed.

When running in **offline mode**, a `policy_manifest_lock.yaml` file is mandatory in the same path. Offline mode means the CLI will not communicate with PolicyHub:

- PolicyHub policies must already be cached locally. If they are not available, the CLI will prompt you to run the command in online mode at least once.
- Local policies continue to work as expected, as they are not dependent on PolicyHub.

### Non-Self-Explanatory Flags

- `--name` (`-n`): Sets the gateway name. If not provided, the current directory name is used by default.
- `--path`: Specifies the directory containing `policy_manifest.yaml`. By default, this is the current working directory.
- `push`: Pushes the built images to Docker.
- `no-cache`: Disables Docker build cache usage.
- `--platform`: Specifies the target platform. By default, the host platform is used.
