# WSO2 API Platform

AI-ready, GitOps-driven API platform for full lifecycle management across cloud, hybrid, and on-premises deployments.

## Overview

The WSO2 API Platform is a complete platform that helps organizations build AI-ready APIs with comprehensive lifecycle management capabilities. The platform supports deployment on the cloud, fully on-premises, or in hybrid mode.

### Platform Scope

The API Platform covers the complete API lifecycle:

- ✅ API ideation and planning
- ✅ API design and documentation
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

## Architecuture

TODO: add image

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
