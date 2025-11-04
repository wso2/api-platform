# API Platform Developer Portal

A comprehensive, multi-tenant developer portal for API management, enabling organizations to showcase APIs, manage developer applications, and provide interactive API documentation with support for REST, GraphQL, SOAP, and AsyncAPI/WebSocket.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Architecture](#architecture)
  - [Core Components](#core-components)
  - [Technology Stack](#technology-stack)
  - [Component Diagram](#component-diagram)
- [Authentication & Authorization](#authentication--authorization)
  - [Authentication Methods](#authentication-methods)
  - [Authorization Model](#authorization-model)
- [API Integration Patterns](#api-integration-patterns)
- [Deployment Options](#deployment-options)
  - [Development Mode](#development-mode)
  - [Production Mode](#production-mode)
  - [Binary Distribution](#binary-distribution)
- [Multi-Tenancy Support](#multi-tenancy-support)

---

## Overview

The API Platform Developer Portal is an enterprise-grade, Node.js-based web application that serves as a customizable developer portal for API management. It provides a platform where organizations can showcase their APIs to developers, enable API discovery and exploration, and facilitate application development with subscription management, key generation, and SDK generation capabilities.

Built on Express.js with Handlebars templating, the portal offers flexible deployment options ranging from local development to containerized production environments with advanced features like Redis caching, PostgreSQL session management, and AI-powered SDK generation.

## Key Features

- **Multi-Organization Support**: Dedicated tenant isolation with organization-specific branding and configuration
- **Comprehensive API Catalog**: Browse, search, and filter APIs with support for REST, GraphQL, SOAP, and AsyncAPI/WebSocket
- **Interactive API Documentation**: Built-in API explorers including GraphiQL for GraphQL and custom AsyncAPI viewer for WebSocket APIs
- **API Try-Out Functionality**: Interactive testing capabilities for APIs directly from the portal
- **Application Management**: Full lifecycle management for developer applications including creation, configuration, and subscription handling
- **Multi-Language SDK Generation**: AI-powered SDK generation service supporting multiple programming languages with progress tracking
- **Advanced Key Management**: API key and OAuth token generation with policy-based throttling
- **Customizable UI**: Organization and API landing pages with Handlebars templates and custom branding support
- **Label-Based Categorization**: Flexible API organization with custom labels and categories
- **Read-Only Mode**: Production deployment option for content distribution without modification
- **Audit Logging**: Comprehensive logging for compliance and security monitoring

---

## Architecture

### Core Components

The developer portal consists of the following components:

#### Application Layer
- **Express.js Server**: Core web application framework handling HTTP requests and responses
- **Handlebars (HBS) Engine**: Server-side templating for dynamic page rendering
- **Controllers**: Request handlers for authentication, API content, applications, organization content, and settings
- **Routes**: RESTful API endpoints for portal operations

#### Business Logic Layer
- **Admin Service**: Organization management and administrative operations
- **API Metadata Service**: API catalog management, discovery, search, and subscription policies
- **DevPortal Service**: Core portal functionality and business logic
- **SDK Job Service**: Multi-language SDK generation with job queue management and AI integration
- **Redis Service**: Distributed caching and session management

#### Data Access Layer
- **Sequelize ORM**: Database abstraction layer for PostgreSQL
- **Data Access Objects (DAO)**: Abstraction for database operations
- **Models**: Database schema definitions (API Metadata, Applications, Organizations, Identity Providers, Subscription Policies)

#### Persistence Layer
- **PostgreSQL**: Primary database for APIs, applications, organizations, and subscriptions
- **Redis**: Distributed cache for performance optimization and session management
- **File Storage**: API definitions, templates, images, and assets

#### Integration Layer
- **Control Plane Connector**: Integration with WSO2 API Manager or other API gateways
- **Identity Provider Integration**: OIDC/OAuth2 compliant authentication with multiple IDP support
- **AI SDK Service Client**: External microservice integration for SDK generation

#### Middleware Layer
- **Passport.js**: Authentication middleware with multiple strategies (OAuth2, OIDC, Local, Custom)
- **Audit Logger**: Comprehensive logging for all operations
- **ensureAuthenticated**: Session validation and authorization enforcement
- **registerPartials**: Handlebars partial registration for template composition

#### Custom Components
- **AsyncAPI Viewer**: Custom React component (Monaco Editor) for WebSocket API visualization and testing

### Technology Stack

**Backend Framework:**
- Node.js v22.0.0
- Express.js
- Handlebars (HBS) templating

**Database & Caching:**
- PostgreSQL with Sequelize ORM
- Redis (IORedis) with TLS support
- connect-pg-simple for session persistence

**Authentication & Security:**
- Passport.js (OAuth2, OIDC, Local strategies)
- JWT (jsonwebtoken, jose)
- express-session
- Mutual TLS (mTLS) support

**Frontend Technologies:**
- React v18.3.1 (AsyncAPI viewer)
- Bootstrap Icons
- GraphiQL (GraphQL explorer)
- Monaco Editor

**API & Integration:**
- Axios HTTP client
- OpenAPI Tools for code generation
- GraphQL support
- Multer for file uploads

**Development & Build:**
- Nodemon for development
- pkg for binary packaging
- Chokidar for file watching
- ESLint for code quality

**Monitoring & Logging:**
- Winston logging framework
- Application Insights (Azure integration)

### Component Diagram

```
+-----------------------------------------------------------------------+
|                     Developer Portal Application                      |
|                                                                       |
|  +------------------------------------------------------------------+ |
|  |                       Application Layer                          | |
|  |  +--------------+  +--------------+  +---------------------+     | |
|  |  |   Express    |  |  Handlebars  |  |    Controllers      |     | |
|  |  |    Server    |  |    Engine    |  | (Auth, API, Apps)   |     | |
|  |  +--------------+  +--------------+  +---------------------+     | |
|  +------------------------------------------------------------------+ |
|                                                                       |
|  +------------------------------------------------------------------+ |
|  |                      Business Logic Layer                        | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  |  |    Admin     |  | API Metadata |  |  DevPortal   |            | |
|  |  |   Service    |  |   Service    |  |   Service    |            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  |  +--------------+  +--------------+                              | |
|  |  |  SDK Job     |  |    Redis     |                              | |
|  |  |   Service    |  |   Service    |                              | |
|  |  +--------------+  +--------------+                              | |
|  +------------------------------------------------------------------+ |
|                                                                       |
|  +------------------------------------------------------------------+ |
|  |                     Data Access Layer                            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  |  |  Sequelize   |  |     DAO      |  |    Models    |            | |
|  |  |     ORM      |  |              |  |              |            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  +------------------------------------------------------------------+ |
|                                                                       |
|  +------------------------------------------------------------------+ |
|  |                      Middleware Layer                            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  |  |  Passport.js |  | Audit Logger |  |    ensureAuth|            | |
|  |  |              |  |              |  |              |            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  +------------------------------------------------------------------+ |
|                                                                       |
|  +------------------------------------------------------------------+ |
|  |                    Persistence Layer                             | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  |  |  PostgreSQL  |  |    Redis     |  | File Storage |            | |
|  |  |              |  |              |  |              |            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  +------------------------------------------------------------------+ |
|                                                                       |
|  +------------------------------------------------------------------+ |
|  |                    Integration Layer                             | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  |  |   Control    |  |  Identity    |  |  AI SDK      |            | |
|  |  |    Plane     |  |   Provider   |  |  Service     |            | |
|  |  +--------------+  +--------------+  +--------------+            | |
|  +------------------------------------------------------------------+ |
+-----------------------------------------------------------------------+

External Dependencies:
  - OIDC/OAuth2 Identity Provider (Asgardeo, WSO2 IS, Enterprise IDPs)
  - AI SDK Generation Service (Optional)
```

---

## Authentication & Authorization

### Authentication Methods

The portal supports multiple authentication mechanisms:

#### 1. OIDC/OAuth2 (Primary)
- **Implementation**: Passport.js with `passport-openidconnect`
- **Features**:
  - JWT token validation using JOSE library
  - Token exchange mechanism for backend communication
  - JWKS endpoint integration for key rotation
  - PKCE (Proof Key for Code Exchange) support
- **Supported IDPs**: Asgardeo, WSO2 Identity Server, enterprise OIDC/OAuth2 providers
- **Federated Identity**: Google, GitHub, Microsoft, and custom federation

#### 2. API Key Authentication
- **Implementation**: Custom header-based authentication
- **Header**: `x-wso2-api-key`
- **Use Cases**: Programmatic access, service-to-service communication
- **Configuration**: Configurable key types and validation rules

#### 3. Mutual TLS (mTLS)
- **Features**: Certificate-based authentication with client certificate validation
- **Use Cases**: High-security environments, service mesh integration

#### 4. Local Authentication
- **Implementation**: Username/password authentication for development
- **Configuration**: Enabled via `defaultAuth` in config.json
- **Use Cases**: Development, testing, offline environments

### Authorization Model

#### Role-Based Access Control (RBAC)
- **Roles**:
  - `superAdmin`: Platform-level administration
  - `admin`: Organization-level administration
  - `subscriber`: Developer/API consumer
- **Role Claims**: Configurable from IDP token claims
- **Organization-Level Mapping**: Roles scoped to specific organizations

#### Scope-Based Security
- **SCOPES.ADMIN**: Administrative operations (create/update/delete organizations, APIs)
- **SCOPES.DEVELOPER**: Developer operations (create applications, subscribe to APIs, generate keys)
- **Implementation**: `enforceSecuirty()` middleware

#### Page-Level Authorization
- **Authenticated Pages**: Require login but no specific role
- **Authorized Pages**: Require specific roles (configured with pattern matching)
- **Pattern Matching**: Minimatch for flexible URL patterns

#### Multi-Tenant Isolation
- **Organization Claim Validation**: JWT claims verified for organization membership
- **Cross-Organization Prevention**: Automatic blocking of unauthorized cross-tenant access
- **Data Isolation**: Database-level tenant separation

---

## API Integration Patterns

### External Integrations

#### 1. Control Plane Integration
- **Purpose**: Connect to WSO2 API Manager or other API gateways
- **Endpoints**:
  - REST API: `/api/am/devportal/v2`
  - GraphQL endpoint support
- **Authentication**: Token exchange for backend communication
- **Features**: API synchronization, subscription management, key generation

#### 2. Identity Provider Integration
- **Protocol**: OIDC/OAuth2 compliant
- **Multi-IDP Support**: Multiple IDPs per organization
- **Configuration**: Per-organization IDP settings
- **Custom Providers**: Extensible IDP configuration framework

#### 3. AI SDK Service Integration
- **Purpose**: External microservice for SDK generation
- **Endpoints**:
  - `POST /merge-openapi-specs`: Merge multiple API specifications
  - `POST /generate-application-code`: Generate multi-language SDKs
- **Features**: Asynchronous job processing, progress tracking via SSE
- **Configuration**: Configurable service URL in config.json

### API Design Patterns

#### RESTful API Design
- Resource-based endpoints following REST principles
- HTTP verb conventions (GET, POST, PUT, DELETE)
- Consistent error handling with HTTP status codes
- JSON request/response format

#### Multipart Form Data
- File uploads with metadata
- Used for API definitions, templates, images, and assets
- Multer middleware for handling multipart data

#### Server-Sent Events (SSE)
- Real-time progress tracking for long-running operations (SDK generation)
- Long-lived HTTP connections
- Event-driven updates to clients

#### Pagination & Filtering
- Query parameter-based filtering
- Search and label-based queries
- Efficient data retrieval for large datasets

### Key API Endpoints

```
# Organization Management
POST   /devportal/organizations                    # Create organization
GET    /devportal/organizations/:orgId             # Get organization details
PUT    /devportal/organizations/:orgId             # Update organization
DELETE /devportal/organizations/:orgId             # Delete organization

# API Management
GET    /devportal/organizations/:orgId/apis        # List APIs
POST   /devportal/organizations/:orgId/apis        # Create API
GET    /devportal/organizations/:orgId/apis/:apiId # Get API details
PUT    /devportal/organizations/:orgId/apis/:apiId # Update API
DELETE /devportal/organizations/:orgId/apis/:apiId # Delete API

# Application Management
GET    /devportal/organizations/:orgId/applications      # List applications
POST   /devportal/organizations/:orgId/applications      # Create application
GET    /applications/:appId                              # Get application details
PUT    /applications/:appId                              # Update application
DELETE /applications/:appId                              # Delete application
POST   /applications/:appId/generate-keys                # Generate OAuth keys
POST   /applications/:appId/generate-sdk                 # Generate SDK
GET    /applications/:appId/sdk-generation-progress      # SDK generation progress (SSE)

# Public Portal Pages
GET    /:orgName                                   # Organization landing page
GET    /:orgName/apis                              # API listing page
GET    /:orgName/api/:apiName                      # API landing page
GET    /:orgName/api/:apiName/tryout               # API try-out page
GET    /:orgName/api/:apiName/documentation        # API documentation
```

---

## Deployment Options

### Development Mode

Local development with auto-reload and live CSS building:

- **Command**: `npm start`
- **Features**:
  - Nodemon for automatic server restart on code changes
  - Chokidar file watcher for live CSS compilation
  - Mock data support for offline development
  - Source path: `./src/`
- **Best For**: Active development, debugging, feature implementation

### Production Mode

Optimized deployment for production environments:

- **Features**:
  - Pre-built static assets
  - Database-backed content management
  - Redis caching enabled for performance
  - Read-only mode option for content distribution
  - Static resources path: `./resources/default-layout/`
- **Configuration**:
  - PostgreSQL database (required)
  - Redis instance (optional, recommended for performance)
  - Identity Provider (OIDC/OAuth2 compliant)
  - Control Plane API integration (optional)
  - AI SDK Service (optional)
- **Docker Deployment**:
  - Base Image: `node:23-bookworm-slim`
  - Port: 8080
  - Non-root user: UID 10001 (security best practice)
  - Java Runtime included for OpenAPI generator
  - Production-only dependencies: `npm ci --only=production`
- **CI/CD Pipeline**:
  - GitHub Actions workflow
  - Automated builds on git tags (`v*`)
  - Multi-platform support (Linux, macOS, Windows)
  - Release artifacts with binaries, configs, and documentation
- **Best For**: Production environments, staging, content distribution

### Binary Distribution

Standalone executable distribution:

- **Command**: `npm run build`
- **Features**:
  - Standalone executables bundling Node.js runtime
  - No Node.js installation required
  - Cross-platform support
- **Supported Platforms**:
  - Linux (x64, ARM)
  - macOS (x64, ARM)
  - Windows (x64)
- **Packaging Tool**: `pkg` for binary compilation
- **Includes**:
  - Compiled binaries
  - Configuration templates (config.json, secret.json)
  - Installation guides
  - Startup scripts (startup.sh, startup.bat)
- **Best For**: Enterprise distribution, offline environments, simplified deployment

### Infrastructure Requirements

**Required:**
- PostgreSQL database
- Identity Provider (OIDC/OAuth2 compliant)

**Optional:**
- Redis instance (recommended for production)
- Control Plane API (WSO2 API Manager or equivalent)
- AI SDK Service (for SDK generation feature)

**Scalability Considerations:**
- Stateless application design (session in PostgreSQL)
- Redis for distributed caching
- Multi-instance deployment support
- Database connection pooling
- Horizontal scaling capabilities

---

## Multi-Tenancy Support

### Organization-Based Tenancy

The developer portal provides comprehensive multi-organization support:

- **Dedicated Tenant Isolation**: Each organization operates in a logically isolated environment
- **Organization-Specific Branding**: Custom logos, color schemes, and landing pages per organization
- **Isolated API Catalogs**: APIs scoped to specific organizations with cross-organization access control
- **Tenant-Specific Configuration**: Custom authentication, authorization, and feature settings per organization

### Tenant Management Features

- **Organization CRUD Operations**: Full lifecycle management via Admin Service
- **Custom IDP per Organization**: Support for organization-specific identity providers
- **Role Mapping**: Organization-level role assignments and permissions
- **Resource Quotas**: Configurable limits for APIs, applications, and subscriptions per organization

### Sub-Organization Support

The portal architecture supports hierarchical organization structures:

- **Nested Organizations**: Sub-organizations inherit parent configuration with override capability
- **Shared Resources**: Optional resource sharing between parent and sub-organizations
- **Independent Administration**: Delegated admin access for sub-organization management

### Data Isolation

- **Database-Level Separation**: Organization ID as tenant discriminator in all data models
- **Session Isolation**: Organization claim validation in JWT tokens
- **Cross-Tenant Prevention**: Automatic blocking of unauthorized cross-organization access
- **Audit Trail**: Organization-specific audit logging for compliance

---

## Customization & Extensibility

### Template Customization
- **Handlebars Templates**: Fully customizable page templates
- **Custom Partials**: Reusable template components
- **Organization Landing Pages**: Markdown + Handlebars support
- **API Landing Pages**: Custom templates per API

### Theme Customization
- **CSS Stylesheets**: Custom styling per organization
- **Bootstrap Icons**: Icon library for UI consistency
- **Responsive Design**: Mobile-first responsive layouts

### Content Management
- **Image/Asset Upload**: Organization and API-specific assets
- **Template Upload/Download**: Import/export custom templates
- **Markdown Content**: Rich text content with Markdown support

---

## Monitoring & Observability

### Logging
- **Winston Framework**: Structured logging with multiple transports
- **Log Levels**: DEBUG, INFO, WARN, ERROR
- **Audit Logging**: Comprehensive operation logging for compliance

### Telemetry
- **Application Insights**: Azure integration for monitoring
- **Custom Metrics**: Configurable telemetry collection
- **Performance Tracking**: Response times, error rates, throughput

### Health Checks
- **Database Connectivity**: PostgreSQL health monitoring
- **Redis Connectivity**: Cache layer health checks
- **External Service Integration**: Control Plane and IDP availability

---

## Security Features

- **HTTPS/TLS**: SSL certificate support with configurable trust stores
- **Session Security**: Secure session management with PostgreSQL persistence
- **CSRF Protection**: Session-based CSRF token validation
- **JWT Verification**: Remote JWKS validation for token security
- **Certificate-Based Authentication**: mTLS support for high-security environments
- **Audit Logging**: Complete operation tracking for compliance and security monitoring
- **Role-Based Access Control**: Fine-grained authorization model
- **Read-Only Mode**: Production deployment option preventing unauthorized modifications

---

## Documentation

For detailed documentation, refer to:
- **Installation Guide**: `/docs/InstallationGuide.md`
- **Design Documentation**: `/docs/DevportalDesign.md`
- **Organization Setup**: `/docs/CreateOrganization.md`
- **Configuration Reference**: `sample_config.json`