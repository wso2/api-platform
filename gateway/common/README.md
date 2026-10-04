# Gateway Common Module

Shared code used by **both** the gateway control plane (`gateway/gateway-controller`)
and the gateway data plane (`gateway/gateway-runtime/policy-engine`).

This is deliberately separate from the repository-root `common/` module. Root
`common/` holds code shared across the whole platform — Platform API, the portals,
the CLI and the event gateway all depend on it. This module holds code that only
the gateway's two halves need, so gateway-specific contracts don't leak into every
other product's dependency graph.

Put a package here when the controller and the runtime must agree on the exact same
definition of something. Put it in root `common/` instead when anything outside
`gateway/` needs it too.

## Structure

- `agentproto/` — version-keyed registry of canonical A2A operations and their
  HTTP+JSON bindings. The controller uses it to validate and transform agent
  definitions; the policy engine uses it to resolve incoming requests to
  operations. A mismatch between the two ends is a routing bug, which is why the
  table is spelled exactly once, here.

## Usage

```go
import "github.com/wso2/api-platform/gateway/common/agentproto"
```

## Local Development

The module is part of the repository `go.work`, so no extra wiring is needed for
workspace builds. Modules that build outside the workspace (Docker images) need a
`replace` directive:

```go
replace github.com/wso2/api-platform/gateway/common => ../common
```

Docker builds pass this module as the `gateway-common` build context, mounted at a
path that mirrors the repository layout. That layout matters: a build tree that
flattens the repo would make `../common` (this module) and `../../common` (root
common) resolve to the same directory.
