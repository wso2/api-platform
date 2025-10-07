# CLI Specification

## Overview

The API Platform CLI provides command-line tools for developers and automation workflows.

---

## Specification Structure

### 1. [Architecture](architecture/architecture.md)
CLI architecture and command structure

### 2. [Design](design/design.md)
Command design and user experience

### 3. [Use Cases](use-cases/use_cases.md)
Common CLI usage scenarios

---

## Quick Reference

**Primary Users**: Developers and CI/CD automation

**Key Capabilities**:
- Gateway management
- API deployment
- API key generation
- Configuration management
- CI/CD integration

**Example Commands**:
```bash
# Gateway operations
api-platform gateway list
api-platform gateway push --file api.yaml

# API key management
api-platform gateway api-key generate --api-name 'MyAPI'
```

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
