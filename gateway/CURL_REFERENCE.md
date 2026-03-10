# Gateway API — Curl Reference

A complete collection of curl commands for testing all gateway functionality.
All commands assume the gateway is running locally via `docker compose up`.

---

## Connection Details

| Service | Host | Auth |
|---|---|---|
| Gateway Controller (Admin API) | `http://localhost:9090` | Basic: `admin:admin` |
| Gateway Runtime (Envoy Router) | `http://localhost:8080` | — |
| Gateway Runtime (HTTPS) | `https://localhost:8443` | — |
| Policy Engine Admin | `http://localhost:9002` | — |
| Envoy Admin | `http://localhost:9901` | — |

---

## 1. Health Checks

### 1.1 Gateway Controller Health
```bash
curl -s http://localhost:9090/health | jq
```

**Expected response:**
```json
{"status":"healthy","timestamp":"2026-03-10T05:09:51Z"}
```

### 1.2 Policy Engine Health
```bash
curl -s http://localhost:9002/health | jq
```

### 1.3 Envoy Admin — Live Check
```bash
curl -s http://localhost:9901/ready
```

---

## 2. Gateway Controller — API Management

### 2.1 List All Deployed APIs
```bash
curl -s -u admin:admin http://localhost:9090/apis | jq
```

**Query filters (optional):**
```bash
# Filter by display name
curl -s -u admin:admin "http://localhost:9090/apis?displayName=Prompt" | jq

# Filter by status
curl -s -u admin:admin "http://localhost:9090/apis?status=deployed" | jq

# Filter by context path
curl -s -u admin:admin "http://localhost:9090/apis?context=/compress-test" | jq
```

---

### 2.2 Get a Specific API
```bash
curl -s -u admin:admin http://localhost:9090/apis/prompt-compression-test | jq
```

---

### 2.3 Create an API (with Prompt Compression Policy)

This creates the `prompt-compression-test` API that is currently deployed.

```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "RestApi",
    "metadata": {
      "name": "prompt-compression-test"
    },
    "spec": {
      "displayName": "Prompt Compression Test API",
      "version": "v1.0",
      "context": "/compress-test",
      "upstream": {
        "main": {
          "url": "http://sample-backend:5000"
        }
      },
      "operations": [
        {
          "method": "POST",
          "path": "/post",
          "policies": [
            {
              "name": "prompt-compression",
              "version": "v0",
              "params": {
                "compressionRatio": 0.5,
                "jsonPath": "$.messages[-1].content",
                "minInputTokens": 50,
                "preserveCodeBlocks": true,
                "preserveJson": true
              }
            }
          ]
        }
      ]
    }
  }' | jq
```

---

### 2.4 Update an API

Change the compression ratio from 0.5 to 0.7 (more aggressive compression):

```bash
curl -s -u admin:admin \
  -X PUT http://localhost:9090/apis/prompt-compression-test \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "RestApi",
    "metadata": {
      "name": "prompt-compression-test"
    },
    "spec": {
      "displayName": "Prompt Compression Test API",
      "version": "v1.0",
      "context": "/compress-test",
      "upstream": {
        "main": {
          "url": "http://sample-backend:5000"
        }
      },
      "operations": [
        {
          "method": "POST",
          "path": "/post",
          "policies": [
            {
              "name": "prompt-compression",
              "version": "v0",
              "params": {
                "compressionRatio": 0.7,
                "jsonPath": "$.messages[-1].content",
                "minInputTokens": 50,
                "preserveCodeBlocks": true,
                "preserveJson": true
              }
            }
          ]
        }
      ]
    }
  }' | jq
```

---

### 2.5 Delete an API
```bash
curl -s -u admin:admin \
  -X DELETE http://localhost:9090/apis/prompt-compression-test | jq
```

> **Note:** After deletion, re-create it with the Create command (§2.3) for further testing.

---

## 3. API Key Management

### 3.1 Create an API Key
```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/apis/prompt-compression-test/api-keys \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-test-key"
  }' | jq
```

Save the `key` value from the response — you'll use it as a bearer token or header value.

---

### 3.2 List API Keys for an API
```bash
curl -s -u admin:admin \
  http://localhost:9090/apis/prompt-compression-test/api-keys | jq
```

---

### 3.3 Regenerate an API Key
```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/apis/prompt-compression-test/api-keys/my-test-key/regenerate \
  -H "Content-Type: application/json" \
  -d '{}' | jq
```

---

### 3.4 Revoke an API Key
```bash
curl -s -u admin:admin \
  -X DELETE http://localhost:9090/apis/prompt-compression-test/api-keys/my-test-key | jq
```

---

## 4. Runtime — Prompt Compression Policy Tests

The sample backend (`sample-service`) echoes back the request it receives. The `body` field in the response shows what the upstream would actually receive — this is how you observe whether compression occurred.

### 4.1 Basic Compression Test (Long Prompt — Will Compress)

```bash
curl -s -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Artificial intelligence and machine learning have revolutionized the way we approach complex problems in software engineering, data science, and many other fields. These technologies enable computers to learn from vast amounts of data, identify intricate patterns, and make sophisticated predictions without being explicitly programmed for every possible scenario. Deep learning, a subset of machine learning, uses neural networks with multiple layers to process and analyze information in ways that mimic human cognitive functions. Natural language processing allows machines to understand, interpret, and generate human language with remarkable accuracy. Computer vision enables systems to extract meaningful information from digital images and videos. Reinforcement learning trains agents to make optimal decisions through trial and error in dynamic environments. Transfer learning allows models trained on one task to be repurposed for related tasks, significantly reducing the amount of training data and computational resources required."
      }
    ]
  }' | jq '.body | fromjson | .messages[-1].content'
```

**Observe:** The `content` field in the response will be shorter than what was sent — filler words like "and", "the", "of", "to" are removed. The `Content-Length` of the upstream request (shown in the `headers` field) will also be smaller than your original payload size.

---

### 4.2 Short Prompt — Below minInputTokens Threshold (No Compression)

```bash
curl -s -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "What is the capital of France?"
      }
    ]
  }' | jq '.body | fromjson | .messages[-1].content'
```

**Observe:** The content will be returned unchanged — too short to compress (`minInputTokens: 50`).

---

### 4.3 Test with a File — Save Request/Response Sizes
```bash
# Using the test file generated during debugging
curl -v -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/test-prompt-long.json 2>&1 | grep -E "Content-Length|< HTTP"
```

**Observe:** The `Content-Length` of your request vs. what the upstream receives shows the compression saving.

---

### 4.4 Test with domainTerms — Preserve Specific Words
```bash
# First, update the API to add domain terms
curl -s -u admin:admin \
  -X PUT http://localhost:9090/apis/prompt-compression-test \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "RestApi",
    "metadata": {
      "name": "prompt-compression-test"
    },
    "spec": {
      "displayName": "Prompt Compression Test API",
      "version": "v1.0",
      "context": "/compress-test",
      "upstream": {
        "main": {
          "url": "http://sample-backend:5000"
        }
      },
      "operations": [
        {
          "method": "POST",
          "path": "/post",
          "policies": [
            {
              "name": "prompt-compression",
              "version": "v0",
              "params": {
                "compressionRatio": 0.5,
                "jsonPath": "$.messages[-1].content",
                "minInputTokens": 50,
                "preserveCodeBlocks": true,
                "preserveJson": true,
                "domainTerms": ["reinforcement", "neural", "backpropagation"]
              }
            }
          ]
        }
      ]
    }
  }' | jq .status

# Then send a request — the domain terms will be preserved even under heavy compression
curl -s -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Artificial intelligence and machine learning have revolutionized the way we approach complex problems in software engineering. Deep learning uses neural networks with multiple layers to process information using backpropagation algorithms. Reinforcement learning trains agents to make optimal decisions through trial and error in dynamic environments and complex systems."
      }
    ]
  }' | jq '.body | fromjson | .messages[-1].content'
```

**Observe:** The words "reinforcement", "neural", and "backpropagation" will be present in the compressed output.

---

### 4.5 Test HTTPS Endpoint
```bash
# -k skips TLS certificate verification (self-signed cert in dev)
curl -sk -X POST https://localhost:8443/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/test-prompt-long.json | jq '.headers["Content-Length"]'
```

---

### 4.6 View Full Response (Headers + Body)
```bash
curl -v -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/test-prompt-long.json 2>&1
```

---

## 5. Policy Engine Admin

### 5.1 Policy Engine Health
```bash
curl -s http://localhost:9002/health | jq
```

### 5.2 Config Dump — View All Routes and Policies
```bash
curl -s http://localhost:9002/config_dump | jq
```

**Useful for verifying:**
- A route exists with `route_key: "POST|/compress-test/post|*"`
- `requires_request_body: true` is set
- The policy chain contains the correct policies and parameters

### 5.3 xDS Sync Status
```bash
curl -s http://localhost:9002/xds_sync_status | jq
```

---

## 6. Envoy Admin

### 6.1 Envoy Config Dump (Full)
```bash
curl -s http://localhost:9901/config_dump | jq . | head -100
```

### 6.2 Envoy Stats
```bash
curl -s http://localhost:9901/stats | grep ext_proc
```

### 6.3 Envoy Clusters
```bash
curl -s http://localhost:9901/clusters | grep -E "upstream|compress"
```

### 6.4 Envoy Listeners
```bash
curl -s "http://localhost:9901/config_dump?resource=dynamic_listeners" | jq '.configs[].dynamic_listeners[].name'
```

---

## 7. Postman Collection

To import into Postman, create a new collection with these environment variables:

| Variable | Value |
|---|---|
| `controller_url` | `http://localhost:9090` |
| `runtime_url` | `http://localhost:8080` |
| `policy_engine_url` | `http://localhost:9002` |
| `envoy_admin_url` | `http://localhost:9901` |
| `api_id` | `prompt-compression-test` |
| `controller_user` | `admin` |
| `controller_pass` | `admin` |

Then use `{{controller_url}}/apis` etc. in your requests.

Set the **Authorization** tab on the collection root to:
- Type: `Basic Auth`
- Username: `{{controller_user}}`
- Password: `{{controller_pass}}`

---

## 8. Quick Smoke Test Script

Run this sequence to verify the full gateway stack is working:

```bash
#!/usr/bin/env bash
set -e

echo "=== 1. Controller Health ==="
curl -sf http://localhost:9090/health | jq .

echo "=== 2. Policy Engine Health ==="
curl -sf http://localhost:9002/health | jq .

echo "=== 3. List APIs ==="
curl -sf -u admin:admin http://localhost:9090/apis | jq '.apis[].id'

echo "=== 4. Prompt Compression — Long Input ==="
RESPONSE=$(curl -sf -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/test-prompt-long.json)

ORIGINAL=$(echo '{"model":"gpt-4","messages":[{"role":"user","content":"placeholder"}]}' | wc -c)
COMPRESSED=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d['body']))")
echo "Body sent to upstream: ${COMPRESSED} bytes (original request: $(wc -c < /tmp/test-prompt-long.json) bytes)"
echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); body=json.loads(d['body']); print('Compressed content:', body['messages'][-1]['content'][:120], '...')"

echo "=== 5. Prompt Compression — Short Input (no compression) ==="
curl -sf -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"What is the capital of France?"}]}' \
  | python3 -c "import sys,json; d=json.load(sys.stdin); body=json.loads(d['body']); print('Content unchanged:', body['messages'][-1]['content'])"

echo "=== All checks passed ==="
```

Save as `smoke-test.sh`, then run: `chmod +x smoke-test.sh && ./smoke-test.sh`
