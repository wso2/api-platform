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

Replace `${version}` with the API Platform Gateway release version you want to run.

```bash
# Download distribution.
wget https://github.com/wso2/api-platform/releases/download/gateway/v1.0.0-rc/wso2apip-api-gateway-1.0.0-rc.zip

# Unzip the downloaded distribution.
unzip wso2apip-api-gateway-1.0.0-rc.zip


# Start the complete stack
cd wso2apip-api-gateway-1.0.0-rc/
docker compose -p gateway up -d

# Verify gateway controller admin endpoint is running
curl http://localhost:9094/health

# Start the sample backend used by the quick start API
docker run -d \
  --name sample-backend \
  --network gateway_gateway-network \
  -p 15000:5000 \
  ghcr.io/wso2/api-platform/sample-service:latest \
  -addr :5000 -pretty

# Deploy an API configuration
curl -X POST http://localhost:9090/rest-apis \
  -u admin:admin \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: x-quickstart-request-header
              value: hello
        response:
          headers:
            - name: x-quickstart-response-header
              value: world
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
EOF


# Test routing through the gateway
curl -i http://localhost:8080/weather/v1.0/us/seattle
curl -ik https://localhost:8443/weather/v1.0/us/seattle
```

### Stopping the Gateway

When stopping the gateway, you have two options:

**Option 1: Stop runtime, keep data (persisted APIs and configuration)**
```bash
docker stop sample-backend
docker rm sample-backend
docker compose -p gateway down
```
This stops the containers but preserves the `controller-data` volume. When you restart with `docker compose -p gateway up`, all your API configurations will be restored.

**Option 2: Complete shutdown with data cleanup (fresh start)**
```bash
docker stop sample-backend
docker rm sample-backend
docker compose -p gateway down -v
```
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted APIs or configuration.
