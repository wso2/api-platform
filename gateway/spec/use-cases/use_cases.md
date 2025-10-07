# API Gateway Use Cases

## 1. Overview

This document outlines the various use cases and deployment scenarios for the API Platform Gateway, including requirements, configurations, and benefits for each scenario.

---

## 2. Development Environment (Basic Gateway)

### Scenario
Developer testing APIs locally

### Requirements
- Lightweight footprint
- Quick startup
- Simple rate limiting
- No persistence needed (ephemeral usage)

### Deployment Configuration
- Basic Gateway type
- No persistence (in-memory only)
- Offline mode operation
- No Redis required

### Benefits
- Minimal resource consumption
- Fast iteration cycles
- No external dependencies

---

## 3. Enterprise Production (Standard Gateway)

### Scenario
Production API management with SLA requirements

### Requirements
- Advanced rate limiting
- High availability
- Distributed quota management
- Persistent counters

### Deployment Configuration
- Standard Gateway type
- Redis Cluster (3 masters + 3 replicas)
- External database persistence
- Hybrid mode with Bijira integration

### Benefits
- Enterprise-grade reliability
- Accurate rate limiting across restarts
- Centralized management

---

## 4. Multi-Tenant SaaS Platform - Free Tier (Bijira)

### Scenario
API Platform serving free tier users with Basic gateway

### Requirements
- Gateway per customer isolation
- Cost optimization
- Freemium tier support
- Time-limited trial access

### Architecture
- Gateway per customer model (Basic gateway type)
- 14-day free trial period
- Standard_D16ls_v5 node (16vCPU, 32GiB) @ $332/month
- 90 gateway instances per node
- Resource allocation: 360 MB / 180 mCPU per gateway

### Gateway Configuration
- Basic Gateway type
- No persistence (in-memory only)
- Local rate limiting only
- No Redis required

### Cost Structure
- Free for 14 days per customer
- ~$44.3 USD per customer per year (infrastructure cost)
- ~$27 USD per day for 100 active free users

### Benefits
- Complete tenant isolation
- Low barrier to entry for new users
- Predictable resource allocation
- Easy upgrade path to paid tier

### Limitations
- No persistence of gateway artifacts
- Limited to basic rate limiting
- 14-day trial period
- Single environment only

---

## 5. Multi-Tenant SaaS Platform - Paid Tier (Bijira)

### Scenario
API Platform serving paid customers with Standard gateway and production-grade features

### Requirements
- Gateway per customer isolation
- Persistent storage for artifacts
- Advanced rate limiting
- Multi-environment support
- High availability
- Production SLA compliance

### Architecture
- Gateway per customer model (Standard gateway type)
- Multiple gateway instances per customer (dev, staging, production)
- Standard_D16s_v5 node (16vCPU, 64GiB) @ $384/month for production workloads
- Resource allocation based on tier:
  - **Starter**: 720 MB / 180 mCPU per gateway
  - **Professional**: 1.5 GB / 500 mCPU per gateway
  - **Enterprise**: Custom allocation

### Gateway Configuration
- Standard Gateway type
- SQLite or external database persistence
- Redis for distributed rate limiting
- Support for multiple environments per customer
- Advanced policy support

### Cost Structure
- Tiered pricing model
- Infrastructure cost + service fee
- Per-gateway pricing with volume discounts
- Additional costs for external database and Redis cluster

### Benefits
- Complete tenant isolation with production-grade features
- Persistent storage for business continuity
- Advanced distributed rate limiting
- Multi-environment deployment (dev/staging/prod)
- Support for sub-organizations
- High availability and failover
- Premium support and SLA guarantees

### Use Case Highlights
- **E-commerce platforms**: Persistent API configurations and subscriptions
- **Financial services**: Compliance-ready with audit trails
- **SaaS providers**: Multi-environment CI/CD workflows
- **Enterprise customers**: Dedicated resources and custom policies

---

## 6. CI/CD Automated Deployment

### Scenario
Automated API deployment pipeline

### Requirements
- Programmatic API deployment
- Version control integration
- Secure control plane access
- Bottom-up deployment pattern

### Configuration
- Secured Gateway Control Plane
- API Key authentication
- CTL integration in CI/CD pipeline

### Workflow
1. Developer commits API definition to Git
2. CI/CD pipeline triggers
3. CTL deploys API to gateway using API Key
4. Gateway stages API for management portal import
5. Management portal validates and promotes

### Benefits
- GitOps-friendly workflow
- Automated testing and deployment
- Audit trail and versioning

---

## 7. Hybrid Cloud Deployment

### Scenario
On-premise gateway with cloud management

### Requirements
- Local API execution
- Remote management capabilities
- Secure registration
- Bi-directional sync

### Configuration
- Hybrid mode operation
- Gateway registered with Bijira
- Both top-down and bottom-up deployment
- Secure control plane connection

### Benefits
- Data sovereignty (on-premise execution)
- Centralized visibility and management
- Flexible deployment options

---

## 8. Edge/IoT Gateway

### Scenario
Lightweight gateway at network edge

### Requirements
- Minimal resource footprint
- Offline operation capability
- Persistence for critical data
- Occasional sync with central management

### Deployment Configuration
- Standard Gateway type (for persistence requirements)
- Offline mode primary operation
- Periodic Hybrid mode sync
- SQLite persistence

### Benefits
- Edge computing support
- Resilient to network failures
- Low infrastructure cost
- Persistent storage for edge scenarios

---

**Document Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
