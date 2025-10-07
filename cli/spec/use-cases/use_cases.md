# CLI Use Cases

## 1. Overview

Common command-line usage scenarios.

---

## 2. Local Development

### Scenario
Developer deploying API to local gateway

### Commands
```bash
# Push API definition
api-platform gateway push --file api.yaml

# Generate API key for testing
api-platform gateway api-key generate \
  --api-name 'My API' \
  --key-name 'dev-key'

# Test API
curl http://localhost:8081/myapi -H 'api-key: <generated-key>'
```

---

## 3. CI/CD Pipeline

### Scenario
Automated API deployment in CI/CD

### Workflow
```bash
# Authenticate
api-platform login --api-key $CI_API_KEY

# Validate API spec
api-platform api validate --file api.yaml

# Deploy to gateway
api-platform gateway push \
  --file api.yaml \
  --gateway production-gateway

# Verify deployment
api-platform gateway status --name production-gateway
```

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
