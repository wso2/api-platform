# Gateway Artifact Templating

Gateway artifact files (YAML/JSON) support Go `text/template` expressions for injecting dynamic values at startup. Templates are rendered on the raw artifact string before YAML/JSON parsing, so expressions work in any string field across the entire artifact — `upstream`, `auth`, policy `params`, metadata, etc.

## Available functions

### `env`

Reads a value from an environment variable.

```
{{ env "KEY" }}
```

Returns an empty string if the variable is not set. Use `| default` to provide a fallback:

```
{{ env "BACKEND_URL" | default "http://localhost:8080" }}
```

### `required`

Like `env`, but fails at startup if the variable is not set or empty. Use this to enforce that a value must be provided:

```
{{ required "BACKEND_URL" }}
```

If the variable is missing, the gateway will not start and will log which variable is required.

### `redact`

Marks a resolved value for redaction in config dumps. Any field rendered with `| redact` will appear as `***REDACTED***` in the gateway's config dump output instead of its actual value.

```
{{ env "API_KEY" | redact }}
```

Use `| redact` for any sensitive value injected from an environment variable (API keys, tokens, passwords).

### `default`

Provides a fallback value when the input is empty:

```
{{ env "LOG_LEVEL" | default "info" }}
```

Can be combined with any other function in a pipeline.

## Combining functions

Functions compose as pipelines:

```
{{ env "KEY" | default "fallback" }}
{{ env "KEY" | redact }}
{{ required "KEY" | redact }}
```

## Example

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: orders-api-v1
spec:
  context: /orders/v1
  upstream:
    main:
      url: '{{ env "ORDERS_BACKEND_URL" | default "http://orders-svc:8080" }}'
    auth:
      apiKey: '{{ required "ORDERS_API_KEY" | redact }}'
  policies:
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: X-Env
              value: '{{ env "DEPLOYMENT_ENV" | default "production" }}'
```

In this example:

- `ORDERS_BACKEND_URL` is optional — falls back to the default URL if not set.
- `ORDERS_API_KEY` is required — the gateway will not start if it is missing, and the value is redacted in config dumps.
- `DEPLOYMENT_ENV` is optional with a default of `production`.
