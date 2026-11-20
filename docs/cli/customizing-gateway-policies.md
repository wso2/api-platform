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
  [--netrc <file> | --git-credential <host>=<username>:<ENV_VAR> ...] \
  [--go-env <KEY>[=<VALUE>] ...]
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
- `--netrc`: A netrc file with credentials for private Go module hosts and proxies. See [Policies from Private Go Module Repositories](#policies-from-private-go-module-repositories).
- `--git-credential`: A username and token for one private Go module host, as `<host>=<username>:<ENV_VAR>`. Repeat it for more hosts. Cannot be combined with `--netrc`.
- `--go-env`: A Go module setting for the gateway-builder container, as `KEY=VALUE`, or `KEY` to copy the value from your environment. Repeatable.

## Policies from Private Go Module Repositories

`gomodule` policies are downloaded by the go command inside the gateway-builder container. A policy in a private repository needs two things there: Go settings that mark the module as private, and credentials for the host that serves it.

### 1. Mark the modules as private

Set `GOPRIVATE` to a pattern matching your module paths. Without it, Go checks private modules against the public checksum database, and the build fails even when the credentials are correct.

```shell
export GOPRIVATE='github.com/acme/*'
```

`GOPRIVATE`, `GONOPROXY` and `GONOSUMDB` are copied from your environment automatically. Other Go settings are passed only when named with `--go-env`, because a value that is right on your machine can be wrong inside the container (a `GOPROXY` on `localhost`, for example):

| Variable | Copied automatically | Passed with `--go-env` |
|---|---|---|
| `GOPRIVATE`, `GONOPROXY`, `GONOSUMDB` | Yes | Yes |
| `GOPROXY`, `GOSUMDB`, `GOVCS`, `GOINSECURE`, `GOAUTH` | No | Yes |

Any other variable is rejected. `--go-env` takes precedence over your environment, and `--go-env GOPRIVATE=` (empty) keeps an exported value out of the build. The command prints every setting it passes and where it came from; `GOAUTH` is shown as `(set)` because it can contain a token.

### 2. Provide credentials

Use one of the following.

**A username and token** (`--git-credential`). The token is read from the environment variable you name, so it never appears on the command line or in the build log:

```shell
export POLICY_TOKEN=<token with read access to the policy repositories>
ap gateway image build --git-credential github.com=<your-github-username>:POLICY_TOKEN
```

Repeat the flag for each host. Use the username your Git provider expects for the token:

- **GitHub personal access token:** your GitHub username. A fine-grained token limited to the policy repositories, with the **Contents: Read-only** permission, is enough.
- **GitHub App installation token** (`ghs_...`): `x-access-token`.
- **Bitbucket access token:** `x-token-auth`.

The CLI writes the credentials to a temporary, owner-only netrc file next to its workspace, mounts it read-only into the builder container, and deletes it when the builder finishes.

**A netrc file** (`--netrc`). Use this when the file already exists, or to cover a private module proxy:

```text
machine github.com
login <your-github-username>
password <token>

machine goproxy.acme.com
login ci-bot
password <token>
```

```shell
chmod 600 ~/.netrc
ap gateway image build --netrc ~/.netrc --go-env GOPROXY=https://goproxy.acme.com,direct
```

The file is mounted read-only into the builder container. The command refuses a netrc file that other users can read. With Docker Desktop or Colima, keep the file under your home directory: other host paths may not be shared with Docker's VM, and the mount then fails with "bind source path does not exist".

Git and the go command both read the netrc file (Go through its default `GOAUTH=netrc`), so it covers direct repository access, module proxies, and the `?go-get=1` lookups that self-hosted Git servers need. If you set `GOAUTH` with `--go-env`, include `netrc` in its list, or Go stops reading the file.

Credentials are passed to the builder container only. They are never written into the build workspace or the gateway images, and they are never read from `build.yaml`. Because `build.yaml` can choose the builder image, the command warns when credentials are about to be passed to an image other than the default one for the gateway version.

### Example: GitHub Actions

```yaml
- name: Build gateway image
  env:
    GOPRIVATE: github.com/acme/*
    POLICY_TOKEN: ${{ secrets.POLICY_REPO_TOKEN }}
  run: ap gateway image build --git-credential github.com=acme-ci:POLICY_TOKEN
```

`POLICY_REPO_TOKEN` is a fine-grained token owned by the `acme-ci` account. The workflow's own `GITHUB_TOKEN` cannot be used: it can read only the repository running the workflow, not a separate policy repository.

### Troubleshooting

When a private module cannot be downloaded, the builder log shows the reason reported by Go, with any credentials in URLs redacted, followed by a hint:

- *The module host asked for credentials or rejected the ones given*: pass `--git-credential` or `--netrc`. If you did, check that the token is valid, can read the repository, and is set for the right host. Git reports a rejected token the same way as a missing one.
- *The module was checked against the public checksum database*: set `GOPRIVATE` or `GONOSUMDB` to match the module path.
- *Could not reach the repository with any protocol*: check the credentials, and that the host is reachable from the builder container.

The builder also logs the Go settings it received and whether a netrc file is present, in its "Go module settings" line.
