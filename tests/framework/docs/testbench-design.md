# Testbench — one collaborative mock server

## The problem

The legacy suite has **eight separate mock servers**: seven Go modules and one Node app,
1774 lines, each with its own `go.mod`, `Dockerfile` and image.

| mock | lines |
|---|---|
| mock-platform-api | 596 |
| mock-embedding-provider | 311 |
| mock-aws-bedrock-guardrail | 283 |
| mock-analytics-collector | 209 |
| mock-interceptor-service | 138 |
| mock-azure-content-safety | 124 |
| mock-jwks | 113 |
| mcp-everything-streamable | Node |

They were written independently by different people. There is no shared code, no common
helpers, and no place for a test developer to extend an existing mock — so the path of least
resistance for a new feature is to write a *ninth* server. That is how eight happened.

Migrating them as-is, which is what this framework did at first, ports the fragmentation
forward and makes it permanent.

## What product-apim does instead

One `node-app-server`: source in the repo, rebuilt with the product, hosting ~21 apps on ports
3000-3020, run as a **single shared container**. A test developer needing a new backend adds an
app to that codebase and gets a port. Features reuse each other's apps rather than each
shipping a bespoke express server.

The collaboration point is the thing that matters, not the container count.

## Design

**One Go module, one binary, one image, one shared container: `testbench`.**

Located inside the new integration code — `tests/framework/testbench/` — so it versions and
reviews with the framework rather than beside the products.

### Services are packages, not repositories

```go
// Service is one mock the testbench hosts.
type Service interface {
    Name() string          // "jwks", "bedrock", "echo"
    Port() int             // its own port; see addressing
    Handler() http.Handler
}
```

A developer adds a file, implements three methods, registers it in one table. They get shared
helpers (JSON responses, request capture, TLS, deterministic signing) that today each mock
reimplements. No new module, no new Dockerfile, no new image, no compose entry.

### Addressing: ONE alias, one port per service

The container has a single network alias, `testbench`, and each service listens on its own
port:

| port | service | replaces |
|---|---|---|
| 3000 | `backend` | sample-service (the generic upstream) |
| 3001 | `jwks` | mock-jwks |
| 3002 | `echo` | echo-backend (go-httpbin) |
| 3003 | `interceptor` | mock-interceptor-service |
| 3004 | `embeddings` | mock-embedding-provider |
| 3005 | `content-safety` | mock-azure-content-safety |
| 3006 | `bedrock` | mock-aws-bedrock-guardrail |
| 3007 | `analytics` | mock-analytics-collector |
| 3008 | `openai` | mock-openapi (the prism LLM upstream) |
| 3009 | `mcp` | mcp-streamable-backend (mcp-everything-streamable) |

```
http://testbench:3001/jwks
http://testbench:3002/anything
```

This is product-apim's model (`nodebackend` on 3000-3020) and it is the right one.

An earlier draft of this document proposed keeping a network ALIAS per service —
`mock-jwks:3001`, `echo-backend:3002` — so feature hostnames would not change. That is
redundant and actively misleading: the port already identifies the service, and per-service
aliases imply separately restartable containers, which is precisely the illusion this
consolidation removes. One container, one name.

Rejected for a different reason: one port with `Host`-header routing, which would leave both
hostname and port untouched. Unsafe here because the product legitimately REWRITES the Host
header — `host-rewrite` is a shipped feature with its own tests — so routing on Host would
break exactly the scenarios that exercise it.

Cost: features change hostname and port together, mechanically. 244 references today.

### Statefulness is declared, not assumed

Sharing one container across blocks is safe only for services that derive their response from
the request. product-apim's javadoc claims its node server is stateless while six of its apps
hold mutable state — a counter that never resets, a one-way boolean flip. That is the failure
mode to design out.

So `Service` carries the claim explicitly and the framework enforces it:

```go
// Stateful reports whether this service retains anything between requests.
// A stateful service cannot be shared across blocks unless it also implements Partitioned;
// the registry refuses to register one that does neither.
Stateful() bool
```

There is a third case, and it needed its own contract: a service that DOES retain state but
keeps a separate copy per block. `Stateful()` is binary and cannot express it, so the claim is
completed by an OPTIONAL interface the registry type-asserts:

```go
// PartitionKey names the framework-guaranteed value this service partitions its state by.
// The only key the framework guarantees is unique per block is testbench.PartitionByBlock.
type Partitioned interface{ PartitionKey() string }
```

One service uses it — `analytics` on 3007 — and the guarantee it rests on is stated where it can
be checked: the key is derived from the block name (`topology.ResolvedBlock.PartitionKey`), two
blocks whose names collapse to one key are rejected at suite load, and nothing outside the
framework can choose it.

### Build

There is no root build here, so a change to the testbench is built with one command:

```
make -C tests/framework testbench
```

CI builds it when `tests/framework/testbench/**` changes — not on every run. Tests consume
the image like any other component; a developer who has not touched it never rebuilds it.

## Migration path

1. Stand up `testbench` with the services the pilot already uses: `backend`, `jwks`, `echo`.
2. Point the migrated features at it (port change only) and verify green.
3. Fold in the remaining Go mocks one at a time, deleting each old module as its service lands.
4. `mcp-everything-streamable` is FOLDED IN, reversing the original decision to keep it
   separate. Being Node was the only argument for separateness, and the official MCP Go SDK
   removed it: the `mcp` service on 3009 speaks streamable HTTP with two tools (`add`, `echo`) —
   the whole surface the features use — in ~100 lines. It runs in the SDK's STATELESS mode
   because a session-holding MCP server would be state the registry refuses to share; that
   rules out sessions, server->client requests (sampling/elicitation/roots) and the
   server-initiated SSE stream, none of which any scenario uses.
5. `mock-platform-api` is NOT a candidate — it mocks our own product and must be replaced by
   the real platform-api (see the mock rule in migration-policy.md), not folded in here.

## What this is not

It is not a consolidation for container-count's sake. One container is a side effect. The point
is that there is **one place** to add a mock service, with shared helpers and a review surface,
so the ninth mock is a 40-line package rather than a new repository.

## The block-partitioned analytics collector

`mock-analytics-collector` was the one mock step 3 could not fold in, and the reason was real: it
ACCUMULATES events behind a `sync.RWMutex` and the steps read them back (`analyticsCount`,
`latestEvent`, and critically `resetAnalytics`). Shared unpartitioned, one block's reset would
wipe another's events mid-scenario, and "exactly one analytics event" would become "one plus
whatever the neighbours sent". The registry refused it, correctly.

It is now the `analytics` service on 3007, shared like every other, because the state is
PARTITIONED BY BLOCK. Both sides of the traffic carry the key as a leading path segment:

    POST /<block>/v1/events, /<block>/v1/events/batch          <- the gateway publishes here
    GET  /<block>/test/events, /<block>/test/events/count      <- the steps read here
    POST /<block>/test/reset

A path prefix rather than a header or a query parameter because it needs NOTHING from the
product: the publisher concatenates its fixed path onto the configured base URI
(moesifapi-go: `Config.BaseURI + "/v1/events"`), so a base URI carrying a prefix arrives intact.
`application_id` would have been the more semantically natural key and IS observable — the client
sends `X-Moesif-Application-Id` — but only on the INGEST half; the steps' readback carries no
such header, so the two halves would be keyed by different mechanisms, and it is a config value
two blocks could set identically.

How the key travels, end to end:

1. `topology.ResolvedBlock.PartitionKey()` derives it from the block name, sanitised to a URL
   path segment. Matrix variants differ (`gateway-core/postgres` -> `gateway-core-postgres`), and
   two block names that would collapse to one key are rejected at suite load.
2. `framework/core/runtime/block.go` substitutes it as `${BLOCK}` into the block's config overlays
   when it boots a component — `components.MergeWithVars`, overlays only, never the product's
   shipped base. So `analytics-moesif.toml` reads
   `moesif_base_url = "http://testbench:3007/${BLOCK}"`. An unsubstituted placeholder is a merge
   error, not literal text.
3. The steps build the same address from the running topology (`g.topo.Block.PartitionKey()`).

The key is FRAMEWORK-INJECTED at both ends and is not expressible in a suite or feature file. If
a feature could choose it, two features could choose the same one, and blocks would see each
other's events — which reads exactly like the gateway double-publishing, the thing "LLM proxy
invocation generates exactly one analytics event" exists to catch. For the same reason the
collector answers a missing, reserved or malformed segment with a 400 naming the problem instead
of falling back to a shared bucket: a loud rejection blames the framework, a silent fallback
blames the product. `analytics_test.go` asserts the partitions stay disjoint across writes and
that a reset in one does not touch another.

## Backlog — CP -> DP is not covered in the sync-disabled configuration

Separate gap, recorded here because it is adjacent. `dp_to_cp_sync_disabled` asserts only the
NEGATIVE half of the contract: with `deployment_sync_enabled=false` the gateway must not push
artifacts up. Its header also claims the gateway "still connects to the control plane" — but no
scenario checks it, and neither did the legacy feature.

That leaves the bad shape for a negative test: the cheapest way to make all four scenarios pass is
to break the control-plane connection entirely. A regression where sync-disabled stopped the
gateway connecting at all would keep this feature green while removing half its function.

Filling it needs an observable that does not currently exist — there is no "connected to control
plane" surface on the controller's management API. The strongest option is to create an artifact ON
the control plane and assert the gateway receives it, which is now feasible since the real
platform-api is wired in, but needs CP-side creation steps the suite does not have.
