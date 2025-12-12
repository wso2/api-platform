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
wget https://github.com/wso2/api-platform/releases/download/gateway-${version}/gateway-${version}.zip

# Unzip the downloaded distribution.
unzip gateway-${version}.zip


# Start the complete stack
cd gateway/
docker compose up -d

# Verify gateway controller is running
curl http://localhost:9090/health
```

## Deploy an LLM provider configuration

Replace `<openai-apikey>` with your openai API key and run the following command to deploy a sample openai provider.

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: llm/provider
spec:
  name: openai-provider
  version: v1.0
  template: openai
  vhost: openai
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: <openai-apikey>
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
curl -X POST http://localhost:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hi"
      }
    ]
  }'
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
