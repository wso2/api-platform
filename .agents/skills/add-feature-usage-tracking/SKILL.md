---
name: add-feature-usage-tracking
description: Add, change or retire a Platform API feature in the feature-usage taxonomy and bind it to the REST API operations that report it. Use when a new Platform API endpoint (or an existing one) should be counted as feature usage, when someone says "track this endpoint", "add a feature to the taxonomy", "report <operation> to Moesif", "add a feature-usage binding", or when TestEveryPlatformAPIFeatureIsBound, TestBindingsAgreeWithTheTaxonomy, TestBindingsAgreeWithTheOpenAPIDocument or TestFeatureUsageBindingsResolveToRealRoutes fails. Edits exactly two files, platform-api/internal/featureusage/taxonomy/api-platform-taxonomy.yaml and platform-api/internal/featureusage/bindings.go, then runs the tests that keep them in agreement.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# Add feature-usage tracking for a Platform API feature

Platform API reports every call to a **bound** REST API operation as one Moesif API event for a taxonomy feature: successful or failed, with the status the caller received. A generic middleware does the reporting, so adding a feature never touches a handler, the middleware or the reporter.

Exactly two files change:

| File | What it holds |
|---|---|
| `platform-api/internal/featureusage/taxonomy/api-platform-taxonomy.yaml` | The feature: code, name, description, dimensions and their enums. It is embedded in the binary and is a copy of the product's reviewed taxonomy (WSO2 feature-taxonomy schema `https://wso2.com/schemas/feature-taxonomy/v1`). Keep it schema-clean: no Platform-API-specific keys. |
| `platform-api/internal/featureusage/bindings.go` | Which OpenAPI `operationId` reports which feature, and where each dimension's value comes from. It is the only Go file that names feature codes. |

Tests in `bindings_test.go` (package `featureusage`) and `internal/server/feature_usage_routes_test.go` check the two files against each other, against `platform-api/resources/openapi.yaml` and against the routes the real handlers register.

## Rules (never break these)

1. **Never invent or rename a feature code.** Codes come from the product taxonomy. If the code you need is not there, the taxonomy owner adds it first. Never change a code to make something compile or pass.
2. **Don't remove a released code or enum value.** Moesif history depends on them. Retire a feature with `status: "deprecated"`, and keep its binding until the endpoint is gone.
3. **IDs only, never free text.** A dimension read from the request must be an `enum` or a `boolean`. The engine refuses a request-derived `string` dimension. Never read a name, description, URL, email, handle or token. To record *which* of two inputs was used (file or URL), use `Present`, never the value.
4. **Actions only.** Bind writes (`POST`, `PUT`, `PATCH`, `DELETE`), never a `GET`.
5. **`outcome` is never bound.** It comes from the response status (2xx is `success`, anything else is `failure`). Every Platform API feature declares it.
6. **Platform API features only.** A binding may only name a feature whose description ends with `Emitted by Platform API.`. Console clicks (`-click` codes), gateway, CLI and portal features are reported by those components.

## Inputs to collect

- The feature code, name, category (the second code segment) and tier.
- The REST API operation(s) that perform it: their `operationId`s in `platform-api/resources/openapi.yaml`.
- Each dimension: name, type (`enum` or `boolean`), enum values, whether it is required, and where its value comes from for each operation.

If any of these are missing, ask. Don't guess an enum or a code.

## Step 1: Add the feature to the taxonomy

Add the entry next to its siblings in `api-platform-taxonomy.yaml`. Follow this shape (copied from `apip.manage-apis.api-thumbnail-upload`):

```yaml
  - code: "apip.<category>.<feature>"
    name: "<Human Name>"
    description: "<What the call does> (<METHOD> <path>, <operationId>). Emitted by Platform API."
    category: "<category>"
    tier: "core"
    status: "stable"
    availability:
      introduced_in: "<product version>"
    tracking:
      event_name: "apip.<category>.<feature>"     # must equal code
      event_category: "lifecycle"                 # or adoption / engagement, as the taxonomy owner decides
      platforms: ["api"]
      dimensions:
        - name: "api_type"
          type: "enum"
          values: ["rest-api", "graphql-api"]
          required: true
        - name: "outcome"
          type: "enum"
          values: ["success", "failure"]
          required: true
          description: "success for a 2xx response, failure otherwise."
      aggregations:
        - type: "count"
          label: "<Human Name>"
        - type: "count_distinct"
          label: "Outcomes"
          dimension: "outcome"
```

- The description must end with exactly `Emitted by Platform API.`. The coverage test uses that marker.
- Dimension names are `snake_case`. Enum values are lowercase codes (`a-z0-9` joined by `-` or `_`). Reserved metadata keys (`growth_action`, `platform`, `source`, `environment`, `action`, `category`, `tier`, `event_category`, `initiating_product`) can't be dimension names.
- **Required or optional:** a value read from the request body is unknown when a call is rejected before its body is parsed. If such a dimension is `required: true`, those failed calls are dropped (logged as `REPORT_DROPPED`, naming the missing dimension). Make body-derived dimensions `required: false` unless every call carries them. Path and fixed (`Set`) dimensions are always known.
- Remind the user to apply the same change to the product's master copy of the taxonomy. The embedded file must stay a byte-for-byte copy of the reviewed taxonomy.

## Step 2: Bind the operation(s) in `bindings.go`

Add one row per operation to `bindings`, in the section for its category:

```go
{Op: "<operationId>", Feature: "apip.<category>.<feature>", Set: Dims{...}, From: Sources{...}},
```

| Where the value comes from | Use | Example |
|---|---|---|
| Fixed for this operation | `Set: Dims{"name": value}` | `UpdateGraphQLAPI`: `Set: Dims{"api_type": "graphql-api"}` |
| A path parameter of the route | `Path("param")` | `{apiType}`: `Path("apiType").Map(apiTypes)` |
| A multipart or urlencoded form field the handler parses | `Form("field")` | document `type`: `Form("type").Map(documentTypes)` |
| JSON inside a form field | `FormJSON("field", "/pointer")` | GraphQL `metadata`: `FormJSON("metadata", "/schemaSource")` |
| A field of the JSON request body | `JSON("/pointer")` | gateway: `JSON("/functionalityType")` |
| Which input was supplied, never its content | `Present("a", "b")` | OpenAPI validation: `Present("file", "url")` |

Refine any source with:

- `.Map(map[string]string{...})` translates the request's spelling to the taxonomy's (`"ACTIVE"` becomes `"active"`). Keys match regardless of case. A value not in the map is left out. Reuse the shared maps (`apiTypes`, `portalAPITypes`, `documentTypes`) where they fit. A path value can have aliases: `{apiType}` accepts both `rest-api` and `RestApi`.
- `.Default(v)` is the value when the request was parsed but omits the field. Use it only for what the API itself assumes when the field is omitted, such as `functionalityType` defaulting to `regular`. It is not used when the request could not be parsed at all.

Guidance:

- A request value is sent only if it is in the dimension's enum (or a bool, for a boolean dimension). Anything else is left out, so an unexpected value can never leak.
- A raw value that already matches the enum needs no `Map` (for example `status` `active`/`disabled` on `updateServiceAccount`).
- `Form`, `FormJSON` and `Present` read the form **the handler parsed**. They work only on routes whose handler calls `ParseMultipartForm`/`ParseForm` on the request it is given. `JSON` reads a copy of the body as the handler reads it, capped at 64 KiB.
- `Path` uses the parameter name **the route registers** (check the handler's `RegisterRoutes`), which can differ from the OpenAPI document.
- An operation is bound at most once. One feature can have several operations (REST and GraphQL variants).
- If a Platform API feature deliberately is not reported (for example the call carries no organization, like `POST /service-accounts/token`), add it to `notBound` with the reason instead of a binding.
- Work that does an operation's job outside its route, such as an MCP tool, reports through `featureusage.RecordOperation(rep, "<operationId>", org, actor, method, route, status, started)`. Only the binding's `Set` dimensions apply there.

If the feature needs a value no source can express, don't work around it in a handler. Raise it: a new source kind is added once to `binding.go`, with tests.

## Step 3: Run the checks

From `platform-api/`:

```bash
go test ./internal/featureusage/... && \
go test ./internal/server/ -run TestFeatureUsageBindingsResolveToRealRoutes && \
go test ./internal/handler/ -run 'Reports|IsReported'
```

| Failing test | Meaning | Fix |
|---|---|---|
| `TestEveryPlatformAPIFeatureIsBound` | A feature marked `Emitted by Platform API.` has no binding, or a `notBound` entry is stale | Add the binding, or a `notBound` entry with a reason |
| `TestBindingsAgreeWithTheTaxonomy` | Unknown code, undeclared dimension, a value outside an enum, a required dimension with no source, `outcome` bound, a string dimension read from the request | Fix the row, or the taxonomy if the row is right and the taxonomy owner agrees |
| `TestBindingsAgreeWithTheOpenAPIDocument` | `operationId` not in `resources/openapi.yaml`, a `GET` bound, or a `Path` parameter the path lacks | Use the exact `operationId`; check the path |
| `TestFeatureUsageBindingsResolveToRealRoutes` | The real handlers don't serve the operation under its OpenAPI path, or don't register the path parameter name | Register the route, or fix the binding's parameter name. If the handler is new, add its `RegisterRoutes` call to that test |
| `TestDocumentTypesCoverTheTaxonomy` | A `document_type` enum value has no mapping | Extend `documentTypes` |

Finish with `go vet ./...` and `gofmt -l internal/featureusage`.

## Step 4: Report back

Tell the user:

- the feature code(s) added or changed, and the operations bound to them;
- each dimension and its source;
- any dimension made optional because failed calls may not carry it;
- that the product's master taxonomy needs the same change, if you edited the embedded copy;
- the test results.

Don't commit unless asked.
