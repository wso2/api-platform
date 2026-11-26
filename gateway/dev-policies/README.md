# Dev Policies

This directory holds locally-developed policies for testing and iteration during development. Policies here are **not** included in production builds unless explicitly listed in `gateway/build.yaml`.

## Adding a dev policy

1. Create a directory under `dev-policies/` for your policy (e.g. `dev-policies/my-policy/`). It must contain at minimum:
   - `go.mod` – module definition (depend on the SDK: `github.com/wso2/api-platform/sdk`)
   - A `.go` file implementing `policy.Policy`
   - `policy-definition.yaml` – the policy's parameter schema

2. Register it in `gateway/build.yaml` using a `filePath` entry:
   ```yaml
   policies:
     - name: my-policy
       filePath: ./dev-policies/my-policy
   ```

3. Build locally:
   ```bash
   cd gateway
   make build
   ```

## Policies mirrored from another repo

A policy under development in `wso2/gateway-controllers` is copied here so it can be built
before it is published. That copy is the one the image compiles — **not** the source repo —
so when the two drift the gateway silently runs an older policy. No build error, no warning:
just behaviour that does not match the code you are reading.

Run the sync before every image build:

```bash
cd gateway
./sync-dev-policies.sh            # mirror each dev policy from ../gateway-controllers
./sync-dev-policies.sh --check    # report drift without writing; non-zero if stale
```

Point it elsewhere with `GATEWAY_CONTROLLERS_DIR=/path/to/checkout`. It only touches
directories that already exist under `dev-policies/`, so it mirrors an existing dev copy
rather than deciding what is under local development.

## Before committing

Remove your `filePath` entry from `gateway/build.yaml` so that CI and production builds are not affected. The policy source under `dev-policies/` can stay — it is ignored by the builder unless referenced in the manifest.
