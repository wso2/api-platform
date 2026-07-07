---
name: sync-cli-with-openapi
description: Synchronize the `ap` CLI (cli/) with a REST API OpenAPI spec after the spec changes. Use when someone edits platform-api/src/resources/openapi.yaml (consumed by the `ap ai-workspace` family via its LLM/MCP endpoints), portals/developer-portal/docs/devportal-openapi-spec-v1.yaml (the `ap devportal` family), or gateway/gateway-controller/api/management-openapi.yaml (the `ap gateway` family) and the matching CLI commands need updating — a changed request/response/parameter on an endpoint an existing command calls, OR a newly added endpoint that needs a brand-new command. Trigger on "update the CLI for this spec change", "the spec changed, sync the CLI commands", "add a CLI command for this new endpoint", or when naming this skill directly. Keeps commands, path constants/helpers, command registration, docs, and tests in lockstep with the spec.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# Sync the `ap` CLI with an OpenAPI spec

Keep the Go CLI under `cli/` in sync with a REST API contract after its OpenAPI spec changes. This covers two jobs:

1. **Update existing commands** when an endpoint they already call changes shape (new/renamed parameter, changed request/response body, new required field, removed operation).
2. **Scaffold new commands** for endpoints newly added to a spec that have no CLI coverage yet.

The linking pin is always the **HTTP path + method**: a spec operation maps to exactly one CLI call site. This skill works by discovering that call site (or its absence) and reconciling it — it does **not** assume a fixed spec→command table, because that drifts.

## Invoking this skill

There are two ways to drive it. Both run the same workflow — they differ only in *scope*.

### Mode A — sync everything that changed in a spec (diff-driven)

Use when a spec file changed (e.g. a `git pull` or merge brought in new/edited endpoints) and you want the CLI reconciled to match. You do **not** list the endpoints — the skill reads the diff and classifies every changed operation itself (Step 1).

Trigger prompts:
- *"The devportal spec changed in the last pull — sync the CLI commands."*
- *"/sync-cli-with-openapi platform-api/src/resources/openapi.yaml"*

**After a pull/merge the change is already committed**, so a plain `git diff` shows nothing — the skill must diff against the pre-change state (Step 1 lists the exact commands). If the user knows the base, they can say so: *"…diff it against `HEAD@{1}`"* or *"…against `main`"*.

### Mode B — convert a named set of endpoints into commands (scope-driven)

Use when you want CLI coverage for *specific* endpoints — regardless of what the diff says. You name the operations; the skill jumps straight to scaffolding them (Step 2 → Step 3, ADDED path) and ignores the rest.

Trigger prompts:
- *"Add `ap` commands for `POST /devportal/v1/webhook-subscribers` and `GET /devportal/v1/webhook-subscribers/{subscriberId}`."*
- *"From platform-api/openapi.yaml, only wire the `llm-provider-templates` endpoints (list, get, copy) into the `ai-workspace` CLI."*

The skill still reads each named operation's real schema from the spec so flags/payloads/validation are correct — it just skips the diff-classification breadth.

**In both modes**, if the user hasn't named the spec, infer it from the paths/family they mention and confirm. If a named endpoint has no consumer *and* no clear family (e.g. a non-LLM platform-api path), report it and ask before inventing a new command family.

## Spec → CLI-family map

Three specs feed the CLI. Each is consumed by a distinct command family with its own client and conventions:

| Spec file | CLI family | Root dir | Client package |
|---|---|---|---|
| `gateway/gateway-controller/api/management-openapi.yaml` | `ap gateway ...` | `cli/src/cmd/gateway/` | `internal/gateway` |
| `portals/developer-portal/docs/devportal-openapi-spec-v1.yaml` | `ap devportal ...` | `cli/src/cmd/devportal/` | `internal/devportal` |
| `platform-api/src/resources/openapi.yaml` | `ap ai-workspace ...` (partial — see note) | `cli/src/cmd/aiworkspace/` | `internal/aiworkspace` |

**Note on `platform-api/src/resources/openapi.yaml`:** the `ap ai-workspace` (AI workspace) family consumes this spec, but **only its LLM/MCP subset** — `/llm-providers`, `/llm-proxies`, `/mcp-proxies` (and the `llm-provider-templates` endpoints). The CLI reaches them under the `/api-proxy/api/v0.9/` prefix via path constants in `cli/src/utils/constants.go` (e.g. `AIWorkspaceLLMProvidersPath = "/api-proxy/api/v0.9/llm-providers"`), so the spec path is *unversioned* (`/llm-providers`) but the CLI call is *versioned* — match on the resource segment, not the whole path. The spec's many other resources (`/organizations`, `/projects`, `/rest-apis`, `/gateways`, `/applications`, `/subscription-plans`, `/subscriptions`, …) have **no `ap` consumer today** — the `ap gateway` commands look similar but actually track the separate *gateway management* spec. So for a platform-api change: if it touches an LLM/MCP endpoint, sync the `ai-workspace` family; otherwise discovery (Step 2) will find no consumer — say so and ask the user whether they want a new command family, rather than inventing edits.

Full anatomy of a command, the exact helpers per family, and copy-ready templates live in **`references/cli-command-anatomy.md`**. Read that file before writing or editing any command — do not work from memory.

## Workflow

### Step 1 — Identify what changed in the spec

**Mode B (named endpoints):** skip the diff — take the operations the user listed as the ADDED set and go to Step 2. Still open the spec file to read each operation's schema.

**Mode A (diff-driven):** get the diff of the spec. Pick the command by where the change lives:

```bash
git diff -- <spec-path>                 # uncommitted local edits (you're mid-editing the spec)
git diff HEAD@{1}..HEAD -- <spec-path>  # change arrived via a pull (compare to pre-pull HEAD)
git diff main...HEAD    -- <spec-path>  # everything new on this branch vs main (PR review)
git diff <old-sha>..<new-sha> -- <spec-path>
```

If none show a diff, the spec is unchanged relative to that base — confirm the base with the user (a pull needs `HEAD@{1}` or the merge commit, not a plain `git diff`).

If the user hasn't said which spec, infer from the path they name and confirm it's one of the three above. From the diff, build a list of **changed operations**, each classified as:

- **ADDED** — a new `path` or a new method under an existing path → candidate for a *new command*.
- **MODIFIED** — an existing operation whose parameters, `requestBody`, responses, required fields, or `operationId`/scopes changed → *update the existing command*.
- **REMOVED** — an operation deleted → *remove or deprecate the command*.

For each operation record: HTTP method, full path (e.g. `POST /devportal/v1/subscriptions/{subId}/change-plan`), the changed parameters/fields, and the OAuth2 scope if the spec declares one.

### Step 2 — Locate the CLI consumer (or confirm there is none)

For each operation, find the call site. The path segment is the search key. Match against how each family stores paths:

- **Gateway family** — paths are constants in `cli/src/utils/constants.go` (e.g. `GatewaySubscriptionPlansPath = "/subscription-plans"`, `%s` for IDs). Grep for the literal path, then for the constant name:
  ```bash
  grep -rn "subscription-plans" cli/src/utils/constants.go
  grep -rn "GatewaySubscriptionPlansPath" cli/src/cmd/gateway/
  ```
- **DevPortal family** — org-scoped paths are built with `internaldevportal.OrgScopedPath(orgID, "<resource>")`, which produces `/o/{orgId}/devportal/{APIVersion}/<resource>` (`APIVersion` = `v1`, in `internal/devportal/helpers.go`). Only org-management endpoints use the raw `/organizations...` path. Grep for the resource segment:
  ```bash
  grep -rn "OrgScopedPath\|\"/organizations" cli/src/cmd/devportal/
  ```
- **AI-workspace family** — paths are constants in `cli/src/utils/constants.go` (`AIWorkspaceLLMProvidersPath` etc., under `/api-proxy/api/v0.9/`), wrapped by helpers in `cli/src/internal/aiworkspace/helpers.go` (`ProviderPath(orgID)`, `ProviderResourcePath(orgID,id)`, `ProviderListPath(orgID,q)`, and the `Proxy*`/`MCPProxy*` equivalents). Org/project scoping is a **query parameter** (`?organizationId=`/`?projectId=`), not a path segment. Match on the resource segment, since the spec path is unversioned:
  ```bash
  grep -rn "llm-providers\|llm-proxies\|mcp-proxies" cli/src/utils/constants.go
  grep -rn "AIWorkspaceLLMProvidersPath\|ProviderPath\|ProviderResourcePath" cli/src/cmd/aiworkspace/ cli/src/internal/aiworkspace/
  ```

Outcomes:
- **Found** → MODIFIED/REMOVED work targets this file. Note its dir (the cobra subcommand group).
- **Not found** for an ADDED op → this is new-command work. Identify the right subcommand group (existing dir like `cli/src/cmd/gateway/subscription/`, or a new one).
- **Not found** for a MODIFIED op → the endpoint has no CLI coverage; treat as ADDED or report it, don't silently skip.

### Step 3 — Reconcile each operation

Open `references/cli-command-anatomy.md` and follow the template for the operation's family. Then, per classification:

**MODIFIED — update an existing command**
- New/renamed **parameter** → add/rename the cobra flag (use or add a `Flag*` constant in `cli/src/utils/flags.go`; wire with `utils.AddStringFlag`/`AddBoolFlag`), update the path/query building, and adjust validation (`MarkFlagRequired`, mutual-exclusion checks).
- Changed **request body** → update the struct/`map` marshalled into the payload and any `--file` CR parsing (`gateway.ParseResourceCR`).
- New **required** field → add validation that returns a clear `fmt.Errorf` before the request; add a flag or CR field for it.
- Changed **response** → update any local response struct and the print path (`PrintJSONResponse` needs no change for pass-through; typed decoders do).
- Changed **path/method** → update the path constant/helper and the client verb (`Get`/`PostJSON`/`Put`/`Delete`/`PostYAML`).

**ADDED — scaffold a new command**
1. Pick the subcommand group dir. Reuse an existing one if the resource already has a group; otherwise create a new dir + `root.go` (a `<Resource>Cmd` cobra group) and register it in the family root (`cli/src/cmd/<family>/root.go`).
2. Add the path constant (gateway) or use `OrgScopedPath` (devportal).
3. Create `<verb>.go` from the matching template in the reference: consts `<Verb>CmdLiteral`/`<Verb>CmdExample`, flag vars, the `cobra.Command`, `init()` wiring flags, and `run<Verb>Command` doing validate → client → request → status-check → print.
4. Register the new command in the group's `root.go` via `AddCommand`.
5. Map the operation's OAuth2 scope/auth to the family's auth model (gateway selection flags vs devportal resolve+`--insecure`) — the reference explains both.

**REMOVED — retire a command**
- Confirm with the user first (removing a command is a breaking CLI change). If confirmed, delete the `<verb>.go`, drop its `AddCommand` line, remove now-unused path constants, and delete its tests.

### Step 4 — Keep the surrounding artifacts in sync

A command change is not done until these match:

- **Flag constants** — new flags need a `Flag*` entry in `cli/src/utils/flags.go` (don't hardcode flag strings).
- **Path constants** — new/changed gateway endpoints need entries in `cli/src/utils/constants.go`.
- **Registration** — every new command/group is wired via `AddCommand` up to the family root.
- **Docs** (manually maintained, not generated) — update the relevant `docs/cli/<area>/README.md` and, if a top-level command/flag changed, `docs/cli/reference.md`. Match the existing entry format (CLI Command + example blocks).
- **Tests** — mirror the neighboring `*_test.go` / `commands_test.go` in the same dir. Add cases for new flags/validation; update expectations for changed payloads.

### Step 5 — Build, test, and report

From `cli/`:

```bash
make build        # builds (runs tests first)
make test         # tests only
# or, faster while iterating, from cli/src:  go build ./...  &&  go test ./...
```

Fix compile/test failures. Then report: which operations were ADDED/MODIFIED/REMOVED, the files touched (commands, constants, flags, registration, docs, tests), the build/test result, and anything that needs manual follow-up (e.g. a REMOVED op awaiting confirmation, or a platform-api change with no CLI consumer).

## Guardrails

- **Never invent an endpoint.** Every path/verb/parameter a command uses must exist in the spec diff or the current spec. If the spec is ambiguous, read the operation's schema in the spec file rather than guessing.
- **Match the family's conventions exactly** — the gateway, devportal, and ai-workspace families differ in client construction, path handling (constants vs `OrgScopedPath` vs query-param scoping), verb vocabulary (`create` vs `push`), auth flags, and error wrapping. Copy the closest existing sibling command; don't blend patterns across families.
- **Treat CLI removals/renames as breaking.** Confirm before removing or renaming a command, flag, or its short form.
- **Keep the license header** — every Go file starts with the Apache-2.0 WSO2 header block (copy it from any sibling file).
- Prefer editing a sibling-derived copy over authoring from scratch; the templates in the reference are a fallback, the real source of truth is the existing commands in the same group.
