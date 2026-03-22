# Debugging Gateway Integration Tests

The integration tests in `it/` use `testcontainers-go` to spin up the full stack via Docker Compose. Running them through VS Code requires a one-time setup so the test process can locate the Docker socket.

### Why this is needed

When VS Code is launched from Spotlight, Dock, or Finder it does **not** inherit your shell environment. Without `DOCKER_HOST` set, `testcontainers-go` cannot find the Docker socket and panics:

```
panic: rootless Docker not found
```

The auto-detection in `suite_test.go` (`checkColimaAndSetupEnv`) only handles Colima and only works when `docker` is on PATH in the VS Code process — which is not guaranteed.

### Step 1: Create `.vscode/settings.json`

Create (or update) `.vscode/settings.json` in the workspace root:

```json
{
  "go.testEnvVars": {
    "DOCKER_HOST": "<your-docker-socket>",
    "TESTCONTAINERS_RYUK_DISABLED": "true"
  },
  "go.testTimeout": "30m"
}
```

Find your socket path by running this in a terminal:

```bash
docker context inspect --format '{{.Endpoints.docker.Host}}'
```

Common values by runtime:

| Runtime | Socket path |
|---|---|
| Rancher Desktop | `unix:///Users/<you>/.rd/docker.sock` |
| Colima | `unix:///Users/<you>/.colima/default/docker.sock` |
| OrbStack | `unix:///Users/<you>/.orbstack/run/docker.sock` |
| Docker Desktop | `unix:///var/run/docker.sock` |

`TESTCONTAINERS_RYUK_DISABLED=true` prevents the Ryuk reaper container from running, which sidesteps permission issues on rootless runtimes (Rancher Desktop, Colima).

### Step 2: Tag Test Images

The integration tests consume `:test`-tagged images. Before running IT tests for the first time (or after a code change), tag the current build:

```bash
make ensure-test-tags
```

This tags `:<VERSION>` or `:latest` images as `:test` so Docker Compose can pull them.

### Step 3: Run or Debug Tests in VS Code

#### Run all IT tests

Open any file in `it/` and use the **Testing** panel, or click the run lens above `TestFeatures` in `suite_test.go`.

#### Run a single feature

Set `IT_FEATURE_PATHS` in `go.testEnvVars` (temporarily, or via a launch configuration):

```json
"go.testEnvVars": {
  "DOCKER_HOST": "unix:///Users/<you>/.rd/docker.sock",
  "TESTCONTAINERS_RYUK_DISABLED": "true",
  "IT_FEATURE_PATHS": "features/health.feature"
}
```

### Cleaning Up

If a test run is interrupted the Docker Compose stack may be left running:

```bash
make clean
```
