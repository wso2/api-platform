---
name: rest-api-oauth-scopes
description: Design REST API resources and the OAuth2 scopes that guard them in the WSO2 API Platform, where scopes are permissions of the tree in platform-api/resources/permission-tree.yaml and every operation's security block is generated from it. Use when adding or editing a resource, path or operation in platform-api/resources/openapi.yaml or a plugin spec; naming a permission; deciding which permissions guard an endpoint; adding a sub-resource or a new action; choosing CRUD vs a custom verb; renaming or moving a node; mapping roles to scopes; or deciding what the console BFFs request from the IDP.
allowed-tools: Bash, Read, Edit, Grep, Glob
---

# REST API Resources & OAuth2 Scopes

Conventions for modelling REST resources and the OAuth2 scopes that guard them. A scope is a **permission of the tree** in `platform-api/resources/permission-tree.yaml`: a node path and an action. The tree also **binds every operation** to the permissions it accepts directly, and `platform-api/scripts/permission_tree.py` expands those bindings by the inheritance rule — `read` and `manage` flow down from parents, nothing else does — into each operation's `security` block. The enforcer then matches a token's scopes against that block by exact string equality, OR-evaluated. Nothing infers privilege from scope strings at request time; everything that admits a request is written out.

## Where things live

| File | Role |
| --- | --- |
| `platform-api/resources/permission-tree.yaml` | Source of truth: nodes, their actions, the pre-tree resource each absorbed (`legacy`), plugin markers, and `x-operation-permissions` (one binding per operation). |
| `platform-api/scripts/permission_tree.py` | Generator. `uv run --with pyyaml python3 scripts/permission_tree.py` rewrites both specs' security blocks, the main spec's scope catalog, the tree's `aggregate_members`, and the AI Workspace's `permissionTree.generated.ts`. `--check` exits 1 when anything is stale. |
| `platform-api/resources/openapi.yaml`, `platform-api/plugins/eventgateway/openapi.yaml` | Generated security blocks. Never hand-edit a `security:` list. |
| `platform-api/resources/legacy-scope-mapping.yaml` | Frozen pre-tree names and the permission each grants; translated per request by the enforcer and by both consoles. |
| `platform-api/resources/role-to-scope-mapping.yaml` | Shipped roles, in tree permissions only. |
| `platform-api/internal/server/permission_tree_test.go`, `scope_alternatives_test.go` | The gates: spec == binding ∪ inheritance, and every alternative admits through the real enforcer. |
| `portals/api-control-plane` (`npm run api:codegen`), `portals/ai-workspace/src/auth/permissionTree.generated.ts` | Console copies of the accepted lists and the legacy map; regenerate after any tree change. |

## Scope format

```
ap:<node>[:<sub-node>...]:<action>
```

- **`ap:`** is the fixed prefix of the Platform API. Every permission shares it; the API Portal's `dp:` namespace is a different component's.
- **`<node>[:<sub-node>...]`** is the node's path in the tree, snake_case handles joined with `:`. Top-level nodes today: `api` (with the API kinds `rest`, `graphql`, `llm`, `mcp`, `agent` beneath it, and `websub`, `webbroker` on an event-gateway build), `application`, `content`, `credential`, `gateway`, `gateway_policy`, `llm_provider`, `meter`, `organization`, `portal`, `project`, `publication`, `subscription`, `subscription_plan`.
- **`<action>`** is a CRUD verb, the `manage` superset, or a custom verb the node defines.

Examples: `ap:project:create`, `ap:gateway:token:create`, `ap:api:rest:deployment:undeploy`, `ap:api:manage`, `ap:manage`.

## The core rules

**Rule 1 — Lock the shape.**
Colon is the only delimiter; snake_case for every segment (`rest_api` never `rest-api`/`restApi`). Nest a node under its parent **only on real ownership**: `gateway:token` (a token belongs to a gateway), `application:api_key`, `api:rest:deployment`. Every API kind lives under `ap:api`, which is what lets `ap:api:read` and `ap:api:manage` cover the whole catalogue. A node that exists only on a plugin build carries `plugin: <name>` and its permissions are declared by the plugin's spec.

**Rule 2 — CRUD verbs are a closed set.**

| Verb | HTTP |
| --- | --- |
| `read` | GET list **and** GET by id, and a side-effect-free POST that only fetches (`fetch-server-info`, `fetch-agent-card`) |
| `create` | POST that makes the resource, including create variants (`import-openapi`, `validate-openapi`) and `deploy` (= `deployment:create`) |
| `update` | PUT / PATCH |
| `delete` | DELETE |

Never synonyms (`list`, `get`, `add`, `remove`, `edit`).

**Rule 3 — Custom verbs only where the node defines them.**
A custom verb is a permission like any other: it must appear in the node's `actions`, lands in the catalog, and roles and BFFs have to know it. The tree defines `restore`, `undeploy` (deployments) and `regenerate` (WebSub HMAC secrets). `publish`, `unpublish` and `deprecate` are **not** verbs: they are the portal's per-kind `manage` (`ap:portal:rest_api:manage`). A side-effect-free GET is always `read` (`GET /rest-apis/versions` → `ap:api:rest:read`, not `:versions`). Add a new verb to a node only when no CRUD verb or `manage` describes the operation, and say so in the PR.

**Rule 4 — Inheritance replaces wildcards, and only `read` and `manage` flow down.**
There is no `:*`. An operation bound to node *N* with action *A* accepts: *A* on *N*; `manage` on *N* and on every ancestor that defines it; and, when *A* is `read`, `read` on every ancestor that defines it, up to the root (`ap:manage` admits every operation, `ap:read` every read). A parent's `create`, `update`, `delete`, `restore`, … is that node's own operation and never reaches a child. The generator writes all of it into the security block; the enforcer never walks the tree. So `ap:api:rest:manage`, `ap:api:manage` and `ap:manage` admit `CreateBuild`, `ap:api:rest:read` admits `GetBuilds`, and `ap:api:rest:create` admits `CreateRESTAPI` only.

**Rule 5 — URLs first, scopes follow.**
Design the REST URL first, then derive node and action from the *subject being operated on*, not from URL segments. Restructure the URL before inventing a verb.

**Rule 6 — Bind, don't hand-edit.**
A `security:` list is generated output. To change what an operation accepts, change its `x-operation-permissions` entry (or the tree) and run the generator. A pre-tree name is never declared in a spec.

## Writing the path (design it first)

- **Casing differs from scopes.** URL segments are **kebab-case**; scopes are **snake_case**. `POST /rest-apis/import-openapi` → `ap:api:rest:create`.
- **Collections are plural nouns.** `/rest-apis`, `/projects`, `/gateways`. Item is `…/{id}`. Never add a literal sibling GET beside `GET /collection/{id}`.
- **A property is not a resource — filter, don't carve a path.** `GET /gateways?status=…`, `GET /api-portals?default=true`, not `/status/gateways` or `/api-portals/default`.
- **Instance actions are `{id}`-scoped.** `POST /…/deployments/{deploymentId}/undeploy`, never `POST /…/deployments/undeploy`.
- **Create-variant actions hang off the collection.** `POST /rest-apis/import-openapi`, `POST /rest-apis/validate-openapi`.
- **Prefer domain nouns + standard verbs over verb paths.** A WebSub publication is `POST /websub-apis/{apiId}/publications` and `DELETE …/publications/{devportalId}`, bound to `ap:api:websub:publication:create` / `:delete`.

## Binding an operation (`x-operation-permissions`)

One entry per operation, keyed by `operationId`, or by `"METHOD /path"` for a plugin operation that has none. List what the operation accepts **directly**; the generator adds the ancestors.

```yaml
x-operation-permissions:
  DeployAPI:
  - ap:api:manage                   # the API aggregate, listed by convention
  - ap:api:rest:deployment:create   # the node's own action  -> binds node + action
  - ap:api:rest:deployment:manage   # the node's manage
  - ap:api:rest:manage              # the parent's manage
  CreateAPIKey:
  - ap:api:manage
  - ap:api:rest:api_key:create
  - ap:api:rest:api_key:manage
  - ap:api:rest:manage
  - ap:credential:api_key:all:manage   # ownership override, on every API-key operation
  GetRESTAPIGateways:
  - ap:api:manage
  - ap:api:read
  - ap:api:rest:gateway:manage
  - ap:api:rest:gateway:read
  - ap:api:rest:manage
  - ap:gateway:manage                  # cross-resource grant: gateway readers may look too
  - ap:gateway:read
```

What the generator derives and checks:

- The **deepest** tree permission names the node; its non-`manage` action (at most one) names the action. Two nodes at the same depth, or two actions on the node, are errors.
- The security block becomes the inheritance set of that binding, then every listed permission of a node **outside** the chain (a cross-resource grant) and the ownership override, in the order listed. A chain permission the rule does not grant (`ap:api:rest:read` on a create, or the parent's `ap:api:rest:create` on a build) is rejected.
- Every operation with a security block must have an entry; an entry no spec declares is only a warning.

Generated for `DeployAPI`:

```yaml
security:
  - OAuth2Security:
      - ap:api:rest:deployment:create
      - ap:api:rest:deployment:manage
      - ap:api:rest:manage
      - ap:api:manage
      - ap:manage
```

## The API-key ownership override

`ap:credential:api_key:all:manage` widens **whose** API keys a caller may see and touch, not which operations. List it on every API-key create/update/revoke/list operation so a holder reaches the endpoint; the handler then checks it by name and passes the result into `canManageAPIKey`, which otherwise compares `created_by` with the caller. It is **never implied** by any permission, the root included, is never requested by a console BFF by default, and must be granted to a role explicitly (`ap_admin` holds `ap:manage` *and* the override). See GO-AUTH-020 in `.claude/rules/authentication_authorization.md`.

## Pre-tree (legacy) names

`legacy-scope-mapping.yaml` is a frozen list of the names that preceded the tree, each mapped 1:1 by action onto the permission it grants (`ap:rest_api:deployment:restore: [ap:api:rest:deployment:restore]`). The enforcer expands a request's scopes through it before matching; the CP console's codegen emits it as `LEGACY_SCOPES` and the AI Workspace's generated module carries it too, so a token or IDP role that still uses old names reaches, in both consoles and the API, exactly what the mapped permission reaches, parent privilege included.

- Never declare a legacy name in a spec; the generator and startup validation both refuse.
- When you **rename or move a node**, keep its old permissions working by adding the old resource prefix to the node's `legacy:` list and one entry per old permission here, mapped by the same action. The generator validates that each target is what the tree accepts for that node and verb.
- Do not add legacy entries for new permissions; a new permission has no old name.

## Catalog descriptions

The main spec's `OAuth2Security` catalog lists exactly the permissions its operations accept. The generator keeps an existing description and writes `<Verb> <node name>` for a new permission; edit that wording in `openapi.yaml` after the first run, it is preserved thereafter. Describe the **most specific entity the permission governs**: `ap:api:rest:api_key:create` → "Create an API key for a REST API" (not "Create REST API"); `:manage` → "Full access to …"; `ap:manage` → "Full access to every Platform API resource and operation". Human prose with proper casing (REST API, LLM provider), never the snake_case segment.

## Roles and what the consoles request

- **`role-to-scope-mapping.yaml`**: grant the highest node that matches the persona and nothing an ancestor already covers. `ap_admin` is `ap:manage` plus the override; `ap_viewer` is `ap:read`; `ap_subscriber` reads the catalogue with `ap:api:read`. Shipped roles use tree permissions only (a test forbids legacy names) and carry no `dp:` scopes.
- **Console BFFs** (`defaultOIDCScopes` in each `bff/internal/config/config.go`, and the AI Workspace `configs/config-template.toml`) request the legacy names plus **exactly the tree permissions the shipped roles grant**. Fine-grained tree actions are not requested; a deployment whose IDP roles grant one must add it to `[auth.oidc] scope`. The AI Workspace test `TestDefaultOIDCScopesRequestGranularScopes` reads the role file and fails when the two drift.

## Checklist for adding a resource / operation

1. **Design the URL first** (Rule 5, "Writing the path").
2. **Find or add the node** in `permission-tree.yaml`: nest under the owner (an API kind under `ap:api`), snake_case `handle`, a `name` for catalog prose, `plugin:` if plugin-only. Give it the actions it genuinely needs; `manage` on every node that has writes.
3. **Pick the action** (Rules 2–3). A verb the node does not define is a deliberate addition to its `actions`.
4. **Add the `x-operation-permissions` entry**: the node's action and `manage`, the parent's `manage`, `ap:api:manage` (and `ap:api:read` for reads) on an API kind, the ownership override on an API-key operation, and any cross-resource grant.
5. **Run the generator**: `cd platform-api && uv run --with pyyaml python3 scripts/permission_tree.py`, then `--check`. Fix the catalog wording for a new permission.
6. **Regenerate the consoles**: `npm run api:codegen` in `portals/api-control-plane`; the AI Workspace module was written by the generator.
7. **Roles**: a parent or root grant usually reaches the new operation already. Only a node outside every shipped role's reach needs a new grant, and then the BFF scope lists too.
8. **Renamed or moved a node?** Add its `legacy:` prefix and the 1:1 entries to `legacy-scope-mapping.yaml`.
9. **Verify**: `go test ./internal/server/ ./internal/middleware/` in `platform-api`; `npx vitest run src/permissions` in the CP console; `npx vitest run src/auth` in the AI Workspace.

## Path placement in the OpenAPI spec

Insert a new path in the **canonical resource-group order** and keep all sub-resources of a parent immediately beneath it.

### Canonical group order

| # | Group | Root path(s) | Tree node |
|---|---|---|---|
| 1 | Organizations | `/organizations` | `ap:organization` |
| 2 | Projects | `/projects` | `ap:project` |
| 3 | REST APIs | `/rest-apis` | `ap:api:rest` |
| 4 | API publications and portals | `/api-publications`, `/api-portals/{id}/apis/…` | `ap:publication`, `ap:portal:*` |
| 5 | API content | `/apis/{apiType}/{apiId}/docs`, `…/thumbnail` | `ap:content:*` |
| 6 | API Portals | `/api-portals` | `ap:portal` |
| 7 | GraphQL APIs | `/graphql-apis` | `ap:api:graphql` |
| 8 | LLM Provider Templates | `/llm-provider-templates` | `ap:llm_provider:template` |
| 9 | LLM Providers | `/llm-providers` | `ap:llm_provider` |
| 10 | LLM Proxies | `/llm-proxies` | `ap:api:llm` |
| 11 | MCP Proxies | `/mcp-proxies` | `ap:api:mcp` |
| 12 | Agent Proxies | `/agent-proxies` | `ap:api:agent` |
| 13 | Gateways | `/gateways`, `/gateway-custom-policies` | `ap:gateway`, `ap:gateway_policy` |
| 14 | Applications | `/applications` | `ap:application` |
| 15 | Subscription Plans | `/subscription-plans` | `ap:subscription_plan` |
| 16 | Subscriptions | `/subscriptions` | `ap:subscription` |
| 17 | Me | `/me` | `ap:credential:api_key` |
| 18 | Secrets | `/secrets` | `ap:credential:secret` |
| plugin | WebSub / WebBroker APIs | `plugins/eventgateway/openapi.yaml`: `/websub-apis`, `/webbroker-apis` | `ap:api:websub`, `ap:api:webbroker` |

### Sub-resource placement rule

A sub-resource path **always follows its parent immediately**. Within a group:

1. Collection (`/resource`)
2. Utility / create-variant actions on the collection (`/resource/validate-openapi`, `/resource/import-openapi`, `/resource/versions`)
3. Item (`/resource/{id}`)
4. Sub-resource collection (`/resource/{id}/sub`)
5. Sub-resource item (`/resource/{id}/sub/{subId}`)
6. Sub-resource lifecycle actions (`/resource/{id}/sub/{subId}/undeploy`, `…/restore`)

Example for REST APIs, as the spec stands:
```
/rest-apis
/rest-apis/validate-openapi
/rest-apis/import-openapi
/rest-apis/versions
/rest-apis/{restApiId}
/rest-apis/{restApiId}/openapi
/rest-apis/{restApiId}/gateways
/rest-apis/{restApiId}/api-keys
/rest-apis/{restApiId}/api-keys/{apiKeyId}
/rest-apis/{restApiId}/builds
/rest-apis/{restApiId}/builds/{buildId}
/rest-apis/{restApiId}/deployments
/rest-apis/{restApiId}/deployments/{deploymentId}
/rest-apis/{restApiId}/deployments/{deploymentId}/undeploy
/rest-apis/{restApiId}/deployments/{deploymentId}/restore
```

### Checklist for placing a new path

1. Identify the group from the table; insert directly after its last existing path.
2. A sub-resource goes immediately after its parent's last sub-resource, not at the end of the group.
3. Never insert between unrelated groups.
4. Add a `tags:` entry if the path introduces a new tag.
5. Give the operation an `operationId`; the binding, the CP console's scope map and the drift tests all key on it.

## Anti-patterns (reject in review)

- Editing a `security:` list by hand, or a spec whose `scripts/permission_tree.py --check` fails.
- An operation with no `x-operation-permissions` entry, or an entry listing a permission the tree does not define.
- A pre-tree name (`ap:rest_api:*`, `ap:llm_proxy:*`, `ap:api_key:all:manage`) in a spec, a shipped role, or a new binding.
- `ap:rest-api:create` / `ap:restApi:create` — snake_case `rest`; `ap:project:list` / `:get` / `:add` — `read` / `create`.
- `:validate`, `:preview`, `:versions`, `:status` on a side-effect-free GET — it is `read`.
- A new custom verb used in a binding without being added to the node's `actions`, or added without a stated reason.
- An API kind modelled outside `ap:api`, or a child nested under a node that does not own it.
- Relying on the override as a route scope only: an API-key handler that forgets to pass `isKeyAdmin` into the service, or a role granting `ap:manage` and expecting cross-user key administration without the override.
- A legacy entry added for a brand-new permission, or a renamed node without `legacy:` and mapping entries.
- Requesting the ownership override from the IDP by default, or a BFF scope list that drifts from what the shipped roles grant.
- A sub-resource scope described by its parent in the catalog ("Create REST API" for `ap:api:rest:api_key:create`).
- snake_case or camelCase URL segments; a literal `/status` or `/default` segment for a filter; a collection-level verb for an instance action.
