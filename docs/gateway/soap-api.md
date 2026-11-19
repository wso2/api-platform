# SOAP API Support

The gateway can front SOAP (SOAP-over-HTTP) backends as a **passthrough proxy**. A
`SoapApi` artifact exposes a single service endpoint at the API context path; the gateway
routes traffic to the backend unchanged and applies the usual HTTP-transport policies
(authentication, rate limiting, analytics, …).

> **Scope (current):** SOAP is proxied as one catch-all resource. There is **no
> operation-aware routing** (no per-`SOAPAction` dispatch) and **no message
> transformation**. These are possible future additions; see [Non-goals](#non-goals). The
> one thing the gateway does change is the service description: the endpoint addresses in a
> served WSDL point at the gateway (see [WSDL rewriting](#wsdl-rewriting)).

> **Maintainers:** for the design rationale and a file-by-file implementation walkthrough,
> see [soap-api-implementation.md](soap-api-implementation.md).

## Contents

- [Resource model](#resource-model)
- [How routing works](#how-routing-works)
- [WSDL rewriting](#wsdl-rewriting)
- [Policies](#policies)
- [Error responses](#error-responses)
- [API keys](#api-keys)
- [Managing SOAP APIs](#managing-soap-apis)
- [Non-goals](#non-goals)
- [Where it lives in the code](#where-it-lives-in-the-code)

## Resource model

A SOAP API is declared with `kind: SoapApi` on apiVersion
`gateway.api-platform.wso2.com/v1`. The spec mirrors the REST API spec minus the
`operations` list (SOAP has none), `upstreamDefinitions` and `resilience`.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: SoapApi
metadata:
  name: stockquote-api-v1.0
spec:
  displayName: StockQuote API
  version: v1.0
  # Base path on the gateway. Use $version to embed the version.
  # /stockquote/$version → /stockquote/v1.0
  context: /stockquote/$version
  upstream:
    main:
      url: http://stockquote-service:8080/services/StockQuoteService
    # sandbox:            # optional second upstream
    #   url: http://stockquote-sandbox:8080/services/StockQuoteService
  # vhosts:               # optional custom virtual hosts
  #   main: api.example.com
  # subscriptionPlans:    # optional subscription plan names
  #   - Gold
  # API-level policies apply to all SOAP traffic (see "Policies" below).
  # policies:
  #   - name: api-key-auth
  #     version: v1
  #     params:
  #       key: X-API-Key
  #       in: header
```

| Field | Required | Notes |
|-------|----------|-------|
| `displayName` | yes | URL-friendly name (letters, numbers, spaces, `-`, `_`, `.`), 1–100 chars. |
| `version` | yes | Semantic version, e.g. `v1.0`. |
| `context` | yes | Base path, must start with `/` and have no trailing slash. `$version` is substituted. |
| `upstream.main` | yes | Backend SOAP service endpoint. Only a direct `url` is supported (there is no `upstreamDefinitions` list for a `ref` to point at). |
| `upstream.sandbox` | no | Optional sandbox backend (direct `url`), served on the sandbox vhost. |
| `vhosts` | no | Custom virtual hosts for `main` / `sandbox` traffic. |
| `subscriptionPlans` | no | Subscription plan names available for this API. |
| `policies` | no | API-level policies applied to all SOAP traffic. |
| `faultPolicies` | no | Policies run only when a request fails, over the error response (see [Error responses](#error-responses)). |
| `deploymentState` | no | `deployed` (default) or `undeployed`. An undeployed API keeps its configuration but its routes are withdrawn; update it with `deployed` to serve it again. |

A runnable sample lives at [`gateway/examples/soap-api.yaml`](../../gateway/examples/soap-api.yaml).

## How routing works

A SOAP service exposes a single endpoint, so the gateway generates exactly **two routes**
over **one upstream cluster** for the API context (and the same pair on the sandbox vhost
when a sandbox upstream is configured):

| Method | Purpose |
|--------|---------|
| `POST <context>` | SOAP invocations (`text/xml` or `application/soap+xml` envelopes). |
| `GET <context>` | WSDL/XSD retrieval — `GET <context>?wsdl` / `?xsd`. A WSDL's endpoint addresses are rewritten to the gateway's. |

Both routes use the wildcard operation path `/*`, which matches the context and the whole
subtree under it. The path after the context is carried over to the backend
(`^<context>(.*)$` → `<upstream path>\1`), so `POST <context>/soap11` reaches
`<upstream path>/soap11`. The query string (`?wsdl`) is preserved on passthrough. No
routing decision is made from the SOAP body or the `SOAPAction` header.

```
Client ──POST /stockquote/v1.0 (SOAP envelope)──▶ Router ──▶ StockQuote backend
Client ──GET  /stockquote/v1.0?wsdl ────────────▶ Router ──▶ StockQuote backend
```

The `SoapApi` transformer (`pkg/transform/soapapi.go`) produces one `RuntimeDeployConfig`
(routes + policy chains). The Envoy routes and clusters and the policy engine's chains are
both built from it, so their route names always agree.

## WSDL rewriting

A WSDL fetched from the backend names the backend: its `soap:address` is where a client
generated from it (SoapUI, `wsimport`, `svcutil`) will send requests. So that clients call
the gateway — and pass through its policies — the gateway points the endpoint addresses of
every service description it serves on the `GET` route at itself, using the URL the client
reached the gateway on. This is always on for a SOAP API and needs no configuration, and it
matches what WSO2 API Manager does to the WSDLs it serves.

A response is rewritten when it is a `2xx` whose root element is a WSDL 1.1 `definitions` or
a WSDL 2.0 `description`. Every endpoint address in it — `soap:address`, `soap12:address`
and `http:address` `location`, and WSDL 2.0 `endpoint` `address` — is replaced, whatever host
it names:

| The backend's address | Becomes |
|---|---|
| The upstream URL's path or below it | the API's context followed by the part below the upstream's path, and the query |
| Anything else (another path or host, a relative value, a placeholder) | the API's context |

With `context: /calc/$version`, upstream `http://calc-backend:3002/calculator/v1`, and a
client calling `https://soap.example.com/calc/v1.0?wsdl`:

```xml
<!-- from the backend -->
<soap:address location="http://localhost:3002/calculator/v1/soap11"/>
<soap12:address location="http://localhost:3002/calculator/v1/soap12"/>
<!-- to the client -->
<soap:address location="https://soap.example.com/calc/v1.0/soap11"/>
<soap12:address location="https://soap.example.com/calc/v1.0/soap12"/>
```

The host is the API's virtual host (the port is the one the request arrived on, left off when
it is the scheme's default); for a wildcard virtual host it is the request's own host.

Imports and includes (`wsdl:import`, `xsd:import` `schemaLocation`, …) are left as the backend
wrote them, as is every other byte of the document. A relative import resolves against the
URL the client fetched, so it is served through the gateway; an absolute one names wherever
the backend put it. Any other response — an error status, an empty body, a document whose
root is not a service description (an XSD, an HTML page) — is forwarded untouched. A
rewritten response has its `ETag`/`Last-Modified` removed and carries
`Cache-Control: no-store`, since its addresses depend on how the client reached the gateway.

A service description the gateway cannot rewrite safely — carrying a `DOCTYPE`, nested more
than 256 levels, malformed, or arriving on a request with no usable host — fails with `502`
(a SOAP Fault, see [Error responses](#error-responses)) rather than being served with the
backend's addresses in it. So does a document the gateway cannot read far enough to tell
whether it is one: UTF-16, a declared encoding other than UTF-8, US-ASCII, ISO-8859-x or
Windows-125x, or a document whose root element cannot be read because of a `DOCTYPE` (an
entity in one of its attributes, say). A description in ISO-8859-x or Windows-125x is
rewritten and served in the encoding it declares.

A description larger than the router's buffer limit
(`[router.http_listener] per_connection_buffer_limit_bytes`, 1 MiB by default) is refused
by the router before it can be rewritten: a `500` with Envoy's plain-text body
(`response_payload_too_large` in the access log), which is a router failure and so becomes a
SOAP Fault only through a fault policy with `handle_upstream_faults` on. Raise that limit for
larger WSDLs; the rewrite's own ceiling follows it.

The rewrite is done by the internal `wso2_apip_sys_soap` system policy, which the controller
places last in the `GET` route's chain, so it runs first in the response phase and every
response policy you attach sees the gateway's URLs.

## Policies

SOAP APIs reuse the REST policy machinery, so any **HTTP-transport** policy works unchanged:

- **Authentication:** `api-key-auth`, `basic-auth`, `jwt-auth`, etc. (SOAP-over-HTTP is
  authenticated at the HTTP layer.)
- **Rate limiting / throttling.**
- **Analytics** and other system policies are injected into the chain just as they are for
  REST.

Policies are declared under `spec.policies` and apply to all traffic on the API (both the
POST and GET routes) — so WSDL retrieval is protected by the same authentication as
invocations. A policy reference that cannot be resolved fails the deployment rather than
being dropped from the chain. `mtls-auth` is REST-only and is rejected.

## Error responses

A SOAP client expects a **SOAP Fault**. When a policy rejects a request and describes the
failure (its `Fault` — code, type, message), the gateway renders it as a Fault in the SOAP
version the request used:

| Request `Content-Type` | Fault | Response `Content-Type` |
|---|---|---|
| `application/soap+xml` | SOAP 1.2 (`env:Sender` for 4xx, `env:Receiver` otherwise) | `application/soap+xml; charset=utf-8` |
| anything else (`text/xml`, or none on `GET ?wsdl`) | SOAP 1.1 (`soapenv:Client` for 4xx, `soapenv:Server` otherwise) | `text/xml; charset=utf-8` |

The message becomes `faultstring` / `env:Reason`. The gateway's error code, type and error id go
in the fault detail as an `error` element in the `urn:wso2:api-platform:gateway:fault`
namespace:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <soapenv:Fault>
      <faultcode>soapenv:Client</faultcode>
      <faultstring>Invalid or expired credentials.</faultstring>
      <detail>
        <error xmlns="urn:wso2:api-platform:gateway:fault"><code>900901</code><type>authentication</type></error>
      </detail>
    </soapenv:Fault>
  </soapenv:Body>
</soapenv:Envelope>
```

The HTTP status is kept as the policy set it (e.g. `401`, `429`), not collapsed to the `500`
SOAP 1.1's HTTP binding uses, so analytics and client retry logic still see it.

What is **not** turned into a Fault by the gateway:

- **Backend faults** — proxied unchanged, with the backend's status.
- **A policy that writes its own body without describing the failure** — its body stands. A
  policy that has not adopted the fault contract yet behaves this way, so its error still
  arrives in its own (usually JSON) format.
- **Router failures** (no route, no healthy upstream, timeout) — Envoy's own reply stands.

`faultPolicies` covers those: with `[policy_engine.fault_policies] handle_upstream_faults = true`
a fault policy also sees router and upstream failures, and the `error-response-formatter`
policy's `soap11` / `soap12` presets shape any failure as a Fault:

```yaml
spec:
  faultPolicies:
    - name: error-response-formatter
      version: v1
      params:
        preset: soap11
```

See [fault-policies.md](fault-policies.md) for how fault policies run.

## API keys

SOAP APIs support the same API-key lifecycle as other API types (generate, list,
regenerate, update, revoke). Keys are validated by the `api-key-auth` policy when it is
applied to the API. See the endpoints under
[SOAP API Management](../rest-apis/gateway/soap-api-management.md).

## Managing SOAP APIs

SOAP APIs are managed through the gateway-controller REST API under `/soap-apis`
(base URL `http://localhost:9090/api/management/v1`). The full reference — every endpoint,
request/response schema, and curl example — is generated from the OpenAPI spec:

➡️ **[SOAP API Management reference](../rest-apis/gateway/soap-api-management.md)**

Deploy the sample:

```bash
curl -X POST http://localhost:9090/api/management/v1/soap-apis \
  -u admin:admin \
  -H 'Content-Type: application/yaml' \
  --data-binary @gateway/examples/soap-api.yaml
```

[`soap-api.postman_collection.json`](soap-api.postman_collection.json) is a demo and test
collection against the calculator SOAP backend: management, SOAP 1.1/1.2 passthrough, WSDL
rewriting, API-key auth failures as SOAP Faults, and fault policies. Every request carries
assertions, so it also runs headless:

```bash
postman collection run docs/gateway/soap-api.postman_collection.json
```

Its prerequisites (gateway policies, `handle_upstream_faults`, where the backend runs) are in
the collection description.

## Non-goals

The following are intentionally **not** part of the current passthrough support. They are
possible future work:

- **Operation-aware routing** — dispatching to different backends/policies per SOAP
  operation or `SOAPAction`.
- **Message transformation** — SOAP↔REST mediation, envelope rewriting, or schema
  validation.

## Where it lives in the code

| Concern | Location |
|---------|----------|
| Management API spec | `gateway/gateway-controller/api/management-openapi.yaml` (SOAP paths + `SoapAPI*` schemas) |
| HTTP handlers (CRUD + api-keys) | `gateway/gateway-controller/pkg/api/handlers/soap_api_handler.go` |
| Deploy pipeline (parse/validate `SoapApi`, vhost sentinels) | `gateway/gateway-controller/pkg/utils/soap_deployment.go` (self-registered into `api_deployment.go`) |
| Policy validation | `gateway/gateway-controller/pkg/config/policy_validator.go` (`ValidateSoapAPIPolicies`) |
| Routing (Envoy routes + policy and fault chains) | `gateway/gateway-controller/pkg/transform/soapapi.go` |
| SOAP Fault rendering | `gateway/gateway-runtime/policy-engine/internal/faultformat` (`soap11Renderer`, `soap12Renderer`) |
| WSDL rewriting | `gateway/system-policies/soap` (`wso2_apip_sys_soap`), attached by `pkg/transform/soapapi.go` |
| Storage (`SoapApi` kind + table) | `gateway/gateway-controller/pkg/storage/`, `pkg/models/stored_config.go` |
| Sample artifact | `gateway/examples/soap-api.yaml` |
