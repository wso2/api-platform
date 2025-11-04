# Platform API Features

This directory contains documentation for the Platform API features.

## Available Features

### [Gateway Management](./gateway-management/README.md)
Provides comprehensive lifecycle management for API gateway instances including registration with token-based authentication, token rotation and revocation, real-time connection status monitoring, gateway classification by criticality and type (regular, AI, event), virtual host configuration, and secure deletion with safety checks. Ensures complete multi-tenant isolation through organization-scoped operations with automatic cascade deletion of dependent resources.

### [Gateway WebSocket Connections](./gateway-websocket-connections/README.md)
Implements a real-time event notification system that enables the platform API to communicate with gateway instances through persistent WebSocket connections. Gateways establish authenticated connections and receive immediate notifications for API deployments, undeployments, or configuration changes, eliminating polling and ensuring real-time synchronization with platform state.

### [Developer Portal Publishing](./devportal-publishing/README.md)
Enables platform-api to publish APIs to the developer portal for developer discovery and subscription. Provides secure developer portal connectivity configuration, automatic organization synchronization with default subscription policies, comprehensive API lifecycle management (publish/unpublish/update), and multi-tenancy isolation with graceful failure handling.
