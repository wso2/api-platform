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

## Deploy an MCP proxy configuration

Start the sample MCP server

```bash
docker run -p 3001:3001 --name everything --network gateway_gateway-network rakhitharr/mcp-everything:v3
```

Run the following command to deploy the MCP proxy.

```bash
curl -X POST http://localhost:9090/mcp-proxies \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: everything-mcp-v1.0
spec:
  displayName: Everything
  version: v1.0
  context: /everything
  specVersion: "2025-06-18"
  upstream:
    url: http://everything:3001
  tools: []
  resources: []
  prompts: []
EOF
```
To test MCP traffic routing through the gateway, add the following URL to your MCP client and connect to the server.

```
http://localhost:8080/everything/mcp
```

## Stopping the Gateway

Stop and remove the MCP backend first.

```bash
docker stop everything
docker rm everything
```

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
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted proxies or configuration.