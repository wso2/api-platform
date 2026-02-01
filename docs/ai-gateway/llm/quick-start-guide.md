## Quick Start

### Using Docker Compose (Recommended)


### Prerequisites

A Docker-compatible container runtime such as:

- Docker Desktop (Windows / macOS)
- Rancher Desktop (Windows / macOS)
- Colima (macOS)
- Docker Engine + Compose plugin (Linux)

Ensure `docker` and `docker compose` commands are available.

```bash
docker --version
docker compose version
```

Replace ${version} with the actual release version of the API Platform Gateway.
```bash
# Download distribution.
wget https://github.com/wso2/api-platform/releases/download/ai-gateway/v0.5.0/ai-gateway-v0.5.0.zip

# Unzip the downloaded distribution.
unzip ai-gateway-v0.5.0.zip


# Start the complete stack
cd ai-gateway-v0.5.0/
docker compose up -d

# Verify gateway controller is running
curl http://localhost:9090/health
```

## Deploy an OpenAI LLM provider configuration

The API Platform Gateway currently includes first-class support for the OpenAI LLM provider. As a platform administrator, replace `<openai-apikey>` with your openai API key and run the following command to deploy a sample OpenAI LLM provider.

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: openai-provider
spec:
  displayName: OpenAI Provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: Bearer <openai-apikey>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
      - path: /models
        methods: [GET]
      - path: /models/{modelId}
        methods: [GET]
EOF
```

To test LLM provider traffic routing through the gateway, invoke the following request.

```bash
curl -X POST https://localhost:8443/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hi"
      }
    ]
  }' -k
```

## Deploy an LLM proxy configuration to consume an LLM provider

The API Platform Gateway provides first-class support for configuring and deploying LLM proxies. As an AI developer, run the following command to deploy a sample LLM proxy that consumes the OpenAI LLM provider previously deployed by the platform administrator.

```bash
curl -X POST http://localhost:9090/llm-proxies \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProxy
metadata:
  name: docs-assistant
spec:
  displayName: Docs Assistant
  version: v1.0
  context: /assistant
  provider: openai-provider
  policies: []
EOF
```

To test LLM proxy traffic routing through the gateway and consume the LLM provider, invoke the following request.

```bash
curl -X POST "https://localhost:8443/assistant/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hi"
      }
    ]
  }' -k
```

## Stopping the Gateway

When stopping the gateway, you have two options:

### Option 1: Stop runtime, keep data (persisted proxies and configuration)

```bash
docker compose down
```

This stops the containers but preserves the `controller-data` volume. When you restart with `docker compose up`, all your API configurations will be restored.

### Option 2: Complete shutdown with data cleanup (fresh start)
```bash
docker compose down -v
```
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted templates or provider configuration.
