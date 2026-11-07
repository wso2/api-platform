# SOAP API Support — Implementation Notes

Audience: reviewers and maintainers. This explains **what** the SOAP API change adds, the
**design** it implements, and **why** each piece looks the way it does. For usage, see
[soap-api.md](soap-api.md) and the generated
[SOAP API Management reference](../rest-apis/gateway/soap-api-management.md).

## 1. What it adds, in one paragraph

A new artifact kind, `SoapApi`, lets the gateway front a SOAP-over-HTTP backend as a
**passthrough proxy**. A SOAP API is modelled as a **single wildcard resource** at the API
context — a `POST` route for invocations and a `GET` route for `?wsdl`/`?xsd` over one
upstream cluster (plus the same pair on the sandbox vhost when configured) — proxied
unchanged to the backend. It is wired into the controller the same way as `GraphQLApi`:
the shared deployment, api-key and policy machinery, with the kind's own parser, validator
and transformer registered through the existing extension points.

## 2. Scope and non-goals

**In scope:** CRUD and API-key management for `SoapApi`; wildcard POST/GET routing with
`main` and optional `sandbox` upstreams and vhosts; HTTP-transport policies (auth, rate
limiting, analytics) through the existing policy pipeline; API-level `faultPolicies`; SOAP
1.1/1.2 Fault rendering of policy-described failures (policy engine `internal/faultformat`);
WSDL endpoint-address rewriting to the gateway's URL (system policy `wso2_apip_sys_soap`).

**Not in this change:**

- **Operation-aware routing** — no per-operation / per-`SOAPAction` dispatch.
- **Message transformation** — no SOAP↔REST mediation, envelope rewriting or schema
  validation.
- **`resilience`** on the spec, and control-plane (platform-api) support for the kind. The
  gateway side is in place — reconnect sync deploys and removes `SoapApi` artifacts
  (`pkg/controlplane/sync.go`) and API keys are bulk-synced from `/soap-apis/api-keys` — but
  platform-api has no `SoapApi` endpoints yet, so until it does the key sync for the kind logs a
  warning on connect and changes nothing.

## 3. Design decisions

### 3.1 Single wildcard resource

A SOAP service exposes **one HTTP endpoint**; the operation is in the body / `SOAPAction`
header, not the URL. Routing per operation would need body parsing, a dispatch policy and
operation metadata in the artifact. Instead the operation path is the wildcard `/*`
(`transform.SoapWildcardPath`), giving one `POST` and one `GET` route under the context.
The translator's wildcard handling matches `^<context>(?:/.*)?$` and rewrites
`^<context>(.*)$` → `<upstream path>\1`, so sub-paths and the query string reach the
backend unchanged.

### 3.2 One producer for both planes

`SoapAPITransformer` builds a `RuntimeDeployConfig`. The policy manager turns it into the
policy engine's chains, and the Envoy translator builds routes and clusters from the same
config (`translateRuntimeConfig`), so route names (`METHOD|PATH|VHOST`, resolver
`route-key`) cannot drift between the two. `SoapApi` is in the registry's `registryKinds`,
so `EnvoyTranslatorKinds()` wires it into the translator automatically. (An earlier
iteration also had a legacy `translateSoapAPIConfig` in `pkg/xds`; it is gone.)

### 3.3 Registered kind, not core switch cases

`pkg/utils/soap_deployment.go` registers the deploy parser, config validator and
vhost-sentinel resolver via `RegisterKindDeployParser`, `RegisterKindConfigValidator` and
`RegisterKindVhostSentinelResolver`, as `graphql_deployment.go` does. Spec rules
(displayName, version, context) are reused from `config.APIValidator`; the upstream must
be a direct http(s) `url` (the shared `validateDirectUpstream`), since the kind has no
`upstreamDefinitions` for a `ref`.

### 3.4 SOAP Faults from the engine's formatter

`SoapApi` is in `faultformat.supportedKinds` (alongside `Agent`): a SOAP client cannot read
the plain JSON an ordinary policy writes. `Negotiate` treats the kind as protocol-mandating —
`application/soap+xml` → `soap12`, anything else → `soap11`, Accept
ignored — and the two renderers build the envelope by hand (escaping all text) with the
gateway's code/type/error id in a namespaced detail entry. The formatter's existing rules are
unchanged: only a policy-*described* failure is rendered, a policy-authored body stands, and
the status is never changed. The route kind is the SDK's `policy.APIKindSoapApi`.

The fault chain of both routes is `spec.faultPolicies` followed by the fault-path system
policies (the analytics collector), built as for a RestApi operation.

### 3.5 WSDL rewriting as a per-route system policy

Modelled on the A2A Agent Card rewrite (`system-policies/a2a`): the transformer appends a
`wso2_apip_sys_soap` instance to each GET route's chain with a `wsdlRewrite` block —
`gatewayPath` (the resolved context), `upstreamUrl` (that route's upstream) and `vhost` —
because the policy engine never sees the route arithmetic. Only that instance buffers its
response (`Mode()`), so POST traffic is never buffered for it. Placed last, it runs first in
the response phase.

It rewrites the endpoint addresses only (`soap`/`soap12`/`http:address`, WSDL 2.0
`endpoint`), which is what a generated client calls and what WSO2 API Manager rewrites
(`WSDL11ProcessorImpl.setAddressUrl`). Like API-M, every address is replaced whatever host it
names — an address left as it was would send clients around the gateway. One addition: an
address below the upstream's base path keeps the part below it, because a SoapApi's
wildcard routes carry sub-paths (the calculator backend serves `/soap11` and `/soap12`);
anything else becomes the context, as in API-M. Imports and includes are not touched.

The rewrite edits the original bytes in place at the attribute values it matched (found via
`xml.Decoder` offsets and a quote-aware walk of the start tag), instead of re-encoding: Go's
encoder rewrites namespace prefixes. The gateway origin is the downstream snapshot's scheme
plus the A2A authority rule (vhost host, request port, wildcard → request authority).

The root element is read before anything else, so a GET returning some other document (an
XSD, an HTML page with a DOCTYPE) is forwarded untouched. For a WSDL, parsing follows
`xxe-xml-processing.md`: a size ceiling (`maxResponseBytes`, which the controller sets to the
router's `per_connection_buffer_limit_bytes` — the router refuses a larger response before the
policy sees it), DOCTYPE rejected, depth 256, 2 s deadline. A description it cannot rewrite
safely fails with a described 502 — rendered as a SOAP Fault — rather than leaking the
backend address; so does a document it cannot read far enough to classify (UTF-16, an
encoding other than UTF-8/US-ASCII/ISO-8859-x/Windows-125x, or one whose prolog runs past
the ceiling). A root element `encoding/xml` cannot decode (an entity from the DOCTYPE in an
attribute, which an entity-aware client still reads) is classified by its raw name:
`definitions`/`description` is refused, anything else passes, and with a DOCTYPE an
unreadable name is refused. A missing policy definition fails the deployment, as for the A2A
system policy.

`encoding/xml` reads only UTF-8, so an ISO-8859-x or Windows-125x description is decoded
(`golang.org/x/text/encoding/charmap`) before parsing, rewritten as UTF-8 — keeping the
offsets and the bytes they index the same — and encoded back to its declared code page. A
byte the code page leaves undefined cannot be written back and fails the rewrite.

An upstream path ending in `/` (`http://backend/calculator/v1/`) is matched against
addresses without it, and the translator gives a SoapApi with such an upstream the rewrite
`^<context>/?(.*)$` → `<base>/\1`, so `/calc/v1.0/soap11` reaches `/calculator/v1/soap11`
(not `//soap11`) and the context itself still reaches `<base>/`.

### 3.6 Policies fail closed

The API-level chain is a SOAP route's whole chain, so a policy that cannot be resolved
fails the deployment (as for `GraphQLApi`) instead of being dropped, which would silently
deploy weaker enforcement. Policies apply to both routes, so `?wsdl` is protected by the
same auth. `mtls-auth` is rejected (REST-only).

### 3.7 Deployment state and rendered policy params

`spec.deploymentState` reaches the generic deploy path through
`APIDeploymentParams.DesiredState`, set by the SOAP handlers on create and update; an
undeployed SoapApi keeps its row and its routes are withdrawn by the translator. Like a
RestApi's, a SoapApi's policy params are stored as written and rendered on consumption, which
leaves templated numbers and booleans as strings; `CoerceSoapAPIPolicies` restores their
schema types in `policies` and `faultPolicies`, in the event listener and in the startup
load.

### 3.8 Storage

`soap_apis` is a new table created with `CREATE TABLE IF NOT EXISTS`. The SQLite schema stays
at version 6: `initSchema` runs the whole schema script against a version 6 database and after
migrating a version 5 one, so a database from a release without SOAP support gains the table on
startup. The PostgreSQL and SQL Server scripts create it behind their usual guards, so re-applying them
brings an existing database up to date.

## 4. Component walkthrough

| Area | Location |
|---|---|
| Spec + generated types | `api/management-openapi.yaml` (`/soap-apis*`, `SoapAPIRequest` / `SoapAPI` / `SoapAPIData`), `pkg/api/management/generated.go` |
| Handlers | `pkg/api/handlers/soap_api_handler.go` (CRUD + api-keys on `*APIServer`, mirroring `graphql_api_handler.go`); list/search envelope `soapApis` in `handlers.go`; `resource_response.go` |
| Deployment + validation | `pkg/utils/soap_deployment.go`; `pkg/config/policy_validator.go` (`ValidateSoapAPIPolicies`) |
| Transformer | `pkg/transform/soapapi.go` (routes, policy/fault chains, WSDL-rewrite attachment); `registry.go` (`NewRegistry` takes the SOAP transformer) |
| WSDL rewrite | `gateway/system-policies/soap` (`soap.go` policy, `wsdl.go` rewrite); registered in `system-build-lock.yaml` and `go.work`; name/params in `pkg/constants` |
| Fault rendering | policy engine `internal/faultformat` (`soap11Renderer`, `soap12Renderer`) |
| Model + storage | `pkg/models/stored_config.go` (`KindSoapApi` + accessors), `data_version.go`; `soap_apis` table in the sqlite/postgres/sqlserver schemas (created on startup in an existing SQLite database); `sql_store.go` kind→table, rehydration, unmarshalling |
| Param coercion | `pkg/config/policy_validator.go` (`CoerceSoapAPIPolicies`), called from `pkg/eventlistener/api_processor.go` and `cmd/controller/runtime_bootstrap.go` |
| Control-plane sync | `pkg/controlplane/sync.go` (fetch and orphan deletion through the generic API path, as for `GraphQLApi`), `client.go` (`apiKeyBulkSyncKinds`), `pkg/utils/api_utils.go` (`FetchAPIKeysByKind` → `/soap-apis/api-keys`) |
| Wiring | `cmd/controller/main.go` (transformer, `SetSoapPolicyValidator`, `/soap-apis` role map), `runtime_bootstrap.go`; event-gateway-controller wires the transformer too, since it serves the shared management API |

## 5. Key flows

**Deploy** `POST /soap-apis` → `DeployAPIConfiguration(Kind: SoapApi)` → registered parser
→ vhost sentinels resolved → validator → persisted (`artifacts` + `soap_apis`) → event →
transformer → Envoy routes/clusters and policy chains.

**Invoke** `POST <context>/...` or `GET <context>?wsdl` → wildcard route → policies →
backend. On GET, a successful service description is rewritten on its way back.

## 6. Tests

- `pkg/transform/soapapi_test.go` — routes, vhosts, sandbox, policy chain on both routes,
  unresolvable policy fails, wrong kind.
- `pkg/transform/soapapi_routing_test.go` — end to end through the translator: the Envoy
  route matches the context subtree and rewrites onto the upstream path (path and root
  upstream).
- `pkg/utils/soap_deployment_test.go` — validator rules (incl. `ref` rejection) and vhost
  sentinel resolution.
- `pkg/transform/soapapi_test.go` also covers fault chains and the WSDL-rewrite attachment
  (GET only, last, per-vhost upstream, missing definition fails).
- `system-policies/soap/soap_test.go` — every address replaced (sub-path kept, anything
  else → context, trailing-slash upstreams), imports untouched, byte-for-byte preservation,
  WSDL 2.0, escaping, look-alike attributes, encodings (including non-ASCII text written
  back in its code page), untouched non-WSDL responses, and every fail-closed case
  (including a DOCTYPE entity in the root).
- `pkg/transform/soapapi_routing_test.go` also covers the trailing-slash upstream rewrite.
- `pkg/storage/sqlite_test.go` — a version 6 database is migrated and its artifacts load.
- `pkg/utils/soap_deployment_test.go` — `DesiredState` is stored and a later deploy restores it.
- `pkg/eventlistener/api_processor_test.go`, `cmd/controller/runtime_bootstrap_test.go` —
  templated params reach the runtime with their schema types.
- Policy engine `faultformat/soap_renderer_test.go` and `kernel/fault_format_soap_test.go`
  — Fault shapes, codes, escaping, negotiation and the kernel path.
- Storage/schema, registry dispatch and the role map are covered by the existing
  table-list, `TestRegistryKindsMatchDispatch` and auth-config tests.
- Integration tests in `tests/framework` (`gateway-core` block, source builds only), against
  the testbench `soap-backend` (port 3027):
  - `gateway/soap/deploy.feature`: create, update, labels, validation, list and get; SOAP 1.1 and 1.2
    invocation; backend Faults passed through; the trailing-slash upstream path;
    `deploymentState`.
  - `gateway/soap/wsdl.feature`: addresses rewritten to the requested authority, an ISO-8859-1 WSDL,
    a WSDL with a DOCTYPE refused with 502, and a non-WSDL GET passed through.
  - `gateway/soap/policies.feature`: `api-key-auth` and `basic-ratelimit` rejections as SOAP 1.1 and
    1.2 Faults, on the WSDL route too, with a templated rate-limit parameter.

## 7. Known limitations / follow-ups

- Router failures and not-yet-migrated policies are not rendered as Faults by default; a
  `faultPolicies` entry (e.g. `error-response-formatter` preset `soap11`) covers them.
- `policy.APIKindSoapApi` is new in sdk/core: the policy engine and the analytics system
  policy build against it through the workspace until their `go.mod` moves to the SDK release
  that adds it.
- WSDL rewriting has no opt-out.
- A WSDL above the router buffer limit (1 MiB default) gets the router's plain-text 500, not
  a SOAP Fault, unless a fault policy handles router failures.
- Imports and includes are not rewritten (as in WSO2 API Manager): an absolute one keeps
  naming the backend, so a client that cannot reach the backend cannot fetch it. Relative
  ones resolve through the gateway.
- The advertised scheme is the one the request reached Envoy on; behind a load balancer that
  terminates TLS it is `http` (the A2A Agent Card rewrite shares this).
