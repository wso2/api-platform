# API Platform House Rules (APR-001..008)

Reference for the `api-platform-rest-api-design-rules` skill. These are the WSO2 API Platform-specific conventions that the generic `api-platform:api-design` assessor does **not** check. Step 4 of the skill reads this file and evaluates each rule against the spec, recording findings in the shape described in SKILL.md.

These conventions are specific to the WSO2 API Platform and tuned to its own contract (`ap:` scopes, handle-or-UUID identifiers, the standard list envelope). Where a convention common in other REST APIs conflicts with an established API Platform pattern, the API Platform pattern wins and the other is intentionally **not** adopted (e.g. `getAll...`-style operationIds, single-tag-per-operation, alternate scope-naming schemes, ETag/tenant request headers).

> **Avoid duplicating the generic assessor.** The base `api-platform:api-design` skill already checks kebab-case plural paths, verb-noun `operationId`s, parameter/operation descriptions, tags, contact info, OWASP error/validation/429/JWT, and path-param placement. Only flag the rules below — the platform-specific delta.

> **Recording blanket-missing findings.** When a rule is violated uniformly across many operations (e.g. a convention that none of the operations follow), record **one representative finding** that names the pattern and gives a couple of example paths, rather than one finding per operation — keep the report signal-rich.

---

## APR-001 — Parameter names must be consistent lowerCamelCase
**Severity: MEDIUM**

Every parameter must use `lowerCamelCase` for both:
1. The `components.parameters.<key>` map key, and
2. The parameter's `name:` value (the wire-level query/path parameter name).

**Accepted suffix convention (do NOT flag):** a query-variant of a path parameter may carry a `-Q` suffix, and an optional query-variant a `-Q-Opt` suffix, on the **component key only** (e.g. `projectId-Q`, `gatewayId-Q`, `apiId-Q-Opt`). This is the established cross-product convention for distinguishing the query form of an identifier from its path form. The stem before the suffix must still be lowerCamelCase, and the wire-level `name:` must not contain the suffix.

A parameter is a violation when the key (ignoring any trailing `-Q`/`-Q-Opt`) or the `name:` value is **not** lowerCamelCase — specifically any of:
- starts with an uppercase letter (PascalCase): `ProjectID`, `GatewayID`, `TokenID`, `DevPortalID`, `ArtifactType-Q` (stem `ArtifactType`).
- uses an all-caps acronym suffix instead of camelCase: `...ID` should be `...Id` (`projectId`, not `projectID`); e.g. `entityID-Q` (stem `entityID`).
- uses `snake_case` or `kebab-case` in the stem: `project_id`, `api-name-Q` (stem `api-name`), `api-version-Q`.

Also flag **inconsistent spelling of the same logical identifier** across the spec — e.g. a path parameter referenced as `{apiId}` in one place and `{ApiId}`/`{api_id}` elsewhere. The same concept must use one spelling everywhere.

> **Why:** The platform's APIs are consumed by generated SDKs, the CLI, and agents that assume one casing convention. A mix of `apiId`, `appId`, `organizationId` (camelCase) with `ProjectID`, `GatewayID` (PascalCase component keys) makes `$ref` targets and generated parameter names unpredictable.

**How to detect:** Scan `components.parameters` keys and every `parameters[].name` (inline and in components). Strip a trailing `-Q` or `-Q-Opt` from the key, then test the stem against `^[a-z][a-zA-Z0-9]*$`; reject embedded `_`, `-`, leading uppercase, or a trailing all-caps run of length ≥ 2 (e.g. `ID`). Test each `name:` against the same pattern (no suffix allowed in `name:`).

**Fix:** Rename to lowerCamelCase: `ProjectID → projectId`, `GatewayID → gatewayId`, `ArtifactType-Q → artifactType-Q`, `api-name-Q → apiName-Q`, `entityID-Q → entityId-Q`.
- Renaming a **component key** (and its `$ref`s) is safe and **autoFixable: true** — it does not change the wire contract.
- Renaming a parameter's `name:` value is a **breaking change** to the URL/query contract → **autoFixable: false**; surface it for confirmation. (In the platform-api spec the `name:` values are usually already camelCase; mostly only component keys need fixing.)

---

## APR-002 — Collection responses must use the standard list envelope
**Severity: MEDIUM**

Every list/collection response body must use the platform's standard envelope:

```yaml
type: object
required: [list, pagination]
properties:
  list:       { type: array, items: { $ref: '...' } }
  pagination: { $ref: '#/components/schemas/Pagination' }
```

The shared `Pagination` schema must expose at least `total`, `offset`, `limit`. Next/previous page links (`next`, `previous`) are recommended for paginated collections but not required. A separate top-level page-size field is not needed — the page size is the length of `list`, and the overall size is `pagination.total`.

Flag a collection response when it:
- names the data array something other than `list` (e.g. `items`, `subscriptions`, `subscriptionPlans`), or
- omits the `pagination` object while the operation accepts `limit`/`offset`, or
- returns a bare `type: array` instead of the wrapped object.

> **Why:** A uniform list envelope lets one client or agent page through every collection the same way. Deviations — a differently named data array (e.g. `items`, or the resource name), a missing `pagination` object, or a bare top-level array — force every consumer to special-case that endpoint.

**How to detect:** For each `*ListResponse` schema (and any 200 response whose schema is an object wrapping an array, or a bare array), check the array property is named `list` and a `pagination` ref is present when the operation has `limit`/`offset` parameters.

**Fix:** Rename the data array to `list`, add the `pagination: $ref '#/components/schemas/Pagination'` property, and add it to `required`. Renaming a **response body key** is a breaking change for consumers → **autoFixable: false**; present these for confirmation.

---

## APR-003 — OAuth2 scope names must follow the ap: convention
**Severity: HIGH**

Scopes declared under `components.securitySchemes.*.flows.*.scopes` and referenced in each operation's `security` block must follow the platform scope convention.

**This rule defers to the `rest-api-oauth-scopes` skill as the source of truth.** Invoke / consult that skill for the canonical format and rules; do not re-derive them here. In summary, that skill requires:
- shape `<prefix>:<resource>[:<sub-resource>...]:<action>`, colon-delimited;
- `snake_case` for every segment (`rest_api`, `llm_provider`, **not** `rest-api`/`restApi`);
- the platform prefix `ap:` on every scope;
- a closed CRUD verb set (`read`, `create`, `update`, `delete`) plus `manage` and a small set of genuine custom verbs (`publish`, `deploy`, `undeploy`, `import`, `restore`, `sync`, …);
- a read-only `GET` is always `:read`, even when the URL segment looks like a verb;
- sub-resources nested only under real ownership.

Additionally, each operation's `security` should accept the broad `<resource>:manage` (and any parent `:manage`) scope **alongside** the fine-grained scope, so a manage-scoped token can perform every operation on the resource. Flag operations whose `security` lists only a fine-grained scope without the corresponding `:manage`.

> **Why:** Scopes are enforced by middleware that OR-evaluates the token's scopes against each operation's `security` block. Off-convention scope names silently fail to match and produce 403s, or worse, grant access the author did not intend.

**How to detect:** Extract the full scope list from the security scheme and from every operation `security` entry. For each scope, apply the `rest-api-oauth-scopes` rules. Flag scopes that break the shape, casing, verb set, or prefix; flag operations whose `security` references a scope not declared in the scheme; flag read-only GETs guarded by a non-`:read` verb; flag operations missing the `:manage` umbrella scope.

**Fix:** Rename the scope to the convention everywhere it appears (scheme + every operation `security` block). Because a scope rename changes the authorization contract, **autoFixable: false** — present proposed renames for confirmation.

---

## APR-004 — Errors must use the standard error response shape
**Severity: HIGH**

Every error response must use the platform's single error shape, served from a shared `components/responses` entry. The body carries a stable, machine-readable string `code` from the error catalog alongside a human-readable message:

```yaml
Error:
  required: [status, code, message]
  properties:
    status:  { type: string, enum: [error] }   # always "error"
    code:    { type: string }                   # catalog code, e.g. REST_API_NOT_FOUND
    message: { type: string }                   # human-readable description
    errors:                                      # optional, per-field validation failures
      type: array
      items: { $ref: '#/components/schemas/FieldError' }

FieldError:
  required: [field, message]
  properties:
    field:   { type: string }                   # offending field path, e.g. spec.context
    message: { type: string }                   # why it failed, e.g. must start with /
```

- `status` — always the literal `"error"`.
- `code` — **required.** A stable machine-readable string from the error catalog, named in `SCREAMING_SNAKE_CASE` as `<DOMAIN>_<REASON>` (e.g. `REST_API_NOT_FOUND`, `MCP_PROXY_CONFLICT`, `AUTH_CONTEXT_MISSING`, `COMMON_INTERNAL_ERROR`). Clients and agents branch on this — not on the HTTP status, which is too coarse.
- `message` — **required**, human-readable.
- `errors[]` — optional; one `{ field, message }` entry per offending field for validation failures.

Flag when:
- the error schema is missing the required string `code`, or uses a numeric `code` / alternate field names (e.g. `title`, `details`, `description`, `moreInfo`) instead of `{ status, code, message, errors[] }`;
- the schema field names disagree with the field names used in the response `example`s (a self-contradiction that breaks generated clients coding against either shape);
- an error `code` value is not a `SCREAMING_SNAKE_CASE` catalog code;
- any error response defines its body inline instead of `$ref`-ing a shared `components/responses` entry;
- a shared error response has no realistic `example`.

> **Why:** A single error contract with a stable string `code` lets every client and agent branch on the failure reliably (HTTP status alone is too coarse), and the `errors[]` array lets agents self-correct on validation failures. A schema that disagrees with its own examples breaks generated clients.

**How to detect:** Inspect the error schema(s) for the `{ status, code, message, errors[] }` shape with a **string** `code`; diff the schema field names against the field names appearing in error `example`s and shared responses; check `code` values match `^[A-Z][A-Z0-9_]*$`; scan operation responses for inline error bodies that bypass `components/responses`.

**Fix:** Align the error schema to `{ status, code, message, errors[] }` (string `code` from the catalog, optional `{ field, message }` items), make every error response a `$ref` to a shared response, and give each shared response an example. Renaming or retyping response body fields is a **breaking change** → **autoFixable: false**; purely adding the new required `code` to an otherwise-matching shape may be safe — confirm first.

---

## APR-005 — Create operations must return a Location header
**Severity: MEDIUM**

Any operation that creates a resource (a `POST` returning `201 Created`) must declare a `Location` response header pointing to the URL of the newly created resource.

Flag any `201` (resource-creating `POST`) response that has no `Location` header.

> **Why:** `Location` lets clients and agents follow the canonical URL of what they just created without guessing how to assemble it from the response body, and is the REST-standard signal for create-like operations.

**How to detect:** For each operation with a `201` response, check `responses.201.headers.Location` exists.

**Fix:** Add a `Location` response header (`schema: { type: string, format: uri }`) to each `201`. Adding a response header is **autoFixable: true** (non-breaking, additive).

---

## APR-006 — Non-CRUD actions use POST on a kebab-case verb sub-path
**Severity: LOW**

Actions that don't map to plain CRUD are expressed as a **kebab-case verb sub-path** under the owning resource (or collection), invoked with `POST` — never as a verb inside a collection noun.

- Good: `/rest-apis/{apiId}/deployments/{deploymentId}/undeploy`, `/devportals/{devportalId}/set-default`, `/gateway-custom-policies/sync`, `/rest-apis/validate-openapi`, `/api-projects/import`.
- Bad: a verb in the collection name (`/getApis`, `/createGateway`), or an action performed with `GET`.

> **Why:** It keeps resource URLs noun-based and predictable while still giving non-CRUD operations a clear, consistent home. This rule also tells the generic "no verbs in paths" check apart from *legitimate* action sub-paths so the platform's `/undeploy`, `/restore`, `/activate` style endpoints are recognised as correct.

**How to detect:** For each path segment that is an action (verb), confirm it is a trailing kebab-case sub-path and the operation is `POST`. Flag verbs embedded in a collection name, action sub-paths invoked with `GET`, or camelCase/`snake_case` action segments.

**Fix:** Move the verb to a trailing kebab-case sub-path and use `POST`. Renaming a path is a **breaking change** → **autoFixable: false**.

---

## APR-007 — Collection list items should use a lightweight item schema
**Severity: MEDIUM**

A collection's `list[]` items should reference a lightweight `*ListItem` / `*Info` schema carrying a minimal set of attributes, not the full single-resource schema returned by the item `GET`.

Flag a `*ListResponse.list` whose items `$ref` the full single-resource schema — one that carries nested arrays/objects (operations, channels, policies, upstream, rate-limiting config, etc.) — when a lighter projection would do. The good pattern is a dedicated `*ListItem`/`*Info` schema holding only the fields a list view needs, reserving the full schema for the single-item `GET`.

> **Why:** Returning the full resource for every item in a list inflates payloads and exhausts an agent's context window when listing many APIs/providers; a `*ListItem` keeps lists cheap and the full schema for single-item reads.

**How to detect:** For each `*ListResponse`, check whether `list.items.$ref` points at the same heavy schema used by the corresponding item `GET`; flag where a lighter `*ListItem`/`*Info` exists or should exist (heavy schema has nested arrays/objects).

**Fix:** Introduce/used a `*ListItem` schema for the collection's items. Changing the item schema shape is a **breaking change** for consumers → **autoFixable: false**.

---

## APR-008 — Collection GETs use the standard pagination/query parameters
**Severity: MEDIUM**

Collection (list) `GET` operations must use the standard query parameters with consistent names, types, and defaults, reused from `components/parameters` where possible:

| Name        | In    | Type    | Default | Purpose                                   |
|-------------|-------|---------|---------|-------------------------------------------|
| `limit`     | query | integer | `20`    | Max items to return (`minimum`/`maximum`) |
| `offset`    | query | integer | `0`     | Starting index in the list                |
| `sortBy`    | query | string  | (enum)  | Field to sort by (where sorting applies)  |
| `sortOrder` | query | string  | `desc`  | `asc` / `desc` (where sorting applies)    |
| `query`     | query | string  | —       | Search filter with `attr:value` modifiers |

`sortBy` / `sortOrder` / `query` are only expected where the endpoint supports sorting/search; `limit` + `offset` are expected on every paginated collection.

Flag a collection GET that:
- defines `limit`/`offset` inline with inconsistent constraints (e.g. missing `minimum`/`maximum`, a different default, or no description) instead of reusing the shared component, so different collections end up accepting different ranges for the same parameter;
- omits `limit`/`offset` on a collection that can grow unbounded.

> **Why:** Consistent pagination parameters let one client/agent paginate every collection the same way; divergent constraints make the allowed range unknowable from the name alone.

**How to detect:** For each collection GET, check `limit`/`offset` are present and match the shared definition (name, type, default, `minimum`/`maximum`, description). Reconcile against `components/parameters`.

**Fix:** Reuse shared `limit`/`offset` parameter components (and `sortBy`/`sortOrder`/`query` where relevant). Tightening or adding constraints/descriptions is generally **autoFixable: true** (non-breaking); changing an existing default is behaviour-affecting → confirm first.
