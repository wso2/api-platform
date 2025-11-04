# API Platform API

A comprehensive backend service providing RESTful APIs and business logic for the API Platform ecosystem, enabling gateway management, API lifecycle operations, developer portal integrations, and multi-tenant operations.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Features Documentation](#features-documentation)
- [Architecture](#architecture)
  - [Core Components](#core-components)
  - [Service Layer Architecture](#service-layer-architecture)
  - [Container Structure](#container-structure)
- [Service Components](#service-components)
  - [Gateway Management Service](#gateway-management-service)
  - [API Management Service](#api-management-service)
  - [Deployment Service](#deployment-service)
  - [MCP Server](#mcp-server)
- [Client Applications](#client-applications)
- [Integration Points](#integration-points)

---

## Overview

The Platform API serves as the central backend service for the API Platform ecosystem, providing RESTful APIs for gateway lifecycle management, API publishing workflows, and developer portal integrations. Built with multi-tenancy at its core, it ensures complete organizational isolation while supporting flexible deployment configurations and real-time gateway monitoring.

## Key Features

- **Gateway Lifecycle Management**: Complete registration, configuration, and monitoring of gateway instances
- **Secure Token Management**: Cryptographic token generation with rotation and revocation support
- **Multi-Tenancy**: Organization-scoped operations with complete tenant isolation
- **Real-time Monitoring**: WebSocket-based connection status tracking and lightweight polling endpoints
- **API Publishing**: Automatic API publishing to developer portals with lifecycle management
- **Organization Synchronization**: Automatic organization creation across platform components
- **Gateway Classification**: Support for regular, AI, and event gateway types with criticality flags
- **MCP Integration**: Model Context Protocol server for AI agent interactions

## Features Documentation

For detailed documentation on individual features, see the [Features Directory](./features/README.md).

---

## Architecture

### Core Components

The Platform API is built on a layered service architecture with clear separation of concerns:

#### Service Layer Architecture
- **RESTful API Layer**: HTTP endpoints for external integrations and client applications
- **Business Logic Layer**: Core platform operations, validation, and workflows
- **Data Access Layer**: Database interactions and data persistence with transaction support
- **Integration Layer**: External system connections including developer portals and MCP server

### Container Structure

```
+-------------------------------------------------------------------------+
|                    Platform API (container)                             |
|  +-------------------+    +-------------------+   +------------------+  |
|  |  HTTPS Server     | -> |   Service Layer   | ->|  WebSocket Mgr   |  |
|  | (REST + WS)       |    | (Business Logic)  |   | (Connections)    |  |
|  +-------------------+    +-------------------+   +------------------+  |
|           |                      |                         |            |
|           v                      v                         v            |
|      +----------+        +---------------+         +--------------+     |
|      | Router   |        | Repositories  |         | Gateway      |     |
|      | (Gin)    |        | (SQLite)      |         | Connections  |     |
|      +----------+        +---------------+         | (sync.Map)   |     |
|                                 |                  +--------------+     |
|                                 v                                       |
|                         +-----------------+                             |
|                         | SQLite Database |                             |
|                         | (api_platform)  |                             |
|                         +-----------------+                             |
+-------------------------------------------------------------------------+
                                   ^
                                   |
                          WebSocket (wss://)
                                   |
                                   v
                         +-------------------+
                         |  Gateway Instance |
                         +-------------------+
```

**Key Components**:
- **HTTPS Server**: Gin router serving `/api/v1/**` and `/api/internal/v1/**` routes with TLS support and WebSocket upgrade capability
- **WebSocket Manager**: Maintains persistent bidirectional connections with gateways, heartbeat monitoring (20-second intervals), thread-safe connection registry
- **Service Layer**: Business logic modules for organizations, projects, gateways, APIs with validation and orchestration
- **Repository Layer**: SQL repositories encapsulating CRUD operations, transactions, and relational data management
- **Database**: SQLite persistence (`./data/api_platform.db`) with schema bootstrapping from `internal/database/schema.sql`

---

## Service Components

The platform consists of the following service components:

#### Gateway Management Service
Provides comprehensive gateway lifecycle management:
- **Gateway Registration**: Secure registration with token-based authentication
- **Token Management**: Cryptographic token generation, rotation, and revocation
- **Configuration Management**: Virtual host configuration, metadata updates, and gateway classification
- **Connection Monitoring**: Real-time status tracking through WebSocket integration
- **Multi-Tenant Isolation**: Organization-scoped operations with composite uniqueness constraints

**Key Capabilities**:
- Secure token generation using crypto/rand with SHA-256 hashing
- Zero-downtime token rotation (maximum 2 active tokens per gateway)
- Gateway classification by type (regular, AI, event) and criticality
- Lightweight status polling optimized for management portals
- Automatic cascade deletion of dependent resources

#### API Management Service
Handles API definition and lifecycle operations:
- **API Definition Storage**: OpenAPI specification storage and validation
- **Metadata Management**: API versioning, descriptions, and ownership tracking
- **Deployment Orchestration**: Coordination of API deployments to gateways
- **Lifecycle State Management**: Publishing, unpublishing, and update workflows
- **Developer Portal Publishing**: Automatic API publishing with subscription policies

**Key Capabilities**:
- Multi-part form-data API publishing to developer portals
- Automatic organization synchronization with default subscription policies
- Support for PUBLIC, PRIVATE, and RESTRICTED visibility settings
- Graceful failure handling with configurable retry logic (3 retries, 15-second timeout)
- API ownership preservation across portal publishing

#### Deployment Service
Coordinates API deployments across gateway infrastructure:
- **Gateway Deployment Coordination**: Orchestrates deployments to registered gateways
- **Rollback Support**: Version management and rollback capabilities
- **Configuration Synchronization**: Ensures consistent configuration across gateways

#### MCP Server
Implements Model Context Protocol for AI integration:
- **Protocol Implementation**: Full MCP specification support
- **Portal API Integration**: Endpoint for AI agent interactions
- **Real-time Communication**: WebSocket-based bidirectional communication
- **Protocol Message Routing**: Efficient message routing and handling

---

## Client Applications

### Portal Applications

The Platform API serves multiple portal applications:

- **Management Portal**: Administrative interface for platform operations and gateway management
- **Enterprise Portal**: Organization-level API and gateway administration
- **API Portal**: Developer-facing portal for API discovery and subscription

### Command Line Interface (CLI)

Provides backend support for CLI tools enabling:
- Gateway registration and management
- API publishing operations
- Configuration management
- Status monitoring and diagnostics

---

## Integration Points

### API Gateway Controller

The Platform API integrates with gateway controllers for operational coordination:

- **Gateway Registration**: Administrators register gateway instances and generate secure authentication tokens
- **Gateway Connection**: Gateways authenticate and connect to the platform using provided tokens
- **Configuration Synchronization**: Real-time configuration updates pushed to gateways
- **Deployment Coordination**: Orchestrated API deployments across gateway instances
- **WebSocket Connection Management**: Persistent connections for status monitoring

### Developer Portal

Integration with developer portals for API publishing:

- **Organization Synchronization**: Automatic organization creation with default policies
- **API Publishing**: Multipart API definitions with metadata and OpenAPI specs
- **Subscription Management**: Default "unlimited" subscription policy assignment
- **Lifecycle Operations**: Publish, unpublish, and update workflows