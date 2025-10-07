# WSO2 API Platform

> AI-ready, GitOps-driven API platform for full lifecycle management across cloud, hybrid, and on-premises deployments.

## Overview

The WSO2 API Platform is a complete platform that helps organizations build AI-ready APIs with comprehensive lifecycle management capabilities. The platform supports deployment on the cloud, fully on-premises, or in hybrid mode.

## Key Principles

- **Developer experience is king**: Optimized workflows and UX for all users
- **Size matters, keep it as small as you can**: Minimal footprint for all components
- **Same control plane/UI experience across cloud and on-premises**: Consistent interface regardless of deployment model
- **Platform components are independent**: No hard dependencies between components
  - Treat each component as a product itself
- **GitOps ready**: Configuration as code for both API configs and gateway configs
  - Separation of Concerns: Spec vs. Execution
- **AI-Ready by design**: Servers are MCP enabled for AI agent integration
- **Docker as the shipping vehicle**: All components distributed via Docker containers
- **API Gateway**:
  - Based on Envoy Proxy
  - Apart from basic proxy features (routing, TLS, etc), everything else is a policy

## Platform Scope

The API Platform covers the complete API lifecycle:

- ✅ API ideation and planning
- ✅ API design and documentation
- ✅ API testing and mocking
- ✅ Runtime management (ingress and egress)
- ✅ API governance and compliance
- ✅ Asset discovery and consumption
- ✅ API analytics and monetization

---

## Platform Components

### 🎨 [API Designer](api-designer/spec/spec.md)
Standalone design tool for REST, GraphQL, and AsyncAPI specifications.

**Key Features:**
- Code + visual split view with real-time updates
- AI-assisted specification and documentation generation
- Built-in mocking and governance checks
- AI-readiness score for APIs
- MCP code generation from specifications

📖 **Documentation:**
- [Architecture](api-designer/spec/architecture/architecture.md)
- [Design](api-designer/spec/design/design.md)
- [Use Cases](api-designer/spec/use-cases/use_cases.md)

---

### ⚙️ [Management Portal](management-portal/spec/spec.md)
Central control plane for managing gateways, APIs, policies, and governance.

**Key Capabilities:**
- Gateway management and orchestration
- API lifecycle management
- Policy and governance rule management
- Identity provider configuration
- API deployment to gateways
- Publishing to developer portals

**Deployment:** Multi-tenant SaaS or on-premises

📖 **Documentation:**
- [Architecture](management-portal/spec/architecture/architecture.md)
- [Design](management-portal/spec/design/design.md)
- [Use Cases](management-portal/spec/use-cases/use_cases.md)

---

### 🚀 [API Gateway](gateway/spec/spec.md)
Envoy-based API gateway for securing and routing API traffic.

**Gateway Types:**
- **Basic**: Lightweight for development, freemium users (14-day trial)
- **Standard**: Production-ready with Redis, persistence, distributed rate limiting

**Key Features:**
- Built on Envoy Proxy
- Policy-first architecture (auth, rate limiting, analytics)
- Runs on VMs, containers, Kubernetes
- Single-tenant mode
- Optimized for AI/agentic flows

📖 **Documentation:**
- [Architecture](gateway/spec/architecture/architecture.md)
- [Design](gateway/spec/design/design.md)
- [Use Cases](gateway/spec/use-cases/use_cases.md)

---

### 🔍 [Enterprise Portal](enterprise-portal/spec/spec.md)
Internal discovery hub for API developers to find and reuse organizational assets.

**Asset Types:**
- Internal and external APIs
- LLM integrations
- Data sources
- Caches and message queues

**Purpose:** Promote reuse and discovery across internal teams

📖 **Documentation:**
- [Architecture](enterprise-portal/spec/architecture/architecture.md)
- [Design](enterprise-portal/spec/design/design.md)
- [Use Cases](enterprise-portal/spec/use-cases/use_cases.md)

---

### 📚 [API Portal](api-portal/spec/spec.md)
Developer portal for API discovery, subscription, and consumption.

**Key Features:**
- API catalog and semantic search
- Try-it console for API testing
- API subscription management
- Application and API key management
- AI-powered discovery

**Primary Users:** Application developers and AI agents

📖 **Documentation:**
- [Architecture](api-portal/spec/architecture/architecture.md)
- [Design](api-portal/spec/design/design.md)
- [Use Cases](api-portal/spec/use-cases/use_cases.md)

---

### 💻 [CLI](cli/spec/spec.md)
Command-line interface for developers and CI/CD automation.

**Key Commands:**
```bash
# Gateway operations
api-platform gateway list
api-platform gateway push --file api.yaml

# API key management
api-platform gateway api-key generate --api-name 'MyAPI'
```

📖 **Documentation:**
- [Architecture](cli/spec/architecture/architecture.md)
- [Design](cli/spec/design/design.md)
- [Use Cases](cli/spec/use-cases/use_cases.md)

---

## Platform Architecture

```
+-----------------------------------------------------------------+
|        Control Plane (Multi-tenant SaaS or On-prem)             |
|                                                                 |
|  +----------+  +----------+  +----------+  +----------+         |
|  |   API    |  |Enterprise|  |Management|  |Developer |         |
|  | Designer |  |  Portal  |  |  Portal  |  |  Portal  |         |
|  +----------+  +----------+  +----------+  +----------+         |
|  +-----------------------------------------------------+        |
|  |                  Platform API                       |        |
|  +-----------------------------------------------------+        |
|                                                                 |
|  +------------------------+                                     |
|  |  Postgres / SQLite     |                                     |
|  +------------------------+                                     |
+-----------------------------------------------------------------+
                              |
                              v
+-----------------------------------------------------------------+
|      Data Plane (Single-tenant SaaS, On-prem, Hybrid)           |
|                                                                 |
|  +------------------------------------------+                   |
|  |          API Gateway                     |                   |
|  |  +--------+  +--------+  +--------+      |                   |
|  |  | Router |  | Policy |  |  Rate  |      |                   |
|  |  |(Envoy) |  | Engine |  |Limiter |      |                   |
|  |  +--------+  +--------+  +--------+      |                   |
|  +------------------------------------------+                   |
|                                                                 |
|  +--------+  +--------+          +--------+                     |
|  | Redis  |  | SQLite |          |  STS   |                     |
|  +--------+  +--------+          +--------+                     |
+-----------------------------------------------------------------+
```

---

## Quick Start

### Hybrid Gateway (Recommended)

Install a local gateway connected to the cloud control plane:

#### Step 1: Sign-up/Login to Bijira
Visit [Bijira](https://bijira.dev) and create an account or login.

#### Step 2: Add a Self-Managed Gateway
1. Navigate to Gateway management in Bijira
2. Click "Add Self-Managed Gateway"
3. Provide gateway details (name, hostname)
4. Copy the installation command provided by the UI

#### Step 3: Run the Installation Command
```bash
# Run the UI-provided command (includes your gateway key)
curl -Ls https://bijira.dev/quick-start | bash -s -- \
  -k $GATEWAY_KEY --name dev-gateway
```

This will:
- Install a locally self-managed gateway connected to Bijira
- Install the API Platform CLI tool

#### Step 4: Verify Installation
```bash
api-platform gateway list
```

#### Step 5: Deploy an API
Create an `api.yaml` file:
```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: https://api.weather.com/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      requestPolicies:
        - name: apiKey
          params:
            header: api-key
```

Deploy to gateway:
```bash
api-platform gateway push --file api.yaml
```

#### Step 6: Generate API Key
```bash
api-platform gateway api-key generate \
  --api-name 'Weather API' \
  --key-name 'my-key'
```

#### Step 7: Test the API
```bash
curl http://localhost:8081/weather/us/boston -H 'api-key: $API_KEY'
```

### Other Deployment Options

- **Fully On-Premise**: All components run locally
- **Agentic Flow**: AI-powered setup via Claude Code, Cursor, Copilot
- **Full Cloud**: Everything runs in Bijira cloud

---

## AI-Readiness Features

### Design & Build
- ✅ AI-assisted specification generation
- ✅ Governance linting for AI consumption
- ✅ Auto-generated, agent-friendly documentation
- ✅ "Chat with your API" playground

### Publish & Discover
- ✅ Semantic API search (intent-based)
- ✅ LLM-optimized documentation formats
- ✅ Interactive try-it playground for AI
- ✅ AI-aware changelogs (changelog.json)

### Monitor & Optimize
- ✅ AI interaction insights
- ✅ Pattern analysis for machine consumers
- ✅ Feedback loop for continuous improvement

---

## Gateway Types Comparison

| Feature | Basic | Standard |
|---------|-------|----------|
| **Components** | Router + Policy Engine + Gateway Controller | All components + Rate Limiter |
| **Persistence** | None (in-memory only) | SQLite (switchable to external DB) |
| **Rate Limiting** | Local only | Distributed (Redis) |
| **Availability** | Freemium (14-day trial) | Paid tier / Self-hosted |
| **Best For** | Development, testing | Production, enterprise |

---

## Use Cases

### Development
- Local API testing with Basic gateway
- Fast iteration cycles
- No external dependencies

### Enterprise Production
- Standard gateway with Redis cluster
- Multi-environment deployments
- High availability and SLA compliance

### Multi-Tenant SaaS
- Gateway per customer isolation
- Free tier: Basic gateway (14-day trial)
- Paid tier: Standard gateway with persistence

### CI/CD Integration
- Automated API deployment
- GitOps workflows
- Version control integration

### Hybrid Cloud
- On-premise gateway execution
- Cloud-based management and visibility
- Data sovereignty compliance

---

## Project Structure

```
api-platform/
├── concepts/              # Core concepts and specifications
│   └── api-yaml-specification.md
├── api-designer/          # Standalone API design tool
│   └── spec/
├── management-portal/     # Central control plane
│   └── spec/
├── gateway/               # Envoy-based API gateway
│   └── spec/
├── enterprise-portal/     # Internal asset discovery
│   └── spec/
├── api-portal/            # Developer portal
│   └── spec/
├── cli/                   # Command-line interface
│   └── spec/
└── README.md              # This file
```

---

## Core Concepts

- **[API.yaml Specification](concepts/api-yaml-specification.md)** - Declarative API definition format

---

(c) Copyright 2012 - 2025 WSO2 Inc.
