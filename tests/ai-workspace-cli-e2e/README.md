# AI Workspace CLI end-to-end tests

A godog (BDD) suite that verifies the `ap` CLI's **AI Workspace** behaviour
persists against a real `platform-api` backend. It exercises the documented
end-to-end flow from [`docs/cli/end-to-end-workflow.md`](../../docs/cli/end-to-end-workflow.md)
and [`docs/cli/ai-workspace/README.md`](../../docs/cli/ai-workspace/README.md):

1. Boot `platform-api` (SQLite, standalone) as the AI Workspace backend.
2. Log in as `admin`/`admin` at `/api/portal/v0.9/auth/login` for a bearer token.
3. Create a server-side project (`POST /api/v0.9/projects`) — its handle is the
   `--project-id` for the project-scoped kinds — and register the gateways the
   artifacts associate with (`POST /api/v0.9/gateways`, handles `prod-eu-01` /
   `prod-eu-02`), since `associatedGateways` handles are validated server-side.
4. Register `platform-api` as an AI Workspace target in an isolated CLI config
   (`ap ai-workspace add --auth oauth`); the token is supplied per call via
   `WSO2AP_AIWORKSPACE_TOKEN`.
5. For each of the three artifact kinds — **LLM provider**, **LLM proxy**,
   **MCP proxy** — `ap project init`, apply the `create` content, read it back
   with both `ap ai-workspace <group> get --id` and `ap ai-workspace <group>
   list`, then apply the `edit` content (which adds `spec.associatedGateways`) to
   confirm the update path persists — verified by reading the associated gateway
   back from the server.

The artifact content comes from a real demo set (`create`/`edit` variants per
kind). The `edit` metadata adds `spec.associatedGateways`; the LLM proxy
references the LLM provider, so scenarios run provider → proxy → MCP.

It runs **once a day** in CI (see
[`.github/workflows/ai-workspace-cli-e2e.yml`](../../.github/workflows/ai-workspace-cli-e2e.yml)),
not on every PR, because it builds the `platform-api` image and the CLI from
source.

## Layout

```
tests/ai-workspace-cli-e2e/
├── docker-compose.yaml        # platform-api only (SQLite, standalone)
├── platform-api-config.toml   # file-based auth (admin/admin)
├── suite_test.go              # boot/login/create-project/register-workspace/teardown + CLI runner
├── steps_test.go              # per-kind init/build/apply/get step definitions
├── features/
│   └── ai-workspace-cli.feature
└── resources/                 # demo artifacts copied over the scaffold
    ├── llm-provider/
    │   ├── definition.yaml     # shared OpenAPI spec (create + edit)
    │   ├── create/{metadata,runtime}.yaml
    │   └── edit/{metadata,runtime}.yaml    # edit metadata adds spec.associatedGateways
    ├── llm-proxy/  (same shape; runtime references the claude-provider)
    └── mcp/        (same shape; definition.yaml is capabilities, not OpenAPI)
```

The `llm-proxy` artifact references the `claude-provider` created by the
`llm-provider` scenario, so scenarios run provider → proxy → MCP in order and
share the suite-scoped backend, token, project, and registered gateways.

## Running locally

Prerequisites: Docker running, Go 1.26.5.

```shell
# 1. Build the platform-api image (tag must match PLATFORM_API_IMAGE below).
make -C platform-api build IMAGE_NAME=platform-api VERSION=it-e2e

# 2. Build the ap CLI binary (lands at cli/src/build/ap).
make -C cli/src build-skip-tests

# 3. Run the suite.
cd tests/ai-workspace-cli-e2e
PLATFORM_API_IMAGE=platform-api:it-e2e go test -run TestFeatures -count=1 -v -timeout 20m ./...
```

### Knobs

| Variable             | Default                 | Purpose                                             |
| -------------------- | ----------------------- | --------------------------------------------------- |
| `PLATFORM_API_IMAGE` | `platform-api:it-e2e`   | platform-api image the compose stack runs.          |
| `AP_CLI_BINARY`      | `../../cli/src/build/ap`| Path to the `ap` binary under test.                 |
| `PA_HOST_PORT`       | `9243`                  | Host port platform-api is published on.             |
| `AW_TAGS`            | (all)                   | godog tag filter, e.g. `@mcp-proxy`.                |
| `AW_KEEP`            | (unset)                 | If set, the compose stack is left running for debug.|

The CLI runs with an isolated `HOME`, so it never touches your real
`~/.wso2ap/config.yaml`.
