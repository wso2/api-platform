# STS (Security Token Service)

A containerized OAuth 2.0 / OIDC authentication and authorization service built on [Asgardeo Thunder](https://github.com/asgardeo/thunder).

## Overview

The STS provides OAuth 2.0 / OIDC capabilities with an integrated authentication UI (Gate App), packaged as a single Docker image for easy deployment.

## Current Status

**Phase 1: Basic Thunder Integration** ✅

- Thunder OAuth 2.0 / OIDC server running in Docker
- Basic health checks configured
- Port 8090 exposed for OAuth endpoints

## Quick Start

### Prerequisites

- Docker installed on your system

### Build the Image

```bash
cd sts
docker build -t wso2/api-platform-sts:latest .
```

### Run the Container

```bash
docker run --rm -p 8090:8090 wso2/api-platform-sts:latest
```

### Verify Installation

Check if Thunder is running:

```bash
curl http://localhost:8090/health
```

## Configuration

### Ports

- **8090** - Thunder OAuth 2.0 / OIDC server

### Custom Configuration (Optional)

Mount a custom Thunder configuration file:

```bash
docker run --rm \
  -p 8090:8090 \
  -v $(pwd)/deployment.yaml:/opt/thunder/repository/conf/deployment.yaml \
  wso2/api-platform-sts:latest
```

## Architecture

```
┌─────────────────────────────────────────┐
│         Docker Container                │
│                                         │
│  ┌──────────────┐                       │
│  │   Thunder    │                       │
│  │   (Core)     │                       │
│  │              │                       │
│  │ - OAuth 2.0  │                       │
│  │ - OIDC       │                       │
│  │ - Token Mgmt │                       │
│  └──────────────┘                       │
│                                         │
└─────────────────────────────────────────┘
```

## Roadmap

- [x] Phase 1: Basic Thunder Integration
- [ ] Phase 2: Gate App Build Integration
- [ ] Phase 3: Component Integration & Networking
- [ ] Phase 4: Pre-configuration & Testing
- [ ] Phase 5: Optimization & Documentation

## Documentation

See the `spec/` directory for detailed documentation:

- [Product Requirements](spec/product/prd.md)
- [Architecture](spec/architecture/architecture.md)
- [Design](spec/design/design.md)
- [Implementation Guide](spec/impl/impl.md)

## Reference

Based on [Asgardeo Thunder](https://github.com/asgardeo/thunder)
