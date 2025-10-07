# API Gateway Architecture

## 1. Overview

This document details the architectural design of the API Platform Gateway, including core components, deployment models, and multi-tenancy support.

---

## 2. Core Components

The API Gateway consists of the following runtime components:

### 2.1 Runtime Components
- **Gateway Controller**: Manages gateway configuration and lifecycle
- **Router**: Handles traffic routing and request forwarding
  - Built on Envoy Proxy
  - Acts as the proxy layer for securing and routing API traffic
  - Handles basic proxy features (routing, TLS termination)
- **Policy Engine**: Executes policy decisions and enforcement
- **Rate Limiter**: Provides advanced rate limiting capabilities (Standard deployment only)

### 2.2 Persistence Layer
- **Redis**: Distributed cache for rate limiting counters and quota management (Standard deployment only)
- **SQLite**: Default storage for APIs, applications, and subscriptions (included in both Basic and Standard)
- **External Database**: Can replace SQLite in Standard deployment (PostgreSQL, Oracle, MySQL, MSSQL)

### 2.3 Policy-First Architecture
**Core Principle**: Everything beyond basic proxy operations (routing, TLS termination) is implemented as a policy

**Policy Categories**:
- **Authentication policies**: API key, OAuth, JWT, etc.
- **Authorization policies**: Role-based access, scope validation
- **Rate limiting policies**: Quota management, throttling
- **Analytics policies**: Usage tracking, logging
- **Custom/extensible policies**: Framework for organization-specific requirements

**Implementation**: Policy engine as a separate component enabling flexible, pluggable functionality

---

## 3. Component Diagram

```
+-----------------------------------------------------------+
|                     API Gateway                           |
|                                                           |
|  +--------------+  +--------------+  +--------------+     |
|  |   Gateway    |  |    Router    |  |    Policy    |     |
|  |  Controller  |  |              |  |    Engine    |     |
|  +--------------+  +--------------+  +--------------+     |
|                                                           |
|  +--------------+  +--------------+  +--------------+     |
|  |     Rate     |  |    Redis     |  |    SQLite    |     |
|  |   Limiter    |  |  (Optional)  |  |              |     |
|  +--------------+  +--------------+  +--------------+     |
+-----------------------------------------------------------+
```

---

## 4. Deployment Architecture

### 4.1 Gateway Types

| Component | Basic | Standard |
|-----------|-------|----------|
| **Router** | [Yes] Included | [Yes] Included |
| **Policy Engine** | [Yes] Included | [Yes] Included |
| **Gateway Controller** | [Yes] Included | [Yes] Included |
| **Rate Limiter** | [No] Not included | [Yes] Included |
| **Rate Limiting** | Basic (built into Router) | Advanced (dedicated Rate Limiter + Redis) |
| **Artifact Persistence** | [No] No persistence of gateway artifacts | SQLite (can be switched to External DB: PostgreSQL, Oracle, MySQL, MSSQL) |
| **Availability** | Freemium users on Bijira platform | Paid tier / Self-hosted |
| **Best For** | Lightweight usage, simple traffic control | Enterprise-grade deployments needing advanced rate limiting & durability |

### 4.2 Deployment Options

#### Standard Deployment
- Includes all gateway runtime components
- Includes Redis for rate limiting
- Includes SQLite persistence (can be switched to external database)
- Deployed using Docker Compose
- Suitable for production environments requiring advanced features

#### Basic Deployment
- Includes Router, Policy Engine, and Gateway Controller only
- No persistence of gateway artifacts
- Lightweight footprint
- Available for freemium users on Bijira platform
- Suitable for development, testing, or simple use cases

---

## 5. Multi-Tenancy Architecture (Bijira Platform)

### 5.1 Gateway per Customer Model
- Each customer receives a dedicated gateway instance
- Resource allocation per gateway:
  - Memory: 360-720 MB
  - CPU: 180 mCPU
- Infrastructure options:
  - **Standard_D16s_v5 (16vCPU, 64GiB)**: ~$384/month, supports ~90 gateways
  - **Standard_D16ls_v5 (16vCPU, 32GiB)**: ~$332/month, supports ~90 gateways

### 5.2 Sub-Organization Support
Two architectural patterns:
1. **Gateway per Sub-Org**: No tenancy required at gateway level
2. **Shared Gateway**: Gateway handles multiple organizations with built-in tenancy

### 5.3 Multi-Environment Support
- Support for multiple gateways per customer
- Gateway groups per environment (dev, staging, production)

### 5.4 AI/Agentic Flow Support
The gateway is optimized for AI agent interactions:
- **MCP (Model Context Protocol) compatibility**: Enables seamless integration with AI agents
- **Lightweight architecture**: Minimal footprint suitable for agent-driven deployments
- **Rapid provisioning**: Supports quick deployment for AI workflows
- **Single-tenant mode**: Runs in isolation for each customer/agent

---

## 6. Non-Functional Requirements

### 6.1 Performance
- Support for high-throughput API traffic
- Low-latency request routing
- Efficient rate limiting algorithm
- Minimal memory footprint (360-720 MB per instance)

### 6.2 Scalability
- Horizontal scaling via multiple gateway instances
- Support for 90+ gateways per node
- Redis cluster for distributed state
- Multi-environment deployment

### 6.3 Reliability
- High availability through Redis Sentinel/Cluster
- Persistent storage options
- Graceful degradation without Redis
- Automatic failover capabilities

### 6.4 Security
- Optional API Key authentication for control plane
- Secure gateway registration
- Policy-based access control
- Support for mTLS and certificate management

### 6.5 Operability
- Docker Compose deployment
- CLI-based management (CTL)
- Monitoring and observability hooks
- Configuration as code support

### 6.6 Deployment Flexibility
- **Cross-platform support**: Runs on VMs, containers, and Kubernetes
- **Immutable infrastructure**: Compatible with immutable deployment patterns
- **Single-tenant mode**: Each gateway instance operates in isolation
- **Multiple deployment targets**: Cloud, on-premises, or hybrid configurations

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
