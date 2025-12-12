## Quick Start

### Using Docker Compose (Recommended)

```bash
## Prerequisites

A Docker-compatible container runtime such as:

- Docker Desktop (Windows / macOS)
- Rancher Desktop (Windows / macOS)
- Colima (macOS)
- Docker Engine + Compose plugin (Linux)

Ensure `docker` and `docker compose` commands are available.

    docker --version
    docker compose version
```

```bash
# Download distribution.
wget https://github.com/wso2/api-platform/releases/download/gateway-v0.0.1/gateway-v0.0.1.zip

# Unzip the downloaded distribution.
unzip gateway-v0.0.1.zip


# Start the complete stack
cd gateway/
docker compose up -d

# Verify gateway controller is running
curl http://localhost:9090/health

# Deploy an API configuration
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: Weather-API
  version: v1.0
  context: /weather/$version
  upstreams:
    - url: http://sample-backend:5000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: ModifyHeaders
          version: v1.0.0
          params:
            requestHeaders:
              - action: SET
                name: operation-level-req-header
                value: hello
            responseHeaders:
              - action: SET
                name: operation-level-res-header
                value: world
    - method: GET
      path: /alerts/active
EOF


# Test routing through the gateway
curl http://localhost:8080/weather/v1.0/us/seattle
curl https://localhost:5443/weather/v1.0/us/seattle -k
```

### Stopping the Gateway

When stopping the gateway, you have two options:

**Option 1: Stop runtime, keep data (persisted APIs and configuration)**
```bash
docker compose down
```
This stops the containers but preserves the `controller-data` volume. When you restart with `docker compose up`, all your API configurations will be restored.

**Option 2: Complete shutdown with data cleanup (fresh start)**
```bash
docker compose down -v
```
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted APIs or configuration.