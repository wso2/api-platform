# Immutable Gateway

This guide explains how to run the API Platform Gateway in **immutable mode**, where API configurations are loaded from files at startup instead of being managed through the REST API.

## Overview

In immutable mode, the gateway controller reads API artifacts from a local directory at startup and applies them automatically. The management REST API remains available for read operations but rejects any mutating requests (`POST`, `PUT`, `PATCH`, `DELETE`) with `405 Method Not Allowed`.

This is the recommended approach for:

- **GitOps workflows** — store artifacts in a Git repository and bake them into a custom gateway image via CI/CD.
- **Immutable infrastructure** — ship a self-contained gateway image that requires no runtime configuration.
- **Kubernetes deployments** — mount a ConfigMap or Secret as a volume at the artifacts path.

## Configuration

Enable immutable mode in `config.toml` or via environment variable:

```toml
[immutable_gateway]
enabled = true
```

To drive this from an environment variable instead, use an interpolation token in the config file —
```bash
APIP_GW_IMMUTABLE_GATEWAY_ENABLED=true
```
```toml
[immutable_gateway]
enabled = '{{ env "APIP_GW_IMMUTABLE_GATEWAY_ENABLED" "false" }}'
```

By default, the gateway controller loads artifacts from `/etc/api-platform-gateway/immutable_gateway/artifacts`. You only need to set `artifacts_dir` if you want to use a different path.

> **Note:** Full configuration reference with all options:
>
> ```toml
> [immutable_gateway]
> enabled = true
> artifacts_dir = "/etc/api-platform-gateway/immutable_gateway/artifacts"
> ```
>
> To make either value environment-driven, wire it with a `{{ env }}` interpolation token in the
> config file:
> ```bash
> APIP_GW_IMMUTABLE_GATEWAY_ENABLED=true
> APIP_GW_IMMUTABLE_GATEWAY_ARTIFACTS_DIR=/etc/api-platform-gateway/immutable_gateway/artifacts
> ```
> ```toml
> [immutable_gateway]
> enabled       = '{{ env "APIP_GW_IMMUTABLE_GATEWAY_ENABLED" "false" }}'
> artifacts_dir = '{{ env "APIP_GW_IMMUTABLE_GATEWAY_ARTIFACTS_DIR" "/etc/api-platform-gateway/immutable_gateway/artifacts" }}'
> ```

## Artifact format

Artifacts use the same Kubernetes-style resource format supported by the gateway. Each file must include `apiVersion`, `kind`, `metadata`, and `spec`.

### Injecting environment variables

Artifact files support Go template expressions for injecting dynamic values. Templates are rendered on the raw artifact string before YAML parsing, so expressions work in any string field — `upstream`, `auth`, policy `params`, etc.

| Function | Redacted in config dumps | Use for |
|---|---|---|
| `{{ env "KEY" }}` | No | Non-sensitive env vars (URLs, policy param values) |
| `{{ env "KEY" \| redact }}` | Yes | Sensitive env vars (tokens, API keys) |

Use `| redact` for sensitive values to hide them from config dumps. A `| default "value"` pipe is available for fallback values. See [Gateway Artifact Templating](artifact-templating.md) for the full function reference.

### Sample: Reading List API

Save the following as `artifacts/reading-list-v1.yaml`:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: reading-list-api-v1
spec:
  displayName: Reading List API
  version: v1.0
  context: /reading-list/$version
  upstream:
    main:
      # read from env, falls back to the public sample backend if not set
      url: '{{ env "BACKEND_URL" | default "https://apis.bijira.dev/samples/reading-list-api-service/v1.0" }}'
  operations:
    - method: GET
      path: /books
    - method: POST
      path: /books
    - method: GET
      path: /books/{id}
    - method: PUT
      path: /books/{id}
    - method: DELETE
      path: /books/{id}
```

### Directory structure

The controller walks all subdirectories, so you can organize artifacts however you like:

```
artifacts/
├── rest-apis/
│   ├── petstore-v1.yaml
│   └── orders-v2.yaml
├── llm-providers/
│   └── openai.yaml
├── llm-proxies/
│   └── chat-proxy.yaml
└── mcp-proxies/
    └── tools-proxy.yaml
```

## Deployment

### Baking artifacts into a custom image

The recommended GitOps approach is to build a custom gateway-controller image with artifacts copied in at build time:

```dockerfile
FROM ghcr.io/wso2/api-platform/gateway-controller:1.0.0

COPY ./artifacts /etc/api-platform-gateway/immutable_gateway/artifacts

# The ENV takes effect only if the mounted config.toml reads it via an interpolation token
# ([immutable_gateway] enabled = '{{ env "APIP_GW_IMMUTABLE_GATEWAY_ENABLED" "false" }}'); there is
# no APIP_GW_ prefix auto-override. Alternatively set [immutable_gateway] enabled = true directly.
ENV APIP_GW_IMMUTABLE_GATEWAY_ENABLED=true
```

### Kubernetes — mounting a ConfigMap volume

You can also mount artifacts as a volume from a ConfigMap or Secret without building a custom image:

```yaml
volumes:
  - name: gateway-artifacts
    configMap:
      name: gateway-artifacts
volumeMounts:
  - name: gateway-artifacts
    mountPath: /etc/api-platform-gateway/immutable_gateway/artifacts
    readOnly: true
```

Enable immutable mode by setting `[immutable_gateway] enabled = true` in the mounted `config.toml`
(or wire that key to `APIP_GW_IMMUTABLE_GATEWAY_ENABLED` with a `{{ env }}` interpolation token in
the config

## Invoking the API

Once the gateway is running with the sample artifact, invoke the Reading List API:

```bash
curl -i http://localhost:8080/reading-list/v1.0/books
```
