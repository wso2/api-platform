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
wget https://github.com/wso2/api-platform/releases/download/gateway-v0.1.0/gateway-v0.1.0.zip

# Unzip the downloaded distribution.
unzip gateway-v0.1.0.zip


# Start the complete stack
cd gateway-v0.1.0/
docker compose up -d

# Verify gateway controller is running
curl http://localhost:9090/health

# Or use the CLI (requires gateway to be added first with: ap gateway add)
ap gateway health

# Deploy an API configuration
curl -X POST http://localhost:9090/apis \
  -u admin:admin \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    - url: http://sample-backend:5000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: modify-headers
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
curl https://localhost:8443/weather/v1.0/us/seattle -k
```

### Managing APIs with CLI

The CLI provides convenient commands for managing your APIs:

**List all APIs:**
```bash
ap gateway api list
```

**Get a specific API:**
```bash
# By name and version
ap gateway api get --display-name "Weather-API" --version v1.0 --format yaml

# By ID
ap gateway api get --id <api-id> --format yaml
```

**Apply an API from a file:**
```bash
ap gateway apply --file petstore-api.yaml
```

**Delete an API:**
```bash
ap gateway api delete --id <api-id>
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