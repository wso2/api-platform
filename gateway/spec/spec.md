# API Gateway Specification

## Overview

This document serves as the main entry point for the API Platform Gateway specification. The complete specification is organized into separate documents covering different aspects of the gateway.

For a comprehensive project overview, see [README.md](./README.md).

---

## Specification Structure

### 1. [README](./README.md)
Project overview and quick start guide:
- Key components and technology stack
- Gateway tiers (Basic vs Standard)
- Core features and architecture highlights
- Development commands and workflows
- Current status and roadmap

### 2. [Product Requirements](./prd.md)
Functional and non-functional requirements:
- Core value proposition
- Feature breakdown (FR1-FR8)
- Success criteria and user personas
- Deployment tiers and use cases
- Roadmap and phases

### 3. [Architecture](architecture/architecture.md)
Detailed architectural design including:
- Core components (Gateway Controller, Router, Policy Engine, Rate Limiter)
- Component diagrams
- Deployment architecture (Basic vs Standard)
- Multi-tenancy architecture for Bijira platform
- Non-functional requirements (Performance, Scalability, Reliability, Security, Operability)

### 4. [Design](design/design.md)
Design decisions and implementation details:
- Operation modes (Offline and Hybrid)
- API deployment patterns (Top-down and Bottom-up)
- Policy architecture
- Gateway control plane design
- Persistence layer design (Redis and SQLite)
- Future considerations and enhancements

### 5. [Implementation Guide](./impl.md)
Development and build procedures:
- Development environment setup
- Project structure and code organization
- Build commands and workflows
- Testing strategy (unit and integration)
- Deployment procedures
- Troubleshooting and debugging

### 6. [Use Cases](use-cases/use_cases.md)
Practical deployment scenarios:
- Development environment setup
- Enterprise production deployment
- Multi-tenant SaaS platform (Free tier)
- Multi-tenant SaaS platform (Paid tier)
- CI/CD automated deployment
- Hybrid cloud deployment
- Edge/IoT gateway scenarios

---

## Quick Reference

| Gateway Type | Components | Best For |
|--------------|------------|----------|
| **Basic** | Router + Policy Engine + Gateway Controller (no persistence) | Development, testing, free tier (14-day trial) |
| **Standard** | All components + Redis + Advanced rate limiting + SQLite (can switch to External DB) | Production, enterprise deployments, paid tier |

---

## Detailed Documentation

### Product Requirements (prds/)
- [API Configuration Management](prds/api-configuration-management.md)
- [xDS Server Implementation](prds/xds-server.md)
- Additional functional requirements (see [prd.md](./prd.md))

### Implementation Guides (impls/)
- [Gateway-Controller xDS Implementation](impls/gateway-controller-xds.md)
- [API Configuration Lifecycle](impls/api-configuration-lifecycle.md)
- Additional implementation details (see [impl.md](./impl.md))

---

## Related Specifications

**Feature 001**: Gateway with Controller and Router
Location: `specs/001-gateway-has-two/`
Documents:
- [Feature Specification](../../specs/001-gateway-has-two/spec.md)
- [Research and Technical Decisions](../../specs/001-gateway-has-two/research.md)
- [Implementation Plan](../../specs/001-gateway-has-two/plan.md)
- [Data Model](../../specs/001-gateway-has-two/data-model.md)
- [Quickstart Guide](../../specs/001-gateway-has-two/quickstart.md)

---

**Document Version**: 2.0
**Last Updated**: 2025-10-13
**Status**: Active Development
