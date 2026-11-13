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
  [--go-env <KEY>[=<VALUE>] ...] \
  [--fetch-timeout <duration>]
```

## Sample Command

```shell
ap gateway image build
```

## Additional Notes for Users

Use `ap gateway image build --help` to view detailed usage information for this command.

A container engine is a prerequisite for executing this command: Docker by default, or Podman or Buildah. The command checks that the selected engine is available at the beginning and fails immediately if it is not.

To build with Podman or Buildah instead of Docker, pass `--container-engine`, or set the `AP_CONTAINER_ENGINE` environment variable, for example in a CI job. The flag takes precedence over the environment variable.

```shell
ap gateway image build --container-engine podman
```

```shell
export AP_CONTAINER_ENGINE=buildah
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
- `--container-engine`: Selects the container engine: `docker` (default), `podman` or `buildah`. Can also be set with the `AP_CONTAINER_ENGINE` environment variable.
- `--netrc`: A netrc file with credentials for private Go module hosts and proxies. See [Policies from Private Go Module Repositories](#policies-from-private-go-module-repositories).
- `--git-credential`: A username and token for one private Go module host, as `<host>=<username>:<ENV_VAR>`. Repeat it for more hosts. Cannot be combined with `--netrc`.
- `--go-env`: A Go module setting for the gateway-builder container, as `KEY=VALUE`, or `KEY` to copy the value from your environment. Repeatable.
- `--fetch-timeout`: How long to wait for each `gomodule` policy to download, for example `10m`. The default is 5 minutes. A private module is fetched with git straight from its repository, including the whole repository at that version, so raise this for a large repository or a slow network. Needs a gateway-builder image that supports it; older builder images stop with `flag provided but not defined`.

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

Any other variable is rejected, as is a value that carries credentials in a URL (`user:token@`, a query string or a fragment); put credentials in the netrc file instead. `--go-env` takes precedence over your environment, and `--go-env GOPRIVATE=` (empty) keeps an exported value out of the build. The command prints every setting it passes and where it came from; `GOAUTH` is shown as `(set)` because it can contain a token.

Because `GOPRIVATE`, `GONOPROXY` and `GONOSUMDB` are copied automatically, they affect every build, including builds that pass no credentials:

- **Broad patterns reach public policies too.** With `GOPRIVATE=github.com/*`, the public policies from `github.com/wso2/gateway-controllers` are also fetched straight from GitHub, without the module proxy and the public checksum database. Keep the patterns as narrow as your private module paths.
- **A malformed value stops the build before the builder container starts**, for example a URL where a module path pattern belongs. Fix the variable, or keep it out of the build with `--go-env GOPRIVATE=`.
- **Values saved with `go env -w` are not copied.** They live in Go's own configuration file, not in your shell environment. Export them, or pass them with `--go-env`.

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

The CLI writes the credentials to a temporary, owner-only netrc file next to its workspace, mounts it read-only into the builder container, and deletes it when the builder finishes, also when the build is interrupted (Ctrl-C, a cancelled CI job, a closed terminal). If `ap` is stopped before it can delete the file, for example with `kill -9`, the next build removes it.

**A netrc file** (`--netrc`). Use this to cover a private module proxy, or several hosts in one file. Create a file just for the gateway build that lists only the hosts serving your policies, rather than passing your personal `~/.netrc` (see [What can read the credentials](#what-can-read-the-credentials)):

```text
machine github.com
login <your-github-username>
password <token>

machine goproxy.acme.com
login ci-bot
password <token>
```

```shell
chmod 600 ~/.netrc-gateway-policies
ap gateway image build --netrc ~/.netrc-gateway-policies \
  --go-env GOPROXY=https://goproxy.acme.com,direct \
  --go-env GONOPROXY=none
```

`GONOPROXY` defaults to `GOPRIVATE`, so without `GONOPROXY=none` the modules marked private in step 1 skip the proxy and are fetched straight from their repository. Keep `GOPRIVATE` set: it still keeps those modules away from the public checksum database. An empty `--go-env GONOPROXY=` does not help, because Go then falls back to the same default.

The file is mounted read-only into the builder container. The command refuses a netrc file that other users can read. With Docker Desktop, Colima or podman machine, keep the file under your home directory: other host paths may not be shared with the engine's VM, and the mount then fails with "bind source path does not exist". With Podman and Buildah on Linux, the file gets a shared SELinux label (`z`), so it stays readable to other containers, while the build workspace gets a private one (`Z`).

Git and the go command both read the netrc file (Go through its default `GOAUTH=netrc`), so it covers direct repository access, module proxies, and the `?go-get=1` lookups that self-hosted Git servers need. If you set `GOAUTH` with `--go-env`, include `netrc` in its list, or Go stops reading the file.

Credentials are passed to the builder container only. They are never written into the build workspace or the gateway images, and they are never read from `build.yaml`. Because `build.yaml` can choose the builder image, the command warns when credentials are about to be passed to an image other than the default one for the gateway version. It also warns when `GOINSECURE` is set together with credentials, because git can then send them to those hosts over plain HTTP, and when `GOSUMDB=off` is passed, because that turns off checksum verification for every module, including the public policies, and makes Go toolchain downloads fail, so a policy that needs a newer Go version than the builder's cannot be built.

### What can read the credentials

Everything that runs in the builder container can read the credentials while it runs: the go command and git, but also the build backend of each Python policy and the build dependencies pip downloads for it. A compromised Python policy or dependency could therefore read them. Limit what a leaked credential can do:

- **Use a token that can only read the policy repositories.** For GitHub, a fine-grained token limited to those repositories, with **Contents: Read-only**.
- **Give the builder only the hosts it needs**, with `--git-credential` or a netrc file made for the gateway build. A personal `~/.netrc` usually also holds credentials for unrelated hosts, and all of them would be readable inside the builder.

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
- *The downloaded module does not match the checksum recorded for this version*: the module changed after it was published, or the download was tampered with. Do not turn checksum verification off to get past it; check the module's source, and publish a legitimate change as a new version.
- *The builder could not reach the checksum database*: let the builder container reach `sum.golang.org`, or the database set in `GOSUMDB`.
- *Downloading the module did not finish within 5m0s and was stopped*: the repository is large or the network is slow. Raise the limit with `--fetch-timeout`, for example `--fetch-timeout 10m`, or check that the builder container can reach the repository or module proxy.
- *Could not reach the repository with any protocol*: check the credentials, and that the host is reachable from the builder container.

The builder also logs the Go settings it received and whether a netrc file is present, in its "Go module settings" line.
