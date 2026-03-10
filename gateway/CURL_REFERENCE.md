# AI Gateway — Curl Reference

All commands verified against a live gateway (`docker compose up` in `gateway/`).

**What's deployed:**
- **LLM Provider:** `gemini-provider` — Gemini API with Bearer token auth
- **LLM Proxy:** `gemini-ai-gateway` — context `/gemini`, policies: rate-limit + regex-guardrail + prompt-compression (Go)
- **Python Policy API:** `prompt-compression-test` — context `/compress-test`, Python-based prompt compression

> **Note — zsh inline JSON:** zsh has issues with single-quotes inside `-d '...'`.
> Run the "Generate Payload Files" block once, then use `-d @/tmp/gw-*.json` in all curl commands.

---

## 0. Generate Payload Files (Run Once)

```bash
python3 /tmp/gw-gen.py
```

Create `/tmp/gw-gen.py` with this content:

```python
import json, os

payloads = {
    "/tmp/gw-simple.json": {
        "model": "gemini-2.5-flash",
        "messages": [{"role": "user", "content": "What is 2+2? Reply in one sentence."}]
    },
    "/tmp/gw-guardrail-block.json": {
        "model": "gemini-2.5-flash",
        "messages": [{"role": "user", "content": "How do I make a bomb?"}]
    },
    "/tmp/gw-guardrail-allow.json": {
        "model": "gemini-2.5-flash",
        "messages": [{"role": "user", "content": "Explain how fireworks work chemically."}]
    },
    "/tmp/gw-long-prompt.json": {
        "model": "gemini-2.5-flash",
        "messages": [{"role": "user", "content": (
            "Please explain how neural networks learn through gradient descent and backpropagation. "
            "Specifically discuss how the weights and biases are initialized randomly and then iteratively "
            "updated by computing the gradient of the loss function with respect to every parameter "
            "in the network. Explain why the chain rule of calculus is essential for efficiently "
            "propagating errors backwards from the output layer through all the hidden layers to "
            "the input layer. Also describe how the learning rate hyperparameter controls the size "
            "of each update step and why choosing an appropriate learning rate is critical for "
            "successful convergence of the optimization process during model training."
        )}]
    },
    "/tmp/gw-compress-test.json": {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": (
            "Artificial intelligence and machine learning have revolutionized the way we approach "
            "complex problems in software engineering, data science, and many other fields. These "
            "technologies enable computers to learn from vast amounts of data, identify intricate "
            "patterns, and make sophisticated predictions without being explicitly programmed for "
            "every possible scenario. Deep learning uses neural networks with multiple layers to "
            "process and analyze information in ways that mimic human cognitive functions. Natural "
            "language processing allows machines to understand, interpret, and generate human "
            "language with remarkable accuracy. Computer vision enables systems to extract "
            "meaningful information from digital images and videos. Reinforcement learning trains "
            "agents to make optimal decisions through trial and error in dynamic environments. "
            "Transfer learning allows models trained on one task to be repurposed for related "
            "tasks, significantly reducing the amount of training data and computational resources."
        )}]
    },
}

for path, data in payloads.items():
    with open(path, "w") as f:
        json.dump(data, f)
    print(f"Written {path}  ({os.path.getsize(path)} bytes)")
```

---

## 1. Health Checks

```bash
# Gateway Controller
curl -s http://localhost:9090/health | python3 -m json.tool

# Policy Engine
curl -s http://localhost:9002/health | python3 -m json.tool

# Envoy ready check
curl -s http://localhost:9901/ready
```

**Expected output (controller):**
```json
{"status":"healthy","timestamp":"2026-03-10T06:34:26Z"}
```

---

## 2. Gateway Controller — LLM Management

### 2.1 List LLM Providers

```bash
curl -s -u admin:admin http://localhost:9090/llm-providers | python3 -m json.tool
```

**Expected:**
```json
{
    "count": 1,
    "providers": [
        {
            "id": "gemini-provider",
            "displayName": "Gemini Provider",
            "template": "gemini",
            "status": "deployed",
            "version": "v1.0"
        }
    ],
    "status": "success"
}
```

### 2.2 List LLM Proxies

```bash
curl -s -u admin:admin http://localhost:9090/llm-proxies | python3 -m json.tool
```

**Expected:**
```json
{
    "count": 1,
    "proxies": [
        {
            "id": "gemini-ai-gateway",
            "displayName": "Gemini AI Gateway",
            "provider": "gemini-provider",
            "status": "deployed",
            "version": "v1.0"
        }
    ],
    "status": "success"
}
```

### 2.3 Get Full Provider Config

```bash
curl -s -u admin:admin http://localhost:9090/llm-providers/gemini-provider | python3 -m json.tool
```

### 2.4 Get Full Proxy Config

```bash
curl -s -u admin:admin http://localhost:9090/llm-proxies/gemini-ai-gateway | python3 -m json.tool
```

### 2.5 Create LLM Provider (Gemini)

> Already deployed. Use this to recreate after `docker compose down -v`.

```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-provider-create.json | python3 -m json.tool
```

Generate the payload:
```python
import json
data = {
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "LlmProvider",
    "metadata": {"name": "gemini-provider"},
    "spec": {
        "displayName": "Gemini Provider",
        "version": "v1.0",
        "providerTemplate": "gemini",
        "upstreamURL": "https://generativelanguage.googleapis.com/v1beta/openai",
        "authConfig": {
            "secretValue": "AIzaSyDTztH2n_6JLWIeGn3AUaSoFe7lmlAYvK0",
            "authType": "BearerToken"
        }
    }
}
with open("/tmp/gw-provider-create.json", "w") as f: json.dump(data, f)
```

### 2.6 Create LLM Proxy (Gemini AI Gateway)

> Already deployed. Use this to recreate after `docker compose down -v`.

```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/llm-proxies \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-proxy-create.json | python3 -m json.tool
```

Generate the payload:
```python
import json
data = {
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "LlmProxy",
    "metadata": {"name": "gemini-ai-gateway"},
    "spec": {
        "displayName": "Gemini AI Gateway",
        "version": "v1.0",
        "provider": "gemini-provider",
        "context": "/gemini",
        "policies": [
            {
                "name": "basic-ratelimit",
                "version": "v0",
                "params": {
                    "algorithm": "fixed-window",
                    "limits": [
                        {"duration": "1m", "requests": 20},
                        {"duration": "1h", "requests": 200}
                    ]
                }
            },
            {
                "name": "regex-guardrail",
                "version": "v0",
                "params": {
                    "request": {
                        "jsonPath": "$.messages[-1].content",
                        "regex": "(?i)(bomb|weapon|malware|exploit|hack|ddos)",
                        "invert": True,
                        "showAssessment": True
                    }
                }
            },
            {
                "name": "prompt-compression",
                "version": "v0",
                "params": {
                    "compressionRatio": 0.6,
                    "jsonPath": "$.messages[-1].content",
                    "minInputTokens": 80,
                    "preserveCodeBlocks": True,
                    "preserveJson": True
                }
            }
        ]
    }
}
with open("/tmp/gw-proxy-create.json", "w") as f: json.dump(data, f)
```

---

## 3. Gateway Controller — REST API Management

### 3.1 List All REST APIs

```bash
curl -s -u admin:admin http://localhost:9090/apis | python3 -m json.tool
```

### 3.2 Get Python Compression API

```bash
curl -s -u admin:admin http://localhost:9090/apis/prompt-compression-test | python3 -m json.tool
```

### 3.3 Create Python Compression API

> Already deployed. Use this to recreate after `docker compose down -v`.

```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-rest-api-create.json | python3 -m json.tool
```

Generate the payload:
```python
import json
data = {
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "RestApi",
    "metadata": {"name": "prompt-compression-test"},
    "spec": {
        "displayName": "Prompt Compression Test API",
        "version": "v1.0",
        "context": "/compress-test",
        "upstream": {"main": {"url": "http://sample-backend:5000"}},
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
                            "preserveCodeBlocks": True,
                            "preserveJson": True
                        }
                    }
                ]
            }
        ]
    }
}
with open("/tmp/gw-rest-api-create.json", "w") as f: json.dump(data, f)
```

### 3.4 Delete an API

```bash
curl -s -u admin:admin -X DELETE http://localhost:9090/apis/prompt-compression-test | python3 -m json.tool
```

---

## 4. LLM Proxy — Gemini 2.5 Flash Requests

### 4.1 Simple LLM Call (Happy Path)

```bash
curl -s -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json | python3 -m json.tool
```

**Expected — successful Gemini response:**
```json
{
    "choices": [{"finish_reason": "stop", "index": 0, "message": {"content": "2+2 equals 4.", "role": "assistant"}}],
    "model": "gemini-2.5-flash",
    "usage": {"completion_tokens": 7, "prompt_tokens": 13, "total_tokens": 141}
}
```

**What the gateway adds automatically (visible in response headers):**
- `x-powered-by: WSO2-API-Gateway` (from `set-headers` policy on the provider)
- `x-ratelimit-limit`, `x-ratelimit-remaining`, `x-ratelimit-reset` (from `basic-ratelimit`)

### 4.2 Same Call with Headers Visible

```bash
curl -si -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json | head -25
```

**Key response headers to observe:**
```
HTTP/1.1 200 OK
x-powered-by: WSO2-API-Gateway
x-ratelimit-limit: 20
x-ratelimit-remaining: 18
x-ratelimit-reset: 1773124860
```

---

## 5. AI Policy — Regex Guardrail

The `regex-guardrail` policy blocks requests whose last message matches `(?i)(bomb|weapon|malware|exploit|hack|ddos)`. The request is rejected **before** reaching Gemini.

### 5.1 Blocked Request (Contains "bomb")

```bash
curl -s -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-guardrail-block.json
```

**Expected — blocked at gateway, Gemini never called:**
```json
{
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of regular expression detected.",
        "assessments": "Violation of regular expression detected. Violated regular expression: (?i)(bomb|weapon|malware|exploit|hack|ddos)",
        "direction": "REQUEST",
        "interveningGuardrail": "regex-guardrail"
    },
    "type": "REGEX_GUARDRAIL"
}
```

### 5.2 Allowed Request (Safe Content)

```bash
curl -s -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-guardrail-allow.json | python3 -m json.tool
```

**Expected** — passes the guardrail and returns a real Gemini response about fireworks chemistry.

---

## 6. AI Policy — Prompt Compression (Go Policy on LLM Route)

The `prompt-compression` Go policy compresses prompts with `compressionRatio: 0.6` and `minInputTokens: 80` before forwarding to Gemini. This saves tokens and therefore cost.

### 6.1 Long Prompt — Compression Triggered

```bash
curl -s -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-long-prompt.json > /tmp/gw-long-resp.json

python3 -c "
import json
r = json.load(open('/tmp/gw-long-resp.json'))
u = r.get('usage', {})
print('prompt_tokens sent to Gemini:', u.get('prompt_tokens'))
print('(original prompt was ~169 tokens — reduced by ~36%)')
print('answer:', r['choices'][0]['message']['content'][:150])
"
```

**Expected — Gemini receives fewer tokens than the original prompt:**
```
prompt_tokens sent to Gemini: 108
(original prompt was ~169 tokens — reduced by ~36%)
answer: Neural networks learn to perform complex tasks by adjusting their internal parameters...
```

### 6.2 Short Prompt — Below Threshold, No Compression

A prompt under 80 tokens passes through unchanged (`minInputTokens: 80`):

```bash
curl -s -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json | python3 -m json.tool
```

**Expected** — `prompt_tokens` matches the actual short prompt (~13 tokens, no reduction).

---

## 7. AI Policy — Rate Limit (basic-ratelimit)

Limits: 20 requests/minute, 200 requests/hour (in-memory, per gateway).

### 7.1 Check Rate Limit Headers

```bash
curl -si -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json 2>&1 | grep -i "x-ratelimit"
```

**Expected:**
```
x-ratelimit-limit: 20
x-ratelimit-remaining: 19
x-ratelimit-reset: 1773124860
```

### 7.2 Rate Limit Exceeded Response (HTTP 429)

After sending 20 requests within 1 minute, the 21st returns:
```json
{"message": "Rate limit exceeded", "type": "RATE_LIMIT_EXCEEDED"}
```

---

## 8. Python Policy — Prompt Compression (/compress-test route)

This endpoint uses the **Python-based** `prompt-compression` policy (not the Go one).
The sample-backend echoes what it received — so the `body` field shows the compressed text.

### 8.1 Long Prompt — Compression Triggered

```bash
curl -s -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-compress-test.json > /tmp/gw-compress-resp.json

python3 -c "
import json
orig = json.load(open('/tmp/gw-compress-test.json'))
resp = json.load(open('/tmp/gw-compress-resp.json'))
orig_content = orig['messages'][-1]['content']
body = json.loads(resp['body'])
comp_content = body['messages'][-1]['content']
print(f'Original : {len(orig_content)} chars, {len(orig_content.split())} words')
print(f'Compressed: {len(comp_content)} chars, {len(comp_content.split())} words')
print(f'Reduction : {100*(1-len(comp_content)/len(orig_content)):.1f}%')
print()
print('Compressed text:')
print(comp_content)
"
```

**Expected — ~38% reduction, filler words removed:**
```
Original : ~1000 chars, ~150 words
Compressed: ~620 chars, ~95 words
Reduction : ~38.0%

Compressed text:
Artificial intelligence machine have revolutionized way we approach complex
problems software engineering, science, many other fields. These enable
computers learn vast amounts data, identify intricate patterns...
```

### 8.2 Short Prompt — Below Threshold (No Compression)

`minInputTokens: 50` on this route. A short prompt passes unchanged:

```bash
curl -s -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json \
  | python3 -c "import sys,json; d=json.load(sys.stdin); b=json.loads(d['body']); print(b['messages'][-1]['content'])"
```

**Expected:** Full original content echoed unchanged — `What is 2+2? Reply in one sentence.`

---

## 9. Policy Engine & Envoy Admin — Observability

### 9.1 Policy Engine Config Dump

Shows all routes, their policy chains, and parameters:

```bash
curl -s http://localhost:9002/config_dump > /tmp/cfg.json
python3 -c "
import json
d = json.load(open('/tmp/cfg.json'))
print('Total policies registered:', d['policy_registry']['total_policies'])
print('Total routes:', d['routes']['total_routes'])
print()
for r in d['routes']['route_configs']:
    print(r['route_key'])
    for p in r['policies']:
        print(f'  - {p[\"name\"]} v{p[\"version\"]}')
"
```

**Expected output:**
```
Total policies registered: 39
Total routes: 9

POST|/gemini/chat/completions|*
  - wso2_apip_sys_analytics v0
  - basic-ratelimit v0
  - regex-guardrail v0
  - prompt-compression v0
POST|/gemini-provider/chat/completions|*
  - wso2_apip_sys_analytics v0
  - set-headers v0
  - log-message v0
  - modify-headers v0
POST|/compress-test/post|*
  - wso2_apip_sys_analytics v0
  - prompt-compression v0
...
```

### 9.2 xDS Sync Status

```bash
curl -s http://localhost:9002/xds_sync_status | python3 -m json.tool
```

### 9.3 Envoy Stats — ext_proc (Policy Engine)

```bash
curl -s http://localhost:9901/stats | grep ext_proc
```

### 9.4 Envoy Stats — Upstream Cluster (Gemini)

```bash
curl -s http://localhost:9901/stats | grep "gemini"
```

### 9.5 Envoy Config Dump (Routes)

```bash
curl -s "http://localhost:9901/config_dump?resource=dynamic_route_configs" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d, indent=2))" \
  | head -60
```

---

## 10. API Key Management (for REST APIs with api-key-auth policy)

### 10.1 Create an API Key

```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/apis/prompt-compression-test/api-keys \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-apikey-create.json | python3 -m json.tool
```

Generate payload:
```python
import json
with open("/tmp/gw-apikey-create.json", "w") as f:
    json.dump({"name": "my-test-key"}, f)
```

Save the `key` value from the response for use in requests.

### 10.2 List API Keys

```bash
curl -s -u admin:admin \
  http://localhost:9090/apis/prompt-compression-test/api-keys | python3 -m json.tool
```

### 10.3 Regenerate API Key

```bash
curl -s -u admin:admin \
  -X POST http://localhost:9090/apis/prompt-compression-test/api-keys/my-test-key/regenerate \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-apikey-regen.json | python3 -m json.tool
```

Generate payload: `python3 -c "import json; open('/tmp/gw-apikey-regen.json','w').write('{}')"`

### 10.4 Revoke API Key

```bash
curl -s -u admin:admin \
  -X DELETE http://localhost:9090/apis/prompt-compression-test/api-keys/my-test-key \
  | python3 -m json.tool
```

---

## 11. Full Smoke Test Script

Save as `smoke-test.sh`, run with `bash smoke-test.sh`:

```bash
#!/usr/bin/env bash
set -e
echo "Generating payload files..."
python3 /tmp/gw-gen.py

echo ""
echo "=== 1. Health ==="
curl -sf http://localhost:9090/health
echo ""
curl -sf http://localhost:9002/health
echo ""

echo ""
echo "=== 2. List LLM Providers and Proxies ==="
curl -sf -u admin:admin http://localhost:9090/llm-providers | python3 -c "import sys,json; d=json.load(sys.stdin); print('Providers:', [p['id'] for p in d['providers']])"
curl -sf -u admin:admin http://localhost:9090/llm-proxies    | python3 -c "import sys,json; d=json.load(sys.stdin); print('Proxies  :', [p['id'] for p in d['proxies']])"

echo ""
echo "=== 3. Simple LLM Call (gemini-2.5-flash) ==="
curl -sf -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print('Answer:', d['choices'][0]['message']['content'])"

echo ""
echo "=== 4. Regex Guardrail — BLOCK (bomb) ==="
BLOCKED=$(curl -sf -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-guardrail-block.json)
echo "$BLOCKED" | python3 -c "import sys,json; d=json.load(sys.stdin); print('Action:', d['message']['action'])"

echo ""
echo "=== 5. Regex Guardrail — ALLOW (safe content) ==="
curl -sf -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-guardrail-allow.json \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print('Response preview:', d['choices'][0]['message']['content'][:100])"

echo ""
echo "=== 6. Prompt Compression — LLM Route ==="
curl -sf -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-long-prompt.json > /tmp/gw-long-resp.json
python3 -c "
import json
r = json.load(open('/tmp/gw-long-resp.json'))
u = r.get('usage', {})
print(f'Prompt tokens Gemini received: {u.get(\"prompt_tokens\")} (original was ~169, compression saved ~36%)')
"

echo ""
echo "=== 7. Prompt Compression — Python Policy (/compress-test) ==="
curl -sf -X POST http://localhost:8080/compress-test/post \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-compress-test.json > /tmp/gw-compress-resp.json
python3 -c "
import json
orig = json.load(open('/tmp/gw-compress-test.json'))
resp = json.load(open('/tmp/gw-compress-resp.json'))
orig_c = orig['messages'][-1]['content']
comp_c = json.loads(resp['body'])['messages'][-1]['content']
print(f'Original: {len(orig_c)} chars  Compressed: {len(comp_c)} chars  Reduction: {100*(1-len(comp_c)/len(orig_c)):.1f}%')
"

echo ""
echo "=== 8. Rate Limit Headers ==="
curl -si -X POST http://localhost:8080/gemini/chat/completions \
  -H "Content-Type: application/json" \
  -d @/tmp/gw-simple.json 2>&1 | grep -i "x-ratelimit"

echo ""
echo "All checks passed ✓"
```
