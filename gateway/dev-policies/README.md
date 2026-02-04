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

2. If you are working on a new policy, copy the `policy-definition.yaml` to the default policies directory:
   ```bash
   cp dev-policies/my-policy/policy-definition.yaml \
      gateway-controller/default-policies/my-policy.yaml
   ```
   **Important**: The file must be renamed to match your policy name (e.g.,
   `my-policy.yaml`) so the gateway controller can load the policy definition.

3. Register it in `gateway/build.yaml` using a `filePath` entry:
   ```yaml
   policies:
     - name: my-policy
       filePath: ./dev-policies/my-policy
   ```

4. Build locally:
   ```bash
   cd gateway/policy-engine
   make build-local
   ```

## Before committing

Remove your `filePath` entry from `gateway/build.yaml` and the copied policy
definition from `gateway-controller/default-policies/` so that CI and
production builds are not affected. The policy source under `dev-policies/`
can stay — it is ignored by the builder unless referenced in the manifest.

## Example

`count-letters/` is a sample dev policy that counts character occurrences
in response bodies. Use it as a reference for structure and conventions.
