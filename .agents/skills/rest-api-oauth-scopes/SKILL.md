---
name: rest-api-oauth-scopes
description: Design REST API resources and their OAuth2 scopes with consistent, enforceable conventions. Use when adding/editing a resource, path, or operation in any OpenAPI-described REST API guarded by OAuth2 scopes; naming a scope; deciding which scopes guard an endpoint; adding a sub-resource; choosing CRUD vs a custom verb; or mapping IDP roles to scopes. Provider-agnostic design rules — bind them to a specific codebase by choosing a scope prefix and an enforcement mechanism.
allowed-tools: Bash, Read, Edit, Grep, Glob
---

# REST API Resources & OAuth2 Scopes

Provider-agnostic conventions for modelling REST resources and the OAuth2 scopes that guard them. These rules keep scope names predictable, reviewable, and mechanically enforceable from an OpenAPI spec. They are independent of any one codebase — a concrete API binds them by choosing a **scope prefix** and an enforcement mechanism (middleware that reads each operation's `security` block and OR-evaluates the token's scopes against it).

## Scope format

```
<prefix>:<resource>[:<sub-resource>...]:<action>
```

- **`<prefix>`** — a short, fixed namespace owned by the API (e.g. `ap:` for the WSO2 Platform API). Every scope in the API shares it. Pick one and never mix.
- **`<resource>`** — primary entity, snake_case (`project`, `rest_api`, `llm_provider`, `gateway`).
- **`<sub-resource>`** (optional, may nest more than once) — an *owned* child entity, snake_case (`gateway:token`, `application:api_key`, `rest_api:deployment`).
- **`<action>`** — a CRUD verb, the `manage` superset, or a genuine custom verb, snake_case.

Example shape: `<prefix>:project:create`, `<prefix>:gateway:token:create`, `<prefix>:rest_api:deployment:undeploy`.

## The core rules

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
- `GET /rest-apis/validate` → `<prefix>:rest_api:read` (not `:validate`)
- `GET /.../{id}/preview` → `<prefix>:...:read` (not `:preview`)

**Rule 4 — Wildcards: `:*`, own-level only.**
`:*` covers all actions directly at that level — never descends into sub-resources, never matches a prefix (e.g. `application*`), never transitive.

| Wildcard | Covers |
|---|---|
| `<prefix>:*` | Every action on **root-level resources** (`gateway:create`, `rest_api:read`, …). Own-level like every `:*` — **not** sub-resources such as `gateway:token:*` or `application:api_key:*`. |
| `<prefix>:<resource>:*` | All actions **directly** on the resource. **Not** its sub-resources. |
| `<prefix>:<resource>:<sub>:*` | All actions directly on that sub-resource. |

This means a token with `ap:gateway:*` satisfies `ap:gateway:create` but **not** `ap:gateway:token:create` — that sub-resource needs its own grant (`ap:gateway:token:*` or `ap:gateway:token:create`).

**Rule 5 — URLs first, scopes follow.**
Design the REST URL first, then derive the scope from the *subject being operated on* — not from literal URL path segments. Restructure the URL (e.g. move an action under its `{id}` path, filter with a query param instead of a nested read) before inventing a verb.

## Writing the path (design it first)

Get the REST URL right *before* you name the scope (Rule 4):

- **Casing differs from scopes — don't force them to match.** URL segments stay **kebab-case**; scopes are **snake_case**. `POST /devportals/{id}/set-default` → scope `<prefix>:devportal:set_default`. `POST /rest-apis/import-openapi` → scope `<prefix>:rest_api:import`.
- **Collections are plural nouns.** `/rest-apis`, `/projects`, `/gateways`, `/versions`. Item is `…/{id}`. Never add a literal sibling GET next to `GET /collection/{id}` (e.g. no `GET /rest-apis/validate` beside `GET /rest-apis/{id}`).
- **A property is not a resource — filter, don't carve a path.** Use a query param instead of a literal segment for state/defaults/filters:
  - `GET /status/gateways` → `GET /gateways` (read the `isActive`/status field; or `?status=…`)
  - `GET /devportals/default` → `GET /devportals?default=true`
  - `GET /rest-apis/validate` → `GET /rest-apis?name=&version=` (filter on the collection — no new path, no new scope)
- **Instance actions are `{id}`-scoped.** Put a lifecycle verb under the item it acts on, not the collection:
  - `POST /…/deployments/undeploy` → `POST /…/deployments/{deploymentId}/undeploy`
  - `POST /…/deployments/restore` → `POST /…/deployments/{deploymentId}/restore`
- **Create-variant actions hang off the collection namespace.** When an action *creates* the resource, name it on the collection: `POST /rest-apis/import-openapi`, `POST /rest-apis/validate-openapi`, `POST /api-projects/import`.
- **Prefer existing domain nouns + standard verbs over custom-verb paths.**

## The `:manage` superset

Every resource (and every owned sub-resource) gets a `:manage` scope — a superset that covers `read` + all writes + custom verbs for that resource. **Every operation lists both** the specific scope **and** the resource's `manage` scope, OR-evaluated, so a broad-access token and a fine-grained token both succeed:

```yaml
security:
  - OAuth2Security:
      - <prefix>:project:create   # fine-grained
      - <prefix>:project:manage   # superset — always include the resource's manage scope
```

For a write on a sub-resource, include the sub-resource's manage scope (and, where ownership implies it, the parent's manage too — e.g. `<prefix>:gateway:token:create` paired with `<prefix>:gateway:token:manage`).

## OR-evaluation

The scope list on an operation is **OR-evaluated**: holding any one scope in the list is sufficient. This is what lets the specific scope and the `:manage` superset coexist on the same operation.

## Catalog descriptions

Each catalog entry's description is **`<action> <the most specific entity the scope governs>`** — name the deepest segment (the sub-resource), not the parent, so the entry reads distinctly from both its parent and its siblings on a consent screen. `<prefix>:rest_api:api_key:create` → "Create an API key for a REST API" (**not** "Create REST API"); `<prefix>:gateway:token:read` → "Read gateway tokens"; `:manage` → "Full access to …". Spell resource names in human prose with proper casing (REST API, LLM provider, DevPortal), not the snake_case scope segment.

## Checklist for adding a resource / operation

1. **Design the URL first** (Rule 5 + "Writing the path"): plural collections, kebab-case segments, `{id}`-scoped actions, query-param filters over property-paths.
2. Pick the **action verb**: CRUD closed set (Rule 2) → use it; side-effect-free GET → `read` (Rule 3); genuine lifecycle op → an allowed custom verb.
3. Build the scope `<prefix>:<resource>[:<sub-resource>]:<action>` — snake_case, colon delimiters, nest only on real ownership (Rule 1).
4. In the operation's `security` block, list the **specific scope + the resource's `:manage` scope**, OR-evaluated.
5. Add **both** scopes (and `:manage`) to the **scope catalog**, each with a description that names the specific entity it governs (see "Catalog descriptions").
6. If a built-in role should get the new scope, add it to the role→scopes map.
7. Build and verify the enforcement point parses the spec and validates the role map.

## Path placement in the OpenAPI spec

When adding a new path to `platform-api/src/resources/openapi.yaml`, insert it in the **canonical resource-group order** below and keep all sub-resources of a parent immediately beneath it.

### Canonical group order

| # | Group | Root path(s) |
|---|---|---|
| 1 | Organizations | `/organizations` |
| 2 | Projects | `/projects` |
| 3 | REST APIs | `/api-projects`, `/rest-apis` |
| 4 | WebSub APIs | `/websub-apis` |
| 5 | WebBroker APIs | `/webbroker-apis` |
| 6 | LLM Provider Templates | `/llm-provider-templates` |
| 7 | LLM Providers | `/llm-providers` |
| 8 | LLM Proxies | `/llm-proxies` |
| 9 | MCP Proxies | `/mcp-proxies` |
| 10 | Gateways | `/gateways`, `/gateway-custom-policies` |
| 11 | Applications | `/applications` |
| 12 | Subscription Plans | `/subscription-plans` |
| 13 | Subscriptions | `/subscriptions` |
| 14 | DevPortals | `/devportals` |
| 15 | Git | `/git` |
| 16 | Me | `/me` |

### Sub-resource placement rule

A sub-resource path **always follows its parent immediately** — never floats to a different group or appears before the parent. The order within a group is:

1. Collection (`/resource`)
2. Item (`/resource/{id}`)
3. Utility / variant actions on the collection (`/resource/import-openapi`, `/resource/validate`)
4. Sub-resource collection (`/resource/{id}/sub`)
5. Sub-resource item (`/resource/{id}/sub/{subId}`)
6. Sub-resource lifecycle actions (`/resource/{id}/sub/{subId}/undeploy`, `.../restore`)

Example for REST APIs:
```
/api-projects/validate
/api-projects/import
/rest-apis
/rest-apis/import-openapi
/rest-apis/validate-openapi
/rest-apis/{apiId}
/rest-apis/{apiId}/gateways
/rest-apis/{apiId}/api-keys
/rest-apis/{apiId}/api-keys/{keyName}
/rest-apis/{apiId}/deployments
/rest-apis/{apiId}/deployments/{deploymentId}
/rest-apis/{apiId}/deployments/{deploymentId}/undeploy
/rest-apis/{apiId}/deployments/{deploymentId}/restore
```

### Checklist for placing a new path

1. Identify which group the root resource belongs to using the table above.
2. Locate the last existing path in that group; insert the new path directly after it.
3. If the new path is a sub-resource, insert it immediately after its parent's last existing sub-resource (not at the end of the group).
4. Never insert a path outside its group or between two unrelated groups.
5. Add a matching entry to the top-level `tags:` list if the path introduces a new tag.

### Anti-patterns for path placement

- A sub-resource path appearing before its parent (e.g. `/llm-proxies/{id}/deployments` before `/llm-proxies`).
- Sub-resources of one group scattered inside another group (e.g. a sub-resource path appearing after an unrelated group).
- A new group inserted at an arbitrary position rather than following the canonical order.
- A tag referenced in an operation's `tags:` list that has no entry in the top-level `tags:` section.

## Anti-patterns (reject in review)

- `<prefix>:rest-api:create` / `<prefix>:restApi:create` — must be snake_case `rest_api`.
- `<prefix>:project:list`, `:get`, `:add` — use `read` / `create`.
- `:validate` / `:preview` / `:status` scopes on a side-effect-free GET — it's `:read`.
- Nesting a sub-resource that isn't owned by the parent — make it a top-level resource instead.
- An operation that lists only the fine-grained scope and omits `:manage`.
- A scope used in a `security` block but missing from the scope catalog (or vice versa).
- A sub-resource scope described by its parent (`<prefix>:rest_api:api_key:create` → "Create REST API") — describe the actual subject ("Create an API key for a REST API").
- snake_case or camelCase URL segments (`/rest_apis`, `/restApis`) — paths are kebab-case (`/rest-apis`).
- A literal `/status`, `/default`, or `/validate` path segment for what is really a property/filter — use a query param on the collection.
- A collection-level action verb (`POST /deployments/undeploy`) when the action targets one instance — scope it `{id}` (`/deployments/{id}/undeploy`).
