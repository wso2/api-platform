# API Platform Gateway

A lightweight, policy-first API Gateway built on Envoy Proxy, designed for modern API management with support for multi-tenancy, AI/agentic workflows, and flexible deployment options.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Architecture](#architecture)
  - [Core Components](#core-components)
  - [Component Diagram](#component-diagram)
- [Deployment Options](#deployment-options)
  - [Standard Deployment](#standard-deployment)
  - [Basic Deployment](#basic-deployment)
  - [Comparison](#comparison)
- [Multi-Tenancy Support](#multi-tenancy-support)
- [AI/Agentic Flow Support](#aiagentic-flow-support)

---

## Overview

The API Platform Gateway provides a robust, scalable solution for API traffic management with a policy-first architecture. Built on Envoy Proxy, it offers flexible deployment options ranging from lightweight basic instances to enterprise-grade standard deployments with advanced rate limiting and persistence capabilities.

## Key Features

- **Policy-First Architecture**: All functionality beyond basic proxying is implemented as pluggable policies
- **Flexible Deployment**: Available in Basic (lightweight) and Standard (enterprise) configurations
- **Multi-Tenancy**: Dedicated gateway instances per customer with efficient resource utilization
- **AI-Ready**: Optimized for AI agent interactions with MCP compatibility
- **Scalable**: Support for 90+ gateway instances per node
- **Database Options**: SQLite by default, with support for PostgreSQL, Oracle, MySQL, and MSSQL
- **Advanced Rate Limiting**: Dedicated rate limiter component with Redis backing (Standard)

---

## Architecture

### Core Components

The gateway consists of the following runtime components:

#### Runtime Components
- **Gateway Controller**: Manages gateway configuration and lifecycle
- **Router**: Handles traffic routing and request forwarding
  - Built on Envoy Proxy
  - Acts as the proxy layer for securing and routing API traffic
  - Handles basic proxy features (routing, TLS termination)
- **Policy Engine**: Executes policy decisions and enforcement
- **Rate Limiter**: Provides advanced rate limiting capabilities (Standard deployment only)

#### Persistence Layer
- **Redis**: Distributed cache for rate limiting counters and quota management (Standard deployment only)
- **SQLite**: Default storage for APIs, applications, and subscriptions
- **External Database**: Optional replacement for SQLite (PostgreSQL, Oracle, MySQL, MSSQL)

#### Policy-First Architecture

**Core Principle**: Everything beyond basic proxy operations (routing, TLS termination) is implemented as a policy

**Policy Categories**:
- **Authentication policies**: API key, OAuth, JWT, etc.
- **Authorization policies**: Role-based access, scope validation
- **Rate limiting policies**: Quota management, throttling
- **Analytics policies**: Usage tracking, logging
- **Custom/extensible policies**: Framework for organization-specific requirements

**Implementation**: Policy engine as a separate component enabling flexible, pluggable functionality

### Component Diagram

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

## Deployment Options

### Standard Deployment

Enterprise-grade deployment with full feature set:

- All gateway runtime components included
- Redis for distributed rate limiting
- SQLite persistence (switchable to external database)
- Docker Compose deployment
- Suitable for production environments
- Advanced rate limiting with dedicated Rate Limiter component
- Available for paid tier and self-hosted installations

**Best For**: Enterprise-grade deployments requiring advanced rate limiting and data persistence

### Basic Deployment

Lightweight deployment for simple use cases:

- Includes Router, Policy Engine, and Gateway Controller only
- No persistence of gateway artifacts
- Basic rate limiting built into Router
- Minimal footprint
- Available for freemium users on Bijira platform
- Ideal for development and testing

**Best For**: Lightweight usage, development, testing, or simple traffic control

### Comparison

| Component | Basic | Standard |
|-----------|-------|----------|
| **Router** | ✓ Included | ✓ Included |
| **Policy Engine** | ✓ Included | ✓ Included |
| **Gateway Controller** | ✓ Included | ✓ Included |
| **Rate Limiter** | ✗ Not included | ✓ Included |
| **Rate Limiting** | Basic (built into Router) | Advanced (dedicated Rate Limiter + Redis) |
| **Artifact Persistence** | ✗ No persistence | SQLite (switchable to PostgreSQL, Oracle, MySQL, MSSQL) |
| **Availability** | Freemium users on Bijira platform | Paid tier / Self-hosted |

---

## Multi-Tenancy Support

### Gateway per Customer Model

The Bijira platform provides dedicated gateway instances for each customer:

- **Resource Allocation**:
  - Memory: 360-720 MB per gateway
  - CPU: 180 mCPU per gateway

- **Infrastructure Options**:
  - **Standard_D16s_v5** (16vCPU, 64GiB): ~$384/month, supports ~90 gateways
  - **Standard_D16ls_v5** (16vCPU, 32GiB): ~$332/month, supports ~90 gateways

### Sub-Organization Support

Two architectural patterns available:

1. **Gateway per Sub-Org**: Each sub-organization gets a dedicated gateway (no tenancy required at gateway level)
2. **Shared Gateway**: Single gateway handles multiple organizations with built-in tenancy support

### Multi-Environment Support

- Support for multiple gateways per customer
- Gateway groups per environment (dev, staging, production)
- Independent scaling and configuration per environment

---

## AI/Agentic Flow Support

The gateway is optimized for AI agent interactions:

- **MCP (Model Context Protocol) compatibility**: Enables seamless integration with AI agents
- **Lightweight architecture**: Minimal footprint suitable for agent-driven deployments
- **Rapid provisioning**: Supports quick deployment for AI workflows
- **Single-tenant mode**: Runs in isolation for each customer/agent
