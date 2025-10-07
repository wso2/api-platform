# API Gateway Specification

## Overview

This document serves as the main entry point for the API Platform Gateway specification. The complete specification is organized into separate documents covering different aspects of the gateway.

---

## Specification Structure

### 1. [Architecture](architecture/architecture.md)
Detailed architectural design including:
- Core components (Gateway Controller, Router, Policy Engine, Rate Limiter)
- Component diagrams
- Deployment architecture (Basic vs Standard)
- Multi-tenancy architecture for Bijira platform
- Non-functional requirements (Performance, Scalability, Reliability, Security, Operability)

### 2. [Design](design/design.md)
Design decisions and implementation details:
- Operation modes (Offline and Hybrid)
- API deployment patterns (Top-down and Bottom-up)
- Policy architecture
- Gateway control plane design
- Persistence layer design (Redis and SQLite)
- Future considerations and enhancements

### 3. [Use Cases](use-cases/use_cases.md)
Practical deployment scenarios and use cases:
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

## Documentation

- **Source**: `gateway/ap-gw.md`
- **Architecture**: `gateway/spec/architecture/architecture.md`
- **Design**: `gateway/spec/design/design.md`
- **Use Cases**: `gateway/spec/use-cases/use_cases.md`

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
