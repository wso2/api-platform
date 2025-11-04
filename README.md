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

- API ideation and planning
- API design and documentation
- API testing and mocking
- Runtime management (ingress and egress)
- API governance and compliance
- Asset discovery and consumption
- API analytics and monetization

---

## Platform Components

### [API Designer](api-designer/docs/README.md)
Standalone design tool for creating API specifications with AI assistance and visual editing capabilities.

**Highlights:**
- Dual editing modes (code + visual) with real-time bidirectional sync
- Multi-format support: REST (OpenAPI), GraphQL, and AsyncAPI
- AI-powered specification and documentation generation
- Built-in governance validation and AI-readiness scoring
- MCP integration for AI agent code generation
- Schema registry integration and built-in mock server

---

### [Platform API](platform-api/docs/README.md)
Comprehensive backend service providing RESTful APIs and business logic for the entire platform ecosystem.

**Highlights:**
- Gateway lifecycle management with secure token-based registration
- Real-time monitoring via WebSocket connections and lightweight polling
- Multi-tenant architecture with complete organizational isolation
- Automatic API publishing to developer portals with subscription policies
- MCP server integration for AI agent interactions
- Built on Gin with SQLite persistence (upgradeable to PostgreSQL/MySQL/Oracle/MSSQL)

---

### [Management Portal](portals/management-portal/docs/README.md)
Centralized control plane for managing gateways, APIs, policies, and multi-tenant operations.

**Highlights:**
- Gateway lifecycle management: registration, monitoring, and orchestration
- API orchestration with artifact generation and deployment coordination
- Policy governance for authentication, authorization, and rate limiting
- Developer and enterprise portal administration
- Identity provider integration (OAuth, SAML, OIDC)
- Flexible deployment: Multi-tenant SaaS or on-premises

---

### [API Gateway](gateway/docs/README.md)
Lightweight, policy-first API gateway built on Envoy Proxy for modern API management.

**Highlights:**
- **Basic**: Lightweight (Router + Policy Engine + Controller) for development and testing
- **Standard**: Enterprise-grade with distributed rate limiting, Redis, and persistence
- Policy-first architecture: Everything beyond routing/TLS is a pluggable policy
- Multi-tenancy support: 90+ gateway instances per node
- AI/agentic flow optimized with MCP compatibility
- Flexible databases: SQLite default, with PostgreSQL/Oracle/MySQL/MSSQL support

---

### [Enterprise Portal](enterprise-portal/docs/README.md)
Internal discovery hub for finding and reusing digital assets across the organization.

**Highlights:**
- Centralized catalog for APIs (REST, GraphQL, gRPC), infrastructure, and AI services
- Advanced search and discovery with tagging and categorization
- Infrastructure assets: Data sources, caching systems, message queues
- AI service catalog: LLM integrations and ML model endpoints
- Dependency mapping and cross-team visibility
- Promotes asset reusability and reduces duplicate development

---

### [Developer Portal](portals/developer-portal/docs/README.md)
Multi-tenant developer portal for API discovery, subscription, and interactive documentation.

**Highlights:**
- Comprehensive API catalog with support for REST, GraphQL, SOAP, and AsyncAPI/WebSocket
- Interactive API documentation with GraphiQL and custom AsyncAPI viewer
- Application management with API key/OAuth token generation
- AI-powered SDK generation supporting multiple programming languages
- Multi-organization support with dedicated tenant isolation and custom branding
- Built on Node.js/Express with PostgreSQL, Redis caching, and Passport.js authentication
- Flexible deployment: Development mode, production mode, and standalone binaries

---

### [CLI](cli/docs/README.md)
Command-line interface for API Platform operations, optimized for developers and CI/CD workflows.

**Highlights:**
- Gateway management: List, configure, and monitor gateway instances
- API deployment: Push and validate API definitions with automatic governance checks
- API key operations: Generate, list, and revoke API keys
- Multiple output formats: JSON, YAML, and table views
- CI/CD ready: Automation-friendly with consistent exit codes and non-interactive mode
- Configuration management: Profiles, contexts, and authentication support

```bash
# Example commands
api-platform gateway list
api-platform gateway push --file api.yaml
api-platform gateway api-key generate --api-name 'MyAPI'
```

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
- AI-assisted specification generation
- Governance linting for AI consumption
- Auto-generated, agent-friendly documentation
- "Chat with your API" playground

### Publish & Discover
- Semantic API search (intent-based)
- LLM-optimized documentation formats
- Interactive try-it playground for AI
- AI-aware changelogs (changelog.json)

### Monitor & Optimize
- AI interaction insights
- Pattern analysis for machine consumers
- Feedback loop for continuous improvement

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
api-platform-specs/
├── specs/                       # Core specifications
│   └── api-yaml.md
├── api-designer/                # Standalone API design tool
│   └── docs/
├── platform-api/                # Backend service and APIs
│   └── docs/
├── gateway/                     # Envoy-based API gateway
│   └── docs/
├── enterprise-portal/           # Internal asset discovery
│   └── docs/
├── portals/
│   ├── developer-portal/        # Developer portal
│   │   └── docs/
│   └── management-portal/       # Central control plane
│       └── docs/
├── cli/                         # Command-line interface
│   └── docs/
├── change-log/                  # Feature change logs
└── README.md                    # This file
```

---

## Core Concepts

- **[API.yaml Specification](specs/api-yaml.md)** - Declarative API definition format

---

(c) Copyright 2012 - 2025 WSO2 Inc.
