# API Gateway Design

## 1. Overview

This document describes the design decisions, operational modes, deployment patterns, and persistence layer design for the API Platform Gateway.

---

## 2. Operation Modes

### 2.1 Offline Mode
- Run gateway in complete isolation
- Deploy APIs using CTL with api.yaml files
- CTL provides all management operations for offline scenarios
- No connectivity to management plane required

#### Workflow
1. Configure gateway locally
2. Run CTL with API definitions
3. Deploy APIs directly to gateway
4. Gateway operates independently

### 2.2 Hybrid Mode
- Integration with Bijira management platform
- Gateway registration and management through control plane
- Hybrid on-premise and cloud operation

#### Workflow
1. Login to Bijira
2. Create gateway with details (name, hostname)
3. Obtain registration credentials/CLI command
4. Register gateway with control plane
5. Manage through both local CTL and remote management portal

---

## 3. API Deployment Patterns

### 3.1 Top-Down Deployment
- Management Portal initiates deployment
- APIs pushed from central management to gateway
- Centralized control and governance

#### Flow
```
Management Portal --> API Definition --> Gateway Deployment
```

### 3.2 Bottom-Up Deployment
- APIs deployed directly to gateway via CTL
- Staged APIs can be imported to Management Portal
- Developer-centric workflow

#### Flow
```
CTL --> Gateway Deployment --> Management Portal Import
```

---

## 4. Policy Architecture

**Core Principle**: Everything beyond basic proxy operations (routing, TLS termination) is implemented as a policy

### 4.1 Policy-First Design Philosophy
The gateway follows a "everything is a policy" approach where all functionality beyond fundamental proxy capabilities is delivered through the policy engine. This provides:
- **Modularity**: Each capability is isolated and independently deployable
- **Extensibility**: New policies can be added without core changes
- **Flexibility**: Policies can be composed and configured per API or operation

### 4.2 Policy Categories

**Authentication Policies**
- API Key authentication
- OAuth 2.0 / OAuth 1.0a
- JWT validation
- Basic authentication
- Custom authentication schemes

**Authorization Policies**
- Role-based access control (RBAC)
- Scope validation
- Attribute-based access control (ABAC)
- Resource-level permissions

**Rate Limiting Policies**
- Request quota management
- Throttling controls
- Spike arrest
- Distributed rate limiting (Standard gateway)

**Analytics Policies**
- Usage tracking
- Request/response logging
- Performance metrics
- Custom event capture

**Custom/Extensible Policies**
- Framework for organization-specific requirements
- Plugin architecture for policy development
- Policy chaining and composition

### 4.3 Policy Execution Model
- **Policy Engine** acts as the orchestration layer
- Policies are evaluated in a defined order
- Support for request and response phase policies
- Conditional policy execution based on context

### 4.4 Policy Enforcement Points
- Per-API level policies
- Per-operation (resource/method) level policies
- Global gateway-level policies
- Environment-specific policy overrides

---

## 5. Gateway Control Plane

### 5.1 Security Model
- **Default**: Unsecured for ease of setup
- **Production**: Can be secured with API Key authentication
- Use case: CI/CD pipelines pushing APIs directly to gateways

### 5.2 Capabilities
- API Key generation and management
- Gateway configuration management
- API deployment orchestration

---

## 6. Persistence Layer Design

### 6.1 Redis (Rate Limiting)
**Purpose**: Distributed rate limiting and counter management

#### Deployment Options

**1. Single Pod (Non-Critical)**
- Simple setup
- Low footprint
- Suitable for development/testing

**2. Redis Sentinel (Small Production)**
- 1 master + 2 replicas + 3 sentinels
- High availability
- Automatic failover

**3. Redis Cluster (High-Scale)**
- 3 masters + 3 replicas (6 pods minimum)
- Horizontal scalability
- Production-grade reliability

#### Persistence Options
- **In-memory (default)**: Counters reset on restart
- **Persistent Volumes (PVCs)**: Recommended for long-duration rate limits (days, months)
- **Managed Services**: AWS ElastiCache, Azure Cache for Redis, GCP Memorystore

### 6.2 SQLite (Metadata Storage)
**Purpose**: Default storage for gateway artifacts

**Status**: Included in Standard deployment only (Basic has no persistence)

#### Data Stored
- API definitions
- Application configurations
- Subscription information
- Local state management

**Note**: For Standard deployments, SQLite can be switched to external databases (PostgreSQL, Oracle, MySQL, MSSQL) for enhanced scalability and durability.

---

## 7. Future Considerations

### 7.1 Planned Enhancements
- [ ] Policy plugin development SDK
- [ ] Enhanced monitoring and observability
- [ ] Custom extension framework for policies
- [ ] Advanced traffic management capabilities
- [ ] Policy marketplace/registry

### 7.2 Open Questions
- Integration patterns with service mesh
- Multi-region deployment strategies
- Custom protocol support beyond HTTP/gRPC
- Policy versioning and backward compatibility

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
