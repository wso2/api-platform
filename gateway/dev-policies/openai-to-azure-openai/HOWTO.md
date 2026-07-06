# How to Run API Platform with the OpenAI Translator Policy

This guide walks you through building and running the full API Platform
gateway with the `openai-to-azure-openai` translator policy enabled.

---

## Prerequisites

| Tool            | Minimum version |
|-----------------|-----------------|
| Go              | 1.26+           |
| Docker / Docker Compose | Latest  |
| Make            | 3.x+            |

---

## 1. Verify the policy is registered in `build.yaml`

The policy should already be listed in `gateway/build.yaml`:

```yaml
policies:
  # ... other policies ...
  - name: openai-to-azure-openai
    filePath: ./dev-policies/openai-to-azure-openai
```

If it is missing, add the entry above at the end of the `policies` list.

---

## 2. Copy the policy definition to default-policies

The gateway controller needs the policy schema at startup:

```bash
cp gateway/dev-policies/openai-to-azure-openai/policy-definition.yaml \
   gateway/gateway-controller/default-policies/openai-to-azure-openai.yaml
```

---

## 3. Build the gateway images

From the `gateway/` directory:

```bash
cd gateway
make build          # builds gateway-runtime, gateway-builder, and gateway-controller
```

This compiles the policy into the gateway-runtime binary and generates the
Docker images:

- `ghcr.io/wso2/api-platform/gateway-runtime:<version>`
- `ghcr.io/wso2/api-platform/gateway-controller:<version>`

---

## 4. Start the platform

```bash
cd gateway
docker compose up -d
```

The platform exposes:

| Service              | Port  | Description                  |
|----------------------|-------|------------------------------|
| Gateway HTTP         | 8080  | API ingress (HTTP)           |
| Gateway HTTPS        | 8443  | API ingress (HTTPS)          |
| Controller REST API  | 9090  | Management REST API          |
| Controller Admin     | 9094  | Admin API                    |
| Envoy Admin          | 9901  | Envoy admin dashboard        |
| Sample Backend       | 15000 | Echo / mock backend          |

---

## 5. Configure the policy on an API route

Use the gateway controller REST API to attach the policy to a route.

### Azure OpenAI example

```json
{
  "policies": [
    {
      "name": "openai-to-azure-openai",
      "parameters": {
        "targetProvider": "azure-openai",
        "apiVersion": "2024-02-15-preview",
        "model": "gpt-4o"
      }
    }
  ]
}
```

### Anthropic example

```json
{
  "policies": [
    {
      "name": "openai-to-azure-openai",
      "parameters": {
        "targetProvider": "anthropic",
        "model": "claude-sonnet-4-20250514"
      }
    }
  ]
}
```

### Mistral example

```json
{
  "policies": [
    {
      "name": "openai-to-azure-openai",
      "parameters": {
        "targetProvider": "mistral",
        "model": "mistral-large-latest"
      }
    }
  ]
}
```

---

## 6. Test with curl

Send an OpenAI-format request to the gateway and verify the translation:

```bash
# Azure OpenAI
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user",   "content": "Hello!"}
    ],
    "max_tokens": 256
  }'
```

```bash
# Anthropic (same OpenAI body — policy translates automatically)
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user",   "content": "Hello!"}
    ],
    "max_tokens": 256
  }'
```

---

## 7. What the policy translates (body only)

| Source (OpenAI)              | Azure OpenAI            | Anthropic                       | Mistral                    |
|------------------------------|-------------------------|---------------------------------|----------------------------|
| `model`                     | deployment id in URL    | `model`                         | `model`                    |
| `messages[role=system]`     | passthrough             | top-level `system` field        | passthrough                |
| `messages`                  | passthrough             | Anthropic message blocks        | passthrough                |
| `max_tokens`                | passthrough             | `max_tokens` (required)         | passthrough                |
| `stop`                      | passthrough             | `stop_sequences`                | passthrough                |
| `tools`                     | passthrough             | `name`/`description`/`input_schema` | passthrough            |
| `tool_choice`               | passthrough             | `{"type":"auto"/"any"/"tool"}`  | passthrough                |
| `tool_calls` (assistant)    | passthrough             | `tool_use` content blocks       | passthrough                |
| `role=tool` messages        | passthrough             | `tool_result` in user messages  | passthrough                |
| `image_url` content parts   | passthrough             | Anthropic `image` source blocks | passthrough                |
| Path                        | `/openai/deployments/…` | `/v1/messages`                  | `/v1/chat/completions`     |
| Unsupported fields          | —                       | stripped                        | stripped (`logprobs`, etc.) |

---

## 8. Cleaning up (before committing)

Per `dev-policies/README.md`, before committing remove the `filePath` entry
from `gateway/build.yaml` and the copied YAML from `default-policies/` so
CI builds are unaffected:

```bash
rm gateway/gateway-controller/default-policies/openai-to-azure-openai.yaml
# Edit gateway/build.yaml and remove the openai-to-azure-openai entry
```

---

## Troubleshooting

- **Policy not found at runtime** — make sure `build.yaml` lists the policy
  *and* `make build` was re-run after adding it.
- **Azure returns 404** — verify `apiVersion` matches a valid Azure API
  version and the model/deployment name is correct.
- **Anthropic returns 401** — the policy does NOT set the `x-api-key` header.
  Use a `set-headers` policy (or route-level config) to inject the Anthropic
  API key.
- **Mistral returns 422** — check that the model name is valid for your
  Mistral subscription.
