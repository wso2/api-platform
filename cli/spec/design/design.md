# CLI Design

## 1. Overview

User experience and command design principles for the CLI.

---

## 2. Design Principles

### 2.1 Consistency
- Predictable command patterns
- Standard flag naming conventions
- Uniform output formats

### 2.2 Developer Experience
- Clear error messages
- Helpful command suggestions
- Interactive prompts when needed
- Progress indicators for long operations

---

## 3. Command Examples

### 3.1 Gateway Operations
```bash
# List all gateways
api-platform gateway list

# Push API to gateway
api-platform gateway push --file api.yaml --gateway dev-gateway

# Get gateway status
api-platform gateway status --name dev-gateway
```

### 3.2 API Key Operations
```bash
# Generate API key
api-platform gateway api-key generate \
  --api-name 'Weather API' \
  --key-name 'my-app-key'

# List API keys
api-platform gateway api-key list

# Revoke API key
api-platform gateway api-key revoke --key-id <key-id>
```

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
