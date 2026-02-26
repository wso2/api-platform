# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

### Prerequisites

**Setup OAuth2 Authentication (STS)**

Before using the Platform API, set up the Security Token Service (STS) for authentication:

1. Follow the instructions in [sts/README.md](../sts/README.md) to start the STS service
2. Run the sample OAuth application and log in
3. Copy the access token displayed after successful login
4. Use this token in the `Authorization: Bearer <token>` header for all Platform API requests

### Build and Run

```bash
# Build
cd platform-api/src
go build ./cmd/main.go

# Run (TLS with self-signed certificates)
cd platform-api/src
go run ./cmd/main.go
```

### Step-by-Step Workflow

**1. Register an Organization**

```bash
curl -k -X POST https://localhost:9243/api/v1/organizations \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{"id":"<org-uuid>","handle":"acme","name":"ACME Corporation","region":"us-east-1"}'
```

**2. Create a Project**

```bash
curl -k -X POST https://localhost:9243/api/v1/projects \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{
    "name": "Production APIs"
  }'
```

**3. Create a Gateway**

```bash
curl -k -X POST https://localhost:9243/api/v1/gateways \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{
    "name": "prod-gateway-01",
    "displayName": "Production Gateway 01",
    "vhost": "localhost",
    "functionalityType": "regular"
  }'
```

Response includes the gateway UUID:
```json
{
  "id": "4dac93bd-07ba-417e-aef8-353cebe3ba73",
  "name": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "createdAt": "2025-10-21T15:12:44.168980842+05:30",
  "updatedAt": "2025-10-21T15:12:44.16898088+05:30"
}
```

**4. Generate Gateway Token**

```bash
curl -k -X POST https://localhost:9243/api/v1/gateways/<gateway-uuid>/tokens \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>'
```

Response includes the gateway authentication token:
```json
{
  "id": "7ed55286-66a4-43ae-9271-bd1ead475a55",
  "token": "QY8Rnm9bJ-incsGU0xtFz2vx16I1IVhEf0Ma_4O5F9s",
  "createdAt": "2025-10-21T15:12:57.60936197+05:30",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

**List Gateway Tokens:**
```bash
curl -k -s https://localhost:9243/api/v1/gateways/<gateway-uuid>/tokens \
  -H 'Authorization: Bearer <your-oauth2-token>'
```

Response:
```json
[
  {
    "id": "7ed55286-66a4-43ae-9271-bd1ead475a55",
    "status": "active",
    "createdAt": "2025-10-21T15:12:57.60936197+05:30"
  }
]
```

**5. Connect Gateway to Platform (WebSocket)**

Install wscat if not already installed:
```bash
npm install -g wscat
```

Connect using the gateway token:
```bash
wscat -n -c wss://localhost:9243/api/internal/v1/ws/gateways/connect \
  -H "api-key: <gateway-token>"
```

Expected output:
```
Connected (press CTRL+C to quit)
< {"type":"connection.ack","gatewayId":"4dac93bd-07ba-417e-aef8-353cebe3ba73","connectionId":"3150a8b6-649d-4d12-8512-7d72e8ec7f13","timestamp":"2025-10-21T14:42:13+05:30"}
```

Keep this connection open to receive real-time deployment events.

**6. Create an API**

```bash
curl -k -X POST 'https://localhost:9243/api/v1/rest-apis' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{
      "id": "weather-api",
      "name": "Weather API",
      "displayName": "Weather API",
      "description": "Weather API with main and sandbox upstreams",
      "context": "/weather",
      "version": "v1.0",
      "projectId": "<project-uuid>",
      "lifeCycleStatus": "CREATED",
      "transport": ["http","https"],
      "vhosts": {
         "main": "example.wso2.com",
         "sandbox": "sand-example.wso2.com"
       },
       "upstream": {
         "main": { "url": "http://sample-backend:5000" },
         "sandbox": { "url": "http://sample-backend:5000/sandbox" }
       }
    }'
```

**7. Deploy API to Gateway**

```bash
curl -k -X POST 'https://localhost:9243/api/v1/rest-apis/weather-api/deployments' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{
    "name": "weather-v1-prod",
    "base": "current",
    "gatewayId": "<gateway-uuid>"
}'
```

Expected response:
```json
[
  {
    "revisionId": "90d10e1c-8560-5c36-9d5a-124ecaa17485",
    "gatewayId": "4dac93bd-07ba-417e-aef8-353cebe3ba73",
    "status": "CREATED",
    "vhost": "mg.wso2.com",
    "displayOnDevportal": false,
    "deployedTime": "2025-10-21T16:15:18+05:30",
    "successDeployedTime": "2025-10-21T16:15:18+05:30"
  }
]
```

The connected gateway will receive a deployment event via WebSocket:
```
< {"type":"api.deployed","payload":{"apiId":"54588845-c860-4a56-8802-c06b03028543","revisionId":"90d10e1c-8560-5c36-9d5a-124ecaa17485","vhost":"mg.wso2.com","environment":"production"},"timestamp":"2025-10-21T16:15:18+05:30","correlationId":"ae7488ec-9559-4a81-bddd-b85e1391d2c0"}
```

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
