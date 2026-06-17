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

**Rule 4 — URLs first, scopes follow.**
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
- **Prefer existing domain nouns + standard verbs over custom-verb paths.** Model publish/unpublish as a `publications` sub-resource: `POST /rest-apis/{apiId}/publications` to publish, `DELETE /rest-apis/{apiId}/publications/{devportalId}` to unpublish — instead of `POST …/devportals/publish` and `…/unpublish`.

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


## Checklist for adding a resource / operation

1. **Design the URL first** (Rule 4 + "Writing the path"): plural collections, kebab-case segments, `{id}`-scoped actions, query-param filters over property-paths.
2. Pick the **action verb**: CRUD closed set (Rule 2) → use it; side-effect-free GET → `read` (Rule 3); genuine lifecycle op → an allowed custom verb.
3. Build the scope `<prefix>:<resource>[:<sub-resource>]:<action>` — snake_case, colon delimiters, nest only on real ownership (Rule 1).
4. In the operation's `security` block, list the **specific scope + the resource's `:manage` scope**, OR-evaluated.
5. Add **both** scopes (and `:manage`) to the **scope catalog** with descriptions.
6. If a built-in role should get the new scope, add it to the role→scopes map.
7. Build and verify the enforcement point parses the spec and validates the role map.

## Anti-patterns (reject in review)

- `<prefix>:rest-api:create` / `<prefix>:restApi:create` — must be snake_case `rest_api`.
- `<prefix>:project:list`, `:get`, `:add` — use `read` / `create`.
- `:validate` / `:preview` / `:status` scopes on a side-effect-free GET — it's `:read`.
- Nesting a sub-resource that isn't owned by the parent — make it a top-level resource instead.
- An operation that lists only the fine-grained scope and omits `:manage`.
- A scope used in a `security` block but missing from the scope catalog (or vice versa).
- snake_case or camelCase URL segments (`/rest_apis`, `/restApis`) — paths are kebab-case (`/rest-apis`).
- A literal `/status`, `/default`, or `/validate` path segment for what is really a property/filter — use a query param on the collection.
- A collection-level action verb (`POST /deployments/undeploy`) when the action targets one instance — scope it `{id}` (`/deployments/{id}/undeploy`).
