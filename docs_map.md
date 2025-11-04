# Documentation Navigation Map

This file provides a comprehensive index of all documentation in the API Platform specifications repository, enabling quick navigation and search.

## Quick Topic Index

### API Management
- **API.yaml Specification** → [specs/api-yaml.md](specs/api-yaml.md)
- **Platform API Overview** → [platform-api/docs/README.md](platform-api/docs/README.md)
- **Platform API Features** → [platform-api/docs/features/README.md](platform-api/docs/features/README.md)
- **API Designer** → [api-designer/docs/README.md](api-designer/docs/README.md)
- **Gateway Features** → [gateway/docs/README.md](gateway/docs/README.md)

### Portal Documentation
- **Developer Portal** → [portals/developer-portal/docs/README.md](portals/developer-portal/docs/README.md)
- **Developer Portal Publishing** → [platform-api/docs/features/devportal-publishing/README.md](platform-api/docs/features/devportal-publishing/README.md)
- **Management Portal** → [portals/management-portal/docs/README.md](portals/management-portal/docs/README.md)
- **Enterprise Portal** → [enterprise-portal/docs/README.md](enterprise-portal/docs/README.md)

### Platform Tools
- **CLI Documentation** → [cli/docs/README.md](cli/docs/README.md)
- **Quick Start Guide** → [README.md](README.md#quick-start)

### Platform Concepts
- **Architecture Overview** → [README.md](README.md#platform-architecture)
- **Key Principles** → [README.md](README.md#key-principles)
- **AI-Readiness Features** → [README.md](README.md#ai-readiness-features)
- **Multi-Tenancy** → [platform-api/docs/README.md](platform-api/docs/README.md#key-features)

---

## Complete Documentation Index

### Root Level
- [README.md](README.md) - Platform overview, architecture, quick start, and component summaries
- [specs/api-yaml.md](specs/api-yaml.md) - Declarative API definition format specification

### Component Documentation

#### API Designer
- [api-designer/docs/README.md](api-designer/docs/README.md) - Standalone design tool for creating API specifications

#### Platform API
- [platform-api/docs/README.md](platform-api/docs/README.md) - Backend service providing RESTful APIs and business logic
- [platform-api/docs/features/README.md](platform-api/docs/features/README.md) - Features overview and index

#### Gateway
- [gateway/docs/README.md](gateway/docs/README.md) - Envoy-based API gateway documentation

#### Portals
- [portals/management-portal/docs/README.md](portals/management-portal/docs/README.md) - Central control plane documentation
- [portals/developer-portal/docs/README.md](portals/developer-portal/docs/README.md) - Developer portal for API discovery
- [enterprise-portal/docs/README.md](enterprise-portal/docs/README.md) - Internal asset discovery hub

#### CLI
- [cli/docs/README.md](cli/docs/README.md) - Command-line interface documentation

---

## Common Questions → Documentation Mapping

### "How do I deploy an API?"
1. [README.md](README.md#quick-start) - Quick start guide with deployment example
2. [specs/api-yaml.md](specs/api-yaml.md) - API.yaml specification format
3. [cli/docs/README.md](cli/docs/README.md) - CLI commands for deployment

### "How do I register a gateway?"
1. [README.md](README.md#quick-start) - Hybrid gateway setup
2. [platform-api/docs/features/gateway-management/README.md](platform-api/docs/features/gateway-management/README.md) - Gateway management details
3. [platform-api/docs/features/gateway-management/quickstart.md](platform-api/docs/features/gateway-management/quickstart.md) - Quick start guide

### "How does WebSocket monitoring work?"
1. [platform-api/docs/features/gateway-websocket-connections/README.md](platform-api/docs/features/gateway-websocket-connections/README.md) - WebSocket connections documentation
2. [platform-api/docs/features/gateway-websocket-connections/quickstart.md](platform-api/docs/features/gateway-websocket-connections/quickstart.md) - Quick start

### "How do I publish APIs to the developer portal?"
1. [platform-api/docs/features/devportal-publishing/README.md](platform-api/docs/features/devportal-publishing/README.md) - Developer portal publishing
2. [platform-api/docs/features/devportal-publishing/quickstart.md](platform-api/docs/features/devportal-publishing/quickstart.md) - Quick start guide
3. [portals/developer-portal/docs/README.md](portals/developer-portal/docs/README.md) - Developer portal overview

### "What's the difference between Basic and Standard gateways?"
1. [README.md](README.md#gateway-types-comparison) - Gateway types comparison table
2. [gateway/docs/README.md](gateway/docs/README.md) - Gateway documentation

### "How does multi-tenancy work?"
1. [platform-api/docs/README.md](platform-api/docs/README.md#key-features) - Platform API multi-tenancy features
2. [platform-api/docs/features/gateway-management/README.md](platform-api/docs/features/gateway-management/README.md) - Multi-tenant isolation details

### "What is the API.yaml format?"
1. [specs/api-yaml.md](specs/api-yaml.md) - Complete API.yaml specification
2. [README.md](README.md#quick-start) - Example API.yaml in quick start

### "How do I use the CLI?"
1. [cli/docs/README.md](cli/docs/README.md) - CLI documentation
2. [README.md](README.md#quick-start) - CLI usage examples

### "What are the platform components?"
1. [README.md](README.md#platform-components) - All components overview
2. Component-specific README files (see Complete Documentation Index above)

### "How does the architecture work?"
1. [README.md](README.md#platform-architecture) - High-level architecture
2. [platform-api/docs/README.md](platform-api/docs/README.md#architecture) - Platform API architecture
3. [gateway/docs/README.md](gateway/docs/README.md) - Gateway architecture

### "How do I delete a gateway?"
1. [platform-api/docs/features/gateway-management/README.md](platform-api/docs/features/gateway-management/README.md) - Gateway management

### "What AI features are available?"
1. [README.md](README.md#ai-readiness-features) - AI-readiness features overview
2. [api-designer/docs/README.md](api-designer/docs/README.md) - AI-powered design features
3. [platform-api/docs/README.md](platform-api/docs/README.md#mcp-server) - MCP integration

---

## Navigation Tips

1. **Start here**: [README.md](README.md) for platform overview
2. **For specific features**: Check [platform-api/docs/features/README.md](platform-api/docs/features/README.md)
3. **For examples**: Look at quick start guides in feature directories
4. **For API format**: See [specs/api-yaml.md](specs/api-yaml.md)

---

Last Updated: 2025-11-04