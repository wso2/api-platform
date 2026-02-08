# Quickstart: Envoy Policy Engine

**Feature**: 001-policy-engine | **Date**: 2025-11-18

## Prerequisites

- Docker and Docker Compose
- Go 1.23+ (for custom policy development)
- make (optional, for convenience commands)

## Important Architecture Note

**The Policy Engine runtime ships with ZERO policies by default.** ALL policies (including sample/reference policies) must be compiled into the binary using the Policy Engine Builder. This ensures minimal attack surface and allows you to include only the policies you need.

## Quick Start: Build Policy Engine with Sample Policies

### 1. Create Policy Manifest

```bash
# Clone repository (only for sample policies)
git clone <repository-url>
cd policy-engine

# Create policy manifest file that declares which policies to compile
cat > policy-manifest.yaml <<EOF
version: v1
policies:
  - name: SetHeader
    filePath: ./policies/set-header/v1.0.0

  - name: JWTValidation
    filePath: ./policies/jwt-validation/v1.0.0

  - name: ApiKeyValidation
    filePath: ./policies/api-key-validation/v1.0.0
EOF
```

### 2. Build Policy Engine Binary with Sample Policies

```bash
# Build Policy Engine with policies declared in manifest
# The Builder image CONTAINS the policy engine framework source code
# You provide the manifest file and mount the policy directories
docker run --rm \
    -v $(pwd)/policy-manifest.yaml:/policy-manifest.yaml \
    -v $(pwd)/policies:/policies \
    -v $(pwd)/output:/output \
    -e BUILD_VERSION=v1.0.0 \
    policy-engine-builder:v1.0.0 \
    --manifest /policy-manifest.yaml

# Expected output:
# ✅ Loaded manifest: 3 policies declared
# ✅ Discovered SetHeader v1.0.0 at ./policies/set-header/v1.0.0
# ✅ Discovered JWTValidation v1.0.0 at ./policies/jwt-validation/v1.0.0
# ✅ Discovered ApiKeyValidation v1.0.0 at ./policies/api-key-validation/v1.0.0
# ✅ Validation passed
# ✅ Code generated (plugin_registry.go, build_info.go)
# ✅ Binary compiled: /output/policy-engine
# ✅ Dockerfile generated: /output/Dockerfile
```

### 3. Build Runtime Docker Image

```bash
cd output

# Build the final runtime image with compiled policies
docker build -t policy-engine:v1.0.0 .

# Verify loaded policies
docker inspect policy-engine:v1.0.0 | \
    jq '.[0].Config.Labels["policy-engine.loaded-policies"]'

# Output: "setHeader:v1.0.0, jwtValidation:v1.0.0, jwtValidation:v2.0.0, apiKeyValidation:v1.0.0, rateLimiting:v1.0.0, securityHeaders:v1.0.0"

cd ..
```

### 4. Start the Development Environment

```bash
# Start Envoy + Policy Engine + Test Backend
docker-compose up -d

# Check services are running
docker-compose ps
```

Expected output:
```
NAME                STATUS              PORTS
envoy-proxy         Up 30 seconds       0.0.0.0:8000->8000/tcp, 0.0.0.0:9000->9000/tcp
policy-engine       Up 30 seconds       0.0.0.0:9001->9001/tcp, 0.0.0.0:9002->9002/tcp
test-backend        Up 30 seconds       8080/tcp
```

### 5. Test Basic Policy Execution

**Test public route (no authentication)**:
```bash
curl -i http://localhost:8000/api/v1/public/health

# Expected: 200 OK with custom header from SetHeader policy
# X-Custom-Header: policy-engine-v1.0.0
```

**Test private route (JWT required)**:
```bash
# Without JWT - should fail
curl -i http://localhost:8000/api/v1/private/users

# Expected: 401 Unauthorized
# {"error": "Unauthorized", "message": "Missing authorization header"}

# With valid JWT - should succeed
JWT="<valid-jwt-token>"
curl -i -H "Authorization: Bearer $JWT" http://localhost:8000/api/v1/private/users

# Expected: 200 OK with upstream response
# X-User-ID header added by JWT policy
```

### 6. Update Policy Configuration (Zero Downtime)

```bash
# Edit configuration
vim configs/xds/route-with-jwt.yaml

# Reload configuration via xDS
curl -X POST http://localhost:9002/reload

# Verify new policies apply immediately without restart
```

### 7. View Logs

```bash
# Policy Engine logs
docker-compose logs -f policy-engine

# Envoy logs
docker-compose logs -f envoy-proxy
```

### 8. Cleanup

```bash
docker-compose down
```

---

## Understanding the Policy Manifest

The `policy-manifest.yaml` manifest is the source of truth for which policies to compile. Key benefits:

- **Explicit declaration**: No directory scanning - you explicitly list what to include
- **Flexible directory structure**: URIs can point anywhere - no naming conventions required
- **Version resolution**: The Builder extracts policy versions from the policy definition
- **Validation**: Builder validates manifest entries match actual policy definitions

**Manifest Structure**:
```yaml
version: v1              # Manifest schema version (required)
policies:
  - name: PolicyName     # Must match name in policy-definition.yaml at filePath
    filePath: ./path/to/policy # Relative or absolute path to policy directory
  - name: SomeOtherPolicy
    gomodule: github.com/org/repo/policies/some@v0.1.0 # hub-based policy
```


## Building Policy Engine with Custom Policies

**Note**: The process above showed building with sample policies. This section shows how to add YOUR OWN custom policies. The Builder process is the same - it compiles any policies (sample or custom) into the runtime framework.

### Step 1: Create Custom Policy

```bash
# Create policy directory
mkdir -p my-policies/custom-auth/v1.0.0
cd my-policies/custom-auth/v1.0.0

# Create policy.yaml
cat > policy.yaml <<EOF
name: customAuth
version: v1.0.0
description: Custom authentication policy
supportsRequestPhase: true
supportsResponsePhase: false
requiresRequestBody: false
requiresResponseBody: false

parameters:
  - name: apiKeyHeader
    type: string
    description: Header containing API key
    required: true
    default: "X-API-Key"
  - name: validKeys
    type: string_array
    description: List of valid API keys
    required: true
    validation:
      minItems: 1
EOF

# Create go.mod
cat > go.mod <<EOF
module github.com/policy-engine/policies/custom-auth

go 1.23

require (
    github.com/policy-engine/sdk v1.0.0
)

replace github.com/policy-engine/sdk => ../../../sdk
EOF

# Create custom_auth.go
cat > custom_auth.go <<'EOF'
package custom_auth

import (
    policy "github.com/policy-engine/sdk/policy/v1alpha"
)

type CustomAuthPolicy struct{}

func GetPolicy(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
    return &CustomAuthPolicy{}, nil
}

func (p *CustomAuthPolicy) Name() string {
    return "customAuth"
}

func (p *CustomAuthPolicy) Validate(config map[string]interface{}) error {
    if _, ok := config["validKeys"]; !ok {
        return fmt.Errorf("missing required parameter: validKeys")
    }
    return nil
}

func (p *CustomAuthPolicy) ExecuteRequest(ctx *policies.RequestContext, config map[string]interface{}) *policies.RequestPolicyAction {
    headerName := config["apiKeyHeader"].(string)
    validKeys := config["validKeys"].([]string)

    apiKeyHeaders := ctx.Headers[strings.ToLower(headerName)]
    if len(apiKeyHeaders) == 0 {
        return &policies.RequestPolicyAction{
            Action: policies.ImmediateResponse{
                StatusCode: 401,
                Headers: map[string]string{"Content-Type": "application/json"},
                Body: []byte(`{"error": "Missing API key"}`),
            },
        }
    }

    apiKey := apiKeyHeaders[0]
    for _, validKey := range validKeys {
        if apiKey == validKey {
            // Valid key - add metadata
            ctx.Metadata["authenticated"] = true
            ctx.Metadata["api_key"] = apiKey
            return nil  // Continue processing
        }
    }

    return &policies.RequestPolicyAction{
        Action: policies.ImmediateResponse{
            StatusCode: 403,
            Headers: map[string]string{"Content-Type": "application/json"},
            Body: []byte(`{"error": "Invalid API key"}`),
        },
    }
}
EOF
```

### Step 2: Build Custom Binary

```bash
# Build using Policy Engine Builder
# The Builder image contains the framework source - you only provide your policies
docker run --rm \
    -v $(pwd)/my-policies:/policies \
    -v $(pwd)/output:/output \
    -e BUILD_VERSION=v1.0.0-custom \
    policy-engine-builder:v1.0.0

# Expected output:
# ✅ Discovered 1 policy (customAuth:v1.0.0)
# ✅ Validation passed
# ✅ Code generated (plugin_registry.go with imports for customAuth)
# ✅ Binary compiled: /output/policy-engine
# ✅ Dockerfile generated: /output/Dockerfile
```

### Step 3: Build Runtime Image

```bash
cd output

# Build final image
docker build -t my-policy-engine:v1.0.0-custom .

# Verify loaded policies
docker inspect my-policy-engine:v1.0.0-custom | \
    jq '.[0].Config.Labels["policy-engine.loaded-policies"]'

# Output: "customAuth:v1.0.0, jwtValidation:v1.0.0, setHeader:v1.0.0, ..."
```

### Step 4: Deploy Custom Engine

```bash
# Update docker-compose.yml to use custom image
# Then restart
docker-compose down
docker-compose up -d
```

---

## Test Scenarios

### Scenario 1: Route-Based Policy Execution
```bash
# Public route - no auth required
curl http://localhost:8000/api/v1/public/status
# Expected: 200 OK (SetHeader policy only)

# Private route - JWT required
curl http://localhost:8000/api/v1/private/users
# Expected: 401 Unauthorized (JWT policy rejects)

curl -H "Authorization: Bearer <valid-jwt>" http://localhost:8000/api/v1/private/users
# Expected: 200 OK (JWT policy validates, adds X-User-ID header)
```

### Scenario 2: Policy Chain Short-Circuit
```bash
# Invalid JWT - chain should stop at JWT validation
curl -H "Authorization: Bearer invalid-token" \
     http://localhost:8000/api/v1/private/users
# Expected: 401 Unauthorized
# Rate limiting policy should NOT execute (short-circuit)
```

### Scenario 3: Conditional Policy Execution
```bash
# GET request - rate limiting skipped (condition: write operations only)
curl -X GET http://localhost:8000/api/v1/data
# Expected: 200 OK (rate limiting policy skipped)

# POST request - rate limiting applies
for i in {1..150}; do
  curl -X POST http://localhost:8000/api/v1/data -d '{"test": true}'
done
# Expected: First 100 succeed, rest get 429 Too Many Requests
```

### Scenario 4: Inter-Policy Communication
```bash
# JWT policy stores user_id in metadata
# Rate limiting policy uses user_id for per-user limits
# Response logging policy reads user_id from metadata

curl -H "Authorization: Bearer <valid-jwt>" http://localhost:8000/api/v1/users

# Check logs to see metadata flow:
docker-compose logs policy-engine | grep user_id
# Should see: user_id extracted by JWT, used by rate limiter, logged in response
```

---

## Configuration Examples

### File-Based Configuration (Development)

```yaml
# configs/policy-engine.yaml
mode: file
config_file: configs/xds/routes.yaml

server:
  extproc_port: 9001
  xds_port: 9002
```

### xDS Configuration (Production)

```yaml
# configs/xds/route-with-jwt.yaml
resources:
  - "@type": type.googleapis.com/envoy.policy.v1.PolicyChainConfig
    metadata_key: "api-v1-private"
    request_policies:
      - name: "jwtValidation"
        version: "v1.0.0"
        enabled: true
        parameters:
          jwksUrl: "https://auth.example.com/.well-known/jwks.json"
          issuer: "https://auth.example.com"
          audiences: ["https://api.example.com"]
          clockSkew: "30s"
      - name: "rateLimiting"
        version: "v1.0.0"
        enabled: true
        execution_condition: 'request.Method in ["POST", "PUT", "DELETE"]'
        parameters:
          requestsPerSecond: 100
          burstSize: 20
          identifierSource: "jwt_claim"
          identifierKey: "sub"
    response_policies:
      - name: "securityHeaders"
        version: "v1.0.0"
        enabled: true
```

---

## Troubleshooting

### Policy Engine Not Starting
```bash
# Check logs
docker-compose logs policy-engine

# Common issues:
# - Invalid policy configuration (check validation errors)
# - Port conflicts (9001, 9002 already in use)
# - Policy registration errors (check plugin_registry.go generation)
```

### Policies Not Executing
```bash
# Verify Envoy can reach Policy Engine
docker-compose exec envoy-proxy nc -zv policy-engine 9001

# Check route configuration matches metadata key
curl http://localhost:9000/config_dump | jq '.configs[] | select(.["@type"] == "type.googleapis.com/envoy.admin.v3.RoutesConfigDump")'

# Verify policy chain loaded for route
curl http://localhost:9002/routes | jq
```

### Invalid Configuration Rejected
```bash
# View validation errors
docker-compose logs policy-engine | grep "validation failed"

# Test configuration before deploying
docker run --rm -v $(pwd)/configs:/configs policy-engine:latest \
    validate --config /configs/xds/routes.yaml
```

---

## Performance Testing

```bash
# Install hey (HTTP load generator)
go install github.com/rakyll/hey@latest

# Test headers-only policy chain (SKIP body mode)
hey -n 10000 -c 100 -m GET \
    -H "Authorization: Bearer <valid-jwt>" \
    http://localhost:8000/api/v1/public/health

# Expected: < 5ms p95 latency

# Test body-requiring policy chain (BUFFERED body mode)
hey -n 10000 -c 100 -m POST \
    -H "Authorization: Bearer <valid-jwt>" \
    -H "Content-Type: application/json" \
    -d '{"test": true}' \
    http://localhost:8000/api/v1/data

# Expected: < 20ms p95 latency
```

---

## Next Steps

1. **Explore Sample Policies**: Review `policies/` directory for reference implementations
2. **Understand Architecture**: The runtime (`src/`) is policy-agnostic - ALL policies (including samples) are compiled via Builder
3. **Create Custom Policies**: Write your own policies following the sample policy patterns
4. **Build Custom Binaries**: Use the Builder to compile only the policies you need (zero policies, sample only, custom only, or any combination)
5. **Configure Routes**: Add policy chains for your API routes in xDS configuration
6. **Monitor Performance**: Use Envoy admin interface at http://localhost:9000
7. **Production Deployment**: Review BUILDER_DESIGN.md for CI/CD integration

**Remember**: The Policy Engine runtime ships with ZERO policies. You control exactly which policies are included by selecting what the Builder compiles.

---

## Resources

- **Full Specification**: See `Spec.md` for complete requirements
- **Implementation Plan**: See `specs/001-policy-engine/plan.md`
- **Builder Design**: See `BUILDER_DESIGN.md`
- **Policy Interface Contracts**: See `specs/001-policy-engine/contracts/policy-api.md`
- **Data Model**: See `specs/001-policy-engine/data-model.md`

---

For issues or questions, see troubleshooting section or check project documentation.
