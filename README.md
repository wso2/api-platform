# WSO2 API Platform

AI-ready, GitOps-driven API platform for full lifecycle management across cloud, hybrid, and on-premises deployments.

## Overview

The WSO2 API Platform is a complete platform that helps organizations build AI-ready APIs with comprehensive lifecycle management capabilities. The platform supports deployment on the cloud, fully on-premises, or in hybrid mode.

### Platform Scope

The API Platform covers the complete API lifecycle:

- ✅ API ideation and planning
- ✅ API design and documentation
- ✅ API deployment handling
- ✅ API testing and mocking
- ✅ Runtime management (ingress and egress)
- ✅ API governance and compliance
- ✅ Asset discovery and consumption
- ✅ API analytics and monetization

### Key Principles

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
- **API Policies**: GoLang based API Policies with Policy Hub support

## Architecture

TODO: add image

## Quick Start

Get up and running in minutes with Docker Compose.

### 1. Clone the Repository

```bash
git clone https://github.com/wso2/api-platform
cd api-platform
```

### 2. Start the Platform

```bash
cd distribution/all-in-one
docker compose up
```

> **Note:** Use `docker compose up --build` to rebuild images when code changes need to be applied. Without `--build`, cached images are used.

### 3. Create a Default Organization

```bash
curl -k --location 'https://localhost:9243/api/v1/organizations' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer <shared-token>' \
  --data '{
    "id": "15b2ac94-6217-4f51-90d4-b2b3814b20b4",
    "handle": "acme",
    "name": "ACME Corporation",
    "region": "US"
}'
```

### 4. Accept the Self-Signed Certificate

Open https://localhost:9243 in your browser and accept the self-signed certificate.

### 5. Open the Management Portal

Navigate to http://localhost:5173 to access the Management Portal.

### 6. Open the API Developer Portal

Navigate to http://localhost:3001 to access the Developer Portal.

### 7. Shutdown

```bash
docker compose down    # shutdown the servers only, data doesn't get removed
docker compose down -v # clear the data too
```

## Platform Components

### API Designer
Standalone design tool for REST, GraphQL, and AsyncAPI specifications.

**Key Features:**
- Code + visual split view with real-time updates
- AI-assisted specification and documentation generation
- Built-in mocking and governance checks
- AI-readiness score for APIs
- MCP code generation from specifications

### Management Portal
Central control plane for managing gateways, APIs, policies, and governance.

**Key Capabilities:**
- Gateway management and orchestration
- API lifecycle management
- Policy and governance rule management
- Identity provider configuration
- API deployment to gateways
- Publishing to developer portals

### API Gateway
Envoy-based API gateway for securing and routing API traffic.

**Key Features:**
- Built on Envoy Proxy
- Policy-first architecture (auth, rate limiting, analytics)
- Runs on VMs, containers, Kubernetes
- Single-tenant mode
- Optimized for AI/agentic flows

### API Developer Portal
Developer portal for API discovery, subscription, and consumption.

**Key Features:**
- API catalog and semantic search
- Try-it console for API testing
- API subscription management
- Application and API key management
- AI-powered discovery


(c) Copyright 2012 - 2025 WSO2 Inc.
