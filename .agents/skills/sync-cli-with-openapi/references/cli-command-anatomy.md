# CLI command anatomy & templates

Reference for the `sync-cli-with-openapi` skill. All paths are relative to the repo root; CLI Go source lives under `cli/src/` (module `github.com/wso2/api-platform/cli`, so imports are e.g. `.../cli/internal/gateway`, `.../cli/utils`).

The two command families differ enough that you must not blend them. Pick the family from the spec (see the table in SKILL.md), then copy the closest existing sibling command in the same subcommand group. These templates are the fallback when no close sibling exists.

---

## 1. Directory & registration layout (both families)

```
cli/src/cmd/<family>/                 # gateway | devportal | platform | aiworkspace | project
  root.go                             # <Family>Cmd cobra group; init() AddCommand(...) each child
  <verb>.go                           # a leaf command directly under the family
  <resource>/                         # a subcommand GROUP for a resource
    root.go                           # <Resource>Cmd group; init() AddCommand(create/list/get/update/delete...)
    create.go list.go get.go update.go delete.go
    commands_test.go                  # table tests for the group
```

Registration chain, bottom-up — a new command is invisible until every link exists:

1. leaf `<verb>.go` defines `var <verb>Cmd = &cobra.Command{...}`.
2. group `root.go` `init()` calls `<Resource>Cmd.AddCommand(<verb>Cmd)`.
3. family `root.go` `init()` calls `<Family>Cmd.AddCommand(<resource>.<Resource>Cmd)`.
4. `<Family>Cmd` is registered on the CLI root in `cli/src/cmd/root.go`.

A new resource group also needs a `root.go` (`<Resource>Cmd`), added at link 3.

### Shared conventions

- **License header**: every `.go` file opens with the Apache-2.0 WSO2 header block (copyright 2026, WSO2 LLC.). Copy it verbatim from any sibling file.
- **Command consts**: each leaf declares `const <Verb>CmdLiteral = "<verb>"` and `<Verb>CmdExample = ` + "`" + `# ...\nap <family> <resource> <verb> ...` + "`" + `.
- **Flag name constants**: never hardcode a flag string. Use a `Flag*` const from `cli/src/utils/flags.go`; add one there if missing. Register flags with `utils.AddStringFlag(cmd, utils.FlagX, &var, "<default>", "<usage>")` / `utils.AddBoolFlag(...)`. Enforce required flags with `cmd.MarkFlagRequired(utils.FlagX)`.
- **Run wrapper**: `Run:` calls a `run<Verb>Command(...)` that returns `error`; on error print `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` then `os.Exit(1)`.
- **Validate before calling**: check required/mutually-exclusive flags and return a clear `fmt.Errorf` first.

---

## 2. Gateway family (`internal/gateway`)

Consumes `gateway/gateway-controller/api/management-openapi.yaml`. Targets the selected/active gateway.

**Key helpers**
- `gateway.AddSelectionFlags(cmd)` — adds the gateway-selection flags. Call in every `init()`.
- `gateway.NewClientFromCommand(cmd)` — builds the client for the selected/active gateway.
- Client verbs: `client.Get(path)`, `client.Post(path, body)`, `client.Put(path, body)`, `client.Delete(path)`, plus `PostYAML`/`PutYAML` for raw YAML bodies.
- `gateway.ParseResourceCR(filePath, expectedKind)` — parses a `--file` custom-resource (YAML/JSON) into `{apiVersion, kind, metadata, spec}`; use `cr.Spec` as the payload.
- `gateway.PrintJSONResponse(resp)` — pretty-prints the response body (pass-through; no typed struct needed).
- **Success check**: `resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices` → error.
- **Paths**: constants in `cli/src/utils/constants.go`, named `Gateway<Resource>Path` (collection) and `Gateway<Resource>ByIDPath` (has one `%s` per path variable). Add a constant for a new endpoint; build with `fmt.Sprintf(utils.Gateway...ByIDPath, id)`.

### Template — gateway create-from-CR (`POST` collection)

```go
// <license header>

package <resource>

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/utils"
)

const kind<Resource> = "<Kind>" // CR kind from the spec

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create a <resource> from a CR file
ap gateway <resource> create --file <resource>.yaml`
)

var createFilePath string

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Create a <resource> on the gateway",
	Long:    "Creates a new <resource> from a <Kind> custom resource file (YAML or JSON).",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	gateway.AddSelectionFlags(createCmd)
	utils.AddStringFlag(createCmd, utils.FlagFile, &createFilePath, "", "Path to the <Kind> CR file (YAML or JSON)")
	createCmd.MarkFlagRequired(utils.FlagFile)
}

func runCreateCommand(cmd *cobra.Command) error {
	if strings.TrimSpace(createFilePath) == "" {
		return fmt.Errorf("--%s is required", utils.FlagFile)
	}
	cr, err := gateway.ParseResourceCR(createFilePath, kind<Resource>)
	if err != nil {
		return err
	}
	// Validate spec-required fields here, e.g.:
	// if v, ok := cr.Spec["<requiredField>"].(string); !ok || strings.TrimSpace(v) == "" {
	// 	return fmt.Errorf("invalid %s: spec.<requiredField> is required", kind<Resource>)
	// }
	data, err := json.Marshal(cr.Spec)
	if err != nil {
		return fmt.Errorf("failed to build <resource> payload: %w", err)
	}
	client, err := gateway.NewClientFromCommand(cmd)
	if err != nil {
		return err
	}
	resp, err := client.Post(utils.Gateway<Resource>sPath, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create <resource>: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("failed to create <resource>: received status code %d", resp.StatusCode)
	}
	fmt.Printf("<Resource> %q created successfully.\n", cr.Metadata.Name)
	return gateway.PrintJSONResponse(resp)
}
```

For **get/list/delete**, copy `cli/src/cmd/gateway/restapi/get.go`, `list.go`, `delete.go` — they show ID-vs-name-lookup, `--format json|yaml` output, and `client.Get`/`client.Delete` against a `...ByIDPath`.

---

## 3. DevPortal family (`internal/devportal`)

Consumes `portals/developer-portal/docs/devportal-openapi-spec-v1.yaml`. Targets a resolved devportal within a platform.

**Key helpers**
- `config.LoadConfig()` → `internaldevportal.ResolveDevPortal(cfg, name, platform)` → `(*config.DevPortal, resolvedPlatform, error)`.
- `internaldevportal.NewClientWithOptions(devPortal, insecure)` — client honoring `--insecure`.
- `internaldevportal.OrgScopedPath(orgID, "<resource>")` → `/o/{orgId}/devportal/v1/<resource>`. The `<resource>` is appended as-is, so escape interpolated segments (`"apis/"+url.PathEscape(apiID)`) and you may append a query string. Org-management endpoints use the raw `/organizations...` path instead.
- Client verbs: `client.Get(path)`, `client.PostJSON(path, []byte)`, `client.PutJSON(path, []byte)`, `client.Delete(path)`, `client.PostMultipartFile(path, field, filePath)`, `client.PutMultipartFile(...)`.
- Errors: transport errors → `internaldevportal.WrapRequestError("<op>", err, insecure)`; non-2xx → `utils.FormatHTTPError("<op>", resp, "DevPortal")`. **Check the exact status the spec documents** (`http.StatusOK` for GET, `http.StatusCreated` for a `201` POST, etc.).
- Output: `internaldevportal.PrintJSONResponse(resp)`.
- Standard flags: `--org` (`FlagOrgID`, usually required), `--display-name` (`FlagName`), `--platform` (`FlagPlatform`), `--insecure` (`FlagInsecure`). Add resource flags as needed.

### Template — devportal create (`POST`, JSON body from flags)

```go
// <license header>

package <resource>

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	CreateCmdLiteral = "create"
	CreateCmdExample = `# Create a <resource>
ap devportal <resource> create --org org_1 --<field> <value>`
)

var (
	createOrgID    string
	create<Field>  string
	createName     string
	createPlatform string
	createInsecure bool
)

var createCmd = &cobra.Command{
	Use:     CreateCmdLiteral,
	Short:   "Create a DevPortal <resource>",
	Long:    "Creates a <resource> in the selected DevPortal.",
	Example: CreateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(createCmd, utils.FlagOrgID, &createOrgID, "", "Organization ID")
	utils.AddStringFlag(createCmd, utils.Flag<Field>, &create<Field>, "", "<field usage>")
	utils.AddStringFlag(createCmd, utils.FlagName, &createName, "", "DevPortal display name")
	utils.AddStringFlag(createCmd, utils.FlagPlatform, &createPlatform, "", "Platform name")
	utils.AddBoolFlag(createCmd, utils.FlagInsecure, &createInsecure, false, "Skip TLS certificate verification")
	_ = createCmd.MarkFlagRequired(utils.FlagOrgID)
}

func runCreateCommand() error {
	orgID := strings.TrimSpace(createOrgID)
	if orgID == "" {
		return fmt.Errorf("organization ID is required")
	}
	payload, err := json.Marshal(map[string]string{"<field>": strings.TrimSpace(create<Field>)})
	if err != nil {
		return fmt.Errorf("failed to build <resource> payload: %w", err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, createName, createPlatform)
	if err != nil {
		return err
	}
	client := internaldevportal.NewClientWithOptions(devPortal, createInsecure)
	resp, err := client.PostJSON(internaldevportal.OrgScopedPath(orgID, "<resource>"), payload)
	if err != nil {
		return internaldevportal.WrapRequestError("create <resource>", err, createInsecure)
	}
	if resp.StatusCode != http.StatusCreated {
		return utils.FormatHTTPError("create <resource>", resp, "DevPortal")
	}
	fmt.Printf("<Resource> created using devportal %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}
```

For **list/get/delete/update**, copy `cli/src/cmd/devportal/subplan/list.go`, `subplan/get.go`, and `subscription/create.go` — they show `OrgScopedPath`, the resolve flow, and status handling. For `--file` JSON bodies use `internaldevportal.ReadJSONFile(path)`. For custom action sub-paths (e.g. `.../change-plan`, `.../regenerate-token`) build `OrgScopedPath(orgID, "subscriptions/"+url.PathEscape(subID)+"/change-plan")`.

---

## 3b. AI-workspace family (`internal/aiworkspace`)

Consumes the **LLM/MCP subset** of `platform-api/resources/openapi.yaml` — `/llm-providers`, `/llm-proxies`, `/mcp-proxies` (and `llm-provider-templates`) — reached under the `/api/v0.9/` prefix. Command literal is `ap ai-workspace`; sources live under `cli/src/cmd/aiworkspace/` (the Go package identifier is `aiws`). Resource groups: `llmprovider` (`llm-provider`), `llmproxy` (`app-llm-proxy`), `mcpproxy` (`mcp-proxy`) — each with `get`, `list`, `delete`. Create-or-update is **not** per-group: two **family-level** commands operate on an API project — **`build`** (validate the project's ai-workspace artifact) and **`apply`** (generate the payload from the project's `metadata.yaml`/`runtime.yaml`/`definition.yaml` and create-or-update it on the server, choosing the endpoint by artifact kind). The organization comes from the auth token, so there is **no `--org` flag**.

**Key helpers**
- `config.LoadConfig()` → `aiworkspace.ResolveAIWorkspace(cfg, name, platform)` → `(*config.AIWorkspace, resolvedPlatform, error)`.
- `aiworkspace.NewClientWithOptions(aiWorkspace, insecure)`.
- Client verbs: `client.Get(path)`, `client.PostJSON(path, []byte)`, `client.PutJSON(path, []byte)`, `client.Delete(path)`, and `client.Exists(path)` — the create-or-update probe (2xx → exists, 404 → absent).
- **Paths**: constants `AIWorkspace<Resource>Path` in `cli/src/utils/constants.go` (carry the `/api/v0.9/` prefix), wrapped by helpers in `cli/src/internal/aiworkspace/helpers.go`:
  - collection: `ProviderPath()` / `ProxyPath()` / `MCPProxyPath()` — the organization is resolved from the token, so it is **not** in the path or query.
  - single resource: `ProviderByIDPath(id)` / `ProxyByIDPath(id)` / `MCPProxyByIDPath(id)` — the id is `metadata.name`.
  - list: `ProviderListPath(aiworkspace.ListQuery{Limit, Offset})`; proxies/MCP are project-scoped — `ProxyListPath(projectID, q)` / `MCPProxyListPath(projectID, q)` add `?projectId=`, the one scope carried as a query parameter (never a path segment).
- Auth is handled by the client from workspace config or env vars (`WSO2AP_AIWORKSPACE_USERNAME`/`_PASSWORD`/`_TOKEN`/`_API_KEY`, header `x-wso2-api-key`).
- Flags: `build`/`apply` take `--file` (project **directory**, not a payload file), `--project-id` (required for the `LlmProxy` and `Mcp` kinds), `--env-file` (resolves `ENV_CLI_*` placeholders), `--display-name`, `--platform`, `--output`, `--insecure`. Read verbs (`get`/`list`/`delete`) take `--id` / `--project-id` / `--limit` / `--offset` as applicable, plus `--display-name`, `--platform`, `--insecure`.

### Template — ai-workspace apply (create-or-update from an API project)

`apply` validates the project, builds the payload in memory (folding in `definition.yaml`), selects the endpoint from the artifact **kind** (`LlmProvider`/`LlmProxy`/`Mcp`), then probes existence by `metadata.name` and PUTs an update or POSTs a create. Copy the closest sibling — `cli/src/cmd/aiworkspace/apply.go` (create-or-update) and `cli/src/cmd/aiworkspace/build.go` (validate only); for read verbs copy `cli/src/cmd/aiworkspace/llmprovider/get.go`, `list.go`, `delete.go`. The create-or-update shape:

```go
func applyAIWorkspaceArtifact(client *aiworkspace.Client, artifact *aiWorkspaceArtifact, body []byte) (*http.Response, string, error) {
	updatePath := aiWorkspaceUpdatePath(artifact.BaseKind, artifact.ResourceName) // <resource>ByIDPath(metadata.name)
	exists, err := client.Exists(updatePath)
	if err != nil {
		return nil, "", err
	}
	if exists {
		resp, err := client.PutJSON(updatePath, body) // update existing
		return resp, "updated", err
	}
	resp, err := client.PostJSON(aiWorkspaceCreatePath(artifact.BaseKind), body) // create new
	return resp, "applied", err
}
```

Register new commands/groups the same way: group `root.go` `AddCommand`, then `cli/src/cmd/aiworkspace/root.go`.

## 4. Resource-group `root.go` template

For a brand-new resource group:

```go
// <license header>

package <resource>

import "github.com/spf13/cobra"

const (
	<Resource>CmdLiteral = "<resource>"
	<Resource>CmdExample = `# List all <resource>s
ap <family> <resource> list`
)

// <Resource>Cmd is the <resource> command group.
var <Resource>Cmd = &cobra.Command{
	Use:     <Resource>CmdLiteral,
	Short:   "Manage <resource>s",
	Long:    "This command allows you to manage <resource>s.",
	Example: <Resource>CmdExample,
	Run:     func(cmd *cobra.Command, args []string) { cmd.Help() },
}

func init() {
	<Resource>Cmd.AddCommand(createCmd)
	<Resource>Cmd.AddCommand(listCmd)
	<Resource>Cmd.AddCommand(getCmd)
	<Resource>Cmd.AddCommand(updateCmd)
	<Resource>Cmd.AddCommand(deleteCmd)
}
```

Then in `cli/src/cmd/<family>/root.go`: import the package and add `<Family>Cmd.AddCommand(<resource>.<Resource>Cmd)` inside `init()`.

---

## 5. Tests, docs, build

- **Tests** — copy the group's `commands_test.go` (e.g. `cli/src/cmd/devportal/subplan/commands_test.go`, `gateway/subscriptionplan` tests). They validate flag wiring, required-flag errors, and payload building. Add a case per new flag/validation branch; update payload expectations on MODIFIED ops.
- **Docs** (hand-maintained) — `docs/cli/gateway/README.md`, `docs/cli/devportal/README.md`, and `docs/cli/reference.md`. Follow the existing "### N. <Action>" + `#### CLI Command` + shell block format. Update the short-flag table in `reference.md` only if a short flag changed.
- **Build/verify** — from `cli/`: `make build` (tests then builds) or `make test`. While iterating, from `cli/src/`: `go build ./...` and `go test ./...`.

## 6. Mapping cheatsheet

| Spec element | CLI counterpart |
|---|---|
| `path` + method | one client call (`Get`/`PostJSON`/`Post`/`Put`/`Delete`) in one `run<Verb>Command` |
| path variable `{id}` | `%s` in a gateway path constant, or `url.PathEscape` inside `OrgScopedPath` |
| query/path parameter | a cobra flag via `utils.Flag*` + `utils.AddStringFlag` |
| `requestBody` schema | the `map`/struct marshalled to JSON, or the CR `spec` (`ParseResourceCR`) |
| required field | a `MarkFlagRequired` / explicit `fmt.Errorf` validation |
| response schema | pass-through `PrintJSONResponse`, or a typed decode struct |
| `201`/`200`/`204` | the exact `resp.StatusCode` compared in the success check |
| OAuth2 scope / security | gateway selection flags, or devportal resolve + `--insecure` (auth handled by the client) |
| new tag / resource | a new subcommand group dir + `root.go`, registered in the family root |
