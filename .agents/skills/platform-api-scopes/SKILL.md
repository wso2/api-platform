---
name: platform-api-scopes
description: Write Platform API resources (REST endpoints) and define their OAuth2 scopes correctly. Use when adding/editing a resource, path, or operation in the Platform API OpenAPI spec; naming a scope; deciding which scopes guard an endpoint; adding a sub-resource; choosing a CRUD vs custom verb; or mapping IDP roles to scopes. Encodes the conventions agreed in discussion #2045.
allowed-tools: Bash, Read, Edit, Grep, Glob
---

# Platform API — Resources & Scopes

How to add a resource to the Platform API and guard it with the right OAuth2 scopes. The rules below come from [discussion #2045](https://github.com/wso2/api-platform/discussions/2045) and are ground-truthed against the live spec and enforcer.

> **Path convention.** Paths are written `<REPO_ROOT>/...`; substitute your
> api-platform checkout. The Platform API lives under `<REPO_ROOT>/platform-api`.

## Where things live

| What | File |
| --- | --- |
| Source of truth — every path, operation, and its required scopes | `platform-api/src/resources/openapi.yaml` |
| Scope catalog (every scope must also be declared here) | same file → `components.securitySchemes.OAuth2Security.flows.clientCredentials.scopes` |
| Scope registry — builds `METHOD:path → [scopes]` from the spec at startup | `platform-api/src/internal/middleware/openapi_scope_registry.go` |
| Authorization middleware (OR-evaluates the token's scopes vs required) | `platform-api/src/internal/middleware/authorization.go` |
| IDP-role → platform-scope mapping loader | `platform-api/src/internal/middleware/role_scope_map.go` (reads `roles.yaml`) |

**The OpenAPI spec is the source of truth.** There are no scope annotations in handler code. The registry parses each operation's `security` block, and the middleware enforces it. If a scope isn't in the spec, it doesn't exist.

## Scope format

```
ap:<resource>[:<sub-resource>...]:<action>
```

- **Prefix** is always `ap:` (this is the implemented short form of `api-platform`; the spec and enforcer use `ap:`).
- **`<resource>`** — primary entity, snake_case (`project`, `rest_api`, `llm_provider`, `gateway`, `gateway_custom_policy`).
- **`<sub-resource>`** (optional, may nest more than once) — an *owned* child entity, snake_case (`gateway:token`, `application:api_key`, `rest_api:deployment`).
- **`<action>`** — a CRUD verb, the `manage` superset, or a genuine custom verb, snake_case.

Real examples from the spec: `ap:project:create`, `ap:application:read`,`ap:gateway:token:create`, `ap:llm_provider:api_key:delete` `ap:rest_api:deployment:undeploy`.

## The five rules (discussion #2045)

**Rule 1 — Lock the shape.**
Colon is the *only* delimiter. snake_case for *every* segment, including compound names (`rest_api`, `llm_provider`, **not** `rest-api` / `restApi`). 
Nest a sub-resource under its parent **only when there is real ownership** — the parent scopes what the child means: `gateway:token:create` (a token belongs to a gateway), `application:api_key:create` (a key belongs to an application). If the child can stand alone, it's a top-level resource, not a sub-resource.

**Rule 2 — CRUD verbs are a closed set.**
Use exactly these for CRUD, never synonyms (no `list`, `get`, `add`, `remove`, `edit`):

| Verb | HTTP |
| --- | --- |
| `read` | GET list **and** GET by-id (one verb covers both) |
| `create` | POST |
| `update` | PUT / PATCH |
| `delete` | DELETE |

**Rule 3 — Custom verbs only for genuine non-CRUD.**
Allowed for real lifecycle / bulk / sync operations: `publish`, `unpublish`, `deploy`, `undeploy`, `import`, `export`, `sync`, `restore`.
A `GET` with no side effects is **always** `:read`, even if the URL segment looks like a verb:
- `GET /rest-apis/validate` → `ap:rest_api:read` (not `:validate`)
- `GET /.../{id}/preview` → `ap:...:read` (not `:preview`)

**Rule 5 — URLs first, scopes follow.**
Design the REST URL first, then derive the scope from the *subject being operated on* — not from literal URL path segments. Restructure the URL (e.g. move an action under its `{id}` path, filter with a query param instead of a nested read) before inventing a verb.

## Writing the path (design it first)

Get the REST URL right *before* you name the scope (Rule 5). Path conventions agreed in #2045:

- **Casing differs from scopes — don't force them to match.** URL segments stay **kebab-case**; scopes are **snake_case**. `POST /devportals/{id}/set-default` → scope `ap:devportal:set_default`. `POST /rest-apis/import-openapi` → scope `ap:rest_api:import`.
- **Collections are plural nouns.** `/rest-apis`, `/projects`, `/gateways`, `/api-projects`, `/versions`. Item is `…/{id}`. Never add a literal sibling GET next to `GET /collection/{id}` (e.g. no `GET /rest-apis/validate` beside `GET /rest-apis/{id}`).
- **A property is not a resource — filter, don't carve a path.** Use a query param instead of a literal segment for state/defaults/filters:
  - `GET /status/gateways` → `GET /gateways` (read the `isActive`/status field; or `?status=…`)
  - `GET /devportals/default` → `GET /devportals?default=true`
  - `GET /rest-apis/validate` → `GET /rest-apis?name=&version=` (filter on the collection — no new path, no new scope)
- **Instance actions are `{id}`-scoped.** Put a lifecycle verb under the item it acts on, not the collection:
  - `POST /…/deployments/undeploy` → `POST /…/deployments/{deploymentId}/undeploy`
  - `POST /…/deployments/restore` → `POST /…/deployments/{deploymentId}/restore`
- **Create-variant actions hang off the collection namespace.** When an action *creates* the resource, name it on the collection: `POST /rest-apis/import-openapi`, `POST /rest-apis/validate-openapi`, `POST /api-projects/import`, `POST /api-projects/validate`.
- **Prefer existing domain nouns + standard verbs over custom-verb paths.** Model publish/unpublish as a `publications` sub-resource: `POST /rest-apis/{apiId}/publications` to publish, `DELETE /rest-apis/{apiId}/publications/{devportalId}` to unpublish — instead of `POST …/devportals/publish` and `…/unpublish`.

## The `:manage` superset

Every resource (and every owned sub-resource) gets a `:manage` scope — a superset that covers `read` + all writes + custom verbs for that resource. **Every operation lists both** the specific scope **and** the resource's `manage` scope, OR-evaluated, so a broad-access token and a fine-grained token both succeed:

```yaml
security:
  - OAuth2Security:
      - ap:project:create   # fine-grained
      - ap:project:manage   # superset — always include the resource's manage scope
```

For a write on a sub-resource, include the sub-resource's manage scope (and, per the spec's pattern, the parent's manage where ownership implies it — e.g. `ap:gateway:token:create` is paired with `ap:gateway:token:manage`).

## Declaring a scope on an operation

Two edits, both in `openapi.yaml`, for every new scope:

1. On the operation, under `security`:

```yaml
paths:
  /projects:
    post:
      operationId: CreateProject
      security:
        - OAuth2Security:
            - ap:project:create
            - ap:project:manage
      tags: [Projects]
      # ... requestBody / responses ...
    get:
      operationId: ListProjects
      security:
        - OAuth2Security:
            - ap:project:read
            - ap:project:manage
```

The scope list is **OR-evaluated**: holding any one scope in the list is sufficient.

2. Register the scope name + a description in the catalog so it's a known scope (the role-map validator rejects any scope not declared here):

```yaml
components:
  securitySchemes:
    OAuth2Security:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://localhost:9243/oauth2/token
          scopes:
            ap:project:create: Create project
            ap:project:read:   Read project
            ap:project:manage: Full access to project
```

## Roles → scopes (IDP integration)

Tokens may carry IDP role names instead of (or alongside) raw scopes.
`role_scope_map.go` loads a `roles.yaml` mapping each IDP role to a list of platform scopes; the union of a token's roles' scopes is enforced at request time.
Every scope referenced in `roles.yaml` **must** be declared in the OpenAPI catalog — `ValidateRoleScopeMap` fails fast at startup on an unknown scope (a typo would otherwise silently deny access). Conceptual platform roles (admin, developer, publisher, operator, viewer) are realized as such mappings.

## Checklist for adding a resource / operation

1. **Design the URL first** (Rule 5). Prefer ID-scoped action paths (`/deployments/{id}/undeploy`) and query-param filters over nested status reads.
2. Pick the **action verb**: CRUD closed set (Rule 2) → use it; side-effect-free GET → `read` (Rule 3); genuine lifecycle op → an allowed custom verb.
3. Build the scope `ap:<resource>[:<sub-resource>]:<action>` — snake_case, colon delimiters, nest only on real ownership (Rule 1).
4. In the operation's `security` block, list the **specific scope + the resource's `:manage` scope**, OR-evaluated.
5. Add **both** scopes (and `:manage`) to the `securitySchemes` **scope catalog** with descriptions.
6. If a built-in sample role should get the new scope, add it to `roles.yaml`.
7. Verify it builds and the registry parses it:
   ```bash
   cd <REPO_ROOT>/platform-api && go build ./... && \
     go test ./src/internal/middleware/...
   ```

## Anti-patterns (reject in review)

- `ap:rest-api:create` / `ap:restApi:create` — must be snake_case `rest_api`.
- `ap:project:list`, `ap:project:get`, `ap:project:add` — use `read` / `create`.
- `:validate` / `:preview` / `:status` scopes on a side-effect-free GET — it's `:read`.
- Nesting a sub-resource that isn't owned by the parent — make it a top-level resource instead.
- An operation that lists only the fine-grained scope and omits `:manage`.
- A scope used in a `security` block but missing from the scope catalog (or vice versa).
