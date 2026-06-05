# WSO2 API Platform Mock Server

Note: Visit browser's https://localhost:8080 and allow since this is self signed before using this in WebApp

This is an HTTPS-enabled Flask mock server that simulates the WSO2 API Platform backend. It accepts Bearer token authentication but does **not validate** the tokens - any Bearer token will be accepted.

##  Features
- ✅ HTTPS with self-signed certificate
- ✅ Bearer token authentication (accepts any token)
- ✅ Full CRUD operations for all resources
- ✅ Mock data for testing
- ✅ LLM Provider endpoints matching your frontend

## Prerequisites

```bash
pip3 install flask flask-cors pyopenssl cryptography
```

**Required packages:**
- `flask` - Web framework
- `flask-cors` - CORS support for cross-origin requests
- `pyopenssl` - SSL/TLS support for HTTPS
- `cryptogrpahy` - In case SSL did't support

## Running the Server

```bash
cd workspaces/apps/ai-workspace/mock-service
HTTPS=true python3 mock_server.py
```

The server will start on: **https://localhost:8080**
Then visit: **https://localhost:8080** in browser to proceed with localhost

(HTTPS true is optional. If not, server runs in http with no token header validation)

### Authentication

If 
```bash
export HTTPS=true
```
All endpoints (except `/api/v1/health`) require a Bearer token in the Authorization header:

```bash
Authorization: Bearer <any-token-here>
```

**Important:** The server does NOT validate tokens. Any value after "Bearer " will work.

## Sample Curls

### Health check (no auth required)
```bash
curl -k https://localhost:8080/api/v1/health
```

### List LLM providers (requires Bearer token)
```bash
curl -k -H "Authorization: Bearer test-token" https://localhost:8080/llm-providers
```

### Get specific provider
```bash
curl -k -H "Authorization: Bearer test-token" https://localhost:8080/llm-providers/wso2-openai-provider
```

### Create a new provider
```bash
curl -k -X POST \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"My OpenAI","type":"openai","endpoint":"https://api.openai.com/v1"}' \
  https://localhost:8080/llm-providers
```

### Update a provider
```bash
curl -k -X PUT \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated Name"}' \
  https://localhost:8080/llm-providers/wso2-openai-provider
```

### Delete a provider
```bash
curl -k -X DELETE \
  -H "Authorization: Bearer test-token" \
  https://localhost:8080/llm-providers/wso2-openai-provider
```