# Dev Policies

This directory holds locally-developed policies for testing and iteration
during development. Policies here are **not** included in production builds
unless explicitly listed in `gateway/build.yaml`.

## Adding a dev policy

1. Create a directory under `dev-policies/` for your policy (e.g.
   `dev-policies/my-policy/`). It must contain at minimum:
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
   cd gateway/policy-engine
   make build-local
   ```

## Before committing

Remove your `filePath` entry from `gateway/build.yaml` so that CI and
production builds are not affected. The policy source under `dev-policies/`
can stay — it is ignored by the builder unless referenced in the manifest.

## Example

`count-letters/` is a sample dev policy that counts character occurrences
in response bodies. Use it as a reference for structure and conventions.
