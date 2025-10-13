# STS (Security Token Service) - Project Specification

## Overview

The STS (Security Token Service) is a containerized authentication and authorization service built on top of [Asgardeo Thunder](https://github.com/asgardeo/thunder). It provides a complete identity and access management solution packaged as a single Docker image.

## Key Components

1. **Thunder Core** - The underlying OAuth 2.0 / OIDC authorization server
2. **Gate App** - Authentication application providing login, registration, and recovery UIs (Next.js, requires Node.js 20+)
3. **Docker Runtime** - Containerized deployment environment

## Quick Links

- [Product Requirements](./product/prd.md)
- [Architecture](./architecture/architecture.md)
- [Design Specifications](./design/design.md)
- [Implementation Guide](./impl/impl.md)

## Technology Stack

- **Base**: [Asgardeo Thunder](https://github.com/asgardeo/thunder)
- **Gate App**: Next.js application with authentication UI
- **Deployment**: Docker containerization

## Project Goals

- Provide a standalone, easy-to-deploy authentication service
- Include pre-configured applications (e.g., management-portal)
- Support multi-tenancy through organization management
- Enable user authentication with organization-scoped access tokens
- Simplify deployment through Docker containerization

## Current Status

- Gate app UI implementation in progress
- Docker integration pending
- Thunder core integration planned
